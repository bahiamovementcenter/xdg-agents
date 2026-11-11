# Agent directories: a draft spec

**Status:** draft. Its choices are decisions 32 to 37 in
`docs/decisions.md`; 32, 33 and 37 are decided. Where this page names an
open option, it's the recommended one, and it changes if the answer
differs.

freedesktop.org has no specification for AI agents. This draft adapts its
two directory specs to them:

- the [XDG Base Directory Specification][basedir], for the files an agent
  keeps for itself (the **agent-baseDir** side), and
- [xdg-user-dirs][user-dirs], for the folders of content a person and their
  agents share (the **agent-homeDir** side).

It also says how the **scopes** agents read files from (managed, user,
project, local) fit around both. It's written so that an agent can follow
it without `xdg-agents`, and so that `xdg-agents status` can check it.

Where agents already agree, it follows them rather than inventing a name.
`docs/survey/` counts where public repositories put these files.

## 1. agent-baseDir: the files an agent keeps for itself

These follow the Base Directory spec unchanged (the table in `AGENTS.md`).
Each agent uses a folder named after itself in each directory (decision
28):

| Kind of file | Goes in | Example |
| :-- | :-- | :-- |
| Configuration you write | `$XDG_CONFIG_HOME/<agent>/` | settings, hooks, MCP servers |
| Data that must survive | `$XDG_DATA_HOME/<agent>/` | databases, login tokens kept in a file |
| State worth keeping between runs, not backing up | `$XDG_STATE_HOME/<agent>/` | transcripts, history, to-do lists, logs |
| Regenerable data | `$XDG_CACHE_HOME/<agent>/` | downloaded packages, plugin installs, model catalogs |
| Sockets, locks, per-session files | `$XDG_RUNTIME_DIR/<agent>/` | running sessions |
| Configuration set by an administrator | `$XDG_CONFIG_DIRS` (`/etc/xdg/<agent>/`) | managed settings |

What agents add:

- **Data, state and cache that belong to one project** stay in the agent's
  user directories, in a subfolder keyed by the project, never in the
  project itself. Claude Code already does this:
  `~/.claude/projects/<project>/`. The key is derived from the project's
  path, with `/` replaced so it's one folder name.
- **One variable per directory**, where an agent offers variables at all,
  is better than one for the whole folder: `CLAUDE_CONFIG_DIR` moves data,
  state and cache into the configuration folder along with the settings.

## 2. agent-homeDir: what a person writes for their agents

Some files aren't an agent's own: a person writes them, every agent can
use them, and they're worth sharing between agents and machines. These go
in **`~/.agents`**, the folder agents already read (decision 32):

```
~/.agents/
  AGENTS.md       # instructions every agent reads
  skills/         # skills: skills/<name>/SKILL.md
  agents/         # subagent definitions: agents/<name>.md
  rules/          # rules: rules/**/*.md, each scoped by its frontmatter
  memory/         # memory: a MEMORY.md index and topic files
```

That's all, for now. Each of these is in use across agents (see
`docs/survey/2026-11-11.tsv`): `.agents/skills` is as common as
`.claude/skills`, a root `AGENTS.md` is more common than `CLAUDE.md`, and
`agents/`, `rules/` and `memory/` are the names agents use for the rest.
A folder is added here once agents use it, not before. Left out for that
reason:

- **Prompts, commands and workflows** are skills: a saved prompt is a skill
  with one file, and a workflow is a skill made of steps (Claude Code
  merged its slash commands into skills in January 2026). Tools that only
  read their own prompt folders get links (section 4).
- **Workspace, inbox, outputs, plans, a profile (`USER.md`)**: each is used
  by one agent at most.

### `agent-dirs.dirs`

Like `user-dirs.dirs`, an `agent-dirs.dirs` file says where each folder is,
so a person can move one:

```sh
# $XDG_CONFIG_HOME/agent-dirs.dirs
AGENT_HOME_DIR="$HOME/.agents"
AGENT_SKILLS_DIR="$HOME/.agents/skills"
AGENT_SUBAGENTS_DIR="$HOME/.agents/agents"
AGENT_RULES_DIR="$HOME/.agents/rules"
AGENT_MEMORY_DIR="$HOME/.agents/memory"
```

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
- **Sharing.** Every folder is shared by every agent. An agent that needs a
  part of its own uses a subfolder named after itself, such as
  `memory/claude/`.

