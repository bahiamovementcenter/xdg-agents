# Upstream fixes

Changes for other projects, drafted here for the owner to submit.

## `xdg-ninja-claude-code.patch`

For [xdg-ninja](https://github.com/b3nj5m1n/xdg-ninja), against commit
`ebf481a` (2026-10-04). Decision 18 in `docs/decisions.md`.

- **`~/.claude.json`:** xdg-ninja says it can't be moved. The
  claude-directory docs say `CLAUDE_CONFIG_DIR` moves it along with
  `~/.claude`, so the patch marks it movable and gives the steps.
- **`~/.claude`:** adds a warning that it holds data, state and cache as well
  as configuration, as xdg-ninja's Codex entry already does for
  `~/.codex`.

To submit it, fork xdg-ninja, then from the fork's checkout:

```sh
git apply /path/to/xdg-agents/docs/upstream/xdg-ninja-claude-code.patch
git commit -am "Claude Code: ~/.claude.json moves with CLAUDE_CONFIG_DIR"
```

and open a pull request.
