# Where Claude Code keeps its files, and where they belong under XDG.
#
# CLAUDE_CONFIG_DIR moves ~/.claude and ~/.claude.json together, so the plan
# for Claude Code is: point CLAUDE_CONFIG_DIR at $XDG_CONFIG_HOME/claude, then
# symlink the state, cache and data directories out of it. Entries are ordered
# as the tool should report them: config, data, state, cache, runtime.
let { Layout, .. } = import "schema.ncl" in
{
  id = "claude-code",
  name = "Claude Code",
  xdg_name = "claude",
  sources = [
    "https://code.claude.com/docs/en/claude-directory",
    "https://code.claude.com/docs/en/authentication",
    "https://code.claude.com/docs/en/managed-settings",
    "https://code.claude.com/docs/en/env-vars",
    "https://code.claude.com/docs/en/settings-reference",
    "https://code.claude.com/docs/en/memory",
    "https://www.schemastore.org/claude-code-settings.json",
    "https://code.claude.com/docs/en/hooks-guide",
  ],

  roots = {
    home = {
      default = "~/.claude",
      env = "CLAUDE_CONFIG_DIR",
      category = 'config,
    },
    tmp = {
      default = "/tmp/claude-<uid>",
      env = "CLAUDE_CODE_TMPDIR",
      category = 'runtime,
    },
    managed = {
      default = "/etc/claude-code",
      default_macos = "/Library/Application Support/ClaudeCode",
      category = 'config,
    },
  },

  paths = [
    # Configuration: things you write, worth backing up and sharing
    { root = "home", path = "settings.json", kind = 'file, category = 'config, move = 'with_root, holds = "Permissions, hooks, environment variables, model defaults" },
    { root = "home", path = "CLAUDE.md", kind = 'file, category = 'config, move = 'with_root, holds = "Instructions loaded in every session" },
    { root = "home", path = "rules", kind = 'dir, category = 'config, move = 'with_root, holds = "Topic-scoped instructions" },
    { root = "home", path = "skills", kind = 'dir, category = 'config, move = 'with_root, holds = "Skills (one folder each)" },
    { root = "home", path = "commands", kind = 'dir, category = 'config, move = 'with_root, holds = "Single-file prompts, same mechanism as skills" },
    { root = "home", path = "agents", kind = 'dir, category = 'config, move = 'with_root, holds = "Subagent definitions" },
    { root = "home", path = "workflows", kind = 'dir, category = 'config, move = 'with_root, holds = "Workflow scripts, each a /<name> command" },
    { root = "home", path = "output-styles", kind = 'dir, category = 'config, move = 'with_root, holds = "Custom output styles" },
    { root = "home", path = "themes", kind = 'dir, category = 'config, move = 'with_root, holds = "Custom color themes" },
    { root = "home", path = "keybindings.json", kind = 'file, category = 'config, move = 'with_root, holds = "Keyboard shortcuts" },
    {
      root = "home",
      path = "hooks",
      kind = 'dir,
      category = 'config,
      move = 'with_root,
      documented = false,
      holds = "Hook scripts, by convention",
      note = "Hooks are registered under \"hooks\" in settings.json and can live anywhere; this folder is only the docs' example location. Scripts referenced by absolute path must be updated if moved.",
    },
    {
      root = "home",
      path = ".claude.json",
      default_path = "~/.claude.json",
      kind = 'file,
      category = 'config,
      move = 'with_root,
      sensitive = true,
      holds = "App state, account details, UI toggles, personal MCP servers",
      note = "The only place \"global config\" settings (autoConnectIde, diffTool and others) are read from, so it is configuration, but it also holds per-project state and account details and can't be split. Kept with the configuration because a file can't safely be symlinked (see schema.ncl).",
    },

    # Data: must survive, worth backing up
    {
      root = "home",
      path = ".credentials.json",
      kind = 'file,
      category = 'data,
      move = 'with_root,
      sensitive = true,
      holds = "Login credentials (Linux; macOS uses the Keychain)",
      note = "Can only move with the whole root, so it stays in the configuration directory. Mode 0600; never copy it anywhere world-readable.",
    },
    {
      root = "home",
      path = "plugins",
      kind = 'dir,
      category = 'data,
      move = 'env,
      env = "CLAUDE_CODE_PLUGIN_CACHE_DIR",
      cleanup = 'kept,
      holds = "Marketplaces, installed plugins and their data",
      note = "Despite its name, the variable moves the whole plugins folder, not only a cache. The docs say not to delete it.",
    },
    { root = "home", path = "agent-memory", kind = 'dir, category = 'data, move = 'symlink, cleanup = 'kept, holds = "Persistent subagent memory" },

    # State: persists between runs, not worth backing up
    {
      root = "home",
      path = "projects",
      kind = 'dir,
      category = 'state,
      move = 'symlink,
      cleanup = 'swept,
      sensitive = true,
      holds = "Session transcripts per project (plaintext), swept after cleanupPeriodDays (30 by default)",
      note = "Also holds projects/*/memory (auto memory, kept), which is data: only the autoMemoryDirectory setting can move it on its own (see settings). With CLAUDE_CONFIG_DIR set, CLAUDE_CODE_PROJECT_DIR_NAME chooses a session's subfolder name here; it doesn't move anything.",
    },
    { root = "home", path = "history.jsonl", kind = 'file, category = 'state, move = 'with_root, cleanup = 'kept, sensitive = true, holds = "Prompt history (up-arrow, Ctrl+R)", note = "A file, so it can't be symlinked out; stays with the configuration." },
    { root = "home", path = "file-history", kind = 'dir, category = 'state, move = 'symlink, cleanup = 'swept, holds = "Pre-edit snapshots for checkpoint restore" },
    { root = "home", path = "plans", kind = 'dir, category = 'state, move = 'symlink, cleanup = 'swept, holds = "Plan-mode files" },
    { root = "home", path = "debug", kind = 'dir, category = 'state, move = 'symlink, cleanup = 'swept, holds = "Debug logs, only in debug mode", note = "CLAUDE_CODE_DEBUG_LOGS_DIR overrides the log file path (a file, not a folder)." },
    { root = "home", path = "paste-cache", kind = 'dir, category = 'state, move = 'symlink, cleanup = 'swept, holds = "Pasted content" },
    { root = "home", path = "uploads", kind = 'dir, category = 'state, move = 'symlink, holds = "Uploaded files, per session" },
    { root = "home", path = "session-env", kind = 'dir, category = 'state, move = 'symlink, holds = "Per-session environment" },
    { root = "home", path = "tasks", kind = 'dir, category = 'state, move = 'symlink, holds = "Per-session tasks" },
    { root = "home", path = "jobs", kind = 'dir, category = 'state, move = 'symlink, cleanup = 'kept, holds = "Background session state" },
    { root = "home", path = "daemon", kind = 'dir, category = 'state, move = 'symlink, cleanup = 'kept, holds = "Background session state" },
    { root = "home", path = "backups", kind = 'dir, category = 'state, move = 'symlink, cleanup = 'swept, holds = "Earlier copies of .claude.json" },
    { root = "home", path = "usage-data", kind = 'dir, category = 'state, move = 'symlink, holds = "Usage data" },
    { root = "home", path = "stats-cache.json", kind = 'file, category = 'state, move = 'with_root, cleanup = 'kept, holds = "Token and cost totals for /usage", note = "Called a cache, but the totals can't be rebuilt once transcripts are swept." },
    { root = "home", path = "feedback", kind = 'dir, category = 'state, move = 'symlink, cleanup = 'swept, holds = "Feedback drafts" },
    { root = "home", path = "feedback-bundles", kind = 'dir, category = 'state, move = 'symlink, holds = "Archives written by /share" },
    { root = "home", path = "dev-mods", kind = 'dir, category = 'state, move = 'symlink, holds = "Mods under development, per session" },
    { root = "home", path = "skills/.trash", kind = 'dir, category = 'state, move = 'with_root, cleanup = 'swept, holds = "Removed synced skills, recoverable", note = "Inside skills/, so it stays with the configuration." },
    { root = "home", path = "plugins/.trash", kind = 'dir, category = 'state, move = 'env, env = "CLAUDE_CODE_PLUGIN_CACHE_DIR", cleanup = 'swept, holds = "Removed synced plugins, recoverable", note = "Inside plugins/, so it moves with it." },
    { root = "home", path = ".last-cleanup", kind = 'file, category = 'state, move = 'with_root, documented = false, holds = "When the cleanup sweep last ran" },
    { root = "home", path = "todos", kind = 'dir, category = 'state, move = 'symlink, legacy = true, holds = "Old to-do lists" },
    { root = "home", path = "logs", kind = 'dir, category = 'state, move = 'symlink, legacy = true, holds = "Old logs" },

    # Cache: regenerable, safe to delete
    { root = "home", path = "cache", kind = 'dir, category = 'cache, move = 'symlink, holds = "Fetched data such as the changelog" },
    { root = "home", path = "remote-settings.json", kind = 'file, category = 'cache, move = 'with_root, holds = "Settings fetched from the server" },
    { root = "home", path = "policy-limits.json", kind = 'file, category = 'cache, move = 'with_root, holds = "Fetched policy limits" },
    { root = "home", path = "mcp-needs-auth-cache.json", kind = 'file, category = 'cache, move = 'with_root, documented = false, holds = "Which MCP servers need sign-in" },
    { root = "home", path = "statsig", kind = 'dir, category = 'cache, move = 'symlink, legacy = true, holds = "Old feature-flag cache" },
    { root = "home", path = "image-cache", kind = 'dir, category = 'cache, move = 'symlink, legacy = true, holds = "Old image cache" },

    # Runtime: only while a session runs
    { root = "home", path = "shell-snapshots", kind = 'dir, category = 'runtime, move = 'symlink, cleanup = 'removed_on_exit, holds = "Shell environment snapshots", note = "Removed on clean exit, like sessions/; leftovers from crashes are swept. Runtime by decision 23." },
    { root = "home", path = "sessions", kind = 'dir, category = 'runtime, move = 'symlink, cleanup = 'removed_on_exit, holds = "One small file per running session" },
    { root = "tmp", path = ".", kind = 'dir, category = 'runtime, move = 'env, env = "CLAUDE_CODE_TMPDIR", holds = "Scratchpads, pasted and attached images (images/ per session)", note = "The variable names the parent: Claude Code appends claude-<uid>/ to it, so CLAUDE_CODE_TMPDIR=$XDG_RUNTIME_DIR gives $XDG_RUNTIME_DIR/claude-<uid>/, not .../claude/. Unset, the parent is os.tmpdir() on Linux ($TMPDIR, else /tmp) and /tmp on macOS." },

    # System-wide, set by an administrator: read only, never moved
    { root = "managed", path = "managed-settings.json", kind = 'file, category = 'config, move = 'fixed, holds = "Managed settings" },
    { root = "managed", path = "managed-settings.d", kind = 'dir, category = 'config, move = 'fixed, holds = "Managed settings drop-ins" },
    { root = "managed", path = "managed-mcp.json", kind = 'file, category = 'config, move = 'fixed, holds = "Managed MCP servers" },
  ],

  settings = [
    {
      name = "autoMemoryDirectory",
      moves = "projects/*/memory",
      for_xdg = 'with_caveat,
      note = "Takes an absolute or ~/ path, so it can point into $XDG_DATA_HOME. But it names one folder: set in user settings, every project shares a single auto memory folder instead of one each. To keep them separate, it has to be set per project (in .claude/settings.local.json).",
    },
    {
      name = "plansDirectory",
      moves = "plans",
      for_xdg = 'no,
      note = "Resolved relative to the project root, so it can only put plans inside the project, never under $XDG_STATE_HOME.",
    },
  ],

  untracked_env = [
    { name = "CLAUDE_CODE_DEBUG_LOGS_DIR", reason = "Sets the debug log file, not a folder; noted on debug" },
    { name = "CLAUDE_JOB_DIR", reason = "Set by Claude Code for each background session, pointing into jobs/" },
    { name = "CLAUDE_CODE_PLUGIN_DIRS", reason = "Extra plugin folders to load, like --plugin-dir; moves nothing" },
    { name = "CLAUDE_CODE_PLUGIN_SEED_DIR", reason = "Read-only plugin folders bundled into images; moves nothing" },
    { name = "CLAUDE_ENV_FILE", reason = "A script run before each Bash command; not storage" },
    { name = "CLAUDE_CLIENT_PRESENCE_FILE", reason = "A file another tool creates; not storage" },
    { name = "CLAUDE_CODE_GIT_BASH_PATH", reason = "Windows: where Git Bash is; not storage" },
    { name = "CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR", reason = "A Bash tool option; not a path" },
    { name = "CLAUDE_CODE_DISABLE_BG_STABLE_PATH", reason = "macOS native installer option; the app it names is already in ~/.local/share/claude" },
  ],
} | Layout