Projects aren't in the list: `XDG_PROJECTS_DIR`, which xdg-user-dirs turned
on by default in 0.20 (April 2026), already says where they live
(decision 37).

### `~/Agents`: a visible view

`~/.agents` is hidden, but these are files a person opens and edits. So
`~/Agents` holds one link per folder, and nothing else (decision 32):

```
~/Agents/
  AGENTS.md  -> ~/.agents/AGENTS.md
  Skills     -> ~/.agents/skills
  Subagents  -> ~/.agents/agents
  Rules      -> ~/.agents/rules
  Memory     -> ~/.agents/memory
```

Linking each folder, rather than `~/Agents` as a whole, means other tools'
files in `~/.agents` (a `tasks/` folder, configuration) don't show up in
`~/Agents`, and the names can differ. `apply` makes the links and `undo`
removes them. Backups must include `~/.agents` itself, since many backup
tools don't follow links.

### `AGENTS.md`

[AGENTS.md](https://agents.md) is plain Markdown with no required fields.
`~/.agents/AGENTS.md` holds instructions for every agent and every project
(decision 36); a project's own is `<project>/AGENTS.md`, at its root, not in
`.agents/`, which is where agents look for it. An agent that reads only its
own file (`CLAUDE.md`, `GEMINI.md`) gets a link (section 4).

### Skills

A skill is a folder in the [Agent Skills][skills] format:

```
skills/
  pdf-processing/              # the folder's name is the skill's name
    SKILL.md                   # required: frontmatter, then instructions
    scripts/                   # optional: code the agent runs
    references/                # optional: read only when SKILL.md points to them
    assets/                    # optional: templates and files used in the output
  release-notes/
    SKILL.md                   # a skill can be one file
```

`SKILL.md` starts with YAML frontmatter: `name` (lowercase letters, digits
and hyphens, at most 64 characters, the same as the folder) and
`description` (what it does and when to use it, at most 1024 characters)
are required; `license`, `compatibility`, `metadata` and `allowed-tools`
are optional. An agent reads every skill's name and description at
startup, the rest of `SKILL.md` when it uses the skill, and `references/`
only when the instructions point there.

What this spec adds:

- **Flat.** Only `skills/<name>/SKILL.md` is found; a skill in a category
  folder (`skills/docs/pdf/`) is hidden. Group skills by a name prefix
  instead (`docs-pdf`, `docs-word`).
- **Relative paths, one level deep.** `SKILL.md` links to
  `references/forms.md` and `scripts/fill.py`, relative to the skill; those
  files don't link on.
- **Nothing per machine or per person.** No tokens, absolute paths or
  caches in a skill, so it's safe to commit and share. Those belong in the
  agent-baseDir.
- **A prompt is a skill.** A skill meant to run only when a person names
  it, not when an agent decides to, says so in its frontmatter (Claude
  Code's `disable-model-invocation: true`).

### Subagents

A subagent is one Markdown file with YAML frontmatter (at least `name` and
`description`; the tools it may use and the model are common optional
fields), followed by its instructions: `agents/<name>.md`. Claude Code and
Gemini CLI use this form; Codex defines subagents in TOML, so its files
can't be shared and stay in its own folder.

### Rules

A rule is a Markdown file of instructions about one topic, with
frontmatter saying when it applies:

```
rules/
  testing.md
  api/
    errors.md
    pagination.md
```

```markdown
---
description: How API handlers report errors
paths: ["src/api/**"]
---

Return problem+json bodies…
```

- **Subfolders are allowed** and are only for organising; every `.md` file
  under `rules/` is a rule.
- **`paths`** (globs, relative to the project) limits a rule to matching
  files, as in Claude Code. Without it, a rule always applies. Tools that
  name this differently (Cursor's `globs`, Copilot's `applyTo`) get a
  translated copy, not a link (section 4).
- **A `rules/` folder deeper in a project** (`<project>/src/api/.agents/rules/`)
  applies only below it, as nested `.cursor/rules` folders do.

### Memory

Agents keep memory as Markdown, and the one convention they share is a
`MEMORY.md` index with topic files beside it (Claude Code's auto memory,
OpenClaw's workspace). `AGENT_MEMORY_DIR` keeps to it (decision 33):

```
memory/
  shared/                  # read by every agent
    MEMORY.md              # index
    <topic>.md
    memory.<agent>.md      # optional: what an agent wants the others to know
  <agent>/                 # each agent's own files, named as the agent names them
  projects/<project-key>/  # personal memory for one project: same layout as memory/
```

Memory for a project comes in two kinds:

- **Personal** (the default): `memory/projects/<project-key>/`, outside the
  repository, as Claude Code keeps auto memory in
  `~/.claude/projects/<project>/memory/`. Agents write memory on their own,
  so it shouldn't land in a repository by default.
- **Shared** (opt in): `<project>/.agents/memory/`, committed, with the
  same layout. A note moves there on purpose once it's worth sharing.

An agent reads personal project memory, then shared project memory, then
`memory/shared/` and its own `memory/<agent>/`.

## 3. Scopes: managed, user, project, local

Agents read the same kind of file from several places. XDG already has two
of them, system (`XDG_CONFIG_DIRS`) and user (`XDG_CONFIG_HOME`); agents
add two more, inside a project (decision 35):

| Scope | Where | Shared with | What belongs there |
| :-- | :-- | :-- | :-- |
| Managed | `$XDG_CONFIG_DIRS` (`/etc/xdg/agents/` for the folders in section 2) | everyone on the machine | Configuration, skills and rules set by an administrator; read only |
| User | sections 1 and 2 above | no one | Everything an agent keeps |
| Project | `<project>/AGENTS.md`, `<project>/.agents/`, and the agent's own folder (`.claude/`, `.codex/`) | everyone with the repository | Committed: instructions, settings, skills, subagents, rules, shared memory |
| Local | Beside the project's files, named `*.local.*` | no one | Personal configuration, never committed |

- **Precedence**, highest first: local, project, user, managed. This is the
  XDG search order with two tiers on top. A project skill, subagent or rule
  with the same name as a user one replaces it; others add to the user's.
- **A project is a folder under `XDG_PROJECTS_DIR`** or any folder an agent
  is started in.
- **No state, cache, logs or credentials in a project** (decision 35).
  They go in the user's directories, keyed by project (section 1). A
  project's shared memory is the exception that's committed: see
  [Memory](#memory).
- **Local files are gitignored,** and never hold credentials or state.

## 4. Agents that read only their own folders

Until an agent reads `~/.agents`, `plan` points its own location at the
shared one. Where the format is the same, that's a link; where it isn't,
`plan` only reports it, since `apply` never writes an agent's files.

| Shared | Linked from (examples; each agent's layout has the exact paths) | Not linkable |
| :-- | :-- | :-- |
| `~/.agents/AGENTS.md` | Claude Code's `CLAUDE.md`, Gemini CLI's `GEMINI.md` in their configuration folders | |
| `~/.agents/skills` | Claude Code's `skills/` | |
| `~/.agents/agents` | Claude Code's and Gemini CLI's `agents/` | Codex (TOML) |
| `~/.agents/rules` | Claude Code's `rules/` | Cursor (`.mdc`, `globs`), Copilot (`applyTo`) |

Codex, Amp, Cline, Cursor and Gemini CLI already read `~/.agents/skills`
and need no link for it.

## What `xdg-agents` checks

- **User scope** (built): `status`, `plan` and `apply`, against each agent's
  layout in `agents/`.
- **agent-homeDir** (not built): layouts need a way to tag a path as one
  of the section 2 folders, so `status` reports it as such rather than as
  data, `plan` can link it (section 4), and `apply` can make `~/Agents`.
  `status` also reports configuration, state or cache found in `~/.agents`,
  which belong in the agent-baseDir.
- **Project and local scopes** (not built): roots in `agents/schema.ncl`
  get a `scope`, with defaults relative to the project. `status` checks the
  project it's run in (or each folder under `XDG_PROJECTS_DIR`) for a local
  file that isn't gitignored, or state and cache inside the repository.
  `apply` never moves a project's files.
- **Runtime** (not built): a root can't yet be placed under
  `$XDG_RUNTIME_DIR` (`docs/follow-ups.md`).

[basedir]: https://specifications.freedesktop.org/basedir-spec/latest/
[user-dirs]: https://www.freedesktop.org/wiki/Software/xdg-user-dirs/
[skills]: https://agentskills.io/specification
