"""Print each pooled account's usage windows, when they reset, and what is about to expire unused.

Usage: URL=https://hara.local MGMT_KEY=... quota.py   (run it through `make quota`)

The provider usage endpoints are called through the proxy's /v0/management/api-call, which
substitutes $TOKEN$ with the account's OAuth token server-side: tokens never reach this script.
"""
from __future__ import annotations

import json
import os
import ssl
import sys
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone

CLAUDE_USAGE_URL = "https://api.anthropic.com/api/oauth/usage"
CODEX_USAGE_URL = "https://chatgpt.com/backend-api/wham/usage"
# A weekly window resetting this soon with capacity left is reported as expiring.
EXPIRING_WITHIN = timedelta(hours=24)
PORTLESS_CA = os.path.expanduser("~/.portless/ca.pem")


def ssl_context() -> ssl.SSLContext:
    # hara.local is signed by portless's own CA, which Python does not read from the Keychain.
    context = ssl.create_default_context()
    if os.path.exists(PORTLESS_CA):
        context.load_verify_locations(PORTLESS_CA)
    return context


def management(base_url: str, key: str, method: str, path: str, payload: dict | None = None) -> dict:
    data = json.dumps(payload).encode() if payload is not None else None
    request = urllib.request.Request(
        f"{base_url}/v0/management/{path}",
        data=data,
        method=method,
        headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=30, context=ssl_context()) as response:
        return json.load(response)


def usage_request(account: dict) -> dict | None:
    """The api-call payload that reads this account's usage, or None for unsupported providers."""
    provider = account.get("provider")
    if provider == "claude":
        return {
            "auth_index": account["auth_index"],
            "method": "GET",
            "url": CLAUDE_USAGE_URL,
            "header": {"Authorization": "Bearer $TOKEN$", "anthropic-beta": "oauth-2025-04-20"},
        }
    if provider == "codex":
        header = {"Authorization": "Bearer $TOKEN$", "User-Agent": "codex_cli_rs"}
        account_id = (account.get("id_token") or {}).get("chatgpt_account_id")
        if account_id:
            header["ChatGPT-Account-Id"] = account_id
        return {"auth_index": account["auth_index"], "method": "GET", "url": CODEX_USAGE_URL, "header": header}
    return None


def parse_time(value) -> datetime | None:
    if value in (None, ""):
        return None
    if isinstance(value, (int, float)):
        return datetime.fromtimestamp(value, timezone.utc)
    return datetime.fromisoformat(str(value).replace("Z", "+00:00"))


def claude_windows(usage: dict) -> list[tuple[str, float, datetime | None]]:
    """(label, used %, reset) for every window in /api/oauth/usage, e.g. five_hour, seven_day_opus."""
    windows = []
    for key, window in usage.items():
        if isinstance(window, dict) and window.get("utilization") is not None:
            label = key.replace("five_hour", "5h").replace("seven_day", "7d").replace("_", " ")
            windows.append((label, float(window["utilization"]), parse_time(window.get("resets_at"))))
    return windows


def codex_window_label(window: dict, fallback: str) -> str:
    seconds = window.get("limit_window_seconds") or 0
    if seconds and seconds % 86400 == 0:
        return f"{seconds // 86400}d"
    if seconds and seconds % 3600 == 0:
        return f"{seconds // 3600}h"
    return fallback


def codex_rate_limit_windows(rate_limit: dict, prefix: str = "") -> list[tuple[str, float, datetime | None]]:
    windows = []
    for name, fallback in (("primary_window", "primary"), ("secondary_window", "secondary")):
        window = rate_limit.get(name)
        if not isinstance(window, dict):
            continue
        reset = parse_time(window.get("reset_at"))
        if reset is None and window.get("reset_after_seconds") is not None:
            reset = datetime.now(timezone.utc) + timedelta(seconds=window["reset_after_seconds"])
        label = (prefix + codex_window_label(window, fallback)).strip()
        windows.append((label, float(window.get("used_percent") or 0), reset))
    return windows


def codex_windows(usage: dict) -> list[tuple[str, float, datetime | None]]:
    """(label, used %, reset) for the main and any additional (per-model) Codex limits."""
    windows = codex_rate_limit_windows(usage.get("rate_limit") or {})
    for extra in usage.get("additional_rate_limits") or []:
        prefix = f"{extra.get('limit_name') or extra.get('metered_feature') or 'extra'} "
        windows += codex_rate_limit_windows(extra.get("rate_limit") or {}, prefix)
    return windows


def until(moment: datetime, now: datetime) -> str:
    seconds = max(0, int((moment - now).total_seconds()))
    days, rest = divmod(seconds, 86400)
    hours, rest = divmod(rest, 3600)
    minutes = rest // 60
    return f"{days}d{hours:02d}h" if days else f"{hours}h{minutes:02d}m"


def describe(label: str, used: float, reset: datetime | None, now: datetime) -> str:
    when = "no active window (starts on first use)"
    if reset is not None:
        when = f"resets {reset.astimezone():%a %d %b %H:%M} (in {until(reset, now)})"
    return f"    {label:<14} {used:5.1f}% used  {max(0.0, 100 - used):5.1f}% left  {when}"


def is_expiring(label: str, used: float, reset: datetime | None, now: datetime) -> bool:
    return "7d" in label.split() and reset is not None and used < 100 and reset - now <= EXPIRING_WITHIN


def report(account: dict, usage: dict, now: datetime) -> list[str]:
    """Print one account and return its expiring windows."""
    windows = claude_windows(usage) if account.get("provider") == "claude" else codex_windows(usage)
    if not windows:
        print("    no usage windows in the response")
    expiring = []
    for label, used, reset in windows:
        print(describe(label, used, reset, now))
        if is_expiring(label, used, reset, now):
            expiring.append(f"{account.get('name')} {label}: {100 - used:.0f}% left, resets in {until(reset, now)}")
    return expiring


def main() -> int:
    base_url = os.environ.get("URL", "https://hara.local").rstrip("/")
    key = os.environ.get("MGMT_KEY", "")
    if not key:
        print("error: MGMT_KEY is not set (run it through `make quota`)", file=sys.stderr)
        return 1

    accounts = management(base_url, key, "GET", "auth-files").get("files", [])
    now = datetime.now(timezone.utc)
    expiring = []
    for account in accounts:
        print(f"{account.get('provider', '?'):<8} {account.get('name')}")
        cooldown = parse_time(account.get("next_retry_after"))
        if account.get("disabled"):
            print("    disabled in the proxy")
        elif cooldown and cooldown > now:
            print(f"    proxy cooldown until {cooldown.astimezone():%a %d %b %H:%M} (in {until(cooldown, now)})")

        payload = usage_request(account)
        if payload is None:
            print("    usage not available for this provider")
            continue
        try:
            result = management(base_url, key, "POST", "api-call", payload)
        except urllib.error.HTTPError as error:
            print(f"    usage request failed: HTTP {error.code} from the proxy")
            continue
        if result.get("status_code") != 200:
            print(f"    usage request failed: HTTP {result.get('status_code')} from {payload['url']}")
            continue
        expiring += report(account, json.loads(result.get("body") or "{}"), now)

    if expiring:
        print(f"\nExpiring unused within {int(EXPIRING_WITHIN.total_seconds() // 3600)}h:")
        for line in expiring:
            print(f"  {line}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
