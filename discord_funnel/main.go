// Command discord-funnel connects to Discord as a bot, listens to the live
// message feed, and forwards each message (or just messages that mention
// the bot, depending on the --mentions-only flag) as JSON to a configurable
// HTTP endpoint via POST.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

type messagePayload struct {
	MessageID string    `json:"message_id"`
	ChannelID string    `json:"channel_id"`
	GuildID   string    `json:"guild_id,omitempty"`
	Author    author    `json:"author"`
	Content   string    `json:"content"`
	Mentions  []mention `json:"mentions"`
	IsMention bool      `json:"is_mention"`
	Timestamp time.Time `json:"timestamp"`
}

type author struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

type mention struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func main() {
	token := flag.String("token", "", "Discord bot token (required)")
	url := flag.String("url", "", "API endpoint URL to POST messages to (required)")
	mentionsOnly := flag.Bool("mentions-only", false, "Only forward messages that mention the bot (default: forward every message)")
	flag.Parse()

	if *token == "" {
		log.Fatal("discord-funnel: --token is required")
	}
	if *url == "" {
		log.Fatal("discord-funnel: --url is required")
	}

	dg, err := discordgo.New("Bot " + *token)
	if err != nil {
		log.Fatalf("discord-funnel: failed to create Discord session: %v", err)
	}

	// GuildMessages + MessageContent are needed to receive message text in
	// guild channels; DirectMessages covers DMs to the bot.
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent | discordgo.IntentsDirectMessages

	httpClient := &http.Client{Timeout: 10 * time.Second}

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

		payload := messagePayload{
			MessageID: m.ID,
			ChannelID: m.ChannelID,
			GuildID:   m.GuildID,
			Author:    author{ID: m.Author.ID, Username: m.Author.Username, Bot: m.Author.Bot},
			Content:   m.Content,
			IsMention: isMention,
			Timestamp: time.Now().UTC(),
		}
		for _, u := range m.Mentions {
			payload.Mentions = append(payload.Mentions, mention{ID: u.ID, Username: u.Username})
		}

		go forward(httpClient, *url, payload)
	})

	if err := dg.Open(); err != nil {
		log.Fatalf("discord-funnel: failed to open Discord session: %v", err)
	}
	defer dg.Close()

	log.Printf("discord-funnel: connected to Discord, mentions_only=%v, forwarding to %s", *mentionsOnly, *url)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("discord-funnel: shutting down")
}

func forward(client *http.Client, url string, payload messagePayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("discord-funnel: failed to marshal payload for message %s: %v", payload.MessageID, err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("discord-funnel: failed to build request for message %s: %v", payload.MessageID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("discord-funnel: failed to POST message %s: %v", payload.MessageID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("discord-funnel: endpoint returned status %d for message %s", resp.StatusCode, payload.MessageID)
	}
}
