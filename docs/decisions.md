# Decisions

Every open question about this project, in one numbered list. Numbers are
never reused. When a question comes up, it's added here, not left in a chat.

**To answer:** reply with numbers and letters, such as `1A 2A 6B`, or "all
recommended", or "all recommended except 6B". Answered items move to
**Decided** with the date. Any that change the design are also recorded
where they belong: an ADR in `docs/adr/`, a layout, or `docs/status.md`.

Facts still to find out (not decisions) are in `docs/follow-ups.md`.

## Open

### Working together

| # | Question | Options | Recommended |
| :-- | :-- | :-- | :-- |
| 1 | When work on the branch is ready, should Claude open its PR without asking? | **A** yes · **B** ask each time | A |
| 2 | Should Claude watch every PR it opens (fix CI, answer reviews) without asking? | **A** yes · **B** ask each time | A |

### Project

| # | Question | Options | Recommended |
| :-- | :-- | :-- | :-- |
| 3 | ADR-0001: keep the dev shell in a Nix flake rather than Organist? | **A** accept · **B** reject (move to Organist) | A |
| 4 | ADR-0002: keep Go and go-nickel rather than Gleam? | **A** accept · **B** reject (move to Gleam with gleam-nickel-bindings) | A |
| 5 | Where do agent layouts live? | **A** here in `agents/`, split into an `xdg-agent-dirs` repo once something else uses them · **B** a separate repo now | A |
| 6 | What does xdg-agents do on macOS, where `~/Library/Application Support` is the platform's own convention? | **A** report only · **B** report, plan and apply, as on Linux · **C** Linux only; ignore macOS | A |
| 7 | Which agent gets a layout next? | **A** Codex (`CODEX_HOME`, plus `log_dir` and `sqlite_home` settings) · **B** Gemini CLI · **C** the desktop app on macOS | A |
| 8 | Accept the judgement calls in `agents/claude-code.ncl`: `~/.claude.json` as config; `projects/` as state; `.credentials.json`, `history.jsonl` and `stats-cache.json` staying with the config root (files can't be symlinked out)? | **A** accept · **B** change some (say which) | A |
| 9 | How should `plan` handle `autoMemoryDirectory`, which shares one memory folder across projects when set in user settings? | **A** offer it per project (`.claude/settings.local.json`) · **B** only mention it · **C** never | A |

### `status` (see `docs/status.md`)

| # | Question | Options | Recommended |
| :-- | :-- | :-- | :-- |
| 10 | Files under an agent's root that belong to something else (plugins, the cloud runner) | **A** their own "not the agent's" group, listed in the layout's `untracked` with a reason · **B** shown as unclassified · **C** hidden | A |
| 11 | Files under investigation (unknown purpose, being looked into) | **A** an `investigating = [{ path, note }]` list in each layout, which `status` reads · **B** shown as unclassified until settled | A |
| 12 | Default view | **A** everything on disk, grouped by category · **B** only what's out of place, with `--all` for the rest | A |
| 13 | `--check` (exit 1 when anything is out of place) | **A** from the start · **B** later | A |
| 14 | `--json` | **A** from the start · **B** later | A |
| 15 | Folder sizes | **A** `--sizes` option, off by default · **B** always shown · **C** never | A |
| 16 | The desktop app in the report | **A** listed in full, like every agent · **B** summarised by category | A |
| 17 | Symbols (✓ ✗ ~ …) | **A** symbols, with plain words when not on a terminal or when `NO_COLOR` is set · **B** plain words only | A |

### Other projects

| # | Question | Options | Recommended |
| :-- | :-- | :-- | :-- |
| 18 | xdg-ninja says `~/.claude.json` can't be moved, which is out of date | **A** Claude drafts the fix as a patch for you to submit · **B** leave it | A |
| 19 | A `tools/` exporter that turns our layouts into xdg-ninja entries | **A** after `status` works · **B** now · **C** never | A |
| 20 | A CI job that installs the desktop app on GitHub's macOS and Linux machines and lists its folder on first launch | **A** no: signed out, it can't show sessions or environments · **B** yes | A |

## Decided

None yet.
