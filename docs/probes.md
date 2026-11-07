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
- **`tools/probe`** works for any agent, with nothing written per agent
  (decision 26). From the agent's own `--help` it works out:
  - **how to run it once, non-interactively:** it tries the usual forms
    (`-p`, `exec`, `run`, `--prompt`, then the prompt alone) until one
    reaches the fake model;
  - **how to let it use tools:** an allowed-tools list if offered, otherwise
    the usual "don't ask" flags;
  - **which variables might move its folders:** any the layout names, any
    `*_HOME` or `*_CONFIG_DIR` in the help, and ones derived from the
    command's name (`codex` → `CODEX_HOME`).

  Every common model variable (`ANTHROPIC_BASE_URL`, `OPENAI_BASE_URL`,
  Gemini's…) points at the fake model at once, which tells the protocol
  from the request path. Requests in a protocol it can't answer yet are
  recorded and refused, so the report says what the agent speaks.

  Each run gets its own throwaway `HOME`, separate XDG directories,
  `TMPDIR` and project folder, and a clean environment. There's a default
  run, then one run per candidate variable, to see what each moves out of
  the home folder. The probe lists names, types and symlink targets, never
  contents, and checks the layout's claims against them.
- **A hook reports the transcript.** An agent that takes a `--settings` file
  gets one from outside its own folders, so nothing is seeded into them. It
  sets up a hook on `SessionStart`, `PostToolUse` and `Stop` that runs the
  probe itself in `-record-hook` mode. That mode writes down only the event
  name and the `transcript_path` the agent passes, never the rest of the
  payload, and it always exits 0.

## Running it

From the development shell, with the agent installed:

```sh
nickel export agents/claude-code.ncl > layout.json   # optional: to compare with
go run ./tools/probe -agent claude -layout layout.json -out probe.json
```

The checks print as it finishes, and `probe.json` has every run, attempt
and file found. Use `-keep` to keep the throwaway folders for a closer look.

**Still to do** (decision 26): replies in the OpenAI and Gemini protocols,
so agents other than Claude Code get real sessions; then a scheduled CI job
that probes every llm-agents.nix package.

## First results

Claude Code 2.1.319, 2026-11-07, in a Claude Code cloud session.

| Check | Result |
| :-- | :-- |
| How to run it, found from `--help` | `-p`, with an `--allowedTools` list, and `--settings` for the hook |
| Both runs finish, using the fake model's tools | ok: 4 tool calls each (`Bash`, `Write`, `ToolSearch`, `TaskCreate`) |
| Variables tried | `CLAUDE_CONFIG_DIR` and `CLAUDE_CODE_TMPDIR` (from the layout), `CLAUDE_HOME` (from the name). Only `CLAUDE_CONFIG_DIR` moves anything out of `~`: both `~/.claude` and `~/.claude.json` |
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
