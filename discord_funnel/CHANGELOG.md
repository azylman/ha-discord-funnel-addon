# Changelog

## 0.2.0

- Replaced the generic webhook-POST behavior with Open WebUI backend chat
  integration: each qualifying Discord message now creates a new Open
  WebUI chat and triggers a completion in it (`POST /api/v1/chats/new`
  then `POST /api/chat/completions`).
- Config options changed: `api_url` replaced by `webui_url`,
  `webui_api_key`, and `webui_model`.

## 0.1.0

- Initial release: relay live Discord messages (or mentions-only) to a
  configurable HTTP endpoint.
