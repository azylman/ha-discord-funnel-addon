// Command discord-funnel connects to Discord as a bot, listens to the live
// message feed, and for each message (or just messages that mention the
// bot, depending on the --mentions-only flag) creates a brand new chat in
// an Open WebUI instance containing a real model reply, using Open WebUI's
// backend chat API:
//
//  1. POST {webui-url}/api/v1/chats/new       — creates the chat with the
//     Discord message as the user turn and an empty assistant placeholder.
//     NEVER retried: it is not idempotent (each call creates a brand new
//     chat), so a retry here would leave duplicate chats behind.
//  2. POST {webui-url}/api/chat/completions   — generates the actual model
//     reply. Deliberately sent WITHOUT chat_id/id: per Open WebUI's docs,
//     including both fields makes the server treat the caller as a live
//     WebUI browser tab and push the reply over that user's WebSocket
//     instead of returning it in the HTTP response — which for a headless
//     caller like this one (no WebSocket ever connected) surfaces to the
//     end user as "Open WebUI: Server Connection Error" and leaves the
//     chat's assistant message with a corrupted parentId. Omitting them
//     gets the completion back synchronously over plain HTTP instead. This
//     call has no persisted side effects in Open WebUI (it only returns
//     text), so it is safe to retry a bounded number of times on failure —
//     worst case a retry costs one extra upstream model call.
//  3. POST {webui-url}/api/v1/chats/{id}      — writes the real reply text
//     into the chat's message tree (both the flat `messages` array and
//     `history.messages`) so it renders correctly in the Open WebUI UI.
//     This sends the full, identical chat body each time, so retrying it
//     converges to the same end state (idempotent full-resource replace)
//     and is also safe to retry.
//
// See https://docs.openwebui.com/reference/api-flow/ and
// https://docs.openwebui.com/reference/api-endpoints/ for the API this
// mirrors.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// message is Open WebUI's message shape, used both in the flat `messages`
// array and (indexed by id) in `history.messages`. Fields are tagged
// omitempty where Open WebUI's docs mark them optional so the JSON stays
// close to their examples.
type message struct {
	ID          string   `json:"id"`
	Role        string   `json:"role"`
	Content     string   `json:"content"`
	Timestamp   int64    `json:"timestamp"`
	Models      []string `json:"models,omitempty"`
	ChildrenIDs []string `json:"childrenIds"`
	ParentID    string   `json:"parentId,omitempty"`
	Model       string   `json:"model,omitempty"`
	ModelName   string   `json:"modelName,omitempty"`
	ModelIdx    int      `json:"modelIdx,omitempty"`
	Done        *bool    `json:"done,omitempty"`
}

type history struct {
	CurrentID string              `json:"currentId"`
	Messages  map[string]*message `json:"messages"`
}

type chatBody struct {
	Title    string     `json:"title"`
	Models   []string   `json:"models"`
	Messages []*message `json:"messages"`
	History  history    `json:"history"`
}

type chatsNewRequest struct {
	Chat chatBody `json:"chat"`
}

type chatsNewResponse struct {
	ID string `json:"id"`
}

type completionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// completionsRequest deliberately omits chat_id/id/session_id — see the
// package doc comment for why. This is the "plain API caller" shape that
// gets the reply back synchronously in the HTTP response.
type completionsRequest struct {
	Messages []completionMessage `json:"messages"`
	Model    string              `json:"model"`
	Stream   bool                `json:"stream"`
}

type completionsResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type chatUpdateRequest struct {
	Chat chatBody `json:"chat"`
}

// retryConfig bounds the retry-with-backoff helper used for the two
// idempotent-in-effect steps (completion fetch, chat update). Chat creation
// never uses this — see package doc comment.
const (
	maxRetries        = 3
	retryInitialDelay = 500 * time.Millisecond
)

func main() {
	token := flag.String("token", "", "Discord bot token (required)")
	webUIURL := flag.String("webui-url", "", "Base URL of the Open WebUI instance, e.g. http://openwebui:8080 (required)")
	apiKey := flag.String("api-key", "", "Open WebUI API key / bearer token (required)")
	model := flag.String("model", "", "Open WebUI model id to use for each new chat (required)")
	mentionsOnly := flag.Bool("mentions-only", false, "Only act on messages that mention the bot (default: act on every message)")
	flag.Parse()

	if *token == "" {
		log.Fatal("discord-funnel: --token is required")
	}
	if *webUIURL == "" {
		log.Fatal("discord-funnel: --webui-url is required")
	}
	if *apiKey == "" {
		log.Fatal("discord-funnel: --api-key is required")
	}
	if *model == "" {
		log.Fatal("discord-funnel: --model is required")
	}

	baseURL := strings.TrimRight(*webUIURL, "/")

	dg, err := discordgo.New("Bot " + *token)
	if err != nil {
		log.Fatalf("discord-funnel: failed to create Discord session: %v", err)
	}

	// GuildMessages + MessageContent are needed to receive message text in
	// guild channels; DirectMessages covers DMs to the bot.
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent | discordgo.IntentsDirectMessages

	httpClient := &http.Client{Timeout: 60 * time.Second}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil || (s.State != nil && s.State.User != nil && m.Author.ID == s.State.User.ID) {
			return // ignore our own messages
		}

		isMention := m.MentionEveryone
		if s.State != nil && s.State.User != nil {
			for _, u := range m.Mentions {
				if u.ID == s.State.User.ID {
					isMention = true
					break
				}
			}
		}

		if *mentionsOnly && !isMention {
			return
		}

		content := m.Content
		if content == "" {
			return
		}

		go createChat(httpClient, baseURL, *apiKey, *model, m.Author.Username, m.ChannelID, m.ID, content)
	})

	if err := dg.Open(); err != nil {
		log.Fatalf("discord-funnel: failed to open Discord session: %v", err)
	}
	defer dg.Close()

	log.Printf("discord-funnel: connected to Discord, mentions_only=%v, creating chats on %s (model=%s)", *mentionsOnly, baseURL, *model)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("discord-funnel: shutting down")
}

