# Changelog

## 0.5.1
- Moved thread vs reply decision logic entirely into the prompt template.
- Default template instructs agent to use `discord_create_thread` only if not already part of a thread, otherwise use `discord_send`.

## 0.4.9
- Updated default template instructions to explicitly direct the agent to create a thread and post its reply in the thread using `channelId` and `replyToMessageId`.

## 0.4.8
- Auto-process all messages inside active Thread channels without requiring repetitive @-mentions.

## 0.4.7
- Expanded target trigger matching: any mention, "gundam", "brain", "bot", inline thread replies, or direct messages.
- Detailed content logging for ignored messages.

## 0.4.6
- Added comprehensive mention detection: support structured mentions, raw name/text mentions, thread message replies, direct messages, and role mentions.

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
