## Why

Adomi can reply to existing Azure DevOps pull request threads, but it cannot create a new PR-level discussion thread for a fresh comment. This leaves agents and users unable to post standalone PR comments through the same conservative PR maintenance surface that already supports `reply`, `resolve`, and `reopen`.

## What Changes

- Add a Phase 1 PR-level thread creation command that posts a new top-level text comment to a pull request.
- Return the created thread ID on plain stdout so the result can be used directly with existing thread maintenance commands.
- Include both thread ID and initial comment ID in JSON output.
- Reuse existing PR maintenance configuration, credential loading, repository ID resolution, message input, and data-only stdout conventions.
- Keep inline file/line review threads as a deliberate Phase 2 follow-up because Azure DevOps inline thread creation can require file position, iteration, and change-tracking context.
- Do not add approval, merge/complete, abandon, reviewer, or policy-governance behavior.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ado-pr-maintenance`: Extend conservative PR maintenance so users can create a new PR-level comment thread in addition to replying to existing threads.
- `cli-command-surface`: Expose the new PR thread creation command in the `adomi ado pr` namespace and preserve stdout/stderr stream conventions.

## Impact

- CLI dispatch and help for `adomi ado pr` in `internal/cli/root.go` and `internal/cli/ado.go`.
- PR maintenance parsing/result structs in `internal/cli/ado.go`.
- Azure DevOps PR thread client types and methods in `internal/ado/pullrequest.go` and `internal/ado/client.go`.
- Tests for CLI parsing/behavior and Azure DevOps request body/URL handling in `internal/cli/ado_test.go` and `internal/ado/pullrequest_test.go`.
- Generated agent skill/help text and Azure DevOps handover documentation that list PR maintenance commands.
