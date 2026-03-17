# lark-cli

Agent-friendly CLI wrapper around [`github.com/go-lark/lark`](https://github.com/go-lark/lark).

## Why this helps skill creation

When skills need to interact with Lark, calling a single deterministic CLI is more reliable than rewriting API calls in each skill:

- Stable command contract (`auth`, `msg`, `api`)
- Stable command contract (`auth`, `msg`, `api`, `users`)
- Machine-readable JSON output by default
- Consistent auth and domain handling through env vars

## Prerequisites

- Go 1.25+
- A Lark/Feishu custom app with `app_id` and `app_secret`

## Build

```bash
go build -o lark-cli .
```

## Configuration

Environment variables:

- `LARK_APP_ID`
- `LARK_APP_SECRET`
- `LARK_DOMAIN` (`lark`, `feishu`, or full URL)
- `LARK_OUTPUT` (`json` or `text`)
- `LARK_USER_ID_TYPE` (optional)

Profile-aware config:

- `--profile <name>` switches credential lookup to `<NAME>_LARK_APP_ID` / `<NAME>_LARK_APP_SECRET` by default.
- `--config <path>` loads `~/.lark-cli/config.toml`-style TOML for explicit profile definitions.

Example `config.toml`:

```toml
version = 1

[profiles.onboard]
output = "json"

[profiles.onboard.lark]
app_id_env = "ONBOARD_LARK_APP_ID"
app_secret_env = "ONBOARD_LARK_APP_SECRET"
domain = "lark"
user_id_type = "open_id"
```

## Commands

```bash
lark-cli auth tenant-token
lark-cli auth tenant-token --show-token
lark-cli msg text --to-type chat_id --to oc_xxx --text "hello"
lark-cli msg send --input @message.json
lark-cli msg update --message-id om_xxx --text "updated"
lark-cli --profile onboard auth tenant-token
lark-cli api call --method GET --path /open-apis/im/v1/chats --params '{"page_size": 20}'
lark-cli users list --department-id 0 --fields name,email,department_ids
```

## Notes

- Default output is JSON, designed for agent/tool consumption.
- `auth tenant-token` redacts token output by default; use `--show-token` to print the full value.
- `msg send` accepts `--input -` (stdin), inline JSON, or `@file`.
- `msg update` edits an existing message using either `--text` for simple text updates or `--input` for a full `OutcomingMessage` JSON payload.
- `api call` supports `GET|POST|PUT|PATCH|DELETE`.
- `users list` fetches paginated org members from Contact API with retry handling.
- For support/onboarding group creation, prefer **private** chats and skip `me_join`; bots can still send the kickoff message directly after private-chat creation.
