# Changelog

## 0.4.3
- Refined default template instruction with explicit channel_id and send_message tool call.

## 0.4.2
- Add `--target-url` flag alias.
- Updated default message template to instruct agent to respond in a Discord thread via MCP.

## 0.4.0
- Refactored server into a generic HTTP forwarding engine decoupled from Open WebUI.
- Added Go `text/template` support for dynamic JSON request formatting.
- Added exponential backoff retry logic for webhook forwarding.

## 0.3.0
- Model config option for Open WebUI.

## 0.2.0
- Add mentions_only option.

## 0.1.0
- Initial release of Discord Funnel Home Assistant add-on.
