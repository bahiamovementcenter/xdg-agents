# ADR-0001: Define the development shell in a Nix flake, not Organist

**Status:** Proposed
**Date:** 2026-11-07
**Deciders:** Repository owner

## Context

The repository was bootstrapped from the `go-nickel` template of
[bahiamovementcenter/nix-dev-environment-templates][templates]. That template
defines the development shell in plain Nix (`flake.nix`) and uses Nickel only
for the application's own configuration (`config.ncl`, validated by
`schema.ncl` through [go-nickel][go-nickel]).

The same templates repository also offers [Organist][organist] templates, which
describe the shell in Nickel (`project.ncl`) instead of Nix. The question is
whether to move this project's shell to Organist so that the shell and the
application configuration are both written in Nickel.

Forces at play:

- **go-nickel needs cgo.** It links Nickel's interpreter as a static library,
  so the shell must provide a C compiler and set `CGO_ENABLED=1`. The current
  flake uses `mkShell` (not `mkShellNoCC`) for exactly this reason.
- **Organist's base shell has no C compiler.** The templates repo's
  `organist/modules/common.ncl` sets `stdenv` to `stdenvNoCC`. A compiler can
  be added as a package (the `java` module adds `gcc`), but it is extra work
  that the flake already does.
- **There is no Organist Go module.** The templates repo has Organist modules
  for `base`, `elixir`, `java`, `nickel` and `python` only. Go, gotools and
  golangci-lint would have to be imported by hand in `project.ncl`, or a `go`
  module written in the templates repo first.
- **Organist evaluates with import-from-derivation.** Per the templates repo,
  `nix flake check --all-systems` cannot evaluate Organist shells for other
  systems from one machine, and they fail on Determinate Nix unless lazy trees
  are turned off (`NIX_CONFIG='lazy-trees = false'`).
- **The project's purpose is not yet defined.** Nothing about what
  `xdg-agents` does depends on how its dev shell is written.

## Decision

Keep the development shell in `flake.nix`, as the `go-nickel` template
provides it. Use Nickel for the application's configuration and its contracts
only.

## Options Considered

### Option A: Plain Nix flake (current)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low: already in place, builds and runs |
| Cost | None: no further work |
| Scalability | Adequate: more packages are added to one list in `flake.nix` |
| Team familiarity | Standard flake shape, same as every other template in the templates repo |

**Pros:**
- Works today: `nix develop` then `go build -o xdg-agents . && ./xdg-agents`.
- C compiler and `CGO_ENABLED=1` already handled.
- `nix flake check --all-systems` works from one machine.
- No dependency on Organist or on the templates repo at evaluation time.

**Cons:**
- Two languages in the repo: Nix for the shell, Nickel for the app config.
- No typed contracts on the shell definition itself.

### Option B: Organist `project.ncl`, Go packages imported by hand

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium: new flake wiring (`organist`, `dev-templates` inputs), hand-written package imports, C compiler and cgo setup |
| Cost | A few hours to port and verify parity with the current shell |
| Scalability | Good for combining languages, through module merging |
| Team familiarity | Less common than plain flakes; Organist is pre-1.0 |

**Pros:**
- Shell and app config both in Nickel, with contracts on both.
- Misspelled options and conflicting settings fail at evaluation.

**Cons:**
- Must add a C compiler and `CGO_ENABLED=1` by hand.
- Import-from-derivation: no cross-system checks from one machine.
- Needs `lazy-trees = false` on Determinate Nix.
- Adds two flake inputs and a dependency on Organist's release cadence.

### Option C: Write an Organist `go` module in the templates repo, then adopt it here

| Dimension | Assessment |
|-----------|------------|
| Complexity | High: new module, parity check against the `go-nickel` template (`check-organist`), then adoption here |
| Cost | Most work of the three; spread across two repositories |
| Scalability | Best long term if several Organist projects use Go |
| Team familiarity | Same as Option B |

**Pros:**
- Reusable: every future Go project can merge `modules.go {}`.
- Keeps `project.ncl` here short, like the other Organist templates.

**Cons:**
- Everything in Option B's cons, plus work in a second repository before this
  project can start.
- Delays the actual work on `xdg-agents` for a benefit it does not yet need.

## Trade-off Analysis

The only thing Organist offers this project is writing the shell in Nickel
instead of Nix. In exchange it adds import-from-derivation, a Determinate Nix
workaround, two flake inputs, and manual setup for the C compiler that
go-nickel needs. The shell is small (Go, gotools, golangci-lint, Nickel, nls,
nixfmt), so the safety Nickel contracts would add to it is small too.

Option C is the right shape if Organist becomes the standard across several
Go projects, but that is a decision for the templates repository, not this
one, and it should not block defining what `xdg-agents` does.

The decision is cheap to reverse: the shell is one file, and the application
code and its Nickel config are unaffected by which option is chosen.

## Consequences

- **Easier:** starting work on the tool itself right away; cross-system
  `nix flake check`; using any Nix installation without extra settings.
- **Harder:** nothing current. Someone who wants a single configuration
  language for the whole repo does not get it.
- **Revisit if:** the templates repo gains an Organist `go` module, the shell
  grows enough that typed contracts on it would catch real mistakes, or
  Organist removes the import-from-derivation and lazy-trees limitations.

## Action Items

1. [ ] Accept or reject this ADR (change **Status** above).
2. [ ] Write the project's goal and first "done" check (ADR-0002 or an
   `AGENTS.md` overview), since that, not the shell, decides the next work.
3. [ ] Optional, in the templates repo: open an issue for an Organist `go`
   module (Option C) if Organist is meant to become the default there.

[templates]: https://github.com/bahiamovementcenter/nix-dev-environment-templates
[go-nickel]: https://github.com/nickel-lang/go-nickel
[organist]: https://github.com/nickel-lang/organist
