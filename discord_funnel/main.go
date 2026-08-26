// Command discord-funnel connects to Discord as a bot, listens to the live
// message feed, and for each message (or just messages that mention the
// bot, depending on the --mentions-only flag) creates a brand new chat in
// an Open WebUI instance and triggers a completion in it, using Open
// WebUI's backend-controlled chat API:
//
//  1. POST {webui-url}/api/v1/chats/new  — creates the chat with the
//     Discord message as the user turn and an empty assistant placeholder.
//  2. POST {webui-url}/api/chat/completions — triggers the actual model
//     reply inside that chat.
//
// See https://docs.openwebui.com/reference/api-flow/ for the API this
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

type completionsRequest struct {
	ChatID    string              `json:"chat_id"`
	ID        string              `json:"id"`
	Messages  []completionMessage `json:"messages"`
	Model     string              `json:"model"`
	Stream    bool                `json:"stream"`
	SessionID string              `json:"session_id"`
}

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

	httpClient := &http.Client{Timeout: 30 * time.Second}

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

// createChat implements the two-step Open WebUI backend chat flow: create
// the chat (user message + empty assistant placeholder), then trigger the
// completion so the chat actually contains a model reply.
func createChat(client *http.Client, baseURL, apiKey, model, authorName, channelID, discordMessageID, content string) {
	userMsgID := newUUID()
	assistantMsgID := newUUID()
	sessionID := newUUID()
	now := time.Now().Unix()
	done := false

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
		Done:        &done,
		Timestamp:   now + 1,
	}

	title := fmt.Sprintf("Discord: #%s (%s)", channelID, authorName)
	if len(title) > 200 {
		title = title[:200]
	}

	reqBody := chatsNewRequest{
		Chat: chatBody{
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
		},
	}

	var created chatsNewResponse
	if err := postJSON(client, baseURL+"/api/v1/chats/new", apiKey, reqBody, &created); err != nil {
		log.Printf("discord-funnel: failed to create chat for discord message %s: %v", discordMessageID, err)
		return
	}
	if created.ID == "" {
		log.Printf("discord-funnel: chat creation for discord message %s returned no chat id", discordMessageID)
		return
	}

	completionReq := completionsRequest{
		ChatID:    created.ID,
		ID:        assistantMsgID,
		Messages:  []completionMessage{{Role: "user", Content: content}},
		Model:     model,
		Stream:    false,
		SessionID: sessionID,
	}

	if err := postJSON(client, baseURL+"/api/chat/completions", apiKey, completionReq, nil); err != nil {
		log.Printf("discord-funnel: chat %s created but completion failed for discord message %s: %v", created.ID, discordMessageID, err)
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
