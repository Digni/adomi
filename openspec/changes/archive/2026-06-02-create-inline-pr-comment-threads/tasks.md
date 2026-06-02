## 1. CLI Contract and Validation

- [x] 1.1 Add failing CLI parser/runner tests for `adomi ado pr comment <pull-request-id> --file <path> --line <line> --message <text>` and `--message-file`, including plain stdout thread ID and JSON output with `threadId`, optional `commentId`, file path, and line. Verification: `go test ./internal/cli -run 'TestADOPullRequestComment'` fails before implementation and passes after the slice.
- [x] 1.2 Add failing validation tests for missing `--file`/`--line` pair, non-positive/non-integer line values, unsupported `--thread`, and validation-before-credentials behavior. Verification: `go test ./internal/cli -run 'TestADOPullRequestComment'` fails before implementation and passes after parser updates.
- [x] 1.3 Extend `parsePRCommentArgs` and PR comment result shaping to preserve PR-level comments while accepting paired inline target flags. Verification: `go test ./internal/cli -run 'TestADOPullRequestComment|TestADOPullRequestReply|TestADOPullRequestResolve|TestADOPullRequestReopen'` passes.

## 2. Azure DevOps Iteration and Change Resolution

- [x] 2.1 Add ADO client models and tests for listing pull request iterations and fetching iteration changes with `$top`/`$skip` pagination, including `changeTrackingId`, `item.path`, `originalPath`, and `changeType`. Verification: `go test ./internal/ado -run 'TestClientPullRequestIterations|TestClientPullRequestIterationChanges'` fails before implementation and passes after the slice.
- [x] 2.2 Implement URL builders and client methods for iterations and iteration changes without changing existing PR/thread URLs. Verification: `go test ./internal/ado -run 'TestClientPullRequestIterations|TestClientPullRequestIterationChanges|TestClientCreatePullRequestThread'` passes.
- [x] 2.3 Add and test a resolver that selects the latest PR file version, normalizes user paths for matching, returns the exact Azure path plus `changeTrackingId`, and rejects missing, duplicate, deleted, renamed, or untracked changes. Verification: `go test ./internal/cli -run 'TestResolvePullRequestInlineCommentTarget'` passes.

## 3. Inline Thread Creation Body

- [x] 3.1 Add failing ADO client tests for inline thread creation body: `threadContext.filePath`, right-side `{line, offset: 1}` start/end, active text comment, and `pullRequestThreadContext` with change tracking and iteration context. Verification: `go test ./internal/ado -run 'TestClientCreatePullRequestThread'` fails before implementation and passes after the slice.
- [x] 3.2 Extend `PullRequestThreadCreateOptions` and `CreatePullRequestThread` to conditionally include inline context while preserving the PR-level body with no `threadContext` when inline fields are absent. Verification: `go test ./internal/ado -run 'TestClientCreatePullRequestThread|TestPullRequestMaintenanceMethods'` passes.
- [x] 3.3 Wire the CLI inline path to fetch PR repository ID, resolve latest inline target metadata, create the inline thread, and keep existing missing-thread-ID and missing-comment-ID handling. Verification: `go test ./internal/cli -run 'TestADOPullRequestComment'` passes.

## 4. Documentation and Agent Guidance

- [x] 4.1 Update PR command help, generated agent guidance, and handover documentation to describe `--file`/`--line` as latest-version right-side inline comments and list deferred unsupported cases. Verification: `go test ./internal/cli -run 'TestAgentSkill|TestRootHelp|TestADOPullRequestHelp'` passes or the nearest existing help tests pass.
- [x] 4.2 Ensure documentation preserves the conservative PR write boundary and does not introduce approval, merge/complete, abandon, reviewer-management, or policy-bypass language. Verification: `rg -n "approve|complete|merge|abandon|reviewer|policy bypass|adomi ado pr comment" docs internal/cli/agent.go openspec` shows only allowed boundary wording.

## 5. Final Verification and Review

- [x] 5.1 Run full verification: `go test ./...`, `openspec validate create-inline-pr-comment-threads --strict`, and `git diff --check`; all must pass.
- [x] 5.2 Run implementation review after code is complete and fix accepted findings without expanding scope. Verification: reviewer reports no P0/P1/P2 issues or accepted issues are fixed with focused tests.
