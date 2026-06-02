## Context

Phase 1 added `adomi ado pr comment` as a PR-level thread command. The current runner dispatches `comment` from `runADOPullRequest` (`internal/cli/ado.go:79-96`), reads exactly one message source, resolves the PR repository, calls `CreatePullRequestThread`, and prints the created thread ID (`internal/cli/ado.go:224-270`). The parser currently only accepts message/profile/output flags and rejects all other arguments as unknown (`internal/cli/ado.go:904-949`).

The ADO model already represents returned file positions through `PullRequestThread.ThreadContext`, `ThreadContext`, and `FilePosition` (`internal/ado/pullrequest.go:38-60`), but thread creation options only contain repository ID, PR ID, and content (`internal/ado/pullrequest.go:106-110`). The HTTP create body currently posts only one active text comment and intentionally omits inline context (`internal/ado/client.go:261-280`). The client has URL builders for PR and thread routes (`internal/ado/client.go:422-459`) but no PR iteration or iteration-change routes yet.

Subagent research found the official Azure DevOps REST shape for inline PR threads: use `threadContext.filePath` plus right-side file positions and, for iteration-supported PRs, include `pullRequestThreadContext.changeTrackingId` and `iterationContext` resolved from the PR iteration changes API. Practical examples and SDK contracts use `offset: 1` for whole-line anchors, while Microsoft docs are inconsistent about whether offsets are 0- or 1-based.

## Goals / Non-Goals

**Goals:**

- Let `adomi ado pr comment <pull-request-id> --file <path> --line <line>` create an inline thread on the latest version of a changed PR file.
- Preserve PR-level comments when `--file`/`--line` are absent.
- Resolve the target file's Azure DevOps `changeTrackingId` from the latest PR iteration changes before creating the thread.
- Keep stdout and JSON contracts unchanged: plain success prints only the created thread ID; JSON includes `threadId` and the initial `commentId` when available.
- Validate invalid inline flag combinations before credential loading or network calls.

**Non-Goals:**

- Left-side/deleted-line comments.
- Multi-line ranges or explicit character offsets.
- Suggestion comments or special Markdown suggestion handling.
- Manual iteration/base-iteration/change-tracking overrides.
- Reviewer governance, approval/vote, merge/complete, abandon, auto-complete, or policy bypass operations.
- Live Azure DevOps probing as part of normal unit tests.

## Decisions

1. **Inline command semantics target the latest file version/right side.**
   - Decision: `--file <path> --line <line>` means line `<line>` in the latest source-side version of that PR file.
   - Rationale: This matches the normal human/agent review model and avoids deleted/left-side complexity.
   - Alternative considered: expose side or iteration flags immediately. Rejected for Phase 2A because it expands scope and adds UX ambiguity before the simple review path is proven.

2. **Require `--file` and `--line` as a pair.**
   - Decision: either both are absent for PR-level comments or both are present for inline comments.
   - Rationale: `parsePRCommentArgs` currently performs validation before client construction (`internal/cli/ado.go:904-949`), and preserving this pattern keeps invalid requests from loading credentials or making network calls.
   - Alternative considered: accept file-only comments. Rejected because Azure examples focus on positioned inline threads, and file-only behavior was not verified.

3. **Use whole-line right-side positions with `offset: 1`.**
   - Decision: create `rightFileStart` and `rightFileEnd` with the same `{ line, offset: 1 }` and leave left-side positions unset.
   - Rationale: Official and practical examples use `offset: 1`, and whole-line anchoring avoids line-length lookup.
   - Alternative considered: calculate line length or use `offset: 0`. Rejected until live API behavior is verified.

4. **Use latest iteration changes to resolve `changeTrackingId`.**
   - Decision: list PR iterations, choose the greatest/latest iteration ID as `secondComparingIteration`, fetch that iteration's changes, and match the requested file path to a changed file's `changeTrackingId`.
   - Rationale: Azure docs state `changeTrackingId` must be set for PRs with iteration support, and the value comes from iteration changes. Adding this avoids relying on undocumented acceptance of bare `threadContext`.
   - Alternative considered: post only `threadContext`. Rejected because it may fail or create poorly tracked comments on iteration-supported PRs.

5. **Use common-commit/full-PR semantics for the latest view, with implementation isolated for correction.**
   - Decision: design the helper so the base comparison choice is isolated in one resolver function. The intended product behavior is the latest whole-PR diff rather than only the latest push delta.
   - Rationale: Human reviewers commonly review the current PR diff as a whole. Azure's documented `$compareTo=0`/`firstComparingIteration == secondComparingIteration` semantics are ambiguous, so isolating this choice reduces future correction blast radius.
   - Alternative considered: use previous iteration as the base. Rejected because that would target only changes since the previous push, not the normal whole-PR review view.