// createChat: create the chat (user message + empty assistant placeholder,
// exactly once — never retried), fetch a real completion synchronously
// over plain HTTP (no chat_id/id — see package doc comment; retried on
// failure since it has no persisted side effects), then write the
// completed assistant message back into the chat (also retried, since
// re-sending the identical full chat body is idempotent).
func createChat(client *http.Client, baseURL, apiKey, model, authorName, channelID, discordMessageID, content string) {
	userMsgID := newUUID()
	assistantMsgID := newUUID()
	now := time.Now().Unix()
	notDone := false

	userMsg := &message{
		ID:          userMsgID,
		Role:        "user",
		Content:     content,
		Timestamp:   now,
		Models:      []string{model},
		ChildrenIDs: []string{assistantMsgID},
	}
	assistantMsg := &message{
		ID:          assistantMsgID,
		Role:        "assistant",
		Content:     "",
		ParentID:    userMsgID,
		ChildrenIDs: []string{},
		Model:       model,
		ModelName:   model,
		ModelIdx:    0,
		Done:        &notDone,
		Timestamp:   now + 1,
	}

	title := fmt.Sprintf("Discord: #%s (%s)", channelID, authorName)
	if len(title) > 200 {
		title = title[:200]
	}

	body := chatBody{
		Title:    title,
		Models:   []string{model},
		Messages: []*message{userMsg, assistantMsg},
		History: history{
			CurrentID: assistantMsgID,
			Messages: map[string]*message{
				userMsgID:      userMsg,
				assistantMsgID: assistantMsg,
			},
		},
	}

	// Step 1: create the chat. Deliberately NOT retried — a retry here
	// would create a second, duplicate chat for the same Discord message
	// if the first request actually succeeded server-side but the response
	// was lost (e.g. a client-side timeout).
	var created chatsNewResponse
	if err := postJSON(client, baseURL+"/api/v1/chats/new", apiKey, chatsNewRequest{Chat: body}, &created); err != nil {
		log.Printf("discord-funnel: failed to create chat for discord message %s: %v", discordMessageID, err)
		return
	}
	if created.ID == "" {
		log.Printf("discord-funnel: chat creation for discord message %s returned no chat id", discordMessageID)
		return
	}

	// Step 2: fetch the completion. Safe to retry — this call never
	// touches chat_id/id, so it has no persisted side effects in Open
	// WebUI regardless of how many times it's attempted.
	var completion completionsResponse
	var completionErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		completionErr = postJSON(client, baseURL+"/api/chat/completions", apiKey, completionsRequest{
			Messages: []completionMessage{{Role: "user", Content: content}},
			Model:    model,
			Stream:   false,
		}, &completion)
		if completionErr == nil {
			break
		}
		log.Printf("discord-funnel: completion attempt %d/%d failed for discord message %s (chat %s): %v", attempt, maxRetries, discordMessageID, created.ID, completionErr)
		if attempt < maxRetries {
			time.Sleep(retryInitialDelay * time.Duration(1<<(attempt-1))) // 500ms, 1s, 2s...
		}
	}

	replyContent := ""
	done := true
	if completionErr != nil {
		replyContent = fmt.Sprintf("(discord-funnel: failed to get a reply after %d attempts: %v)", maxRetries, completionErr)
	} else if len(completion.Choices) > 0 {
		replyContent = completion.Choices[0].Message.Content
	} else {
		log.Printf("discord-funnel: chat %s created but completion for discord message %s returned no choices", created.ID, discordMessageID)
		replyContent = "(discord-funnel: model returned no response)"
	}

	assistantMsg.Content = replyContent
	assistantMsg.Done = &done

	updateBody := chatBody{
		Title:    title,
		Models:   []string{model},
		Messages: []*message{userMsg, assistantMsg},
		History: history{
			CurrentID: assistantMsgID,
			Messages: map[string]*message{
				userMsgID:      userMsg,
				assistantMsgID: assistantMsg,
			},
		},
	}

	// Step 3: write the reply into the chat. Safe to retry — the request
	// body is a full, identical resource replace each time, so re-sending
	// it converges to the same end state.
	var updateErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		updateErr = postJSON(client, baseURL+"/api/v1/chats/"+created.ID, apiKey, chatUpdateRequest{Chat: updateBody}, nil)
		if updateErr == nil {
			break
		}
		log.Printf("discord-funnel: chat update attempt %d/%d failed for discord message %s (chat %s): %v", attempt, maxRetries, discordMessageID, created.ID, updateErr)
		if attempt < maxRetries {
			time.Sleep(retryInitialDelay * time.Duration(1<<(attempt-1)))
		}
	}
	if updateErr != nil {
		log.Printf("discord-funnel: chat %s created and completion fetched but failed to persist reply for discord message %s after %d attempts: %v", created.ID, discordMessageID, maxRetries, updateErr)
		return
	}

	log.Printf("discord-funnel: created Open WebUI chat %s for discord message %s", created.ID, discordMessageID)
}

func postJSON(client *http.Client, url, apiKey string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// newUUID generates a random UUID v4 without pulling in an extra dependency.
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read failing is effectively unrecoverable; fall back to
		// a timestamp-derived value so callers still get a unique-ish string.
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
