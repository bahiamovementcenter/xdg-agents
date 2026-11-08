# Where OpenAI's Codex CLI keeps its files, and where they belong under XDG.
#
# CODEX_HOME moves all of ~/.codex, so the plan for Codex is the same shape
# as for Claude Code: point CODEX_HOME at $XDG_CONFIG_HOME/codex, then
# symlink the data, state and cache folders out of it. The docs say a
# CODEX_HOME that is set must already exist; apply moves the folder there
# before Codex next starts, so it does.
#
# Written from the docs only (developers.openai.com/codex now redirects to
# learn.chatgpt.com/docs). What the docs leave unnamed, such as the SQLite
# state file, is left out on purpose, so status reports it as unclassified
# (see docs/follow-ups.md).
let { Layout, .. } = import "schema.ncl" in
{
  id = "codex",
  name = "Codex CLI",
  xdg_name = "codex",
  sources = [
    "https://learn.chatgpt.com/docs/config-file/environment-variables",
    "https://learn.chatgpt.com/docs/config-file/config-advanced",
    "https://learn.chatgpt.com/docs/config-file/config-reference",
    "https://learn.chatgpt.com/docs/auth",
    "https://learn.chatgpt.com/docs/agent-configuration/agents-md",
    "https://learn.chatgpt.com/docs/agent-configuration/rules",
    "https://learn.chatgpt.com/docs/build-skills",
  ],

  roots = {
    home = {
      default = "~/.codex",
      env = "CODEX_HOME",
      category = 'config,
    },
    managed = {
      default = "/etc/codex",
      category = 'config,
    },
  },

  paths = [
    # Configuration: things you write, worth backing up and sharing
    { root = "home", path = "config.toml", kind = 'file, category = 'config, move = 'with_root, holds = "Settings" },
    { root = "home", path = "*.config.toml", kind = 'file, category = 'config, move = 'with_root, holds = "Profiles, laid over config.toml" },
    { root = "home", path = "hooks.json", kind = 'file, category = 'config, move = 'with_root, holds = "Hooks" },
    { root = "home", path = "AGENTS.md", kind = 'file, category = 'config, move = 'with_root, holds = "Instructions loaded in every session" },
    { root = "home", path = "AGENTS.override.md", kind = 'file, category = 'config, move = 'with_root, holds = "A temporary replacement for AGENTS.md" },
    {
      root = "home",
      path = "rules",
      kind = 'dir,
      category = 'config,
      move = 'with_root,
      holds = "Command rules",
      note = "Codex itself writes default.rules here when you allow a command in the TUI, but it's still configuration.",
    },
    {
      root = "home",
      path = "skills",
      kind = 'dir,
      category = 'config,
      move = 'with_root,
      holds = "Skills",
      note = "The environment variables page lists skills under CODEX_HOME, but the skills page names $HOME/.agents/skills as the user folder. ~/.agents is shared with other agents, so it isn't in this layout.",
    },

    # Data: must survive, worth backing up
    {
      root = "home",
      path = "auth.json",
      kind = 'file,
      category = 'data,
      move = 'with_root,
      sensitive = true,
      holds = "Login tokens, when credentials are stored in a file (cli_auth_credentials_store); otherwise the OS keyring",
      note = "The docs say to treat it like a password. A file, so it can only move with the whole root and stays in the configuration directory.",
    },
    {
      root = "home",
      path = "memories",
      kind = 'dir,
      category = 'data,
      move = 'symlink,
      holds = "Memories (the v1 store)",
      note = "Memories are off by default. The docs name this folder only to say that its presence selects the v1 store.",
    },

    # State: persists between runs, not worth backing up
    {
      root = "home",
      path = "history.jsonl",
      kind = 'file,
      category = 'state,
      move = 'with_root,
      sensitive = true,
      holds = "Session transcripts, unless history.persistence = \"none\"",
      note = "A file, so it can't be symlinked out; stays with the configuration. history.max_bytes caps its size.",
    },
    { root = "home", path = "sessions", kind = 'dir, category = 'state, move = 'symlink, sensitive = true, holds = "Sessions" },
    {
      root = "home",
      path = "log",
      kind = 'dir,
      category = 'state,
      move = 'symlink,
      holds = "Logs",
      note = "The log_dir setting can also move it (see settings).",
    },

    # Cache: regenerable, safe to delete
    {
      root = "home",
      path = "packages/standalone",
      kind = 'dir,
      category = 'cache,
      move = 'symlink,
      holds = "The standalone installer's package cache and metadata",
      note = "The docs call it a cache. CODEX_INSTALL_DIR (default ~/.local/bin) only moves the visible codex command, not this.",
    },

    # System-wide, set by an administrator: read only, never moved
    { root = "managed", path = "skills", kind = 'dir, category = 'config, move = 'fixed, holds = "Administrator skills" },
  ],

  settings = [
    {
      name = "log_dir",
      moves = "log",
      for_xdg = 'with_caveat,
      note = "Can point at $XDG_STATE_HOME/codex/log, but setting it at all also turns on the plaintext codex-tui.log.",
    },
  ],

  untracked_env = [
    { name = "CODEX_SQLITE_HOME", reason = "Moves the SQLite state database, whose file name the docs don't give (see docs/follow-ups.md). Relative paths resolve from the working directory." },
    { name = "CODEX_INSTALL_DIR", reason = "Where the codex command is installed; already ~/.local/bin by default" },
    { name = "CODEX_CA_CERTIFICATE", reason = "A certificate file to read; not storage" },
    { name = "OPENAI_IDENTITY_TOKEN_FILE", reason = "A token file to read; not storage" },
  ],

  untracked_settings = [
    { name = "sqlite_home", reason = "Moves the SQLite state database, like CODEX_SQLITE_HOME, and takes precedence over it" },
    { name = "model_instructions_file", reason = "A file to read instead of AGENTS.md; not storage" },
    { name = "experimental_compact_prompt_file", reason = "A file to read; not storage" },
    { name = "model_catalog_json", reason = "A file to read; not storage" },
  ],
} | Layout