6. **Reject unsupported changed-file shapes conservatively.**
   - Decision: if the matched change is deleted, renamed, lacks an item path, or cannot provide a `changeTrackingId`, fail with a helpful error before posting the thread.
   - Rationale: Deleted/renamed placement may require left positions or original path semantics, which are explicitly out of scope.
   - Alternative considered: attempt a best-effort post. Rejected because creating duplicate or misplaced review comments is worse than a clear failure.

7. **Keep the create API backward compatible.**
   - Decision: extend `PullRequestThreadCreateOptions` with optional inline context fields rather than adding a separate top-level method for PR-level vs inline posting.
   - Rationale: `runADOPullRequestComment` already calls `CreatePullRequestThread` (`internal/cli/ado.go:251-255`), and the existing HTTP method can conditionally add `threadContext`/`pullRequestThreadContext` while preserving Phase 1 body assertions for PR-level comments.
   - Alternative considered: add `CreateInlinePullRequestThread`. Rejected because the final POST endpoint is the same and would duplicate validation/response handling.

## Risks / Trade-offs

- **Azure common-commit comparison metadata may be wrong** → Keep iteration/change resolution in focused helpers and unit-test the exact request/query/body shapes. If live validation later shows Azure expects a different `iterationContext`, change only the resolver/body assembly.
- **Path matching may miss files because users omit or include a leading slash** → Normalize input to Azure's leading-slash path convention for matching, but use the exact `item.path` returned by iteration changes in the POST body.
- **Offset semantics may differ by API version** → Use `offset: 1` only for whole-line anchors and do not expose offset flags until live-tested.
- **Large PRs may require iteration-change pagination** → Implement `$top`/`$skip` support using Azure's `nextSkip`/`nextTop` fields instead of assuming the first page contains the file.
- **Shared PR maintenance code could regress reply/resolve/reopen** → Keep parser changes scoped to `parsePRCommentArgs`; run focused PR maintenance tests after each implementation slice.

## Edge Cases

- PR fetch fails or response has no repository ID: existing behavior exits before thread creation (`internal/cli/ado.go:247-250`); inline flow must preserve this.
- Iterations list returns an error or no iterations: exit non-zero before creating a thread.
- Iteration changes pagination returns empty pages, missing `changeEntries`, or no matching path: exit non-zero with a message that the file is not present in the latest PR diff.
- Multiple matching paths after normalization: fail rather than guess.
- Matching change has `delete`/`rename` or missing positive `changeTrackingId`: fail because Phase 2A only supports right-side changed files with stable tracking.
- Azure accepts the create request but returns no positive thread ID: preserve the existing missing-ID guard (`internal/cli/ado.go:259-260`, `internal/ado/client.go:277-279`).
- JSON output when Azure omits initial comment ID: omit/zero the comment ID while still including `threadId`, matching current result behavior (`internal/cli/ado.go:262-268`).

## Planning Verification

- [x] Every file/line reference was read directly by me, not from subagent summary alone.
- [x] I ran diagnostic commands myself: `git status --short`, `openspec list --json`, `find openspec -maxdepth 3 -type f`, `grep -R`, `go test ./internal/ado ./internal/cli`, and `openspec validate create-pr-comment-threads --strict`.
- [x] Each implementation task will include a verification checkpoint with a concrete command and expected outcome.
- [x] I searched for existing patterns before proposing new ones: PR command dispatch/parser, create-thread client method, URL builders, fake clients, tests, docs, and OpenSpec specs.
- [x] I checked current filesystem state for paths and names, including `openspec/changes`, `openspec/specs`, `internal/ado`, `internal/cli`, and docs.
- [x] Obsidian lookup was not needed for this plan because the scope is defined by current repository OpenSpec artifacts, code, and Azure DevOps REST behavior rather than external project history or customer decisions.
- [x] Blast radius listed for shared code: `internal/cli/ado.go`, `internal/ado/client.go`, `internal/ado/pullrequest.go`, CLI/ADO tests, docs, and agent guidance.
- [x] Edge cases documented for every integration point and data transformation.

## Pre-Mortem

1. **Inline comments appear on the wrong diff because Azure expects different common-commit iteration metadata.** The risk comes from adding `pullRequestThreadContext` to the POST assembled in `internal/ado/client.go:261-280`. The plan mitigates this by isolating the comparison resolver and keeping live verification as a follow-up if available.
2. **Valid files are rejected due to path normalization mismatch.** The risk comes from mapping a user-provided `--file` parsed in `internal/cli/ado.go:904-949` to Azure iteration-change paths. The plan mitigates this by normalizing only for comparison and posting the exact Azure path.
3. **Existing PR-level comments regress because create options grow inline fields.** The risk comes from extending `PullRequestThreadCreateOptions` (`internal/ado/pullrequest.go:106-110`) and `CreatePullRequestThread` (`internal/ado/client.go:261-280`). The plan mitigates this by preserving PR-level tests that assert no inline context is sent when no file/line is provided.
