# lark-cli — DEPRECATED

> ⚠️ **This repository is deprecated and archived.**
>
> Use the official Lark/Feishu CLI instead:
> **https://github.com/larksuite/cli**
>
> ```bash
> npx @larksuite/cli@latest install
> ```

## Why deprecated

When I built this in March 2026, there was no official CLI. Lark has since
released [`larksuite/cli`](https://github.com/larksuite/cli) — an officially
maintained tool that is strictly broader than this repo in every dimension:

- **200+ commands across 18 domains** (Messenger, Docs, Base, Sheets, Slides,
  Calendar, Mail, Tasks, Wiki, Meetings, Approval, OKR, Attendance, and more).
  This repo covered 4: `auth`, `msg`, `api`, `users`.
- **26 first-party AI Agent Skills**, designed for Claude Code and similar
  agents — vs. 1 skill (`lark-support-group`) here.
- **One-command install via npm**, vs. building from Go source.
- **Interactive login + OS keychain credential storage**, vs. env vars only.
- **Three-layer architecture** (Shortcuts → API Commands → Raw API) gives
  agents the right granularity for each task.

There is no functionality in this repo that the official CLI does not cover
better. Migrate.

## Migration

| This repo | Official `lark-cli` |
|---|---|
| `lark-cli auth tenant-token` | `lark-cli auth login` |
| `lark-cli msg text --to-type chat_id --to … --text …` | `lark-cli im +send` (see `lark-im` skill) |
| `lark-cli api call --method GET --path …` | `lark-cli api call …` (raw API layer) |
| `lark-cli users list --department-id …` | `lark-cli contact …` (see `lark-contact` skill) |

The one custom piece here, the `lark-support-group` skill under `skills/`,
can be rewritten as a small wrapper over the official CLI's `im` + `contact`
commands.
