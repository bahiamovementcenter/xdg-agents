# `xdg-agents plan`

```
xdg-agents plan [--json] [AGENT...]
```

`plan` turns the `status` report into the exact steps that would put each
installed agent's files where XDG says they belong. Like `status` it's
read-only: it changes nothing, and it looks only at names, types and symlink
targets. `apply` will carry out a plan; it isn't built yet.

## What it plans, in order

1. **The root.** If the agent's main folder has a variable (Claude Code:
   `CLAUDE_CONFIG_DIR`) and isn't where it belongs, the whole folder moves
   to its category's XDG directory (`~/.claude` → `$XDG_CONFIG_HOME/claude`),
   and the variable is set to point there. Files the variable moves from
   elsewhere move into it too (`~/.claude.json`).
2. **Folders with a variable of their own** (`plugins/`, with
   `CLAUDE_CODE_PLUGIN_CACHE_DIR`) move to their XDG place, and that
   variable is set.
3. **Everything else that can move** goes to its XDG place, with a symlink
   left at its old location inside the root's new place.
4. **Variables** are listed last, as `export` lines to add wherever the
   agent is started, with paths written using `$HOME`.

A root whose category is runtime (Claude Code's temp folder) only gets its
variable: its files are temporary, so nothing is carried over.

## What it never does

- **Overwrite.** If a target already exists, that move isn't planned, and a
  note says to merge the two by hand.
- **Delete.** Legacy folders get a note saying they're safe to delete.
- **Change symlinks someone made.** One that points somewhere other than
  its target gets a note to decide by hand.
- **Plan on macOS** (ADR-0004: report only there).

## Notes

These explain what isn't planned:

- files that can only move with their whole root;
- entries with no target, such as runtime files when `XDG_RUNTIME_DIR` is
  unset;
- settings that could also move something, such as `autoMemoryDirectory`,
  offered per project (decision 9);
- unclassified files. When the root moves they go with it, so the note says
  to check what they are first.

## Example

Claude Code 2.1.319, defaults, `XDG_RUNTIME_DIR` unset (shortened):

```
Plan for Claude Code (nothing has been changed)
  Quit Claude Code first.
   1. move  ~/.claude → ~/.config/claude
   2. move  ~/.claude.json → ~/.config/claude/.claude.json
   3. move  ~/.config/claude/plugins → ~/.local/share/claude/plugins
   4. move  ~/.config/claude/projects → ~/.local/state/claude/projects
      link  ~/.config/claude/projects → ~/.local/state/claude/projects
   …
  Then set these wherever Claude Code is started (your shell profile, for example):
      export CLAUDE_CONFIG_DIR="$HOME/.config/claude"
      export CLAUDE_CODE_PLUGIN_CACHE_DIR="$HOME/.local/share/claude/plugins"
  Notes:
    - ~/.config/claude/.credentials.json: a file can only move with its whole root
    - ~/.config/claude/projects: can also be moved with the autoMemoryDirectory setting, …
    - ~/.config/claude/sessions: no runtime directory (XDG_RUNTIME_DIR is unset)
```
