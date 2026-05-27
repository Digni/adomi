## Why

Agents can already gather Azure DevOps pull request context with `adomi ado pr <pull-request-id>`, but they cannot open or maintain the PR that represents their own branch. Adding a conservative write surface lets agents create/update their PR, reply to review threads, and resolve/reopen threads without granting `adomi` broad merge, approval, or PR administration behavior.

## What Changes

- Add an agent-oriented Azure DevOps PR maintenance workflow that runs from inside a Git repository and infers repository context from the repo root, git remotes, and current branch.
- Add an idempotent `adomi ado pr ensure` command that finds the active PR for the current/source branch and target branch, then creates it if missing or updates supported fields if it already exists.
- Add explicit PR thread commands for review maintenance: reply to a thread, resolve a thread as fixed, and reopen a thread as active.
- Canonicalize PR context fetching as `adomi ado pr fetch <pull-request-id>` while keeping `adomi ado pr <pull-request-id>` as a compatibility alias.
- Update generated `adomi` agent skill content so agents learn the new PR maintenance commands, required safety boundaries, and when to fetch context before replying/resolving review feedback.
- Keep successful stdout machine-readable and minimal: PR write commands print the PR identifier or JSON when requested; prompts, validation errors, and diagnostics stay on stderr.
- Keep destructive or governance-sensitive PR lifecycle actions out of scope: no approve/reject voting, auto-complete, complete/merge, abandon, policy bypass, or broad reviewer management.

## Capabilities

### New Capabilities

- `ado-pr-maintenance`: Defines repository-inferred Azure DevOps PR creation/update and PR comment thread reply/status operations for agent workflows.

### Modified Capabilities

- `cli-command-surface`: Adds the `adomi ado pr` subcommand namespace for `fetch`, `ensure`, `reply`, `resolve`, and `reopen`, while preserving the existing `adomi ado pr <pull-request-id>` fetch behavior as a compatibility alias.
- `agent-skill-creation`: Updates generated `adomi` skill guidance to describe conservative PR maintenance commands and boundaries.

## Impact

- Affected code: Cobra command construction in `internal/cli/root.go`, PR argument parsing and command orchestration in `internal/cli/ado.go`, Azure DevOps PR client methods in `internal/ado/client.go`, PR models/workflow helpers in `internal/ado/pullrequest.go` or a sibling file, and test fakes/interfaces in `internal/cli` and `internal/ado` tests.
- Affected repository inference: existing repo-root detection and git remote helpers are reused/extended, and new git current-branch discovery is required.
- Affected specs/docs: `openspec/specs/cli-command-surface/spec.md`, generated `adomi` skill guidance and tests, and user-facing docs that describe PR commands.
- APIs/dependencies: Uses Azure DevOps Git REST APIs for listing, creating, and updating pull requests plus creating PR thread comments and updating thread status; no new third-party Go dependency is expected.
- Systems: Requires PAT scopes capable of code write / PR thread write operations. Existing read-only fetch, work item fetch, profile loading, PAT storage, and export layout remain unchanged.
