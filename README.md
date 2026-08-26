# ha-discord-funnel-addon

A Home Assistant add-on repository containing **Discord Funnel** — a small
persistent Go service that connects to Discord as a bot, listens to the live
message feed, and POSTs each (or just mention) message as JSON to a
configurable HTTP endpoint.

## Installing this repository in Home Assistant

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**.
2. Click the **⋮** menu (top right) → **Repositories**.
3. Add: `https://github.com/azylman/ha-discord-funnel-addon`
4. Close the dialog, refresh, and find **Discord Funnel** in the store.
5. Install, configure (see below), and start it.

## Add-ons in this repository

- [`discord_funnel`](./discord_funnel/README.md) — relays live Discord
  messages to an HTTP endpoint.
