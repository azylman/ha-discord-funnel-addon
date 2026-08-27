#!/bin/bash
set -euo pipefail

CONFIG_PATH=/data/options.json

DISCORD_TOKEN="$(jq -r '.discord_token // empty' "$CONFIG_PATH")"
TARGET_URL="$(jq -r '.target_url // .webui_url // empty' "$CONFIG_PATH")"
PAYLOAD_TEMPLATE="$(jq -r '.payload_template // empty' "$CONFIG_PATH")"
MENTIONS_ONLY="$(jq -r '.mentions_only // false' "$CONFIG_PATH")"

if [ -z "$DISCORD_TOKEN" ]; then
  echo "discord-funnel: discord_token is not set in the add-on configuration" >&2
  exit 1
fi

if [ -z "$TARGET_URL" ]; then
  echo "discord-funnel: target_url is not set in the add-on configuration" >&2
  exit 1
fi

ARGS=(
  --token="${DISCORD_TOKEN}"
  --target-url="${TARGET_URL}"
  --mentions-only="${MENTIONS_ONLY}"
)

if [ -n "$PAYLOAD_TEMPLATE" ]; then
  ARGS+=(--template="${PAYLOAD_TEMPLATE}")
fi

exec /usr/bin/discord-funnel "${ARGS[@]}"
