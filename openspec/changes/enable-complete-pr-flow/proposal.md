## Why

Adomi can create and maintain Azure DevOps pull requests but stops before the lifecycle actions that finish or close them. Users and coding agents therefore have to leave the repository-local Adomi workflow to complete a ready PR, schedule it for policy-gated auto-completion, or abandon it.

## What Changes

- Add explicit `adomi ado pr complete <pull-request-id>`, `adomi ado pr auto-complete <pull-request-id>`, `adomi ado pr cancel-auto-complete <pull-request-id>`, and `adomi ado pr abandon <pull-request-id>` commands for existing Azure DevOps pull requests.
- Add explicit `adomi ado pr approve <pull-request-id>`, `adomi ado pr approve-with-suggestions <pull-request-id>`, and `adomi ado pr reject <pull-request-id>` commands that cast the authenticated user's Azure DevOps reviewer vote without accepting an arbitrary reviewer identity.
- Let completion and auto-completion carry the common policy-respecting completion preferences: merge strategy, source-branch deletion, linked-work-item transition, and an optional merge commit message. Preserve existing server-side preferences for options the user does not supply and do not introduce an Adomi-local merge default.
- Preflight the pull request and requested state before mutation, pin immediate completion to the fetched source commit, resolve the authenticated identity required for auto-complete and reviewer votes, and validate the returned lifecycle or vote state before printing success.
- Define idempotent outcomes for an already requested lifecycle state or vote and reject incompatible terminal-state transitions before a write.
- Preserve Adomi's existing repository/profile/PAT resolution and stdout/JSON contracts, while teaching command help, generated agent skills, and public documentation that lifecycle actions require explicit user authorization.
- Keep arbitrary reviewer management, vote reset, wait-for-author voting, policy bypass, abandoned-PR reactivation, completed-PR reversion, and completion polling outside this change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `ado-pr-maintenance`: Expand the conservative PR write boundary with explicit, state-aware completion, auto-completion cancellation, abandonment, and authenticated-user reviewer-vote operations while retaining policy-bypass and arbitrary reviewer-management exclusions.
- `cli-command-surface`: Add the lifecycle verbs, argument validation, side-effect-free help, deterministic plain/JSON output, and failure contracts.
- `agent-skill-creation`: Teach generated Adomi skills when and how agents may use the lifecycle and reviewer-vote commands and require explicit user authorization for them.
- `repository-documentation`: Make the new workflows, permissions, completion preferences, voting semantics, asynchronous auto-complete behavior, and remaining safety boundaries discoverable.

## Impact

- Affected CLI code: PR dispatch and help in `internal/cli/ado_pr.go`, `internal/cli/commands.go`, and `internal/cli/help.go`; a focused lifecycle handler/parser; generated skill content in `internal/cli/agent.go`; and their tests.
- Affected Azure DevOps code: pull request identity, reviewer vote, commit, completion-option, and lifecycle result models in `internal/ado/pullrequest.go`; authenticated-user lookup, repository-scoped pull-request PATCH behavior, and reviewer-vote PUT behavior in `internal/ado`; and HTTP contract tests.
- Azure DevOps integration: reads the requested PR and authenticated identity, updates PR lifecycle state through the Git REST API, and casts the caller's reviewer vote through the pull-request reviewer API. The PAT requires code write permission. Existing branch policies remain enforced, and no policy-bypass request is emitted.
- Affected public contracts: `README.md`, `docs/azure-devops.md`, canonical OpenSpec requirements, generated agent guidance, command help, and stdout/JSON result shapes.
- No new third-party Go dependency or configuration schema change is expected.
