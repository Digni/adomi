## Context

`adomi` already uses a Cobra command tree while preserving `Run(args, stdin, stdout, stderr)` as the testable execution boundary (`internal/cli/root.go:68-72`). Cobra help and usage are intentionally wired to stderr so stdout stays reserved for successful command data (`internal/cli/root.go:74-90`). Most Azure DevOps action commands disable Cobra flag parsing and delegate to custom parsers (`internal/cli/root.go:136-193`), which preserves existing behavior but means help must be handled manually for commands that route through `runADOPullRequest` and other custom parser paths. Routed PR operation handlers currently receive only `stdout` after `newADOPullRequestCommand` calls `r.runADOPullRequest(args, stdout)` (`internal/cli/root.go:186-191`), so operation-specific help must be intercepted before dispatch or the stderr writer must be explicitly threaded through.

Current help coverage is uneven. Namespace help exists for `ado`, `ado work-item`, and `ado pr`, and tests assert stderr-only output plus supported operation lists (`internal/cli/ado_test.go:815-857`, `internal/cli/ado_test.go:2360-2375`). However, a diagnostic run showed `go run ./cmd/adomi ado pr ensure --help` exits with `unknown argument "--help"` instead of command-specific usage, while `adomi --help` and `adomi ado pr --help` print help on stderr with empty stdout. Generated agent skill guidance already teaches common PR maintenance commands (`internal/cli/agent.go:191-205`), but agents should also be able to query command help directly at execution time.

Obsidian lookup found prior adomi context in `1.Projects/adomi/1. Daily Work/2026/05/2026-05-26.md`: the PR maintenance feature intentionally exposed conservative `ensure`, `reply`, `resolve`, and `reopen` operations while excluding merge/approval/reviewer-management actions. This change keeps that conservative boundary visible in command help rather than broadening the command surface.

## Goals / Non-Goals

**Goals:**
- Add stderr-only `--help`/`-h` output for public action commands that use custom parsing or routed subcommands.
- Make PR maintenance help, especially `adomi ado pr ensure --help`, explicit enough for AI agents to discover required inputs, optional flags, stdout semantics, and safety boundaries before writing to Azure DevOps.
- Preserve existing successful stdout contracts and non-help command behavior.
- Cover the behavior with focused CLI tests using the existing `Runner.Run` boundary.

**Non-Goals:**
- Replacing the current custom argument parsers with full Cobra flag binding.
- Adding new Azure DevOps write capabilities, approval/merge/reviewer-governance actions, or changing existing API calls.
- Changing generated skill content except where tests reveal stale guidance after help text updates.
- Moving help output to stdout.

## Decisions

1. **Keep custom parsers and add an explicit help layer.**
   - Decision: retain `DisableFlagParsing: true` for existing commands and add a small, shared helper that detects `--help`/`-h` before validation, credential loading, repo discovery, or network calls. Because no Cobra flags are registered for these custom-parsed commands, flag and option guidance must be hand-authored in `Long` text or bespoke help strings rather than relying on Cobra's auto-generated flag sections.
   - Rationale: current commands rely on hand-written parser semantics in `internal/cli/ado.go` and tests enforce stdout/stderr behavior; a Cobra flag migration would create unnecessary behavior churn for this change.
   - Alternative considered: migrate each command to Cobra flags and subcommands. Rejected because it is broader than command help and risks changing validation/error ordering.

2. **Treat routed PR operations as separate help targets and intercept them before stdout-only dispatch.**
   - Decision: `adomi ado pr --help` remains namespace help, while `adomi ado pr ensure --help`, `comment --help`, `reply --help`, `resolve --help`, and `reopen --help` each return operation-specific help. Implement routed PR operation help in `newADOPullRequestCommand`'s `RunE`, before calling `r.runADOPullRequest(args, stdout)`, so the implementation can use Cobra's stderr-configured output writer and does not need to write help through stdout. Threading `stderr`/`cmd` into the PR handlers is acceptable only if tests prove stdout stays empty.
   - Rationale: `runADOPullRequest` currently dispatches these operations from the first arg (`internal/cli/ado.go:134-152`), so parent Cobra help cannot explain operation-specific required flags without becoming too dense, and the routed handlers do not currently have a stderr writer.
   - Alternative considered: only expand the parent `ado pr` Long text. Rejected because agents commonly probe leaf commands before execution and the current failure mode occurs at the leaf.

