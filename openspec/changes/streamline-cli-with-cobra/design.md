## Context

The current executable delegates from `cmd/adomi/main.go` to `cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)`, and `main` prints returned errors to stderr before exiting non-zero (`cmd/adomi/main.go:10`). `internal/cli/root.go` currently performs manual top-level dispatch with only the `ado` command accepted (`internal/cli/root.go:59`), and `internal/cli/ado.go` performs another manual switch for `fetch`, `login`, `logout`, `profiles list`, and `config init` (`internal/cli/ado.go:19`). The existing tests exercise these raw argument slices directly, including `adomi ado config init` (`internal/cli/ado_test.go:588`) and the current root usage text (`cmd/adomi/main_test.go:31`).

The original handover explicitly said to avoid Cobra for the MVP (`docs/adomi_azure_devops_handover.md:210`), but that was scoped to the MVP. The implemented command set has now grown beyond the original MVP command list (`docs/adomi_azure_devops_handover.md:77`), and config is no longer Azure DevOps-only plumbing from the user's point of view.

## Goals / Non-Goals

**Goals:**

- Replace manual argument dispatch with a Cobra command tree while keeping `cli.Run(args, stdin, stdout, stderr)` as the testable boundary.
- Make `adomi config init [--global]` the supported config command.
- Preserve current Azure DevOps command behavior for `adomi ado fetch`, `adomi ado login`, `adomi ado logout`, and `adomi ado profiles list`.
- Keep successful `fetch` stdout as only the exported path, because tests enforce that contract (`internal/cli/ado_test.go:284`).
- Keep config file creation semantics, permissions, repo/global scope, and overwrite protection unchanged (`internal/cli/ado.go:198`).

**Non-Goals:**

- Do not change the YAML config schema, config path precedence, or PAT/keyring model.
- Do not change Azure DevOps fetch, attachment, traversal, or export behavior.
- Do not add interactive config editing beyond the existing commented template creation.
- Do not introduce shell completions or persistent CLI configuration in this change.

## Decisions

1. Use Cobra only at the CLI boundary.
   - Rationale: `internal/cli/root.go:59` and `internal/cli/ado.go:19` are the only command dispatch layers; the domain work is already isolated behind `Runner` methods and injected `Dependencies`.
   - Alternative considered: keep manual parsing and add more switches. That would preserve zero dependencies, but it continues duplicating usage, validation, and hierarchy behavior as the CLI grows.

2. Keep `Run(args, stdin, stdout, stderr)` and `Runner` dependency injection.
   - Rationale: the test suite creates `Runner{deps: ...}` fakes extensively, such as config loading and export fakes in `internal/cli/ado_test.go:289`, so the Cobra tree should call existing runner methods instead of moving behavior into package-level globals.
   - Alternative considered: expose a package-global Cobra root command. That would be simpler for `main`, but it would make stdin/stdout/stderr and dependency injection harder to test without shared mutable state.

3. Promote config to `adomi config init [--global]`.
   - Rationale: `runADOConfigInit` writes `.adomi/config.yaml` or `~/.config/adomi/config.yaml` through the config package (`internal/cli/ado.go:228`), and the user-facing concern is "configure adomi", not "perform an Azure DevOps operation".
   - Alternative considered: `adomi ado config init` only. The user explicitly called this out as nonsensical, and it keeps a general setup command inside a provider namespace.

4. Keep `adomi ado config init` as a hidden compatibility alias during migration.
   - Rationale: current tests and any existing scripts may invoke `ado config init` (`internal/cli/ado_test.go:588`), while hiding it from help makes `adomi config init` the documented surface.
   - Alternative considered: remove the old route immediately. That is cleaner, but it creates an avoidable script break for a command that can delegate to the same implementation.

5. Factor command construction by feature, not by provider-only files.
   - Rationale: `internal/cli/ado.go` currently contains both Azure DevOps commands and general config initialization (`internal/cli/ado.go:198`). Cobra command constructors should separate root/config/ado construction while reusing existing action methods.
   - Alternative considered: keep all Cobra constructors in `ado.go`. That minimizes files, but it preserves the current namespace confusion in code structure.

## Risks / Trade-offs

- Cobra may print usage or errors in addition to `main` printing returned errors -> set command output/error streams explicitly and disable duplicate error printing where needed; update tests around stderr.
- Cobra flag parsing changes edge-case error strings -> assert stable behavior at the user contract level, not exact internal parser phrasing, except where the message is intentionally useful.
- Hidden alias may keep the old command alive too long -> document it as hidden compatibility only and omit it from help/spec examples.
- Dependency addition may affect module resolution -> add Cobra through `go get github.com/spf13/cobra` during implementation and verify `go test -count=1 ./...`.

## Migration Plan

1. Add Cobra dependency and create Cobra root construction around the existing `Runner`.
2. Add top-level `config init` command that calls the existing config-init behavior.
3. Convert Azure DevOps commands to Cobra subcommands while preserving current actions and dependencies.
4. Add hidden `ado config init` alias delegating to the same config command action.
5. Update tests and command examples.
6. Rollback is straightforward: revert the Cobra dependency and command-constructor changes; no persistent data or config migration is involved.

## Open Questions

- None blocking. The design intentionally keeps `ado config init` as a hidden compatibility alias; if approval requires a hard break, remove that alias from the tasks before implementation.

## Planning Verification

- [x] Every file/line reference was read directly by me: `cmd/adomi/main.go`, `cmd/adomi/main_test.go`, `internal/cli/root.go`, `internal/cli/ado.go`, `internal/cli/ado_test.go`, `internal/config/config.go`, `docs/adomi_azure_devops_handover.md`, `go.mod`, and `openspec/config.yaml`.
- [x] I ran diagnostic commands myself for facts in the plan: `rg --files`, `rg -n`, `find openspec`, `find docs`, `git status --short --branch`, `go test ./internal/cli ./cmd/adomi`, and OpenSpec status/instructions commands.
- [x] Each implementation step in `tasks.md` has a verification checkpoint with a concrete command and expected outcome.
- [x] I searched for existing patterns before proposing new ones: current CLI parsing and tests were read directly before proposing Cobra constructors.
- [x] I checked current filesystem state for counts, paths, and names: there are no existing `openspec/specs` files, and the change directory was created under `openspec/changes/streamline-cli-with-cobra`.
- [x] Blast radius listed: CLI entrypoint, command parser, CLI tests, Go module files, and command documentation.
- [x] Edge cases documented for command parsing, hidden alias behavior, stdout/stderr behavior, config creation errors, and dependency addition.

## Pre-Mortem

1. The migration fails because Cobra writes usage/errors directly while `cmd/adomi/main.go:11` also prints returned errors. Mitigation: set output/error writers on every constructed command and configure Cobra error printing deliberately.
2. The migration breaks fetch scripting because Cobra help or diagnostics leak to stdout, violating the path-only contract tested at `internal/cli/ado_test.go:341`. Mitigation: keep successful command output exactly in the action method and send help/errors/prompts to stderr.
3. The migration changes config initialization semantics while moving it out of `ado`, especially global behavior from `internal/cli/ado.go:198`. Mitigation: delegate both `config init` and hidden `ado config init` to the same existing action and preserve the current tests for repo/global paths and overwrite refusal.
