## Why

The current CLI is hand-parsed and exposes Azure DevOps plumbing as a required top-level namespace, so common commands such as config initialization require awkward forms like `adomi ado config init`. Moving to Cobra now gives the project a maintainable command tree, consistent help/usage behavior, and room to streamline user-facing commands after the MVP's standard-library parser constraint has served its purpose.

## What Changes

- Introduce Cobra for the `adomi` executable command tree.
- Promote general commands out of the Azure DevOps namespace so users can run `adomi config init` instead of `adomi ado config init`.
- Preserve Azure DevOps-specific commands under `adomi ado`, including `fetch`, `login`, `logout`, and `profiles list`.
- Keep existing successful fetch stdout behavior path-only, with diagnostics and prompts on stderr.
- Add command-level help and argument validation through Cobra instead of manual top-level switches.
- Deprecate `adomi ado config init` as a user-facing command; keep it only as a hidden compatibility alias during this migration so existing scripts do not fail immediately.

## Capabilities

### New Capabilities

- `cli-command-surface`: Defines the supported `adomi` command hierarchy, streamlined command names, help behavior, and compatibility expectations.

### Modified Capabilities

- None. There are no existing OpenSpec specs under `openspec/specs/`.

## Impact

- `go.mod` / `go.sum`: add the Cobra dependency and its transitive dependencies.
- `cmd/adomi/main.go`: continue to own process exit and error printing while delegating command execution to `internal/cli`.
- `internal/cli/root.go`: replace manual top-level dispatch with a Cobra root command while preserving the testable `Run(args, stdin, stdout, stderr)` boundary.
- `internal/cli/ado.go`: migrate Azure DevOps subcommands to Cobra command constructors and keep the existing fetch/login/logout/profile behavior.
- `internal/cli/ado_test.go` and `cmd/adomi/main_test.go`: update command invocation and usage expectations for the new surface.
- `docs/adomi_azure_devops_handover.md` or a follow-up doc: update command examples that currently describe `adomi ado config init` and the MVP-only "Avoid Cobra" dependency note.
