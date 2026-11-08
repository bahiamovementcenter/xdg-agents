# `xdg-agents apply` and `undo`

```
xdg-agents apply [--yes] [AGENT...]
xdg-agents undo [--journal FILE]
```

`apply` carries out the plan `xdg-agents plan` shows, on Linux only
(ADR-0004).

## How to use it

1. Run `xdg-agents plan` and read it.
2. Add the plan's `export` lines to your shell profile, and open a new
   shell. `apply` won't start until they're set: if `~/.claude` moved while
   `CLAUDE_CONFIG_DIR` wasn't set, Claude Code would start a fresh, empty
   `~/.claude` on its next run.
3. Quit the agent.
4. Run `xdg-agents apply`. It checks everything, shows the plan again, and
   makes the changes only after you type `yes` (or with `--yes`).
5. Start the agent. `xdg-agents status` should now show everything that
   can move as in place.

If anything looks wrong, `xdg-agents undo` reverses the last apply.

## What it checks first

Nothing is changed unless all of these hold. Otherwise it lists every
problem and stops.

- **The agent isn't running.** It looks for processes named after the
  layout's id or folder name (`claude-code`, `claude`), including as the
  script an interpreter runs, such as `node …/claude`.
- **Every variable in the plan is already set** to its planned value, in
  the shell `apply` runs from.
- **For every move, in order:** the source exists, the target doesn't, and
  both are on the same filesystem. Moves are renames, so a move to another
  disk is refused rather than done as copy and delete.
- **For every symlink:** nothing is at the link's path, and its target
  exists.

Between setting the variables and running `apply`, the variable points at
a folder that doesn't exist yet. `status` and `plan` report that as
"`CLAUDE_CONFIG_DIR` set, files not moved yet" and still find the files at
the default location.

## What it does

The plan's steps, in order:

- **Move:** creates any missing parent folders, then renames.
- **Symlink:** creates the link.

Each change goes into a journal as soon as it's made, at
`$XDG_STATE_HOME/xdg-agents/journal-<time>.json`. The journal holds paths
only, and is rewritten whole after each change, so a crash never leaves
half a journal. If a step fails partway, `apply` stops, and the journal
holds what was done.

It never deletes, overwrites or edits anything, including shell profiles.

## Undo

`xdg-agents undo` reverses the latest journal that hasn't been undone, or
the one `--journal` names, last change first:

- removes each symlink apply made, only if it still points where apply
  pointed it;
- moves each moved item back, only if nothing has taken its old place;
- removes each folder apply created, only if it's empty again.

If something on disk no longer matches, undo stops there and says why,
leaving the rest for a person to look at.

## Decisions

Decided on 2026-11-08, as numbers 28 to 31 in `docs/decisions.md`: one
folder per agent (`~/.config/claude`, not a shared group folder),
variables first, the agent must not be running, and no editing of shell
profiles.
