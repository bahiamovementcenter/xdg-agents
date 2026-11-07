# Follow-ups

Files and folders seen on disk that no layout defines yet, facts that are
still unconfirmed, and decisions still to make. When an item is settled, move
it into the layout (or an ADR) in the same change and delete it here.

Every item says where it was seen. Classify from documentation or from what
the agent demonstrably does with the file (a probe, as in
`agents/claude-desktop.ncl`), never from its name alone. Probes list names,
types and key names only, never contents.

Where seen:

- **hornicorn**: the owner's Linux machine, Claude desktop app with Claude
  Code 2.1.280, 2026-11-07
- **cloud**: a Claude Code cloud session container, 2026-11-07

## Claude Code (`~/.claude`)

Not in the docs, so `tools/docsdrift` can't catch them.

| Path | Seen | What we know | To settle it |
| :-- | :-- | :-- | :-- |
| `bridge-spawn/` | hornicorn | One folder per spawned session (`cse_…`), holding `append-system-prompt.txt` | Find what creates and removes it (Remote Control? cloud sessions?); likely state or runtime |
| `state/` | hornicorn, cloud | Holds `mcp-discover-verdicts.json` | Confirm it's the MCP discovery cache's state, then classify (state or cache) |
| `sessions/*.key` | hornicorn | One per running session, next to `sessions/<pid>.json` | Check it's removed on exit like `sessions/`. Treat as sensitive until known. |
| `hooks/` | (docs example only) | In the layout as `documented = false`: the docs use it in examples, but hooks can live anywhere | Nothing to do unless the docs make it a real location |

Inside folders the layout already covers, so they move with their parent.
Listed only so `status` doesn't report them as unknown:
`cache/model-catalog/`, `projects/*/bridge-pointer.json`,
`projects/*/<session>/custom-title.json`, `plugins/cache/**/.in_use/<pid>`.

Not Claude Code's own files. Decide how `status` labels them (see Decisions):

| Path | Seen | Source |
| :-- | :-- | :-- |
| `.ponytail-active`, `ponytail-modes/`, `ponytail-statusline.sh` | hornicorn | The `ponytail` plugin |
| `environment-manager` (symlink to `/opt/env-runner`), `launcher-settings.json`, `codesign-mcp-config.json` | cloud | The cloud session runner |
| `stop-hook-git-check.sh`, `user-prompt-submit-reply-reminder.py`, `session-start-setup-script-failure.sh` | cloud | The cloud session runner's hooks |

## Claude desktop app (`~/.config/Claude`, Linux)

Left out of `agents/claude-desktop.ncl` on purpose: purpose unknown.

| Path | Seen | Notes |
| :-- | :-- | :-- |
| `buddy-tokens.json` | hornicorn | Changed during the probe. Sounds like credentials: treat as sensitive. |
| `ant-did`, `ant-device-registry.json` | hornicorn | Probably a device identifier and registry. Sensitive until known. |
| `ccd-ids.json` | hornicorn | "ccd" also prefixes the environment file |
| `config.json` | hornicorn | Separate from `claude_desktop_config.json` |
| `plan-usage-history.json` | hornicorn | Changed during the probe |
| `cowork-enabled-cli-ops.json` | hornicorn | |
| `os-entry-points.json` | hornicorn | |
| `extensions-blocklist.json` | hornicorn | Possibly fetched, and so a cache |
| `chrome-import-sites.json` | hornicorn | |
| `ca-bundle.pem` | hornicorn | Possibly a copy of system CAs (cache) or a custom bundle (config) |
| `fcache` | hornicorn | |
| `git-shadow/` | hornicorn | One subfolder per hash. Possibly checkpoint snapshots. |
| `spaces-present/` | hornicorn | One subfolder per account |

Chromium side files, such as `Cookies-journal`, `DIPS-wal` and
`Trust Tokens-journal`, belong to the file they're named after. `status`
should group them with it rather than list them separately.

## Settings that move files

Found through SchemaStore's schema for Claude Code's `settings.json`, then
checked against the settings reference. Both are in
`agents/claude-code.ncl` under `settings`.

- **`autoMemoryDirectory`** can move auto memory out of
  `projects/*/memory` into `$XDG_DATA_HOME`, but it names *one* folder.
  Set in user settings, every project shares a single memory folder. Decide
  whether `plan` should offer it per project (`.claude/settings.local.json`
  in each), only mention it, or skip it.
- **`plansDirectory`** is relative to the project root, so it can't reach
  an XDG folder. `plans/` keeps moving by symlink.
- **`sshConfigs`** in `~/.claude/settings.json` defines SSH connections for
  the desktop app's environment picker: a second, documented place where
  desktop SSH environments can live.

## Other projects

- **[xdg-ninja](https://github.com/b3nj5m1n/xdg-ninja)** (MIT) checks the
  top level of `$HOME` against a database of 655 programs. Its
  `programs/Claude Code.json` (checked 2026-11-07, upstream commit
  `ebf481a`) says `~/.claude.json` can't be moved. That's out of date: the
  claude-directory docs say `CLAUDE_CONFIG_DIR` moves it too. To do: draft
  an upstream fix for the owner to submit. Later, a `tools/` exporter could
  turn each layout's roots and variables into xdg-ninja entries.
- **[SchemaStore](https://www.schemastore.org)** has settings schemas for
  other agents, a starting point for their layouts. Codex has `log_dir` and
  `sqlite_home` (or `CODEX_SQLITE_HOME`) besides `CODEX_HOME`. Gemini CLI
  has `general.plan.directory` and keeps binaries in `~/.gemini/bin`. Qwen
  Code, opencode, Aider and Cursor have schemas too.
- **The settings check relies on SchemaStore**, which isn't Anthropic's
  documentation. If the schema falls behind the settings reference,
  `docsdrift` misses new settings. A check of the reference itself would
  close that gap.

## Unconfirmed facts

- **Saved environments' names and folders:** most likely in
  `~/.config/Claude/Local Storage` (the only store besides logs and session
  records where the probe's name appeared). Confirming means reading
  Chromium's database, so it stays "most likely" unless the docs say.
- **`coworkUserFilesPath`** in `claude_desktop_config.json` probably sets
  where `~/Claude` goes. If so, that folder can be moved by a setting.
- **`~/Claude/`** (artifacts and scheduled-task output, per Anthropic's data
  storage page) isn't in any layout. It sits directly in the home directory.
- **Does the desktop app honour `XDG_CONFIG_HOME`?** Electron apps usually
  do on Linux, which would make the variable a root `env` (moving the whole
  folder). Not tested.
- **macOS and Windows paths** for the desktop app:
  `~/Library/Application Support/Claude` on macOS; on Windows
  `%LOCALAPPDATA%\Claude-Data\` since 3.1623.0, according to
  [anthropics/claude-code#103860](https://github.com/anthropics/claude-code/issues/103860).

## Decisions

- **macOS scope.** `~/Library/Application Support` is the platform's own
  convention. Should `xdg-agents` only report there, or also plan and apply?
  Needs an ADR before any macOS layout.
- **Files that aren't the agent's own** (plugins, the cloud runner): should
  `status` list them separately, list them as unknown, or skip them? Each
  layout could name known foreign files with an `untracked` reason.
