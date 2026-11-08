# `xdg-agents status`

What `status` shows and how it decides. Its open choices were settled as
decisions 10 to 17 in `docs/decisions.md` (2026-11-07), and this page
describes the result.

`status` is read-only. It never opens a file inside an agent's folders. It
looks only at names, types and symlink targets, so it can't print a
credential even by mistake.

## What's built

`xdg-agents list` and `xdg-agents status [--check] [--json] [--all]
[AGENT...]` work as described below, except:

- **`--sizes`** isn't built yet.
- **Under investigation (`?`)** needs the `investigating` list in each
  layout (decision 11), which isn't in the schema yet. Until then those files
  show as unclassified.
- **The output differs slightly in places:** headers show every root that
  exists (for Claude Code, also its temp folder), and entries keep their
  category group even when their state is "stays".

## Command line

```
xdg-agents list
xdg-agents status [--check] [--json] [--all] [--sizes] [AGENT...]
xdg-agents plan   [AGENT...]
xdg-agents apply  [AGENT...]
```

- **`AGENT`** is a layout's `id`, or a prefix of one: `claude` matches
  `claude-code` and `claude-desktop`. Several are allowed. With none, every
  agent installed on this machine is shown. A name that matches nothing is
  an error that lists the known ids.
- **Agents aren't flags** (`--claude`). A new `agents/*.ncl` then works with
  no code change, and agent names can't clash with options.
- **`list`** prints every layout's id and name, and whether the agent is
  installed here (its root exists).

## Output

For the machine in `docs/follow-ups.md` (hornicorn):

```
$ xdg-agents status claude-code

XDG directories
  config   ~/.config            (default)
  data     ~/.local/share       (default)
  state    ~/.local/state       (default)
  cache    ~/.cache             (default)
  runtime  /run/user/1000       (XDG_RUNTIME_DIR)

Claude Code — root ~/.claude (CLAUDE_CONFIG_DIR unset)
  config
    ✗ settings.json        → ~/.config/claude/settings.json      via CLAUDE_CONFIG_DIR
    ✗ skills/              → ~/.config/claude/skills/            via CLAUDE_CONFIG_DIR
    ✗ ~/.claude.json  🔒   → ~/.config/claude/.claude.json        via CLAUDE_CONFIG_DIR
  data
    ✗ plugins/             → ~/.local/share/claude/plugins/      via CLAUDE_CODE_PLUGIN_CACHE_DIR
    ~ .credentials.json 🔒 stays with config: a file can only move with the whole root
  state
    ✗ projects/  🔒        → ~/.local/state/claude/projects/     symlink
        includes projects/*/memory (data): autoMemoryDirectory can move it, with a caveat
  cache
    ✗ cache/               → ~/.cache/claude/cache/              symlink
  runtime
    ✗ sessions/            → /run/user/1000/claude/sessions/     symlink
    ✗ shell-snapshots/     → /run/user/1000/claude/shell-snapshots/  symlink
  legacy (no longer written; safe to delete)
    · todos/  statsig/
  under investigation (docs/follow-ups.md)
    ? bridge-spawn/  state/  sessions/*.key
  not Claude Code's
    – .ponytail-active  ponytail-modes/  ponytail-statusline.sh   (ponytail plugin)
  unclassified
    ! something-new.json

Summary: 2 in place, 31 to move (9 via variables, 22 by symlink), 4 stay put
Run `xdg-agents plan claude-code` to see the exact steps.
```

How the output is laid out:

- **The XDG directories come first,** each with where its value came from:
  `(default)`, the variable's name, or `(XDG_DATA_HOME ignored: not an
  absolute path)`. Problems show up before anything depends on them.
- **Each agent is one section,** headed by its root and how that root was
  found (the variable's value, or "unset" and the default).
- **Entries are grouped by category,** in the spec's order: config, data,
  state, cache, runtime. Then come legacy, under investigation, not the
  agent's, and unclassified.
- **One line per entry:** state, path (relative to the root, `/` for
  folders), 🔒 if `sensitive`, target, and how it gets there.
- **Only what exists on disk.** An entry for a file this machine doesn't
  have isn't listed. `--all` lists those too, marked `(not present)`.
- **Patterns stay patterns.** `projects/` is one line, not one per project.
- **Side files are grouped** with the file they belong to:
  `Cookies-journal` and `DIPS-wal` go with `Cookies` and `DIPS`.
- **The summary** counts the states for each agent and points to `plan`.

## States

Each entry gets exactly one state. They are worked out in this order, and
the first match wins.

| Mark | State | When |
| :-- | :-- | :-- |
| ✓ | in place | It's at its target already, or it's a symlink that resolves to its target. |
| ⚠ | symlinked elsewhere | It's a symlink, but not to its target. Never "fixed" silently: `plan` has to ask. |
| ~ | stays put | `move = 'with_root` and its category isn't the root's, so it can only move with the root (for example `.credentials.json`, data in the config root). The reason is printed. |
| · | legacy | `legacy = true`. |
| ✗ | to move | Every other entry in the layout. The "via" column says how: the root's variable, the entry's own `env`, a `settings` entry, or `symlink`. |
| ⊘ | fixed | `move = 'fixed`: the agent only looks in one place. Report only. |
| ? | under investigation | On disk under a root, matching an open item in `docs/follow-ups.md`. |
| – | not the agent's | On disk under a root, matching an `untracked` path (plugins, the cloud runner). |
| ! | unclassified | On disk under a root, matched by nothing above. |

