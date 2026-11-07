# Where the Claude desktop app keeps its files on Linux, and where they belong
# under XDG.
#
# Facts come from a listing of a real ~/.config/Claude (names and types only,
# Claude desktop with Claude Code 2.1.280, 2026-11-07), not from
# documentation: Anthropic doesn't document this folder, so every entry is
# documented = false. The desktop app is an Electron (Chromium) app, so most
# entries are Chromium profile files with well-known purposes.
#
# The app is already under $XDG_CONFIG_HOME, but keeps everything there:
# caches, logs, crash reports, web storage and lock files. No setting moves
# these, so the plan is to symlink directories out to the cache, state and
# data directories while the app isn't running.
#
# Left out on purpose, so `status` reports them as unclassified until someone
# finds out what they hold: ant-device-registry.json, ant-did,
# buddy-tokens.json, ca-bundle.pem, ccd-ids.json, chrome-import-sites.json,
# config.json, cowork-enabled-cli-ops.json, extensions-blocklist.json, fcache,
# git-shadow, os-entry-points.json, plan-usage-history.json, spaces-present.
#
# ccd-environment-config.json, git-worktrees.json and remote-control-state.json
# were placed by a probe (followed by key names only, via jq paths): saving a local environment named "xdgprobe" for a
# folder ~/xdgprobe-folder, then listing which files were created or mention
# the name (names only, never contents).
#
# Linux only. On macOS the app uses ~/Library/Application Support/Claude,
# which is the platform's own convention; whether xdg-agents should touch
# macOS at all is an open decision (no ADR yet).
let { Layout, .. } = import "schema.ncl" in
{
  id = "claude-desktop",
  name = "Claude desktop app",
  xdg_name = "Claude",
  sources = [
    "Listing of ~/.config/Claude on Linux, 2026-11-07 (names and types only)",
    "Saved-environment probe on Linux, 2026-11-07 (which files were created or mention the environment's name)",
    "https://code.claude.com/docs/en/desktop",
    "https://code.claude.com/docs/en/desktop-linux",
  ],

  roots = {
    home = {
      default = "~/.config/Claude",
      category = 'config,
    },
  },

  paths = [
    # Configuration
    { root = "home", path = "claude_desktop_config.json", kind = 'file, category = 'config, move = 'with_root, documented = false, sensitive = true, holds = "MCP servers for the chat app, and app preferences", note = "Named in the docs, but not its location. Top-level keys: mcpServers (env blocks can hold API keys), coworkUserFilesPath, and preferences: UI state, per-account toggles, Cowork trusted folders and Remote Control pinned folders. Mixes configuration with UI state; kept as configuration." },
    {
      root = "home",
      path = "ccd-environment-config.json",
      kind = 'file,
      category = 'config,
      move = 'with_root,
      documented = false,
      sensitive = true,
      holds = "Environment variables for local sessions, encrypted",
      note = "Created when an environment was first saved. A JSON object with one key, envVars, whose value is a single encrypted string: a variable saved in the probe was not in the file as plain text, matching the docs' 'stored encrypted on your machine'. The environments themselves (names, folders) aren't here; see Local Storage.",
    },
    { root = "home", path = "mcp-user-tool-toggles.json", kind = 'file, category = 'config, move = 'with_root, documented = false, holds = "Which MCP tools are turned on" },
    { root = "home", path = "Preferences", kind = 'file, category = 'config, move = 'with_root, documented = false, holds = "Chromium profile preferences" },

    # Data: must survive
    { root = "home", path = "Cookies", kind = 'file, category = 'data, move = 'with_root, documented = false, sensitive = true, holds = "Chromium cookies, including the claude.ai sign-in", note = "A file, so it stays put; its -journal file goes with it." },
    { root = "home", path = "Local Storage", kind = 'dir, category = 'data, move = 'symlink, documented = false, sensitive = true, holds = "Chromium local storage for claude.ai", note = "Most likely where saved local and SSH environments (names and folders) are kept: the only store besides logs and session records where the probe's environment name appeared." },
    { root = "home", path = "IndexedDB", kind = 'dir, category = 'data, move = 'symlink, documented = false, sensitive = true, holds = "Chromium IndexedDB for claude.ai" },
    { root = "home", path = "WebStorage", kind = 'dir, category = 'data, move = 'symlink, documented = false, holds = "Chromium storage quota database" },
    { root = "home", path = "blob_storage", kind = 'dir, category = 'data, move = 'symlink, documented = false, holds = "Chromium blob storage" },
    { root = "home", path = "Partitions", kind = 'dir, category = 'data, move = 'symlink, documented = false, holds = "Separate Chromium profiles, such as the Cowork launch preview" },
    { root = "home", path = "claude-code", kind = 'dir, category = 'data, move = 'symlink, documented = false, holds = "The Claude Code version the app runs, one folder per version", note = "Like the native installer's ~/.local/share/claude, which is already data." },
    { root = "home", path = "claude-code-vm", kind = 'dir, category = 'data, move = 'symlink, documented = false, holds = "Claude Code for the Cowork VM, per version" },
    { root = "home", path = "vm_bundles", kind = 'dir, category = 'data, move = 'symlink, documented = false, holds = "The Cowork VM image" },

    # State: persists, not worth backing up
    { root = "home", path = "claude-code-sessions", kind = 'dir, category = 'state, move = 'symlink, documented = false, holds = "One record per Claude Code session started from the app, per account and organization", note = "Transcripts themselves are in ~/.claude/projects (see claude-code.ncl)." },
    { root = "home", path = "local-agent-mode-sessions", kind = 'dir, category = 'state, move = 'symlink, documented = false, holds = "Local agent (Cowork) sessions, per account" },
    { root = "home", path = "scratch-workspaces", kind = 'dir, category = 'state, move = 'symlink, documented = false, holds = "Per-account scratch workspaces" },
    { root = "home", path = "Session Storage", kind = 'dir, category = 'state, move = 'symlink, documented = false, holds = "Chromium session storage" },
    { root = "home", path = "logs", kind = 'dir, category = 'state, move = 'symlink, documented = false, holds = "App, MCP, SSH, Remote Control and VM logs" },
    { root = "home", path = "Crashpad", kind = 'dir, category = 'state, move = 'symlink, documented = false, holds = "Crash reports" },
    { root = "home", path = "sentry", kind = 'dir, category = 'state, move = 'symlink, documented = false, holds = "Queued error reports" },
    { root = "home", path = "remote-control-state.json", kind = 'file, category = 'state, move = 'with_root, documented = false, holds = "Folders offered through Remote Control", note = "Machine-specific: the folders are paths on this computer." },
    { root = "home", path = "git-worktrees.json", kind = 'file, category = 'state, move = 'with_root, documented = false, holds = "Worktrees the app created, per folder" },
    { root = "home", path = "window-state.json", kind = 'file, category = 'state, move = 'with_root, documented = false, holds = "Window size and position" },
    { root = "home", path = "Local State", kind = 'file, category = 'state, move = 'with_root, documented = false, holds = "Chromium browser-wide state" },
    { root = "home", path = "Network Persistent State", kind = 'file, category = 'state, move = 'with_root, documented = false, holds = "Chromium network state" },
    { root = "home", path = "TransportSecurity", kind = 'file, category = 'state, move = 'with_root, documented = false, holds = "Chromium HSTS state" },
    { root = "home", path = "Trust Tokens", kind = 'file, category = 'state, move = 'with_root, documented = false, holds = "Chromium trust tokens" },
    { root = "home", path = "DIPS", kind = 'file, category = 'state, move = 'with_root, documented = false, holds = "Chromium bounce-tracking state" },
    { root = "home", path = "declarative_performance_observer.db", kind = 'file, category = 'state, move = 'with_root, documented = false, holds = "Chromium performance data" },

    # Cache: regenerable
    { root = "home", path = "Cache", kind = 'dir, category = 'cache, move = 'symlink, documented = false, holds = "Chromium HTTP cache" },
    { root = "home", path = "Code Cache", kind = 'dir, category = 'cache, move = 'symlink, documented = false, holds = "Compiled JavaScript and WebAssembly" },
    { root = "home", path = "GPUCache", kind = 'dir, category = 'cache, move = 'symlink, documented = false, holds = "GPU shader cache" },
    { root = "home", path = "DawnGraphiteCache", kind = 'dir, category = 'cache, move = 'symlink, documented = false, holds = "GPU shader cache" },
    { root = "home", path = "DawnWebGPUCache", kind = 'dir, category = 'cache, move = 'symlink, documented = false, holds = "WebGPU shader cache" },
    { root = "home", path = "Shared Dictionary", kind = 'dir, category = 'cache, move = 'symlink, documented = false, holds = "Chromium compression dictionaries" },
    { root = "home", path = "Dictionaries", kind = 'dir, category = 'cache, move = 'symlink, documented = false, holds = "Downloaded spell-check dictionaries" },

    # Runtime: only while the app runs. Chromium creates these on start and
    # uses them to find a running copy, so they must never be moved.
    { root = "home", path = "SingletonLock", kind = 'file, category = 'runtime, move = 'fixed, documented = false, holds = "Symlink naming the running instance" },
    { root = "home", path = "SingletonSocket", kind = 'file, category = 'runtime, move = 'fixed, documented = false, holds = "Symlink to the running instance's socket" },
    { root = "home", path = "SingletonCookie", kind = 'file, category = 'runtime, move = 'fixed, documented = false, holds = "Symlink holding the instance cookie" },
  ],
} | Layout
