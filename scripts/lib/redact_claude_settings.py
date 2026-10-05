"""Copy Claude Code settings.json, replacing autoMode.environment with a placeholder.

Usage: redact_claude_settings.py SOURCE DEST
"""
import json
import sys


def redact(settings: dict) -> dict:
    environment = settings.get("autoMode", {}).get("environment")
    if environment:
        settings["autoMode"]["environment"] = [
            f"<redacted: {len(environment)} Omniyat-specific auto mode environment lines;"
            " restore them from ~/.claude/settings.json>"
        ]
    return settings


def main(source: str, dest: str) -> None:
    with open(source) as f:
        settings = redact(json.load(f))
    with open(dest, "w") as f:
        json.dump(settings, f, indent=2, ensure_ascii=False)
        f.write("\n")


if __name__ == "__main__":
    main(*sys.argv[1:3])
