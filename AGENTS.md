# AGENTS.md

Instructions for AI coding agents (and people) working in this repository, in
the [AGENTS.md](https://agents.md) format. Keep it current: an agent follows
what it says, including the commands below.

## Project overview

**Goal.** `xdg-agents` makes AI coding agents (Claude Code, Codex and others)
keep their files where the [XDG Base Directory Specification][basedir] says
they belong, instead of in their own dot-folders in the home directory
(`~/.claude`, `~/.codex`, ...).

It does this in three steps, each usable on its own:

1. **Report**: for each agent installed on the machine, show where its files
   are now, what kind of file each one is (configuration, data, state, cache,
   runtime), and where the spec says it should be.
2. **Plan**: show exactly what would move where, and which environment
   variables or settings would point the agent at the new locations.
3. **Apply**: carry out a plan, reversibly, only when asked.

**Who it's for.** People who want a tidy home directory, predictable backups
(back up config and data, skip cache), and dotfile setups that can be shared
between machines.

**Not goals.**

- Not a dotfile manager, and not a sync tool.
- Never changes an agent's own files or code, and never patches its binary.
- Doesn't move anything on its own: "report" and "plan" are read-only, and
  "apply" only runs a plan the user has seen.
- Doesn't decide what an agent's files *mean* by guessing at run time: each
  agent's layout is written down in Nickel (see below) and reviewed.

### What the spec says (basedir 0.8)

| Variable | Holds | Default when unset or empty |
| :-- | :-- | :-- |
| `XDG_CONFIG_HOME` | User configuration | `$HOME/.config` |
| `XDG_DATA_HOME` | User data that should survive | `$HOME/.local/share` |
| `XDG_STATE_HOME` | State that persists between runs but isn't worth backing up (history, logs, recent files) | `$HOME/.local/state` |
| `XDG_CACHE_HOME` | Non-essential, regenerable data | `$HOME/.cache` |
| `XDG_RUNTIME_DIR` | Sockets, locks and other per-session files | No default: if unset, warn and fall back |
| `XDG_CONFIG_DIRS` | System-wide configuration, in order of preference | `/etc/xdg` |
| `XDG_DATA_DIRS` | System-wide data, in order of preference | `/usr/local/share/:/usr/share/` |

User executables go in `$HOME/.local/bin`. A path in any of these variables
that isn't absolute is invalid and must be ignored. Each agent's files go in
a subdirectory named after it, such as `$XDG_CONFIG_HOME/claude/`.

### How it fits together

- **Agent layouts** (`agents/*.ncl`): one Nickel file per agent, saying
  which of its files are configuration, data, state, cache or runtime, and
  how the agent can be pointed elsewhere (an environment variable, a
  setting, or only a symlink). `agents/schema.ncl` holds the contract every
  layout must satisfy, so a mistake in one is reported before the tool runs.
  Every fact in a layout comes from the agent's documentation (listed in its
  `sources`) or is marked `documented = false`.
- **The tool** (`main.go` for now): resolves the XDG variables as the table
  above says, evaluates the layouts through [go-nickel][go-nickel], and
  reports, plans or applies.
- **Decisions** are recorded in `docs/adr/`.
- **What `status` shows** and how it decides each file's state is proposed
  in `docs/status.md`.
- **Open decisions** are numbered in `docs/decisions.md`, the one place
  questions for the owner go. Don't leave a question only in a chat: add it
  there, and record the answer there when it comes.
- **Open facts** (unknown files seen on disk, unconfirmed behaviour) are
  tracked in `docs/follow-ups.md`. Settle one by moving it into a layout or
  an ADR, and delete it there in the same change.

### Current state

- `agents/claude-code.ncl`: Claude Code's layout, written from its docs.
- `agents/claude-desktop.ncl`: the Claude desktop app's layout on Linux,
  written from a listing of a real `~/.config/Claude` (the folder isn't
  documented). Its unknown files are left out on purpose, to be reported as
  unclassified.
- `main.go` loads every layout in `agents/` through go-nickel (which runs
  their contracts) and prints a line per layout. `status`, `plan` and
  `apply` are still to be written on top of it.

### First "done" check

The first slice is finished when this works on a machine with Claude Code
installed:

```sh
./xdg-agents status
```

and prints, for Claude Code only: each file or folder it uses now, its XDG
category, its target location, and how Claude Code can be pointed there
(for example the `CLAUDE_CONFIG_DIR` environment variable, if it covers that
file). It is read-only and needs no flags. A test covers the XDG variable
rules in the table above, including empty and relative values.

Work that doesn't move this check, or a later one written here, closer to
passing is drift: cut it, or update this section on purpose first.

## Setup

Enter the development shell with `nix develop` (or `direnv allow`). Without
flakes enabled, add `--extra-experimental-features 'nix-command flakes'`. The
shell provides Go, gotools, golangci-lint, Nickel and its language server
(`nls`), and a C compiler: go-nickel links Nickel's interpreter through cgo,
so `CGO_ENABLED=1` is set.

## Build and test

From the development shell:

```sh
go build -o xdg-agents .    # build
./xdg-agents                # run (loads agents/*.ncl)
go test ./...               # must pass before a change is done
go vet ./...                # must pass
golangci-lint run           # must pass
nickel typecheck agents/claude-code.ncl  # and any other .ncl file you change
nickel export agents/claude-code.ncl     # also runs the contracts
nix flake check             # when flake.nix changes
```

### Keeping layouts current

`.github/workflows/docs-drift.yml` runs `tools/docsdrift` every Monday (and
on pull requests that touch `agents/`). It fetches Claude Code's
`claude-directory` and `env-vars` pages and its settings schema from
SchemaStore, and fails when they list a path, path-like variable or
path-like setting (a top-level setting named `...Directory`, `...Path` and
so on) that `agents/claude-code.ncl` doesn't cover, or stop listing one it
relies on. To run it locally:

```sh
curl -sSfLO https://code.claude.com/docs/en/claude-directory.md
curl -sSfLO https://code.claude.com/docs/en/env-vars.md
curl -sSfLO https://www.schemastore.org/claude-code-settings.json
nickel export agents/claude-code.ncl > layout.json
go run ./tools/docsdrift -layout layout.json \
  -directory-doc claude-directory.md -env-doc env-vars.md \
  -settings-schema claude-code-settings.json
```

When it reports drift, read the docs' own description of each item, then
add an entry (with its category), mark it `legacy`, or list it under
`untracked` / `untracked_env` / `untracked_settings` with a reason. Don't guess a category from the
name. Exit status 2 means a page's format changed: fix the extraction in
`tools/docsdrift/drift.go` rather than lowering its minimums.

## Code style

- `gofmt` (or `goimports`) on Go, `nickel format` on Nickel, `nix fmt` on Nix.
- Comments say why, not what.
- Anything that touches the file system outside a test's temporary directory
  must be behind the "apply" step and must be reversible.

## Commits and pull requests

- One logical change per commit, with a short imperative subject line.
- Record a decision that changes the design as an ADR in `docs/adr/`,
  numbered in order, before or with the change.

## Security

- Agents' folders hold credentials (API keys, OAuth tokens). Never print,
  copy or log file *contents* from them; the tool deals in paths only.
- Tests must use temporary directories and a fake `HOME`, never the real one.
- Don't run `apply` against a real home directory while developing.

[basedir]: https://specifications.freedesktop.org/basedir-spec/latest/
[go-nickel]: https://github.com/nickel-lang/go-nickel
