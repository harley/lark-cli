#!/usr/bin/env python3
import argparse
import json
import subprocess
import sys
from typing import Any, Dict, List


def run_lark_cli(lark_cli: str, args: List[str]) -> Dict[str, Any]:
    proc = subprocess.run(
        [lark_cli, *args],
        capture_output=True,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        err = proc.stderr.strip() or proc.stdout.strip()
        raise RuntimeError(f"lark-cli failed ({proc.returncode}): {err}")

    try:
        payload = json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"invalid JSON from lark-cli: {exc}") from exc

    if not payload.get("ok", False):
        raise RuntimeError(payload.get("error", "unknown lark-cli error"))

    return payload.get("data", {})


def parse_ids(repeated: List[str], comma_joined: str) -> List[str]:
    out: List[str] = []
    for value in repeated:
        v = value.strip()
        if v:
            out.append(v)
    for value in comma_joined.split(","):
        v = value.strip()
        if v:
            out.append(v)

    # Stable dedupe.
    seen = set()
    deduped: List[str] = []
    for value in out:
        if value in seen:
            continue
        seen.add(value)
        deduped.append(value)
    return deduped


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Create and bootstrap a Lark support group using lark-cli."
    )
    parser.add_argument("--name", required=True, help="Group chat name.")
    parser.add_argument(
        "--member-open-id",
        action="append",
        default=[],
        help="Member open_id (repeat flag for multiple members).",
    )
    parser.add_argument(
        "--member-open-ids",
        default="",
        help="Comma-separated member open_ids.",
    )
    parser.add_argument("--owner-open-id", default="", help="Optional owner open_id.")
    parser.add_argument("--description", default="", help="Optional group description.")
    parser.add_argument(
        "--chat-type",
        default="private",
        choices=["private", "public"],
        help="Lark chat type (default: private).",
    )
    parser.add_argument(
        "--skip-bot-join",
        action="store_true",
        help="Skip adding bot/app account itself to the group.",
    )
    parser.add_argument(
        "--kickoff-text",
        default="",
        help="Optional kickoff message to send after setup.",
    )
    parser.add_argument(
        "--lark-cli",
        default="lark-cli",
        help="lark-cli binary path/name (default: lark-cli).",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print plan without calling Lark APIs.",
    )
    args = parser.parse_args()

    member_ids = parse_ids(args.member_open_id, args.member_open_ids)
    if not member_ids:
        print(json.dumps({"ok": False, "error": "no member open_ids provided"}))
        return 2

    create_params: Dict[str, Any] = {
        "name": args.name,
        "chat_mode": "group",
        "chat_type": args.chat_type,
    }
    if args.owner_open_id.strip():
        create_params["owner_id"] = args.owner_open_id.strip()
    if args.description.strip():
        create_params["description"] = args.description.strip()

    if args.dry_run:
        print(
            json.dumps(
                {
                    "ok": True,
                    "dry_run": True,
                    "create_chat_params": create_params,
                    "member_open_ids": member_ids,
                    "bot_join": not args.skip_bot_join,
                    "kickoff_text": args.kickoff_text,
                }
            )
        )
        return 0

    create_data = run_lark_cli(
        args.lark_cli,
        [
            "api",
            "call",
            "--method",
            "POST",
            "--path",
            "/open-apis/im/v1/chats?user_id_type=open_id",
            "--params",
            json.dumps(create_params),
        ],
    )
    chat_id = str(create_data.get("data", {}).get("chat_id", "")).strip()
    if not chat_id:
        raise RuntimeError("chat creation succeeded but chat_id is missing")

    add_member_data = run_lark_cli(
        args.lark_cli,
        [
            "api",
            "call",
            "--method",
            "POST",
            "--path",
            f"/open-apis/im/v1/chats/{chat_id}/members?member_id_type=open_id",
            "--params",
            json.dumps({"id_list": member_ids}),
        ],
    )

    bot_joined = False
    if not args.skip_bot_join:
        run_lark_cli(
            args.lark_cli,
            [
                "api",
                "call",
                "--method",
                "PATCH",
                "--path",
                f"/open-apis/im/v1/chats/{chat_id}/members/me_join",
                "--params",
                "{}",
            ],
        )
        bot_joined = True

    kickoff_sent = False
    kickoff_message_id = ""
    if args.kickoff_text.strip():
        msg_data = run_lark_cli(
            args.lark_cli,
            [
                "msg",
                "text",
                "--to-type",
                "chat_id",
                "--to",
                chat_id,
                "--text",
                args.kickoff_text.strip(),
            ],
        )
        kickoff_sent = True
        kickoff_message_id = str(msg_data.get("message_id", ""))

    result = {
        "ok": True,
        "chat_id": chat_id,
        "group_name": args.name,
        "member_open_ids": member_ids,
        "bot_joined": bot_joined,
        "kickoff_sent": kickoff_sent,
        "kickoff_message_id": kickoff_message_id,
        "create_chat_response": create_data,
        "add_members_response": add_member_data,
    }
    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except RuntimeError as exc:
        print(json.dumps({"ok": False, "error": str(exc)}))
        raise SystemExit(1)
