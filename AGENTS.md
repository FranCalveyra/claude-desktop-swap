# claude-desktop-swap

A CLI tool to switch between multiple Claude Desktop accounts without logging out.

> This file is agent-agnostic project guidance. Any AI coding agent (Codex, Claude,
> Cursor, etc.) should follow it. It describes the project, not a specific assistant.

## Project Context

Claude Desktop is an Electron/Chromium app. Sessions live in:
- **Cookies** (`~/Library/Application Support/Claude/Cookies`) — SQLite DB, Chromium-encrypted via OS keychain
- **Local Storage** (`~/Library/Application Support/Claude/Local Storage/leveldb/`) — LevelDB, holds device-trust / elevated-auth state

Switching accounts = swapping the `sessionKey`, `sessionKeyLC`, `routingHint`, `lastActiveOrg`, and related claude.ai cookies in that SQLite DB, plus restoring the account's Local Storage snapshot so the device stays verified.

## Language & Stack

- **Primary**: Go
- **Fallback**: Python (if required libraries are unavailable in Go)
- No unnecessary abstractions. If it works in 50 lines, don't write 200.

## Architecture

- CLI-first, single binary
- Profiles stored in `~/.claude-swap/profiles/<name>/` as snapshots of the Cookies SQLite DB plus the Local Storage leveldb
- No daemon, no background process — swap is a one-shot operation
- Cross-platform paths resolved at runtime (macOS / Windows)

## Commands

```
claude-desktop-swap                    # interactive dashboard (prints help when not in a terminal)
claude-desktop-swap save <name>        # snapshot current session as a named profile
claude-desktop-swap add <name>         # add a new account interactively without manual logout
claude-desktop-swap use [name]         # switch to a saved profile (kills + restarts Claude); no name = interactive picker
claude-desktop-swap list               # list saved profiles
claude-desktop-swap delete <name>      # remove a profile
claude-desktop-swap status [--check]   # show the active profile; --check exits 1/2 when expiring/expired
```

## Rules for AI agents

- **Never run `go build` or `go run` after edits** — the user runs builds manually.
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
| macOS   | `~/Library/Application Support/Claude/`                    |
| Windows | `%APPDATA%\Claude\`                                        |

## Cookie Encryption

Chromium encrypts cookie values using the OS keychain. On the **same machine**, all profiles share the same encryption key — so encrypted blobs can be copied verbatim between profile snapshots without decryption. Never decrypt cookie values; always work with raw encrypted blobs.

## Security Notes

- Never log or print cookie values.
- Profile directories should be created with `0700` permissions.
- Never store decrypted session data anywhere.

## Testing

- Unit-test file path resolution and profile CRUD logic.
- SQLite operations should be tested with a temp DB, not the real Claude data.
- Never touch `~/Library/Application Support/Claude/` in tests.
