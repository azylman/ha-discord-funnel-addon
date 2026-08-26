# Discord Funnel

A persistent Go service (using [`bwmarrin/discordgo`](https://github.com/bwmarrin/discordgo),
the most widely used Go Discord library) that connects to Discord as a bot,
consumes the live gateway message feed, and for each qualifying message
**creates a brand new chat in an Open WebUI instance** and triggers a model
reply in it — one Discord message in, one new Open WebUI chat out.

It follows Open WebUI's documented backend-controlled chat flow
(`POST /api/v1/chats/new` then `POST /api/chat/completions`); see
[Backend-Controlled API Flow](https://docs.openwebui.com/reference/api-flow/)
and [API Endpoints](https://docs.openwebui.com/reference/api-endpoints/) for
the underlying contract this mirrors.

## One-time Discord setup

1. Create an application + bot at the [Discord Developer Portal](https://discord.com/developers/applications).
2. Under **Bot**, copy the bot **Token** (you'll paste it into the add-on config as `discord_token`).
3. Under **Bot → Privileged Gateway Intents**, enable **MESSAGE CONTENT INTENT**
   (required for the bot to read message text — without it, messages will
   be skipped since there's no content to send). Enable **SERVER MEMBERS
   INTENT** too if you plan to extend this later to resolve member info.
4. Under **OAuth2 → URL Generator**, select the `bot` scope and at least the
   `Read Messages/View Channels` + `Read Message History` permissions, then
   use the generated URL to invite the bot to your server(s).

## One-time Open WebUI setup

1. In Open WebUI, go to **Settings → Account** and generate an API key
   (you'll paste it into the add-on config as `webui_api_key`).
2. Note the model id you want new chats to use (as it appears in Open
   WebUI's model list / URL, e.g. `gpt-5.6-sol`) — this becomes
   `webui_model`.
3. Note the base URL of your Open WebUI instance reachable from this
   add-on's container, e.g. its internal HA hostname
   (`http://<slug>-open-webui:8080`) or an externally reachable URL — this
   becomes `webui_url`. No trailing path (no `/api/...`) — just the base.

## Add-on configuration

| Option | Type | Description |
| --- | --- | --- |
| `discord_token` | string | The bot token from the Developer Portal. |
| `webui_url` | url | Base URL of the Open WebUI instance (no path suffix). |
| `webui_api_key` | string | Open WebUI API key (Settings → Account). |
| `webui_model` | string | Model id to use for each new chat. |
| `mentions_only` | bool | If `true`, only act on messages that @-mention the bot (or `@everyone`/`@here`). If `false` (default), act on every message the bot can see. |

These are surfaced in the Supervisor **Configuration** tab and passed
straight through to the Go binary as `--token`, `--webui-url`, `--api-key`,
`--model`, and `--mentions-only` CLI flags (see `run.sh`), so the binary is
also runnable/testable standalone outside HA:

```sh
go run . \
  --token="$DISCORD_TOKEN" \
  --webui-url="http://localhost:8080" \
  --api-key="$WEBUI_API_KEY" \
  --model="gpt-5.6-sol" \
  --mentions-only=true
```

## What happens per qualifying message

For each Discord message that passes the `mentions_only` filter and has
non-empty text content:

1. A new chat is created in Open WebUI (`POST /api/v1/chats/new`) titled
   `Discord: #<channel_id> (<author>)`, with the Discord message as the
   user turn and an empty assistant placeholder — following Open WebUI's
   required message-tree shape (`history.currentId`, `childrenIds`,
   `parentId`, caller-generated UUIDs for every message id).
2. A completion is triggered in that chat (`POST /api/chat/completions`)
   using `webui_model`, non-streaming, so the assistant reply is generated
   and persisted into the chat.

The chat then exists in Open WebUI exactly as if a user had typed the
Discord message in as a new conversation — visible and continuable from
the Open WebUI UI itself.

## Notes / limitations

- The bot's own messages are never processed.
- Messages with empty content (e.g. attachment-only messages, or any
  message where `MESSAGE CONTENT INTENT` isn't enabled) are skipped.
- Each qualifying message becomes its own **new** chat — there's no
  merging of a user's multiple messages into a single running
  conversation, and no reply is sent back into Discord itself; the
  resulting chat only exists in Open WebUI.
- Delivery is fire-and-forget (best effort, 30s timeout per request, no
  retries); failures are logged to the add-on log but do not crash the
  service or block message processing.
- No message queue/persistence — if the add-on is down, messages sent
  during that window are not replayed.
