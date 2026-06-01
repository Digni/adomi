## 1. Test the Phase 1 contract

- [x] 1.1 Add CLI tests for `adomi ado pr comment <pull-request-id> --message <text>` creating a PR-level thread, printing only the created thread ID, and preserving empty stderr on success. Verification: `go test ./internal/cli -run 'TestADOPullRequestComment'` fails before implementation for the missing command and passes after implementation.
- [x] 1.2 Add CLI tests for `--message-file`, `--json`, missing/invalid PR ID, missing/conflicting/blank message sources, unsupported existing-thread/inline context flags such as `--thread`, `--file`, and `--line`, missing repository ID, Azure DevOps write failure, and response missing thread ID. Verification: `go test ./internal/cli -run 'TestADOPullRequestComment|TestADOPullRequestThread'` passes and existing reply/resolve/reopen tests remain green.
- [x] 1.3 Add Azure DevOps client tests for POSTing to the PR thread collection URL with a body containing one text comment and active status, plus error cases for missing repository ID, HTTP errors, decode errors, and missing thread ID. Verification: `go test ./internal/ado -run 'TestClientCreatePullRequestThread'` passes.

## 2. Implement PR-level thread creation

- [x] 2.1 Add PR thread creation option/result types and extend the PR maintainer interface with a `CreatePullRequestThread` operation. Verification: `go test ./internal/ado ./internal/cli` compiles to the next expected failing tests rather than interface/type errors.
- [x] 2.2 Implement the Azure DevOps client method using the existing pull request thread collection URL, request authorization, JSON helper conventions, and response validation. Verification: `go test ./internal/ado -run 'TestClientCreatePullRequestThread'` passes.
- [x] 2.3 Add CLI parsing and dispatch for `adomi ado pr comment`, reusing existing profile/global/json/message-source behavior without changing `reply`, `resolve`, or `reopen`. Verification: `go test ./internal/cli -run 'TestADOPullRequestComment|TestADOPullRequestReply|TestADOPullRequestResolve|TestADOPullRequestReopen'` passes.
- [x] 2.4 Implement the `comment` runner path: validate content before credentials, create a PR maintenance client, fetch the PR repository ID, create the PR-level thread, print thread ID by default, and include thread/comment IDs in JSON. Verification: `go test ./internal/cli -run 'TestADOPullRequestComment'` passes.

## 3. Update command surface and generated guidance

- [x] 3.1 Update `adomi ado pr` help text to list `comment` alongside `fetch`, `ensure`, `reply`, `resolve`, and `reopen`, including the asserted supported-operation slice in `TestADOPullRequestNamespaceHelpListsSupportedOperations`, while continuing to omit governance commands. Verification: `go test ./internal/cli -run 'TestADOPullRequestNamespaceHelpListsSupportedOperations'` passes.
- [x] 3.2 Update agent skill/help generation and Azure DevOps handover documentation so agents know `comment` creates a new PR-level thread and plain stdout returns the thread ID. Verification: `go test ./internal/cli -run 'TestAgentSkill'` passes and `rg -n "ado pr comment|reply <pull-request-id>" internal/cli/agent.go docs README.md` shows consistent wording.

## 4. Final verification

- [x] 4.1 Run the full Go test suite. Verification: `go test ./...` passes.
- [x] 4.2 Validate the OpenSpec change. Verification: `openspec validate create-pr-comment-threads --strict` passes.
- [x] 4.3 Review the final diff against Phase 1 scope and confirm no inline file/line thread support or PR governance behavior was added. Verification: `git diff --stat` and targeted diff review show only planned PR-level thread creation changes.
