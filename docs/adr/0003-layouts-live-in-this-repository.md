# ADR-0003: Keep agent layouts in this repository for now

**Status:** Accepted (decision 5 in `docs/decisions.md`, 2026-11-07)
**Date:** 2026-11-07
**Deciders:** Repository owner

## Context

The agent layouts (`agents/*.ncl` and `agents/schema.ncl`) say where each
agent keeps its files and where they belong under XDG. That data is useful
beyond this tool: an exporter could feed xdg-ninja, and dotfile or Home
Manager setups could read it. A sister repository, `xdg-agent-dirs`, was
suggested for it.

The format is still changing: `settings`, `untracked_env` and
`untracked_settings` were added this week, and `investigating` is planned
(decision 11).

## Decision

Keep the layouts in `agents/` in this repository. Move them to a separate
`xdg-agent-dirs` repository once something other than xdg-agents uses them,
or once their format has stopped changing.

## Consequences

- One pull request can change the schema, the layouts and the code that
  reads them together, and CI checks all three at once.
- The layouts must stay free of anything specific to this tool or to Go,
  so that moving them out later is just a move.
- **Revisit when:** a second consumer appears (the xdg-ninja exporter,
  decision 19, would count), or the schema goes a few months without a
  change.
