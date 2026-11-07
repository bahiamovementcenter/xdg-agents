# ADR-0002: Keep Go and go-nickel for now; don't wait for gleam-nickel-embedded

**Status:** Proposed
**Date:** 2026-11-07
**Deciders:** Repository owner

## Context

`xdg-agents` (goal in `AGENTS.md`) is a command-line tool that people run on
their own machines. Each run evaluates a handful of small Nickel files (one
layout per agent) once, then reads the file system and prints a report or a
plan.

Three ways to evaluate Nickel from the tool are on the table:

- **Go with [go-nickel][go-nickel]** (what the repository has now). Nickel's
  official Go bindings, which link the interpreter into the binary through
  cgo. Pre-release: the module is pinned to a pseudo-version
  (`v0.0.0-20251217025043-827dc613bd42`).
- **Gleam with [gleam-nickel-bindings][bindings]** (Gleam package `nickel`,
  v0.1.0, unreleased, not on Hex). Runs the `nickel` command-line program for
  every call and reads its JSON output. Measured at about 33 ms per trivial
  call. Needs Erlang/OTP 27+ and the `nickel` program installed. Has the
  fuller feature set: typechecking, contracts from files, customize-mode
  assignments, field focus, metadata queries, timeouts.
- **Gleam with gleam-nickel-embedded**, which does not exist yet. Its
  handoff document (`docs/handoff-gleam-nickel-embedded.md` in the bindings
  repository) plans a Rust NIF or a long-lived port program built on the
  `nickel-lang` crate. Open problems listed there: building and shipping
  native code with Gleam, no timeouts, a native crash stopping the whole VM,
  and `Context`/`Expr` not being `Send`. Its own plan starts eager-only and
  benchmarks before claiming speed gains.

## Decision

Keep Go and go-nickel. Don't wait for gleam-nickel-embedded.

If the project moves to Gleam later, use gleam-nickel-bindings (the CLI
package) at that point, not the embedded one: for this tool's workload the
embedded package's advantages don't apply.

## Options Considered

### Option A: Go + go-nickel (current)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low: builds and runs today |
| Cost | None now; cgo means per-platform builds |
| Distribution | One binary with the interpreter inside; users install nothing else |
| Familiarity | Go file-system and process APIs are mature and well documented |

**Pros:** single binary; no runtime or `nickel` program to install; strong
standard library for paths, symlinks, permissions and environment variables;
already in place.
**Cons:** go-nickel is pre-release and its API may change; cgo complicates
cross-compiling; fewer Nickel features than the Gleam CLI bindings (no
metadata query or typecheck entry point, as far as the C API goes).

### Option B: Gleam + gleam-nickel-bindings (CLI)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium: rewrite the scaffold; add file-system and environment packages |
| Cost | A port now, plus taking a dependency that is not on Hex yet |
| Distribution | Users need Erlang/OTP 27+ and the `nickel` program (Nix makes this easy; others must install both) |
| Familiarity | Same author as the bindings, so the API is known and changeable |

**Pros:** richest Nickel feature set (typecheck, `query` for field
documentation, which suits explaining *why* a file is in a category);
timeouts; a Nickel crash can't take the tool down; 33 ms per call is
irrelevant for a tool that evaluates a few files once per run.
**Cons:** two runtimes to install; an unreleased dependency; Gleam has no
file-system API in its standard library.

### Option C: Wait for gleam-nickel-embedded

| Dimension | Assessment |
|-----------|------------|
| Complexity | High, and in another repository that has not been started |
| Cost | Blocks this project on unbuilt work with open design questions |
| Distribution | Planned to start by requiring a Rust toolchain at build time; prebuilt binaries later |
| Familiarity | Rustler, NIFs and native packaging for Hex are new ground |

**Pros:** no `nickel` program to install (once prebuilt binaries exist); full
Nickel values (enum tags, exact rationals).
**Cons:** doesn't exist; no timeouts; a native crash stops the VM; fewer
features than the CLI bindings (no typecheck, contracts or field focus
without wrapper source); its main benefit, speed, doesn't matter for this
workload.

## Trade-off Analysis

The embedded package exists to make many evaluations per second cheap and to
keep full Nickel values. This tool evaluates a few small files once per run,
and its layouts only need strings, records and arrays, so neither benefit
applies. Waiting would block the project on a harder project for no gain
here. That rules out Option C.

Between A and B, the deciding question is distribution. The tool runs on
users' machines, and a single binary with nothing else to install is the
strongest argument for Go. Gleam's real advantages (the author knows the
bindings, richer Nickel features) are worth having but don't outweigh asking
every user to install Erlang and `nickel`, unless users are expected to get
the tool through Nix anyway.

The Nickel files (`schema.ncl`, `agents/*.ncl`) are the same whichever
language evaluates them, so the most valuable work, writing down each
agent's layout, carries over unchanged if this is reversed.

## Consequences

- **Easier:** shipping a single binary; starting the first slice now.
- **Harder:** using Nickel features go-nickel doesn't expose (field
  documentation via `query`); tracking go-nickel's pre-release API.
- **Revisit if:** the tool will be distributed only through Nix (then
  Option B's install cost disappears); Gleam becomes the preferred language
  across these projects; go-nickel's API changes break the build badly; or
  the tool needs Nickel field metadata that go-nickel can't provide.

## Action Items

1. [ ] Accept or reject this ADR.
2. [ ] Keep the Nickel layouts free of anything Go-specific, so a move to
   Gleam stays cheap.
3. [ ] Decide where agent layouts live (this repository or a separate one);
   see ADR-0003 when written.

[go-nickel]: https://github.com/nickel-lang/go-nickel
[bindings]: https://github.com/bahiamovementcenter/gleam-nickel-bindings
