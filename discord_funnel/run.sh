#!/bin/bash
set -euo pipefail

CONFIG_PATH=/data/options.json

DISCORD_TOKEN="$(jq -r '.discord_token' "$CONFIG_PATH")"
WEBUI_URL="$(jq -r '.webui_url' "$CONFIG_PATH")"
WEBUI_API_KEY="$(jq -r '.webui_api_key' "$CONFIG_PATH")"
WEBUI_MODEL="$(jq -r '.webui_model' "$CONFIG_PATH")"
MENTIONS_ONLY="$(jq -r '.mentions_only' "$CONFIG_PATH")"

if [ -z "$DISCORD_TOKEN" ] || [ "$DISCORD_TOKEN" = "null" ]; then
  echo "discord-funnel: discord_token is not set in the add-on configuration" >&2
  exit 1
fi

if [ -z "$WEBUI_URL" ] || [ "$WEBUI_URL" = "null" ]; then
  echo "discord-funnel: webui_url is not set in the add-on configuration" >&2
  exit 1
fi

if [ -z "$WEBUI_API_KEY" ] || [ "$WEBUI_API_KEY" = "null" ]; then
  echo "discord-funnel: webui_api_key is not set in the add-on configuration" >&2
  exit 1
fi

if [ -z "$WEBUI_MODEL" ] || [ "$WEBUI_MODEL" = "null" ]; then
  echo "discord-funnel: webui_model is not set in the add-on configuration" >&2
  exit 1
fi

exec /usr/bin/discord-funnel \
  --token="${DISCORD_TOKEN}" \
  --webui-url="${WEBUI_URL}" \
  --api-key="${WEBUI_API_KEY}" \
  --model="${WEBUI_MODEL}" \
  --mentions-only="${MENTIONS_ONLY}"
