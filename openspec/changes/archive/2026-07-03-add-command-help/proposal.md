## Why

Several `adomi` commands are optimized for machine use, but command-specific help is uneven: parent namespaces describe supported operations while action commands such as PR `ensure` do not expose their own usage and option guidance. Adding actionable help lets AI agents discover valid flags, required arguments, stdout/stderr behavior, and conservative write boundaries before executing Azure DevOps actions.

## What Changes

- Add explicit `--help`/`-h` output for command leaves and routed subcommands, including PR maintenance operations such as `adomi ado pr ensure`, `comment`, `reply`, `resolve`, and `reopen`.
- Keep help/usage text on stderr and keep successful command data on stdout.
- Document required arguments, valid flags, safety boundaries, and examples or concise guidance where that reduces accidental writes.
- Preserve existing command behavior for non-help execution and existing compatibility aliases.

## Capabilities

### New Capabilities

### Modified Capabilities
- `cli-command-surface`: command-specific help behavior changes for action commands so users and AI agents can inspect supported usage before running them.

## Impact

- CLI command definitions and manual help routing in `internal/cli/root.go`, especially the current Cobra command setup and routed `ado pr` namespace.
- PR command dispatch and argument parsing in `internal/cli/ado.go` for help requests that currently route through execution paths.
- CLI help tests in `internal/cli/ado_test.go` and possibly `internal/cli/root_test.go`/`internal/cli/agent_test.go` to lock down stderr-only help output and command-specific content.
- Canonical OpenSpec requirement coverage in `openspec/specs/cli-command-surface/spec.md`; no dependency or external API changes expected.
