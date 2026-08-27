package main

import (
	"bytes"
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
	"text/template"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	defaultTemplate = `{"prompt": "Discord Message:\n{{range $k, $v := .}}- {{$k}}: {{$v | escapeJSON}}\n{{end}}"}`
	maxRetries      = 3
	initialDelay    = 500 * time.Millisecond
)

func escapeJSON(v any) string {
	b, err := json.Marshal(fmt.Sprint(v))
	if err != nil {
		return fmt.Sprint(v)
	}
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		return string(b[1 : len(b)-1])
	}
	return string(b)
}

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func buildMessageData(m *discordgo.Message) map[string]any {
	data := make(map[string]any)

	// Top-level Discord fields
	data["id"] = m.ID
	data["channel_id"] = m.ChannelID
	data["guild_id"] = m.GuildID
	data["content"] = m.Content
	data["timestamp"] = m.Timestamp.Format(time.RFC3339)
	data["mention_everyone"] = m.MentionEveryone
	data["pinned"] = m.Pinned
	data["tts"] = m.TTS
	data["type"] = int(m.Type)
	data["webhook_id"] = m.WebhookID

	if m.EditedTimestamp != nil {
		data["edited_timestamp"] = m.EditedTimestamp.Format(time.RFC3339)
	} else {
		data["edited_timestamp"] = ""
	}

	if m.Author != nil {
		data["author_id"] = m.Author.ID
		data["author_username"] = m.Author.Username
		data["author_discriminator"] = m.Author.Discriminator
		data["author_global_name"] = m.Author.GlobalName
		data["author_bot"] = m.Author.Bot
		data["author"] = map[string]any{
			"id":            m.Author.ID,
			"username":      m.Author.Username,
			"discriminator": m.Author.Discriminator,
			"global_name":   m.Author.GlobalName,
			"bot":           m.Author.Bot,
		}
	}

	if m.Member != nil {
		data["member_nick"] = m.Member.Nick
		data["member_roles"] = m.Member.Roles
	}

	// Mentions
	var mentionUsernames []string
	var mentionIDs []string
	for _, u := range m.Mentions {
		mentionUsernames = append(mentionUsernames, u.Username)
		mentionIDs = append(mentionIDs, u.ID)
	}
	data["mentions"] = mentionUsernames
	data["mention_ids"] = mentionIDs
	data["mention_roles"] = m.MentionRoles

	// Attachments
	var attachmentURLs []string
	for _, a := range m.Attachments {
		attachmentURLs = append(attachmentURLs, a.URL)
	}
	data["attachments"] = attachmentURLs

	return data
}

func forwardMessage(client *http.Client, targetURL string, tmpl *template.Template, m *discordgo.Message) {
	data := buildMessageData(m)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Printf("discord-funnel: template execution failed for message %s: %v", m.ID, err)
		return
	}

	payload := buf.Bytes()

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(payload))
		if err != nil {
			log.Printf("discord-funnel: failed to build HTTP request: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				log.Printf("discord-funnel: successfully forwarded message %s to %s (status %d)", m.ID, targetURL, resp.StatusCode)
				return
			}
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(respBody), 200))
		} else {
			lastErr = err
		}

		log.Printf("discord-funnel: attempt %d/%d failed to forward message %s to %s: %v", attempt, maxRetries, m.ID, targetURL, lastErr)
		if attempt < maxRetries {
			time.Sleep(initialDelay * time.Duration(1<<(attempt-1)))
		}
	}

	log.Printf("discord-funnel: permanently failed to forward message %s after %d attempts: %v", m.ID, maxRetries, lastErr)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func main() {
	token := flag.String("token", "", "Discord bot token (required)")
	targetURL := flag.String("target-url", "", "Target URL to POST payload to (required)")
	webUIURL := flag.String("webui-url", "", "Alias for target-url (for backwards compatibility)")
	tmplStr := flag.String("template", defaultTemplate, "Go text/template for the request payload")
	mentionsOnly := flag.Bool("mentions-only", false, "Only act on messages that mention the bot (default: act on every message)")
	flag.Parse()

	if *token == "" {
		log.Fatal("discord-funnel: --token is required")
	}

	finalURL := *targetURL
	if finalURL == "" {
		finalURL = *webUIURL
	}
	if finalURL == "" {
		log.Fatal("discord-funnel: --target-url is required")
	}

	rawTemplate := *tmplStr
	if strings.TrimSpace(rawTemplate) == "" {
		rawTemplate = defaultTemplate
	}

	tmpl, err := template.New("payload").Funcs(template.FuncMap{
		"escapeJSON": escapeJSON,
		"json":       toJSON,
		"toJson":     toJSON,
		"quote":      func(v any) string { return fmt.Sprintf("%q", fmt.Sprint(v)) },
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"trim":       strings.TrimSpace,
	}).Parse(rawTemplate)
	if err != nil {
		log.Fatalf("discord-funnel: invalid payload template: %v", err)
	}

	dg, err := discordgo.New("Bot " + *token)
	if err != nil {
		log.Fatalf("discord-funnel: failed to create Discord session: %v", err)
	}

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

		if m.Content == "" && len(m.Attachments) == 0 && len(m.Embeds) == 0 {
			return
		}

		go forwardMessage(httpClient, finalURL, tmpl, m.Message)
	})

	if err := dg.Open(); err != nil {
		log.Fatalf("discord-funnel: failed to open Discord session: %v", err)
	}
	defer dg.Close()

	log.Printf("discord-funnel: connected to Discord, mentions_only=%v, forwarding messages to %s", *mentionsOnly, finalURL)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("discord-funnel: shutting down")
}
