"""Initialize private state and launch containerized Hara tools."""

import json
import os
from pathlib import Path
import secrets
import sys
import tempfile
import urllib.error
import urllib.request

import bcrypt

STATE = Path(os.environ.get("HARA_STATE", "/state"))
TEMPLATE = Path(os.environ.get("HARA_TEMPLATE", "/template.yaml"))
FIELDS = {"claude-api-key": "API_KEY", "management-password": "MGMT_KEY"}


def private_write(path: Path, value: str) -> None:
    """Atomically replace a private file; never leave a partially written config."""
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd, temporary = tempfile.mkstemp(dir=path.parent)
    try:
        with os.fdopen(fd, "w") as output:
            output.write(value)
        owner = STATE.stat()
        os.chown(temporary, owner.st_uid, owner.st_gid)
        os.replace(temporary, path)
    finally:
        Path(temporary).unlink(missing_ok=True)


def read_key(field: str) -> str:
    if field not in FIELDS:
        raise ValueError("unknown credential field")
    value = (STATE / "secrets" / field).read_text().removesuffix("\n")
    validate_key(value)
    return value


def validate_key(value: str) -> None:
    if not 20 <= len(value.encode()) <= 72 or any(c in value for c in "\r\n\x00"):
        raise ValueError("credentials must be 20–72 UTF-8 bytes without line breaks")


def initialize(mode: str = "init") -> None:
    directory = STATE / "secrets"
    present = [(directory / field).is_file() for field in FIELDS]
    if mode == "import":
        values = [sys.stdin.readline().rstrip("\r\n") for _ in FIELDS]
        if sys.stdin.read():
            raise ValueError("import requires exactly two credential lines")
    elif mode == "rotate":
        if not all(present):
            raise ValueError("initialize or import credentials before rotating")
        values = [secrets.token_hex(32) for _ in FIELDS]
    elif any(present) and not all(present):
        raise ValueError("incomplete credential files; restore both from your private backup")
    elif all(present):
        values = [read_key(field) for field in FIELDS]
    else:
        if (STATE / "proxy" / "config.yaml").exists():
            raise ValueError("existing installation: import its two credentials before starting; see the migration guide")
        values = [secrets.token_hex(32) for _ in FIELDS]
    for value in values:
        validate_key(value)
    template = TEMPLATE.read_text()
    if template.count("__API_KEY__") != 1 or template.count("__MANAGEMENT_PASSWORD_BCRYPT__") != 1:
        raise ValueError("configuration template must contain both credential placeholders exactly once")
    hashed = bcrypt.hashpw(values[1].encode(), bcrypt.gensalt(rounds=10, prefix=b"2a")).decode()
    config = template.replace('"__API_KEY__"', json.dumps(values[0]))
    config = config.replace('"__MANAGEMENT_PASSWORD_BCRYPT__"', json.dumps(hashed))
    if "__API_KEY__" in config or "__MANAGEMENT_PASSWORD_BCRYPT__" in config:
        raise ValueError("credential placeholders must be quoted")
    STATE.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(STATE, 0o700)
    for name in ("secrets", "proxy", "tailscale"):
        path = STATE / name
        path.mkdir(exist_ok=True, mode=0o700)
        owner = STATE.stat()
        os.chown(path, owner.st_uid, owner.st_gid)
        os.chmod(path, 0o700)
    for field, value in zip(FIELDS, values):
        private_write(directory / field, value + "\n")
    private_write(STATE / "proxy" / "config.yaml", config)
    print("Private configuration ready. Existing credentials and provider logins are preserved unless import or rotate was requested.")


def request(path: str, field: str | None = None) -> bytes:
    url = os.environ.get("URL", "http://proxy:8317").rstrip("/") + path
    headers = {"Authorization": "Bearer " + read_key(field)} if field else {}
    with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=10) as response:
        return response.read()


def status() -> None:
    # Compose reports container state on the host; no Docker socket is mounted here.
    request("/healthz")
    print("✓ proxy health")
    request("/management.html")
    print("✓ management panel")
    models = json.loads(request("/v1/models", "claude-api-key"))
    print(f"✓ client key accepted, {len(models.get('data', []))} models")
    accounts = json.loads(request("/v0/management/auth-files", "management-password"))
    count = len(accounts.get("files", []))
    print(f"✓ management key accepted, {count} provider accounts" if count else "! no provider accounts; connect one in the management panel")


def main(args: list[str]) -> None:
    command, *rest = args or ["status"]
    if command in ("init", "import", "rotate"):
        initialize(command)
    elif command == "python-tests":
        os.execvp("python3", ["python3", "-m", "unittest", "discover", "-s", "/usr/local/lib/hara", "-p", "test_*.py", "-v"])
    elif command == "key":
        if len(rest) != 1:
            raise ValueError("usage: key claude-api-key|management-password")
        print(read_key(rest[0]))
    elif command == "status":
        status()
    elif command == "health":
        request("/healthz")
    elif command in ("ops", "quota", "wssmoke", "bench"):
        if command == "ops":
            if not rest or rest[0] == "status":
                raise ValueError("use the status command for containerized health checks")
            if "-url" not in rest:
                rest = [rest[0], "-url", os.environ.get("URL", "http://proxy:8317"), *rest[1:]]
        fields = ["management-password"] if command == "quota" else list(FIELDS) if command == "ops" else ["claude-api-key"]
        for field in fields:
            os.environ[FIELDS[field]] = read_key(field)
        os.execv("/usr/local/bin/" + command, [command, *rest])
    else:
        raise ValueError("unknown tool; choose init, import, rotate, key, status, health, ops, quota, wssmoke or bench")


if __name__ == "__main__":
    try:
        main(sys.argv[1:])
    except (OSError, ValueError, urllib.error.URLError) as error:
        # Never include HTTP response bodies or credential values in failures.
        print(f"error: {error}", file=sys.stderr)
        sys.exit(1)
