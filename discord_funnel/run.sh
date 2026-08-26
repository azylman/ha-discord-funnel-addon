#!/bin/bash
set -euo pipefail

CONFIG_PATH=/data/options.json

DISCORD_TOKEN="$(jq -r '.discord_token' "$CONFIG_PATH")"
API_URL="$(jq -r '.api_url' "$CONFIG_PATH")"
MENTIONS_ONLY="$(jq -r '.mentions_only' "$CONFIG_PATH")"

if [ -z "$DISCORD_TOKEN" ] || [ "$DISCORD_TOKEN" = "null" ]; then
  echo "discord-funnel: discord_token is not set in the add-on configuration" >&2
  exit 1
fi

if [ -z "$API_URL" ] || [ "$API_URL" = "null" ]; then
  echo "discord-funnel: api_url is not set in the add-on configuration" >&2
  exit 1
fi

exec /usr/bin/discord-funnel \
  --token="${DISCORD_TOKEN}" \
  --url="${API_URL}" \
  --mentions-only="${MENTIONS_ONLY}"
