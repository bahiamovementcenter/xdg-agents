# Where OpenClaw (the personal assistant gateway) keeps its files, and where
# they belong under XDG.
#
# Almost everything in ~/.openclaw is data the docs say to back up: SQLite
# state, credentials, per-agent sessions and the workspace. So the root goes
# to $XDG_DATA_HOME/openclaw (OPENCLAW_STATE_DIR), the config file moves on
# its own to $XDG_CONFIG_HOME/openclaw (OPENCLAW_CONFIG_PATH), and logs and
# sandboxes are symlinked out to state.
#
# Written from the docs only. Retired files that doctor imports are left out
# on purpose, so status reports them as unclassified rather than as safe to
# delete: the docs say to keep them until doctor has run.
let { Layout, .. } = import "schema.ncl" in
{
  id = "openclaw",
  name = "OpenClaw",
  xdg_name = "openclaw",
  sources = [
    "https://docs.openclaw.ai/help/environment",
    "https://docs.openclaw.ai/gateway/configuration",
    "https://docs.openclaw.ai/gateway/security/secrets-and-storage",
    "https://docs.openclaw.ai/help/faq/where-things-live-on-disk",
    "https://docs.openclaw.ai/help/faq/logging-and-debugging",
    "https://docs.openclaw.ai/install/backups",
    "https://docs.openclaw.ai/install/uninstall",
    "https://docs.openclaw.ai/gateway/doctor/state-and-sessions",
  ],

  roots = {
    home = {
      default = "~/.openclaw",
      env = "OPENCLAW_STATE_DIR",
      category = 'data,
      note = "The gateway usually runs as a systemd user service (openclaw-gateway.service), which doesn't read your shell profile. Stop it before apply (systemctl --user stop openclaw-gateway), and give it the same variables (systemctl --user edit openclaw-gateway, then Environment= lines) before starting it again, or it starts afresh in an empty ~/.openclaw. If you installed with install-cli.sh, the openclaw command itself lives in ~/.openclaw/bin: moving the folder breaks it.",
    },
    logs = {
      default = "/tmp/openclaw",
      category = 'state,
    },
  },

  paths = [
    # Configuration: things you write, worth backing up and sharing
    {
      root = "home",
      path = "openclaw.json",
      kind = 'file,
      category = 'config,
      move = 'env,
      env = "OPENCLAW_CONFIG_PATH",
      sensitive = true,
      holds = "Settings (JSON5); may include gateway tokens and allowlists",
      note = "The variable names the file itself, not a folder. Earlier versions are kept in a .bak ring beside it.",
    },
    {
      root = "home",
      path = ".env",
      kind = 'file,
      category = 'config,
      move = 'with_root,
      sensitive = true,
      holds = "Provider API keys and other variables",
      note = "A file, so it can only move with the whole root.",
    },
    { root = "home", path = "skills", kind = 'dir, category = 'config, move = 'symlink, holds = "Shared skills" },

    # Data: must survive, worth backing up
    { root = "home", path = "state", kind = 'dir, category = 'data, move = 'with_root, sensitive = true, holds = "openclaw.sqlite: pairing, approved senders, MCP OAuth tokens, auth profiles", note = "The docs say to back up the database with its own tools, never by copying the live .sqlite, -wal and -shm files." },
    { root = "home", path = "agents", kind = 'dir, category = 'data, move = 'with_root, sensitive = true, holds = "Per agent: its SQLite state (auth profiles, sessions, transcripts), session archives, Codex home" },
    { root = "home", path = "credentials", kind = 'dir, category = 'data, move = 'with_root, sensitive = true, holds = "Channel credentials (WhatsApp and others), legacy OAuth imports" },
    { root = "home", path = "secrets.json", kind = 'file, category = 'data, move = 'with_root, sensitive = true, holds = "Secrets for file-backed secret references" },
    {
      root = "home",
      path = "workspace",
      kind = 'dir,
      category = 'data,
      move = 'with_root,
      holds = "The agent's workspace: AGENTS.md, SOUL.md, IDENTITY.md, USER.md, MEMORY.md, memory/",
      note = "Already moves with the root. agents.defaults.workspace or OPENCLAW_WORKSPACE_DIR can put it elsewhere; the docs suggest keeping it in a private git repository.",
    },

    # State: persists between runs, not worth backing up
    { root = "home", path = "logs", kind = 'dir, category = 'state, move = 'symlink, holds = "Health logs" },
    { root = "home", path = "sandboxes", kind = 'dir, category = 'state, move = 'symlink, sensitive = true, holds = "Tool sandbox workspaces, with copies of files read or written there" },
    {
      root = "logs",
      path = ".",
      kind = 'dir,
      category = 'state,
      move = 'fixed,
      sensitive = true,
      holds = "Daily log files (openclaw-YYYY-MM-DD.log)",
      note = "Lost on reboot where /tmp is cleared. The logging.file setting names one stable log file instead, such as $XDG_STATE_HOME/openclaw/openclaw.log; apply doesn't change settings.",
    },
  ],

  untracked_env = [
    { name = "OPENCLAW_HOME", reason = "Replaces $HOME for every OpenClaw default; OPENCLAW_STATE_DIR and OPENCLAW_CONFIG_PATH move the files more precisely" },
    { name = "OPENCLAW_PROFILE", reason = "Uses ~/.openclaw-<profile> instead; profiles aren't in this layout yet (docs/follow-ups.md)" },
    { name = "OPENCLAW_WORKSPACE_DIR", reason = "Moves the workspace, which already moves with the root" },
    { name = "OPENCLAW_DIAGNOSTICS_TIMELINE_PATH", reason = "A diagnostics file with no default" },
    { name = "OPENCLAW_GIT_DIR", reason = "The source checkout for development updates; not storage" },
  ],

  untracked_settings = [
    { name = "agents.defaults.workspace", reason = "Moves the workspace, which already moves with the root" },
    { name = "logging.file", reason = "Names a log file; noted on the /tmp/openclaw logs" },
    { name = "gateway.controlUi.root", reason = "Static files to serve; not storage" },
  ],
} | Layout
