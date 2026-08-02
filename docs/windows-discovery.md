# Windows Findings

What Claude Desktop actually does on Windows, measured against Claude Desktop
`1.24012.9.0` (Microsoft Store / MSIX build) on Windows 11. This replaces the
pre-implementation checklist; it is a record, not a plan.

## 1. App data is not where it appears to be

`%APPDATA%\Claude` is **not** the app-data directory on the Store build. Every
entry under it — including the directory itself — carries a `Target` pointing
into the package's private store:

```
C:\Users\<user>\AppData\Local\Packages\Claude_pzs8sxrjxfjjc\LocalCache\Roaming\Claude
```

That projection exists **only while the packaged app is running**. Stop Claude
and `%APPDATA%\Claude\Network` disappears entirely.

This is the trap that cost the most time. Any check run with Claude *running*
confirms the wrong answer:

```powershell
Test-Path "$env:APPDATA\Claude\Network\Cookies"   # True while running, False when stopped
```

`save` and `use` both stop Claude before reading, so they operate in exactly
the state where the projection is gone. `AppDataPath` probes the package store
first and falls back to `%APPDATA%\Claude` for the unpackaged standalone
installer.

**Anything verifying a Windows path must be verified with Claude stopped.**

## 2. Cookies moved under `Network\`

The database is `Network\Cookies`; there is no top-level `Cookies`. Only a
zero-byte `Network\Cookies-journal` accompanies it — no WAL, no `-shm`.
`profile.CookiesPath` probes for the relocated file and falls back to the
legacy top-level path **only** on `ErrNotExist`, so a locked or ACL-blocked
file is not misreported as missing.

## 3. `claude.exe` is an ambiguous image name

Claude Code installs its own `claude.exe` inside the app-data tree
(`...\Claude\claude-code\<version>\claude.exe`). Killing by image name would
take out the user's running CLI sessions along with the desktop app.
`claudeProcesses` enumerates via `CreateToolhelp32Snapshot`, resolves each
full path with `QueryFullProcessImageName`, and excludes anything under
app-data. Processes whose path cannot be read are skipped rather than assumed.

The desktop app runs as one root process plus ~11 children.

## 4. The live cookie database is exclusively locked

While Claude runs, `Network\Cookies` cannot be opened under **any** sharing
mode, nor copied:

```
share=Read FAIL   share=Write FAIL   share=ReadWrite FAIL
share=None FAIL   share=Delete FAIL  Copy-Item FAIL
```

`config.json`, `Local State`, and `Preferences` stay readable. Hence
`Inspection.Locked`: `MatchLive` treats a lock as evidence of a live session
and identifies the account from `config.json`, rather than reporting `unknown`
for as long as the app is open. Session *expiry* genuinely cannot be read in
that state and is reported as `-`.

## 5. Launching

The Store build installs under a versioned `WindowsApps` directory that is not
listable without elevation, so there is no exe path to resolve. Shell
activation is the only route:

```
shell:AppsFolder\Claude_pzs8sxrjxfjjc!Claude
```

`explorer.exe` exits `1` whether activation succeeded or not, so its status is
useless — `LaunchApp` ignores it and confirms by polling for the process. The
standalone installer, when present, is launched directly from
`%LOCALAPPDATA%\AnthropicClaude\claude.exe`.

Stopping works as expected: `taskkill /PID` posts `WM_CLOSE`, with `/F /T` as
the forced fallback after a 5-second poll.

## 6. Encryption

`Local State` carries a DPAPI-wrapped `os_crypt.encrypted_key` (base64 prefix
`RFBBUEk` = `"DPAPI"`), mirroring the macOS keychain arrangement. Since the
swap path never decrypts anything, this does not affect save/restore.

It is **not implemented**, so account email/plan and the post-restore server
check in `use` remain macOS-only. Whether cookies use v10 (AES-GCM) or
App-Bound v20 was never determined — the database is locked while the app runs
and reading it would require stopping Claude first.

## 7. Denylist

Every pre-existing entry in both denylists exists on Windows and is correctly
classified. Two Windows-only additions: `logs` (diagnostic output) and
`lockfile` (a stale one restored into live app-data leaves Electron's
single-instance check pointing at a dead process).

`Partitions`, `WebStorage`, `Shared Dictionary`, `DIPS`, `InterestGroups`, and
`SharedStorage` are captured by default. They were classified by reasoning
about what they hold, not by isolating each one in a switch test.
