---
name: lark-support-group
description: Create and bootstrap Lark support groups for a person and their manager using lark-cli. Use when asked to create a group chat, add specific members, add the bot/assistant to the group, and optionally post a kickoff message for ongoing support.
---

# Lark Support Group

Use this skill to create a support group chat in Lark with deterministic CLI/script steps.

## Prerequisites

- Ensure `lark-cli` is installed and reachable on `PATH`.
- Ensure auth environment is configured (`LARK_APP_ID`, `LARK_APP_SECRET`).
- Ensure app scopes allow:
  - creating chats
  - adding chat members
  - listing users (if resolving by email)

## Inputs

- Group name
- Person email/open_id
- Manager email/open_id
- Optional kickoff message

## Workflow

1. Resolve emails to open IDs when needed.
2. Create the group and add members.
3. Join bot account to the group (unless explicitly skipped).
4. Post kickoff message if requested.
5. Return `chat_id` and final member IDs.

## Commands

Resolve user IDs from emails:

```bash
python3 scripts/resolve_open_ids.py \
  --emails person@example.com manager@example.com \
  --require-all
```

Create support group and bootstrap it:

```bash
python3 scripts/create_support_group.py \
  --name "Support - Person Name" \
  --member-open-id ou_xxx_person \
  --member-open-id ou_xxx_manager \
  --kickoff-text "Hi team, I will run support check-ins in this thread."
```

## Output Format

Return a short status summary:

- `chat_id`
- `group_name`
- `member_open_ids`
- `bot_joined`
- `kickoff_sent`

