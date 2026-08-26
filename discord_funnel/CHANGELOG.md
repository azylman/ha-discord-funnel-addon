# Changelog

## 0.3.0

- Added bounded retry (3 attempts, exponential backoff: 500ms/1s/2s) for
  the completion fetch and chat-update steps, to ride out intermittent
  upstream flakiness (observed: Open WebUI's connection pool to a
  keep-alive backend occasionally raising
  `aiohttp.client_exceptions.ClientOSError: Can not write request body`).
- Chat creation (`POST /api/v1/chats/new`) is deliberately never retried,
  since each call creates a brand new chat — retrying it on a
  false-negative (e.g. a client-side timeout after the server actually
  succeeded) would leave a duplicate chat behind for the same Discord
  message. The completion fetch and chat-update steps are safe to retry
  because they have no such duplication risk: the completion call has no
  persisted side effects in Open WebUI (chat_id/id are never sent — see
  0.2.1), and the chat-update call is a full-resource replace with an
  identical body each time, so repeating it converges to the same state.

## 0.2.1

- Fixed "Open WebUI: Server Connection Error" on every generated chat: the
  completion request was including `chat_id`/`id`, which Open WebUI
  interprets as "this caller has a live WebSocket, push the reply there"
  — since discord-funnel never opens one, the push failed and also
  corrupted the assistant message's `parentId`, producing the follow-on
  "parent message not found" on retry.
- Completions are now requested without `chat_id`/`id` (plain synchronous
  HTTP), and the reply is written into the chat via a separate
  `POST /api/v1/chats/{id}` update once we have it.

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
