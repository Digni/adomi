## Why

Adomi can now create top-level Azure DevOps pull request comment threads, but agents still cannot place review feedback on the latest version of a changed file. Inline PR comments are needed so automated and human-assisted review output can point at the exact file and line being reviewed.

## What Changes

- Extend `adomi ado pr comment <pull-request-id>` to accept `--file <path> --line <line>` for creating a new inline review thread on the latest PR file version.
- Preserve the existing PR-level behavior when no inline file context is provided.
- Limit the first inline release to right-side, single-line, whole-line comments on the latest changed file version.
- Auto-resolve Azure DevOps PR iteration/change metadata needed to create a tracked inline thread when available.
- Keep plain stdout as the created thread ID and JSON output as the structured result with `threadId` and initial `commentId` when available.
- Defer left-side/deleted-line comments, ranges, character offsets, suggestions, manual iteration selection, reviewer management, PR approval, merge/complete, and abandon flows.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ado-pr-maintenance`: Allow PR comment creation to target the latest version of a changed file by file path and line number.
- `cli-command-surface`: Document and validate the inline comment CLI flags while preserving existing PR maintenance output contracts.

## Impact

- Affected CLI code: PR comment parser/runner, help/guidance text, fake clients, and tests under `internal/cli`.
- Affected ADO client code: PR thread creation options/request body plus new PR iteration and iteration-change retrieval helpers under `internal/ado`.
- External system impact: additional Azure DevOps REST calls to fetch PR iterations and iteration changes before posting an inline thread.
- No new third-party dependencies are expected.
