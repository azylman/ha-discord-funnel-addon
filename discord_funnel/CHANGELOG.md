# Changelog

## 0.4.5
- Added gateway lifecycle handlers (Ready, Disconnect, Resumed) for enhanced connection diagnostics.
- Enhanced message logging with channel ID, author, and content preview.

## 0.4.4
- Set template to exact format requested for forwarding payload to Gundam Brain.

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
