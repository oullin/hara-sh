#!/usr/bin/env python3
"""Check publication candidates without printing private matching values."""

import argparse
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parent.parent
PATTERNS = {
    "personal absolute path": re.compile(r"/(?:Users|home)/[\w.-]+/"),
    "literal infrastructure account ID": re.compile(
        r"(?:accountId|account_id)[\"']?\s*[:=]\s*[\"'][a-f0-9]{32}[\"']"
    ),
    "private key": re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"),
    "provider credential": re.compile(
        r"\b(?:sk-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9]{20,}|AKIA[A-Z0-9]{16})\b"
    ),
}


def git(*args: str) -> bytes:
    return subprocess.check_output(["git", *args], cwd=ROOT)


def private_file(path: str) -> bool:
    return (
        path.startswith(("claude/", ".claude/"))
        or (path.startswith("codex/") and path != "codex/proxy.config.toml")
        or Path(path).name in {".env", ".dev.vars", "auth.json", ".claude.json"}
    )


def findings(path: str, content: bytes) -> list[str]:
    issues = ["personal client state or credential file"] if private_file(path) else []
    text = content.decode("utf-8", errors="replace")
    for name, pattern in PATTERNS.items():
        lines = [str(i) for i, line in enumerate(text.splitlines(), 1) if pattern.search(line)]
        if lines:
            issues.append(f"{name} at line(s) {', '.join(lines)}")
    return issues


def current_files() -> bool:
    paths = set(git("ls-files", "--cached", "--others", "--exclude-standard", "-z").decode().split("\0"))
    failed = False
    reviewed = 0
    for path in sorted(paths):
        file = ROOT / path
        if not path or not file.is_file():
            continue
        reviewed += 1
        for issue in findings(path, file.read_bytes()):
            print(f"{path}: {issue}")
            failed = True
    print(f"Reviewed {reviewed} current publication candidates; {'blocked' if failed else 'no guard findings'}.")
    return failed


def history_files() -> bool:
    # Inspect unique historical blobs, including files deleted from the current tree.
    objects = git("rev-list", "--objects", "--all").decode().splitlines()
    failed = False
    reviewed = 0
    for entry in objects:
        oid, separator, path = entry.partition(" ")
        if not separator or git("cat-file", "-t", oid).strip() != b"blob":
            continue
        reviewed += 1
        for issue in findings(path, git("cat-file", "blob", oid)):
            print(f"history {oid[:12]} {path}: {issue}")
            failed = True
    print(f"Reviewed {reviewed} historical blobs; {'publication blocked' if failed else 'no guard findings'}.")
    return failed


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--history", action="store_true", help="also inspect all locally available Git refs")
    args = parser.parse_args()
    failed = current_files()
    if args.history:
        failed = history_files() or failed
    return int(failed)


if __name__ == "__main__":
    sys.exit(main())
