# T3 configuration and recovery

`settings/` holds a reviewed portable snapshot: preferences, keybindings and
provider templates. The Linear and previous Codex palettes live in `themes/`.
The snapshot omits credentials, account labels, machine paths, project mappings,
custom vocabulary and free-form instructions. Configure the provider home
placeholders locally before merging settings; these files are not a drop-in
replacement for T3's entire state directory.

The snapshot binds the desktop server to loopback and keeps Tailscale Serve as
the optional private route for systems where Tailscale is configured. New T3
Connect devices require approval. New threads default to Supervised. Review
permissions separately in existing threads and scheduled tasks. Workflows and
Linked environments are enabled. Workflow definitions, paired devices, and
environment links are configured locally.

`attribution/` keeps agent credit lines out of new pull requests and commits.
See [its README](attribution/README.md).

## Chrome opens a blank or crashed tab from T3

If the expected URL appears in Chrome but the tab stays blank or shows
**Untitled – Crashed**, open another site directly in Chrome. If that tab
crashes too, enter `chrome://restart` in the address bar to restart Chrome.
Then click the original link in T3 again and confirm the page loads in Chrome.
If Chrome still crashes, see [Google's Chrome troubleshooting guide](https://support.google.com/chrome/answer/6098869).

## Encrypted recovery backup

Install `age`, `sqlite3` and Go, then finish running agents and quit T3 Code.
If T3 runs as a background service, stop that service too. The command refuses
to copy a running T3 server or macOS desktop profile and never stops them for you.
If T3 crashed and left a stale `userdata/server-runtime.json` whose process ID now
belongs to another program, the backup recognises it and continues.

Run from the folder where the encrypted archive should be saved:

```sh
/path/to/hara-sh/t3/backup.sh
```

Or supply a destination explicitly:

```sh
./t3/backup.sh "$PWD"
./t3/backup.sh /path/to/private-backups
```

The default asks `age` for a passphrase in your terminal. Keep it separately in
your password manager. For unattended encryption, supply an age public recipient:

```sh
./t3/backup.sh --recipient age1YOUR_PUBLIC_RECIPIENT /path/to/private-backups
```

Keep the corresponding private key separately from the archive. A public
recipient is not a password. `--dry-run` checks the source roots and destination
without making an archive. Flags may come before or after the destination. `make t3 backup BACKUP_DIR=/path/to/private-backups`
uses the same implementation; pass extra flags with `ARGS`.

The archive is named `t3-backup-<UTC timestamp>.tar.gz.age` and is owner-only.
Only encrypted bytes reach the destination; unsuccessful partial files are
removed. Backup filenames are ignored by Git in this checkout.

## Scope and restoration

The backup includes the full T3 home (`~/.t3`, or `T3_HOME`), standard Claude,
Codex and Cursor homes, shared agent settings, custom provider/shadow homes
declared in T3, and the macOS T3 desktop profiles when present. It includes
conversations, attachments, secrets and worktrees, so allow sufficient space.
The T3 home must be inside your home directory. Other sources outside it, such as
a provider home on another volume, are skipped with a warning naming them.
Symlinked source roots include their targets when inside that home. Other symbolic links are preserved;
their external targets, original project repositories, Hara's private proxy
state, portless certificates and macOS Keychain are not copied. Back those up
separately. Some providers will require a fresh login.

SQLite databases use the SQLite backup API via `sqlite3`. Their snapshots exist
briefly in a private temporary directory and are removed afterwards; no plaintext
archive is written. Transient SQLite sidecars and desktop/server lock files are
omitted. Other applications using provider homes should also be closed for a
consistent copy of their ordinary files; the command warns when Claude Code, Codex
or Cursor agent processes are still running. Files they delete during the backup
are skipped with a warning. A file truncated while it is copied stops the backup,
because its archived copy would be incomplete. Do not change configuration during backup.

Decrypt into a private staging directory with `age --decrypt --identity KEY`
(or `age --decrypt` for a passphrase) and extract the resulting tar stream there.
Review `manifest.json` and the `home/` tree before restoring files with all
affected applications stopped. Adjust absolute symlinks and machine-specific
paths on a different computer. This command creates backups; it does not
overwrite your live configuration or automatically restore it.
