# Discord Funnel

A persistent Go service (using [`bwmarrin/discordgo`](https://github.com/bwmarrin/discordgo),
the most widely used Go Discord library) that connects to Discord as a bot,
consumes the live gateway message feed, and forwards each message as JSON via
HTTP POST to a URL you configure.

## One-time Discord setup

1. Create an application + bot at the [Discord Developer Portal](https://discord.com/developers/applications).
2. Under **Bot**, copy the bot **Token** (you'll paste it into the add-on config as `discord_token`).
3. Under **Bot → Privileged Gateway Intents**, enable **MESSAGE CONTENT INTENT**
   (required for the bot to read message text — without it, `content` will
   always be empty). Enable **SERVER MEMBERS INTENT** too if you plan to
   extend this later to resolve member info.
4. Under **OAuth2 → URL Generator**, select the `bot` scope and at least the
   `Read Messages/View Channels` + `Read Message History` permissions, then
   use the generated URL to invite the bot to your server(s).

## Add-on configuration

| Option | Type | Description |
| --- | --- | --- |
| `discord_token` | string | The bot token from the Developer Portal. |
| `api_url` | url | The HTTP endpoint to POST each message to as JSON. |
| `mentions_only` | bool | If `true`, only forward messages that @-mention the bot (or `@everyone`/`@here`). If `false` (default), forward every message the bot can see. |

These are surfaced in the Supervisor **Configuration** tab and passed straight
through to the Go binary as `--token`, `--url`, and `--mentions-only` CLI flags
(see `run.sh`), so the binary is also runnable/testable standalone outside HA:

```sh
go run . --token="$DISCORD_TOKEN" --url="https://example.com/webhook" --mentions-only=true
```

## Payload shape

Each forwarded message is POSTed as `application/json` with this shape:

```json
{
  "message_id": "1234567890",
  "channel_id": "1111111111",
  "guild_id": "2222222222",
  "author": { "id": "333", "username": "someone", "bot": false },
  "content": "hey @funnel-bot, ping",
  "mentions": [{ "id": "444", "username": "funnel-bot" }],
  "is_mention": true,
  "timestamp": "2026-01-01T00:00:00Z"
}
```

`guild_id` is omitted for DMs. `is_mention` is true when the bot is
directly @-mentioned or `@everyone`/`@here` is used.

## Notes / limitations

- The bot's own messages are never forwarded.
- Delivery is fire-and-forget (best effort, 10s timeout per request, one
  retry-free attempt); failures are logged to the add-on log but do not
  crash the service or block message processing.
- No message queue/persistence — if the add-on is down, messages sent during
  that window are not replayed.
