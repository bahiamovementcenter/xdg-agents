# Probes

A probe runs an agent through a real session and records what it leaves on
disk, to check a layout against what the agent actually does (decisions 24
and 25 in `docs/decisions.md`).

- **`internal/fakemodel`** is a stand-in for the model. It speaks the
  Anthropic Messages API (streaming or not), needs no login, and gives the
  same replies every time. It follows a fixed script, calling tools that
  the agent offers, matched by purpose:
  1. a harmless shell command (`echo probe`);
  2. writing `probe.txt` in the probe's project folder;
  3. a to-do list.

  It then ends with a text naming the tools it was offered and called. Its
  log records only which tools were offered and called, never messages.
- **`tools/probe`** runs the agent twice in throwaway folders, with a clean
  environment pointing it at the fake model. Each run gets its own `HOME`,
  separate XDG directories, `TMPDIR` and project folder:
  - once with the agent's defaults;
  - once with its config variable set (`CLAUDE_CONFIG_DIR`).

  It then lists names, types and symlink targets, never contents, and
  checks the layout's claims against them.

## Running it

From the development shell, with the agent installed:

```sh
nickel export agents/claude-code.ncl > layout.json
go run ./tools/probe -layout layout.json -out probe.json
```

The checks print as it finishes, and `probe.json` has every file found. Use
`-keep` to keep the throwaway folders for a closer look.

Only Claude Code is supported so far. Running probes inside a Nix build,
with agents from llm-agents.nix, is the next step.

## First results

Claude Code 2.1.319, 2026-11-07, in a Claude Code cloud session.

| Check | Result |
| :-- | :-- |
| Both runs finish, using the fake model's tools | ok: 2 tool calls each (shell, write) |
| By default, files go to `~/.claude` and `~/.claude.json` | ok |
| With `CLAUDE_CONFIG_DIR` set, nothing goes to `~/.claude` | ok |
| With `CLAUDE_CONFIG_DIR` set, `.claude.json` moves into it | ok, as the docs say |
| Anything in `~/.claude` the layout doesn't cover | none |
| In `$XDG_CONFIG_HOME`, `$XDG_DATA_HOME`, `$XDG_STATE_HOME`, `$XDG_CACHE_HOME` | nothing |
| In `$XDG_RUNTIME_DIR` | **`cc-socks`**: undocumented, not in the layout |

On a first run, `~/.claude` gets `backups/`, `plugins/`, `projects/`,
`session-env/`, `sessions/` and `shell-snapshots/`, and the temp folder gets
`$TMPDIR/claude-0/`, as the layout says.

**Gaps** (in `docs/follow-ups.md`):

- **The to-do step didn't run.** Claude Code offers its to-do and task tools
  only through `ToolSearch`, so the script needs a step that calls
  `ToolSearch` first. Until then, `tasks/` isn't exercised.
- **One kind of session.** The probe runs `claude -p` once. Files only
  other features create (plans, checkpoints after edits, debug logs)
  need more script steps or flags.
