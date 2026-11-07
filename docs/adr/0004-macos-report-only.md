# ADR-0004: On macOS, report only

**Status:** Accepted (decision 6 in `docs/decisions.md`, 2026-11-07)
**Date:** 2026-11-07
**Deciders:** Repository owner

## Context

The XDG Base Directory Specification is a freedesktop.org convention for
Linux and similar systems. macOS has its own: `~/Library/Application
Support`, `~/Library/Caches` and so on. The Claude desktop app follows it,
in `~/Library/Application Support/Claude`.

Moving such a folder into `~/.config` would fight the platform. Tools like
Time Machine and the system's storage management expect `~/Library`. But
some agents, such as Claude Code with `~/.claude`, ignore both conventions
on every platform, and macOS users still benefit from seeing that.

## Decision

On macOS, `xdg-agents` runs `status` (and `list`) as on Linux, but `plan`
and `apply` refuse to move anything, with a message saying why.

Layouts may give macOS locations (`default_macos`) so that `status` reports
correctly there.

## Consequences

- `status` works the same everywhere, which helps people with dotfiles
  shared across Linux and macOS.
- `plan` and `apply` need a platform check, with a test.
- Whether macOS should get its own targets (its `~/Library` folders, rather
  than XDG ones) is left open. Add it to `docs/decisions.md` if someone
  wants it.
- **Revisit when:** there's a clear macOS convention for these agents that
  isn't `~/Library`, or users ask for macOS moves.
