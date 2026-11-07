# Decisions

Every open question about this project, in one numbered list. Numbers are
never reused. When a question comes up, it's added here, not left in a chat.

**To answer:** reply with numbers and letters, such as `1A 2A 6B`, or "all
recommended", or "all recommended except 6B". Answered items move to
**Decided** with the date. Any that change the design are also recorded
where they belong: an ADR in `docs/adr/`, a layout, or `docs/status.md`.

Facts still to find out (not decisions) are in `docs/follow-ups.md`.

## Open

| # | Question | Options | Recommended |
| :-- | :-- | :-- | :-- |
| 26 | How should probing scale beyond Claude Code? | **A** automatic: a scheduled CI job probes every [llm-agents.nix](https://github.com/numtide/llm-agents.nix) package, and again when its version changes. Nothing is written per agent: the program comes from the package (`meta.mainProgram`); every common model variable (`ANTHROPIC_BASE_URL`, `OPENAI_BASE_URL`, Gemini's…) points at the fake model at once, which tells the protocol from the request path; how to run once non-interactively, how to allow tools, and which `*_HOME`/`*_CONFIG_DIR` variable moves the folder are found from `--help` and tried in turn. Results are saved per agent and version, and what's reviewed is the change. A small override is written only for an agent the heuristics can't start, which the run reports · **B** a hand-written `probe` block in each layout | A |

## Decided

All answered "all recommended" on 2026-11-07. Where a decision changed
the design, it's recorded in the place it names: ADR-0001 to ADR-0004,
`docs/status.md`.


### Working together

| # | Question | Options | Answer |
| :-- | :-- | :-- | :-- |
| 1 | When work on the branch is ready, should Claude open its PR without asking? | **A** yes · **B** ask each time | **A** (2026-11-07) |
| 2 | Should Claude watch every PR it opens (fix CI, answer reviews) without asking? | **A** yes · **B** ask each time | **A** (2026-11-07) |

### Project

| # | Question | Options | Answer |
| :-- | :-- | :-- | :-- |
| 3 | ADR-0001: keep the dev shell in a Nix flake rather than Organist? | **A** accept · **B** reject (move to Organist) | **A** (2026-11-07) |
| 4 | ADR-0002: keep Go and go-nickel rather than Gleam? | **A** accept · **B** reject (move to Gleam with gleam-nickel-bindings) | **A** (2026-11-07) |
| 5 | Where do agent layouts live? | **A** here in `agents/`, split into an `xdg-agent-dirs` repo once something else uses them · **B** a separate repo now | **A** (2026-11-07) |
| 6 | What does xdg-agents do on macOS, where `~/Library/Application Support` is the platform's own convention? | **A** report only · **B** report, plan and apply, as on Linux · **C** Linux only; ignore macOS | **A** (2026-11-07) |
| 7 | Which agent gets a layout next? | **A** Codex (`CODEX_HOME`, plus `log_dir` and `sqlite_home` settings) · **B** Gemini CLI · **C** the desktop app on macOS | **A** (2026-11-07) |
| 8 | Accept the judgement calls in `agents/claude-code.ncl`: `~/.claude.json` as config; `projects/` as state; `.credentials.json`, `history.jsonl` and `stats-cache.json` staying with the config root (files can't be symlinked out)? | **A** accept · **B** change some (say which) | **A** (2026-11-07) |
| 9 | How should `plan` handle `autoMemoryDirectory`, which shares one memory folder across projects when set in user settings? | **A** offer it per project (`.claude/settings.local.json`) · **B** only mention it · **C** never | **A** (2026-11-07) |

### `status` (see `docs/status.md`)

| # | Question | Options | Answer |
| :-- | :-- | :-- | :-- |
| 10 | Files under an agent's root that belong to something else (plugins, the cloud runner) | **A** their own "not the agent's" group, listed in the layout's `untracked` with a reason · **B** shown as unclassified · **C** hidden | **A** (2026-11-07) |
| 11 | Files under investigation (unknown purpose, being looked into) | **A** an `investigating = [{ path, note }]` list in each layout, which `status` reads · **B** shown as unclassified until settled | **A** (2026-11-07) |
| 12 | Default view | **A** everything on disk, grouped by category · **B** only what's out of place, with `--all` for the rest | **A** (2026-11-07) |
| 13 | `--check` (exit 1 when anything is out of place) | **A** from the start · **B** later | **A** (2026-11-07) |
| 14 | `--json` | **A** from the start · **B** later | **A** (2026-11-07) |
| 15 | Folder sizes | **A** `--sizes` option, off by default · **B** always shown · **C** never | **A** (2026-11-07) |
| 16 | The desktop app in the report | **A** listed in full, like every agent · **B** summarised by category | **A** (2026-11-07) |
| 17 | Symbols (✓ ✗ ~ …) | **A** symbols, with plain words when not on a terminal or when `NO_COLOR` is set · **B** plain words only | **A** (2026-11-07) |

### Other projects

| # | Question | Options | Answer |
| :-- | :-- | :-- | :-- |
| 18 | xdg-ninja says `~/.claude.json` can't be moved, which is out of date | **A** Claude drafts the fix as a patch for you to submit · **B** leave it | **A** (2026-11-07) |
| 19 | A `tools/` exporter that turns our layouts into xdg-ninja entries | **A** after `status` works · **B** now · **C** never | **A** (2026-11-07) |
| 20 | A CI job that installs the desktop app on GitHub's macOS and Linux machines and lists its folder on first launch | **A** no: signed out, it can't show sessions or environments · **B** yes | **A** (2026-11-07) |

### Tools

| # | Question | Options | Answer |
| :-- | :-- | :-- | :-- |
| 21 | Use Jev (TypeSafe's typed decision model) to propose a category for each new path the drift check finds, from the docs' own description, for a person to review? Never inside `xdg-agents` at run time. | **A** evaluate first: run it over the 49 reviewed Claude Code entries and measure agreement and confidence · **B** add it to the drift workflow now (needs `TYPESAFE_API_KEY` as a GitHub secret) · **C** don't use it | **A** (2026-11-07) |
| 22 | After the evaluation in `docs/evals/jev-categories.md` (69% agreement from descriptions, confidence not reliable), should the drift workflow use Jev? | **A** advisory only: when drift finds a new path, show Jev's pick and probabilities (from the docs' description, never the path) next to the docs text; never write the layout. Needs `TYPESAFE_API_KEY` as a GitHub secret · **B** not now: rerun the evaluation when a new Jev version comes out · **C** drop it | **B** (2026-11-07) |
| 23 | Reclassify `shell-snapshots/` in `agents/claude-code.ncl` from state to runtime? The docs say it's removed on a clean exit, the same as `sessions/`, which is already runtime. Jev's most confident disagreement (0.86). | **A** runtime · **B** keep state | **A** (2026-11-07) |
| 24 | Probe agents automatically using [llm-agents.nix](https://github.com/numtide/llm-agents.nix) as a flake input: a Nix build runs each agent offline with a throwaway `HOME` and separate XDG folders, records what appears (names and types only), and reruns with the layout's variables to check they move what we claim. Classifying stays a person's job. | **A** prototype one probe, for Claude Code, and compare it with `agents/claude-code.ncl` · **B** probes for every packaged agent, plus draft layouts (no categories) for agents without one · **C** don't | **A** (2026-11-07) |
| 25 | Probes that run real sessions, to see the files only sessions create (`projects/`, `sessions/`, `file-history/`, `tasks/`…)? | **E** a fake model server of our own (`tools/fakemodel`, Go) that speaks the Anthropic and OpenAI APIs, needs no login and works offline inside the Nix sandbox and CI. It replies with a fixed script of tool calls, matched by purpose to the tools each agent offers (a harmless shell command, an edit in the probe's throwaway project, a to-do list), then a closing text naming the tools the agent offered and used. One script, with a thin adapter per wire protocol (Anthropic Messages, OpenAI, Gemini); the script's shape (replies made of text, thinking and tool-call blocks) follows the faux provider in [pi](https://github.com/earendil-works/pi)'s `ai` package, which is in-memory only and so can't serve other agents itself. Agents are pointed at it with `ANTHROPIC_BASE_URL` (and their equivalents) and a dummy key · **A** on the owner's machine with a real login, in a fresh, empty `CLAUDE_CONFIG_DIR` (`/login` saves the login inside that folder, so deleting the folder removes it), then listing names; for what only real sessions show · **B** real logins in CI, with a token (`claude setup-token` → `CLAUDE_CODE_OAUTH_TOKEN`) stored as a GitHub secret · **C** through [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), which re-exposes subscription logins to other clients: likely against providers' terms, and it holds live tokens · **D** none | **E** (2026-11-07) |
