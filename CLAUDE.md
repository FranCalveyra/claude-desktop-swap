# claude-desktop-swap

A CLI tool to switch between multiple Claude Desktop accounts without logging out.

## Project Context

Claude Desktop is an Electron/Chromium app. Its app-data directory holds the
full signed-in state, not just cookies: **Cookies** (SQLite, Chromium-encrypted
via OS keychain), **Local Storage**/**IndexedDB**/**Session Storage** (LevelDB),
and `config.json` (OAuth token cache, `lastKnownAccountUuid`, and other
account-scoped keys mixed with device/UI preferences).

A profile is a **denylist-filtered mirror of the whole app-data tree**, not an
allowlist of known account files. Two small denylists exclude cache/runtime
junk (`Cache`, `Crashpad`, ...) and machine/work state that must never travel
between accounts (`ant-did`, MCP config, worktree state, ...); everything else
is captured by default, so state Anthropic adds later is captured too instead
of silently leaking across accounts (the recurring "sign in again" bug this
replaced). See `cacheDenylist`/`workDenylist` in `internal/profile/store.go`.

Session expiry is a **separate axis from `Health`**, deliberately. `Health`
answers "does this profile work at all?"; `Inspection.ExpiresAt` answers "until
when?". A session expiring in two days is fully usable, so `Renewal`
(`ok`/`soon`/`expired`/`unknown`, 7-day window) is never folded into the
`Health` enum — several `if Health != HealthUsable { abort }` guards in
`save`/`use` would start refusing valid profiles. Two expiry signals exist and
`renewalFor` reconciles them: the local `expires_utc`, and the server's 401
that `enrichLiveAccounts` folds into `HealthExpired`. The server verdict wins.
`Meta.SessionExpiresAt` is never persisted (`json:"-"`) because the sliding
token would make a saved value stale, and the active profile's expiry is read
from live app-data rather than its frozen snapshot.

Account identity for `status`/`list`/picker highlighting is
`config.json`'s `lastKnownAccountUuid`, not the Cookies digest — the
`sessionKey` cookie is a sliding ~28-day token the app rotates on its own, so
a live digest routinely diverges from what was last saved even for the
correct account.

## Language & Stack

- **Primary**: Go
- **Fallback**: Python (if required libraries are unavailable in Go)
- No unnecessary abstractions. If it works in 50 lines, don't write 200.

## Architecture

- CLI-first, single binary
- Profiles stored in `~/.claude-swap/profiles/<name>/` as a denylist-filtered mirror of the app-data tree
- No daemon, no background process — swap is a one-shot operation
- Cross-platform paths resolved at runtime (macOS / Windows)

## Commands (planned)

```
claude-desktop-swap save <name>        # snapshot current session as a named profile
claude-desktop-swap use <name>         # switch to a saved profile (kills + restarts Claude)
claude-desktop-swap list               # list saved profiles
claude-desktop-swap delete <name>      # remove a profile
claude-desktop-swap status             # show which profile is active (if trackable)
```

## Rules for Claude

- **Never run `go build` or `go run` after edits** — user runs builds manually.
- Use `bat`, `rg`, `fd`, `sd`, `eza` for file operations in shell commands. Never `cat`, `grep`, `find`, `sed`.
- Use conventional commits. No AI attribution in commit messages.
- Default to short, direct answers. Ask one question at a time and wait.
- No defensive error handling for impossible cases. Trust the OS and stdlib.
- No comments unless the WHY is non-obvious (hidden constraint, workaround, subtle invariant).
- No docstrings or multi-line comment blocks.
- Prefer editing existing files over creating new ones.
- Never add features beyond what's currently scoped.

## OS Path Conventions

| OS      | Claude app data path                                      |
|---------|-----------------------------------------------------------|
| macOS   | `~/Library/Application Support/Claude/`                   |
| Windows (Store/MSIX) | `%LOCALAPPDATA%\Packages\Claude_pzs8sxrjxfjjc\LocalCache\Roaming\Claude\` |
| Windows (standalone) | `%APPDATA%\Claude\`                            |

On Windows the Store build is MSIX-packaged, and `%APPDATA%\Claude` is **not**
the app-data directory — it is the container's projection of the package's
private store, which exists only while the packaged app is running. `save` and
`use` stop Claude before reading, so they would find nothing there. Resolution
probes the package store first and falls back to `%APPDATA%\Claude` for the
unpackaged standalone installer. Verify with the app *stopped*: a check that
passes while it runs proves nothing.

Three more Windows invariants, all non-obvious and all load-bearing:

- The cookie database is at **`Network\Cookies`**, not the top-level `Cookies`
  macOS uses. `profile.CookiesPath` probes for it and only falls back to the
  legacy path on a genuine `ErrNotExist` — any other stat error must keep the
  relocated path, or a locked/ACL-blocked file gets misreported as "missing".
- Claude Code ships its own **`claude.exe` inside the app-data tree**, so
  matching processes by image name would kill the user's CLI sessions.
  `claudeProcesses` filters by executable path, and skips processes whose path
  cannot be read rather than assuming they are Claude Desktop's.
- Claude holds the live `Cookies` file open **denying every sharing mode**, so
  it cannot be read or even copied while the app runs. `Inspection.Locked`
  carries that state; `MatchLive` treats it as evidence of a live session
  (identity still comes from the readable `config.json`) instead of failing.

## Cookie Encryption

Chromium encrypts cookie values using the OS keychain. On the **same machine**, all profiles share the same encryption key — so encrypted blobs can be copied verbatim between profile snapshots without decryption. Capture and restore work at the level of whole files/directories (denylist-filtered copy in, mirror copy out) — nothing in that path parses or decrypts `Cookies`, `config.json`, or any other captured file. The **swap path never decrypts anything; always work with raw encrypted blobs and opaque files.**

**Exception — account info (`internal/account`):** the `list`/picker account-info feature and the post-restore verification in `use` decrypt the `sessionKey` in memory to call the claude.ai API (for email/plan, and to detect a server-rejected session). This is the only place decryption is allowed. It is transient (never written to disk), only the resulting email/plan/rejection status are persisted, and the raw session is never stored.

This exception is **macOS-only**: the decryptor reads the key from the macOS
keychain, and `internal/account` is built behind `//go:build darwin` with a
no-op fallback elsewhere. Windows would need DPAPI (`Local State` carries a
`DPAPI`-prefixed `os_crypt.encrypted_key`), which is not implemented — so on
Windows `list` shows no email/plan and `use` skips the server check.

## Security Notes

- Never log or print cookie values.
- Profile directories should be created with `0700` permissions.
- Never store decrypted session data anywhere (email/plan derived from the account API is not session data and may be cached in `meta.json`).
- The account-info feature is the only component that decrypts a cookie value and the only one that makes network calls. `save`, `list`'s own file operations, and the restore/mirror mechanics in `use` stay fully local — but `use` is **not** 100% offline end-to-end: after restoring, it calls the claude.ai API (macOS only) to confirm the server accepts the restored session, and fails clearly instead of launching a session the server will reject. This call is best-effort and silently skipped off macOS or when offline; only an explicit 401 blocks the switch.

## Testing

- Unit-test file path resolution and profile CRUD logic.
- SQLite operations should be tested with a temp DB, not the real Claude data.
- Never touch `~/Library/Application Support/Claude/` in tests.
