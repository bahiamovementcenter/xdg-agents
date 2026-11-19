# xdg-agents

Make AI coding agents (Claude Code, Codex and others) keep their files where the
[XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/)
says they belong, instead of in their own dot-folders in your home directory
(`~/.claude`, `~/.codex`, ...).

It works in three steps, each usable on its own:

1. **Report**: where each installed agent's files are now, what kind of file each one is
   (config, data, state, cache, runtime), and where the spec says it should be.
2. **Plan**: exactly what would move where, and which environment variables or settings
   would point the agent at the new locations.
3. **Apply**: carry out a plan, reversibly, only when asked.

Each agent's layout is written down in Nickel under [`agents/`](agents/).

## Usage

From the development shell (`nix develop`):

```sh
go build -o xdg-agents .
./xdg-agents status   # report this machine (read-only)
./xdg-agents plan     # what would move where (read-only)
./xdg-agents list     # the layouts, and which agents are installed
```

See [AGENTS.md](AGENTS.md) for build, test and contribution details, and [`docs/`](docs/)
for the spec, decisions and status.