3. **Make help informational and side-effect free.**
   - Decision: help requests exit successfully, write only to stderr, leave stdout empty, and do not load configuration, credentials, repository state, or Azure DevOps clients.
   - Rationale: existing help tests assert empty stdout (`internal/cli/ado_test.go:823-824`, `internal/cli/ado_test.go:844-845`, `internal/cli/ado_test.go:2367-2368`), and side-effect-free help is safer for AI agents.
   - Alternative considered: allow parsers to partially validate flags before printing help. Rejected because help should work even when required runtime context is absent.

4. **Document command contracts, not implementation internals.**
   - Decision: help text should include usage, supported flags, required message-source rules, stdout result shape, and safety boundaries. It should not describe Azure DevOps endpoint internals.
   - Rationale: the canonical CLI spec already defines command-surface behavior, including stdout separation and conservative write boundaries (`openspec/specs/cli-command-surface/spec.md`). Agents need operational guidance, not API implementation details.

## Risks / Trade-offs

- **Risk: help text drifts from parser behavior.** → Mitigation: add tests for representative command help content and update hand-authored help text near the parser command definitions to keep review locality.
- **Risk: routed PR operation help accidentally writes to stdout because handlers only receive `stdout`.** → Mitigation: intercept operation help in `newADOPullRequestCommand` before dispatch, or explicitly thread a stderr/help writer through the handlers, with tests asserting empty stdout.
- **Risk: broad help coverage becomes a large rewrite.** → Mitigation: prioritize all public action commands that currently use custom parsing; do not redesign Cobra command structure in this change.
- **Risk: safety-critical boundaries are hidden in prose only.** → Mitigation: include explicit negative assertions for unsupported PR/work-item write actions in namespace or operation help where relevant.
- **Risk: `--help` ordering changes validation behavior for malformed invocations.** → Mitigation: only short-circuit when a help token is present in the command's documented help position or for routed operation help; non-help invalid inputs keep existing parser behavior.

## Migration Plan

1. Add/adjust tests for stderr-only command-specific help and side-effect-free execution.
2. Implement the help detection/output helpers and wire them into custom-parsed commands.
3. Run `gofmt -w internal cmd`, `go test ./...`, `go build -o bin/adomi ./cmd/adomi`, and `openspec validate add-command-help --strict`.
4. Rollback is code-only: revert the command help helper/tests if issues appear; no persisted data or external service migration is involved.

## Open Questions

None. The change is limited to command help behavior and does not require product or Azure DevOps API decisions.

## Planning Verification

- [x] Every file/line reference was read directly by me.
- [x] I ran diagnostic commands myself: `go run ./cmd/adomi --help`, `go run ./cmd/adomi ado pr --help`, and `go run ./cmd/adomi ado pr ensure --help`.
- [x] Each implementation chunk in `tasks.md` will include a concrete verification checkpoint.
- [x] I searched for existing patterns before proposing new ones: existing Cobra command/help setup in `internal/cli/root.go` and help tests in `internal/cli/ado_test.go`/`internal/cli/agent_test.go`.
- [x] I checked current filesystem state for OpenSpec specs and active changes.
- [x] I checked relevant Obsidian project notes for prior adomi PR-maintenance context.
- [x] Blast radius is limited to CLI command definitions, custom parser help routing, CLI tests, and the `cli-command-surface` spec.
- [x] Edge cases are documented below and reflected in tasks/specs.

## Edge Cases

- Help must work without repo config, credentials, a Git repository, readable files, or network access.
- Help for commands that accept `--message`/`--message-file` must not read message files.
- Help for `ensure` must not infer repository/source/target branches or load credentials.
- Help output must remain stderr-only; stdout must stay empty to preserve machine-readable command contracts, including routed PR operations whose execution handlers currently only receive stdout.
- Shared config init help should cover both `adomi config init --help` and the hidden compatibility alias `adomi ado config init --help` without making the hidden alias visible in namespace help.
- Unknown commands that are not help requests must continue to fail without making Azure DevOps requests.
- Unsupported governance actions remain absent from PR help; unsupported work-item mutations remain absent from work-item help.

## Pre-Mortem

1. The change fails because `--help` is detected too late, after parsers return errors like the current `unknown argument "--help"` for `ado pr ensure --help`; tasks therefore require tests before implementation for routed PR help.
2. The change fails because help writes to stdout via Cobra defaults; tasks require asserting empty stdout for every new help test, matching `root.go:87-89`.
3. The change fails because generated or namespace guidance advertises unsupported write actions; specs and tests require conservative PR/work-item boundaries to remain absent from help.
