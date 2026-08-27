# Changelog

## 0.4.0
- Replaced Open WebUI-specific chat creation flow with generic target URL webhook forwarding.
- Added Go `text/template` payload customization via `payload_template` option and `--template` flag.
- Default template formats all Discord message keys and values suitable for `gundam-brain` server.
- Automatic retry with exponential backoff on HTTP POST requests.

## 0.3.0
- Initial release of Discord Funnel.
