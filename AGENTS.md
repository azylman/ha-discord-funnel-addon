# Agent Instructions & Architecture Guidelines: Discord Funnel

## 1. Capabilities in Code, Behavior in Templates (CRITICAL INVARIANT)
- **The only logic that belongs in Go code is capability and routing**:
  - Discord Gateway connection and event lifecycle management.
  - Event filtering and targeting rules (e.g., mention, role, or DM detection).
  - Extracting raw Discord payload fields into generic data maps.
  - Go `text/template` execution and HTTP delivery with retry/backoff.
- **ALL behavior, decision-making, and agent instructions MUST be encoded in the template/prompt**:
  - NEVER write Go code heuristics to decide agent behavior (e.g., do not write Go logic to branch whether the agent should create a thread vs send an inline reply).
  - All behavioral steering, operational instructions, format constraints, and tool directions belong strictly inside `PAYLOAD_TEMPLATE` / `defaultTemplate`.

## 2. Generic Message Data Exposure
- Go code must expose standard, unopinionated Discord platform data fields (`id`, `channel_id`, `guild_id`, `content`, `author`, `attachments`, etc.).
- Do not synthesize conversational directives or semantic branching in Go code.

## 3. Secrets & Credential Isolation
- **NEVER commit tokens, API keys, or secrets into git**.
- Default values in `config.yaml` must always remain empty strings (`""`).
- All credentials (Discord bot tokens, webhook endpoints) must be configured exclusively through Home Assistant add-on options (`/data/options.json`) or environment variables.

## 4. Lightweight & Deterministic Transport
- `discord_funnel` is strictly a headless event forwarder. Keep it minimal, fast, deterministic, and free of conversational reasoning logic.
