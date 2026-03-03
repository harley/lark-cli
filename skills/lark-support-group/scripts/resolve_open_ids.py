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


def norm_email(value: str) -> str:
    return value.strip().lower()


def parse_emails(raw: List[str]) -> List[str]:
    out: List[str] = []
    for item in raw:
        for piece in item.split(","):
            email = norm_email(piece)
            if email:
                out.append(email)
    # Keep stable order while deduping.
    seen = set()
    deduped: List[str] = []
    for email in out:
        if email in seen:
            continue
        seen.add(email)
        deduped.append(email)
    return deduped


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Resolve Lark user emails to open IDs via lark-cli users list."
    )
    parser.add_argument(
        "--emails",
        nargs="+",
        required=True,
        help="Email list (space- or comma-separated).",
    )
    parser.add_argument(
        "--department-id",
        default="0",
        help="Department ID (default: 0 for top-level).",
    )
    parser.add_argument(
        "--department-id-type",
        default="open_department_id",
        help="Department ID type (default: open_department_id).",
    )
    parser.add_argument(
        "--lark-cli",
        default="lark-cli",
        help="lark-cli binary path/name (default: lark-cli).",
    )
    parser.add_argument(
        "--require-all",
        action="store_true",
        help="Exit non-zero if any email cannot be resolved.",
    )
    args = parser.parse_args()

    emails = parse_emails(args.emails)
    if not emails:
        print(json.dumps({"ok": False, "error": "no emails provided"}))
        return 2

    data = run_lark_cli(
        args.lark_cli,
        [
            "users",
            "list",
            "--department-id",
            args.department_id,
            "--department-id-type",
            args.department_id_type,
            "--fields",
            "name,email,open_id,user_id,union_id",
        ],
    )

    users = data.get("users", [])
    by_email: Dict[str, Dict[str, Any]] = {}
    for user in users:
        email = norm_email(str(user.get("email", "")))
        if email:
            by_email[email] = user

    matched: List[Dict[str, Any]] = []
    unmatched: List[str] = []
    for email in emails:
        user = by_email.get(email)
        if user is None:
            unmatched.append(email)
            continue
        matched.append(
            {
                "email": email,
                "name": user.get("name", ""),
                "open_id": user.get("open_id", ""),
                "user_id": user.get("user_id", ""),
                "union_id": user.get("union_id", ""),
            }
        )

    result = {
        "ok": len(unmatched) == 0,
        "requested_count": len(emails),
        "matched_count": len(matched),
        "unmatched_count": len(unmatched),
        "matched": matched,
        "unmatched": unmatched,
    }
    print(json.dumps(result))

    if args.require_all and unmatched:
        return 1
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except RuntimeError as exc:
        print(json.dumps({"ok": False, "error": str(exc)}))
        raise SystemExit(1)
