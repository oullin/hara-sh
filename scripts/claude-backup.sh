#!/usr/bin/env bash
# Copy the Claude Code config from ~/.claude into claude/ for backup.
# settings.json is copied with autoMode.environment redacted (Omniyat-internal details).
set -euo pipefail
cd "$(dirname "$0")/.."
SRC="$HOME/.claude"
mkdir -p claude
cp -p "$SRC/settings.local.json" "$SRC/CLAUDE.md" "$SRC/.claude.json" claude/
python3 - "$SRC/settings.json" claude/settings.json <<'PY'
import json, sys
src, dst = sys.argv[1], sys.argv[2]
data = json.load(open(src))
env = data.get("autoMode", {}).get("environment")
if env:
    data["autoMode"]["environment"] = [
        f"<redacted: {len(env)} Omniyat-specific auto mode environment lines; restore them from ~/.claude/settings.json>"
    ]
with open(dst, "w") as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write("\n")
PY
echo "backed up ~/.claude config to claude/ (autoMode.environment redacted)"
