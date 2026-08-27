# Changelog

## 0.5.2
- Pass `conversation_id` in default payload template to support continuous thread conversation scoping in Gundam Brain.

## 0.5.1
- Moved thread vs reply decision logic entirely into the prompt template.
- Default template instructs agent to use `discord_create_thread` only if not already part of a thread, otherwise use `discord_send`.

## 0.4.9
- Updated default template instructions to explicitly direct the agent to create a thread and post its reply in the thread using `channelId` and `replyToMessageId`.
