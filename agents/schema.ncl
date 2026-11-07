# The contract every agent layout in this folder must satisfy. A layout says
# where an agent keeps its files, which XDG category each one belongs to, and
# how the agent can be pointed at the new location. The tool computes the
# target itself: $XDG_<CATEGORY>_HOME/<xdg_name>/<path>.
let Category
  | doc "XDG Base Directory categories (basedir 0.8)"
  = [| 'config, 'data, 'state, 'cache, 'runtime |]
in

# "~/..." (relative to $HOME) or an absolute path, so a layout never depends on
# the directory the tool runs from
let Location =
  std.contract.from_predicate (fun s =>
    std.is_string s
    && (std.string.is_match "^~/" s || std.string.is_match "^/" s)
  )
in

# Relative to its root, and unable to climb out of it. "*" stands for one
# path component (a project or session name).
let RelPath =
  std.contract.from_predicate (fun s =>
    std.is_string s
    && s != ""
    && !(std.string.is_match "^/" s)
    && !(std.string.is_match "(^|/)\\.\\.(/|$)" s)
  )
in

let Root = {
  default
    | Location
    | doc "Where the root is when its variable is unset",
  default_macos
    | Location
    | doc "Where the root is on macOS, if different"
    | optional,
  env
    | String
    | doc "The environment variable that moves the whole root, if any"
    | optional,
  category
    | Category
    | doc "Where the root itself goes when its variable is set: everything under it of another category must be symlinked out",
}
in

let Entry = {
  root
    | String
    | doc "Which of the layout's roots the path is under",
  path | RelPath,
  default_path
    | Location
    | doc "Where the file is when the root's variable is unset, if not under the root's default (for example ~/.claude.json)"
    | optional,
  kind | [| 'file, 'dir |],
  category | Category,
  move
    | [| 'with_root, 'env, 'symlink, 'fixed |]
    | doc m%"
      How it reaches its target:
      - 'with_root: moves when the root's variable is set
      - 'env: has a variable of its own (see `env`)
      - 'symlink: no setting reaches it, so it is moved and symlinked back.
        Only safe for directories: a program that saves a file by writing
        a new one and renaming it over the old replaces the symlink.
      - 'fixed: the agent only looks in one place; report it, never move it
      "%,
  env
    | String
    | doc "The environment variable for 'env"
    | optional,
  cleanup
    | [| 'swept, 'kept, 'removed_on_exit |]
    | doc "What the agent itself does with it over time"
    | optional,
  sensitive
    | Bool
    | doc "Holds credentials or private data: report the path, never read it"
    | default
    = false,
  legacy
    | Bool
    | doc "Only written by older versions of the agent"
    | default
    = false,
  documented
    | Bool
    | doc "False if seen on disk but not in the agent's documentation"
    | default
    = true,
  holds | String | doc "What it is, in a few words",
  note | String | optional,
}
in

let ConsistentEntry =
  std.contract.from_predicate (fun e =>
    (e.move != 'env || std.record.has_field "env" e)
    && !(e.move == 'symlink && e.kind == 'file)
  )
in

{
  Category = Category,
  Layout =
    std.contract.Sequence [
      {
        id | String | doc "The agent's identifier in this tool, such as claude-code",
        name | String | doc "The agent's name, for people",
        xdg_name
          | String
          | doc "The subdirectory created under each XDG directory, such as claude",
        sources
          | Array String
          | doc "Where the facts in this layout come from",
        roots | { _ : Root },
        paths | Array (std.contract.Sequence [Entry, ConsistentEntry]),
        untracked
          | Array { path | String, reason | String }
          | doc "Paths the agent's documentation lists that this layout leaves out on purpose, so the docs check (tools/docsdrift) doesn't report them"
          | default
          = [],
        untracked_env
          | Array { name | String, reason | String }
          | doc "Path-like environment variables the documentation lists that don't relocate the agent's files"
          | default
          = [],
      },
      # Every entry's root must be one of the layout's roots
      std.contract.from_predicate (fun l =>
        std.array.all (fun e => std.record.has_field e.root l.roots) l.paths
      ),
    ],
}
