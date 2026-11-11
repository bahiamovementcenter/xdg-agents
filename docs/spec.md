# Agent directories: a draft spec

**Status:** draft. Its open choices are decisions 32 to 37 in
`docs/decisions.md`. Where this page names an option, it's the recommended
one, and it changes if the answer differs.

freedesktop.org has no specification for AI agents. This draft adapts its
two directory specs to them:

- the [XDG Base Directory Specification][basedir], for the files an agent
  keeps for itself (the **agent-baseDir** side), and
- [xdg-user-dirs][user-dirs], for the folders of content a person and their
  agents share (the **agent-homeDir** side).

It also says how the **scopes** agents read files from (managed, user,
project, local) fit around both. It's written so that an agent can follow
it without `xdg-agents`, and so that `xdg-agents status` can check it.

## 1. agent-baseDir: the files an agent keeps for itself

These follow the Base Directory spec unchanged (the table in `AGENTS.md`).
Each agent uses a folder named after itself in each directory (decision
28):

| Kind of file | Goes in | Example |
| :-- | :-- | :-- |
| Configuration you write | `$XDG_CONFIG_HOME/<agent>/` | settings, hooks, rules |
| Data that must survive | `$XDG_DATA_HOME/<agent>/` | databases, login tokens kept in a file |
| State worth keeping between runs, not backing up | `$XDG_STATE_HOME/<agent>/` | transcripts, history, logs |
| Regenerable data | `$XDG_CACHE_HOME/<agent>/` | downloaded packages, model catalogs |
| Sockets, locks, per-session files | `$XDG_RUNTIME_DIR/<agent>/` | running sessions |
| Configuration set by an administrator | `$XDG_CONFIG_DIRS` (`/etc/xdg/<agent>/`) | managed settings |

What agents add:

- **Configuration shared by every agent** goes in `$XDG_CONFIG_HOME/agents/`,
  such as a global `AGENTS.md` (decision 36). An agent reads it before its
  own folder, so its own folder wins.
- **Data, state and cache that belong to one project** stay in the agent's
  user directories, in a subfolder keyed by the project, never in the
  project itself. Claude Code already does this:
  `~/.claude/projects/<project>/`. The key is derived from the project's
  path, with `/` replaced so it's one folder name.
- **One variable per directory**, where an agent offers variables at all,
  is better than one for the whole folder: `CLAUDE_CONFIG_DIR` moves data,
  state and cache into the configuration folder along with the settings.

## 2. agent-homeDir: folders a person and their agents share

Like `user-dirs.dirs`, an `agent-dirs.dirs` file names well-known folders
for content: things a person opens, edits or hands over, as opposed to the
agent's own files above.

```sh
# $XDG_CONFIG_HOME/agent-dirs.dirs
AGENT_HOME_DIR="$HOME/Agents"
AGENT_WORKSPACE_DIR="$HOME/Agents/Workspace"
AGENT_MEMORY_DIR="$HOME/Agents/Memory"
AGENT_SKILLS_DIR="$HOME/Agents/Skills"
AGENT_PROMPTS_DIR="$HOME/Agents/Prompts"
AGENT_INBOX_DIR="$HOME/Agents/Inbox"
AGENT_OUTPUTS_DIR="$HOME/Agents/Outputs"
```

| Variable | Holds | Like (xdg-user-dirs) |
| :-- | :-- | :-- |
| `AGENT_HOME_DIR` | The agent-homeDir; the others default under it (decision 32) | `$HOME` |
| `AGENT_WORKSPACE_DIR` | Scratch space for work that isn't in any project | `XDG_DESKTOP_DIR` |
| `AGENT_MEMORY_DIR` | Memory a person can read and edit, in a subfolder per agent (decision 33) | `XDG_DOCUMENTS_DIR` |
| `AGENT_SKILLS_DIR` | Skills a person writes or installs, shared by every agent (decision 34) | none |
| `AGENT_PROMPTS_DIR` | Reusable prompts and commands | `XDG_TEMPLATES_DIR` |
| `AGENT_INBOX_DIR` | Files handed to agents | `XDG_DOWNLOAD_DIR` |
| `AGENT_OUTPUTS_DIR` | What agents make for a person | none |

Projects aren't in the list: `XDG_PROJECTS_DIR`, which xdg-user-dirs turned
on by default in 0.20 (April 2026), already says where they live
(decision 37).

The rules are those of `user-dirs.dirs`:

- **Format.** One `NAME="value"` per line, where the value is `"$HOME/…"`
  or an absolute path, so that a shell can source the file. Other lines
  are comments or blank.
- **Where.** `$XDG_CONFIG_HOME/agent-dirs.dirs` for the person;
  `$XDG_CONFIG_DIRS/agent-dirs.defaults` (normally
  `/etc/xdg/agent-dirs.defaults`) for a system's defaults, with values
  relative to `$HOME`.
- **Optional.** A folder that's unset, or deleted, isn't created again: an
  agent falls back to its own default for it, as xdg-user-dirs falls back
  to `$HOME`.
- **Lookup.** `agent-dir SKILLS` prints a folder, like `xdg-user-dir`. An
  environment variable of the same name, when set to an absolute path,
  overrides the file.
- **Sharing.** The folders are shared by every agent. An agent that needs a
  part of its own uses a subfolder named after itself, such as
  `Memory/claude/`.

## 3. Scopes: managed, user, project, local

Agents read the same kind of file from several places. XDG already has two
of them, system (`XDG_CONFIG_DIRS`) and user (`XDG_CONFIG_HOME`); agents
add two more, inside a project (decision 35):

| Scope | Where | Shared with | What belongs there |
| :-- | :-- | :-- | :-- |
| Managed | `$XDG_CONFIG_DIRS` | everyone on the machine | Configuration only; read only |
| User | sections 1 and 2 above | no one | Everything an agent keeps |
| Project | `<project>/.agents/`, and the agent's own folder (`.claude/`, `.codex/`) | everyone with the repository | Configuration only, committed: `AGENTS.md`, settings, skills, prompts |
| Local | Beside the project's files, named `*.local.*` | no one | Personal configuration, never committed |

- **Precedence**, highest first: local, project, user, managed. This is the
  XDG search order with two tiers on top.
- **A project is a folder under `XDG_PROJECTS_DIR`** or any folder an agent
  is started in. `<project>/.agents/skills/` adds to `AGENT_SKILLS_DIR`
  rather than replacing it, and the same goes for prompts.
- **Nothing but configuration lives in a project.** Data, state and cache
  for a project go in the user's directories, keyed by project (section 1).
- **Local files are gitignored,** and never hold credentials or state.

## What `xdg-agents` checks

- **User scope** (built): `status`, `plan` and `apply`, against each agent's
  layout in `agents/`.
- **agent-homeDir** (not built): layouts need a way to tag a folder as one
  of the `AGENT_*_DIR` folders, so `status` reports it as such rather than
  as data, and `plan` can move or link it there.
- **Project and local scopes** (not built): roots in `agents/schema.ncl`
  get a `scope`, with defaults relative to the project. `status` checks the
  project it's run in (or each folder under `XDG_PROJECTS_DIR`) for a local
  file that isn't gitignored, or state and cache inside the repository.
  `apply` never moves a project's files.
- **Runtime** (not built): a root can't yet be placed under
  `$XDG_RUNTIME_DIR` (`docs/follow-ups.md`).

[basedir]: https://specifications.freedesktop.org/basedir-spec/latest/
[user-dirs]: https://www.freedesktop.org/wiki/Software/xdg-user-dirs/