**Where things are:**

- **Current location:** if the root's `env` is set to an absolute path, the
  root is there; otherwise it's the root's `default` (`default_macos` on
  macOS). An entry's `default_path` replaces its location under the
  default root, for example `~/.claude.json`.
- **Target:** `$XDG_<CATEGORY>_HOME/<xdg_name>/<path>`, using
  `$XDG_RUNTIME_DIR` for runtime. `xdg_name` follows the layout exactly
  (`claude`, `Claude`).
- **Not installed:** an agent whose root doesn't exist isn't shown, unless
  it was named on the command line. Then it says "not installed".

## XDG variables

These follow basedir 0.8, as summarised in `AGENTS.md`:

- **Unset or empty:** the default is used.
- **Not an absolute path:** invalid and ignored. The default is used, and
  the header says so.
- **`XDG_RUNTIME_DIR` unset:** there's no default. The header warns,
  runtime entries show `→ (no runtime directory)`, and they don't count as
  "to move".

## `--json`

The same report as data, for scripts and for other tools (the xdg-ninja
exporter idea in `docs/follow-ups.md`). Build the report as data first and
print text from it, so the two can't disagree.

```json
{
  "xdg": { "config": { "path": "/home/u/.config", "source": "default" } },
  "agents": [
    {
      "id": "claude-code",
      "root": { "path": "/home/u/.claude", "source": "default", "env": "CLAUDE_CONFIG_DIR" },
      "entries": [
        {
          "path": "settings.json",
          "kind": "file",
          "category": "config",
          "state": "to_move",
          "target": "/home/u/.config/claude/settings.json",
          "via": { "env": "CLAUDE_CONFIG_DIR" },
          "sensitive": false
        }
      ],
      "summary": { "in_place": 2, "to_move": 31, "stays": 4 }
    }
  ]
}
```

Paths in JSON are absolute. Text output shortens `$HOME` to `~`.

## Exit status

| Status | Without `--check` | With `--check` |
| :-- | :-- | :-- |
| 0 | the report was printed | everything shown is in place, stays put, fixed or legacy |
| 1 | not used | something is to move, symlinked elsewhere, or unclassified |
| 2 | a layout failed to load, or an argument was invalid | the same |

`--check` lets dotfile setups and CI ask "is this machine tidy?".

## What to run next

The text report ends with a "Next:" section, left out of `--json`:

- **`xdg-agents plan`,** with the same agent names, when the plan would have
  steps. Plan decides what can move, so status never suggests an empty or
  refused plan.
- **`xdg-agents undo`,** with the journal's path, when an apply hasn't been
  undone.

## Decisions

Decided on 2026-11-07, as numbers 10 to 17 in `docs/decisions.md`:

- **Default view:** everything on disk, grouped by category (12).
- **`--check` and `--json`:** from the start (13, 14).
- **`--sizes`:** an option, off by default (15).
- **The desktop app:** listed in full, like every agent (16).
- **Symbols,** with plain words when not on a terminal or when `NO_COLOR`
  is set (17).
- **Files that aren't the agent's own:** their own group, from the layout's
  `untracked` list (10).
- **Under investigation:** an `investigating = [{ path, note }]` list in each
  layout, which `status` reads (11). Not in the schema yet.

## Not `status`'s job

- **Moving anything,** or writing variables or settings: that's `plan` and
  `apply`.
- **Reading file contents,** for any reason. Classification comes from the
  layouts, and the probes that built them use names and key names only.
- **Guessing a category** for an unclassified file. It's reported as `!`,
  and someone adds it to the layout after finding out what it is.
