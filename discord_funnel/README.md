# Discord Funnel

A lightweight Go service that connects to Discord as a bot, consumes the live message feed, and for each qualifying message renders a custom Go `text/template` payload and POSTs it with retries to a configured `target_url` (e.g. Gundam Brain).

## Configuration Options

| Option | Type | Description |
| --- | --- | --- |
| `discord_token` | string | The Discord bot token. |
| `target_url` | url | Target URL to send POST requests to (e.g. `http://b4a16ffd-gundam-brain:8080/api/prompt`). |
| `payload_template` | string (optional) | Go `text/template` string used to generate the request payload. Defaults to formatting all Discord message keys and values for Gundam Brain. |
| `mentions_only` | bool | If `true`, only forwards messages that mention the bot (default: `false`). |

## Default Template

```gotemplate
{"prompt": "Discord Message:\n{{range $k, $v := .}}- {{$k}}: {{$v | escapeJSON}}\n{{end}}"}
```
