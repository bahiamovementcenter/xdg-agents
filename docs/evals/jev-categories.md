# Evaluation: can Jev propose XDG categories?

Decision 21 in `docs/decisions.md`: before using TypeSafe's Jev decision
model to propose categories for new paths, measure it against categories we
have already reviewed. Run on 2026-11-07 against `jev-latest`, which
resolved to **jev-1.14.0**.

## Method

- **Test set** (`jev/items.json`, built by `jev/build.py`): each row of
  Claude Code's `claude-directory` docs page that maps exactly onto an entry
  in `agents/claude-code.ncl`, giving the docs' own description and the
  layout's category. That's 39 rows; four are left out by `jev/run.py`,
  leaving 35:
  - `plugins`: its only description is for a file inside it;
  - `todos`, `statsig`, `logs`: described only as "legacy directories";
  - `projects/*/memory`, left out by `build.py`: it's data inside the state
    folder `projects/`, so the layout's answer for it would be wrong.
- **Question** (`jev/run.py`): one `choice` question, "Under the XDG Base
  Directory Specification, which category does this file or folder belong
  to?", with the five categories as options. Each option's description is
  the spec's definition, worded as in `AGENTS.md`.
- **Two variants:**
  - **description only**: the program's name and the docs' description;
  - **description and path**: the same, plus the path (for example
    `~/.claude/paste-cache`).
- Only public documentation text was sent; no file names or contents from
  anyone's machine.

To rerun, from `docs/evals/jev/`, with the docs page and an exported layout:

```sh
python3 build.py claude-directory.md layout.json items.json
python3 run.py items.json desc results-description.json
python3 run.py items.json path results-description-and-path.json
```

## Results

| | Description only | Description and path |
| :-- | :-- | :-- |
| Agrees with the layout | 24 of 35 (69%) | 27 of 35 (77%) |
| Answers with confidence ≥ 0.8 | 9, of which 8 right | 19, of which 17 right |
| Answers with confidence ≥ 0.6 | 17, of which 14 right | 23, of which 20 right |
| Median reply time | 201 ms | 210 ms |
| Tokens, whole run | 17,222 in, 1,820 out | 17,707 in, 1,820 out |

Where it disagreed, most confident first:

| Path | Layout | Jev, description only | Jev, with path |
| :-- | :-- | :-- | :-- |
| `shell-snapshots` | state | runtime (0.86) | state |
| `session-env` | state | runtime (0.75) | state |
| `backups` | state | config (0.66) | state |
| `.credentials.json` | data | config (0.59) | config (0.76) |
| `workflows` | config | data (0.51) | config |
| `CLAUDE.md` | config | runtime (0.39) | config |
| `uploads` | state | data (0.37) | data (0.36) |
| `usage-data` | state | cache (0.36) | state |
| `paste-cache` | state | config (0.27) | **cache (0.95)** |
| `plugins/.trash` | state | data (0.18) | state |
| `skills/.trash` | state | data (0.16) | config (0.19) |
| `stats-cache.json` | state | state | **cache (0.84)** |
| `dev-mods` | state | state | config (0.42) |
| `tasks` | state | state | data (0.30) |
| `feedback-bundles` | state | state | config (0.24) |

## What it shows

- **The path helps for the wrong reason.** Adding it raises agreement, but
  its two most confident mistakes are files named "cache".
  `stats-cache.json` is the case where the layout overrides the name on
  purpose, because the totals it holds can't be rebuilt. That is guessing
  from the name, which `AGENTS.md` rules out. If Jev is used at all, it must
  get the description only.
- **From the description alone it's right about two times in three,** and
  its confidence doesn't reliably separate right from wrong: it's wrong at
  0.86 and 0.75, and right at 0.18. It can't classify unaided.
- **Some disagreements are fair questions about the layout.**
  `shell-snapshots` (0.86 runtime): the docs say Claude Code removes it on a
  clean exit, the same cleanup as `sessions/`, which the layout already
  calls runtime. Decision 23 made it runtime. `session-env` is different: the docs
  sweep it by age, not on exit, so state stands.
- **It's fast and cheap,** about 200 ms and 500 tokens a question, but
  TypeSafe's speed and cost claims weren't what was being tested.

## Conclusion

Not good enough to propose categories on its own, and not reliable enough
that its confidence could decide which proposals to trust. As a second
opinion beside the docs text it did find one real question about the
layout. Decision 22: not now; rerun this evaluation when a new Jev version
comes out (the reply's `model` field says which version answered).
