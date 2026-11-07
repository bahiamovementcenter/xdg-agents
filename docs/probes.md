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
  3. a to-do list. If no to-do tool is offered but a tool search is (Claude
     Code's `ToolSearch` loads its task tools only on request), it searches
     for one first, and skips the step if the search brings none in.

  It then ends with a text naming the tools it was offered and called. It
  works out where it is from the tools already called, not by counting
  turns, because Claude Code merges consecutive assistant turns and a
  search changes the tools on offer. Its
  log records only which tools were offered and called, never messages.
- **`tools/probe`** runs the agent twice in throwaway folders, with a clean
  environment pointing it at the fake model. Each run gets its own `HOME`,
  separate XDG directories, `TMPDIR` and project folder:
  - once with the agent's defaults;
  - once with its config variable set (`CLAUDE_CONFIG_DIR`).

  It then lists names, types and symlink targets, never contents, and
  checks the layout's claims against them.
- **A hook reports the transcript.** Each run passes the agent a settings
  file, with `--settings`, from outside the agent's own folders, so nothing
  is seeded into them. It sets up a hook on `SessionStart`, `PostToolUse`
  and `Stop` that runs the probe itself in `-record-hook` mode. That mode
  writes down only the event name and the `transcript_path` the agent
  passes, never the rest of the payload, and it always exits 0. So the
  agent itself says where its transcript is, and the probe checks that the
  file exists and that the layout covers it.

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
| Both runs finish, using the fake model's tools | ok: 4 tool calls each (`Bash`, `Write`, `ToolSearch`, `TaskCreate`) |
| By default, files go to `~/.claude` and `~/.claude.json` | ok |
| With `CLAUDE_CONFIG_DIR` set, nothing goes to `~/.claude` | ok |
| With `CLAUDE_CONFIG_DIR` set, `.claude.json` moves into it | ok, as the docs say |
| Anything in `~/.claude` the layout doesn't cover | none |
| Where the agent says its transcript is (hook `transcript_path`) | `~/.claude/projects/<project>/<session>.jsonl`; with `CLAUDE_CONFIG_DIR` set, the same path inside it. The file exists both times. |
| In `$XDG_CONFIG_HOME`, `$XDG_DATA_HOME`, `$XDG_STATE_HOME`, `$XDG_CACHE_HOME` | nothing |
| In `$XDG_RUNTIME_DIR` | **`cc-socks`**: undocumented, not in the layout |

On a first run, `~/.claude` gets `backups/`, `plugins/`, `projects/`,
`session-env/`, `sessions/`, `shell-snapshots/` and `tasks/`, and the temp
folder gets `$TMPDIR/claude-0/`, as the layout says.

**Gaps** (in `docs/follow-ups.md`):

- **One kind of session.** The probe runs `claude -p` once. Files only
  other features create (plans, checkpoints after edits, debug logs)
  need more script steps or flags.
