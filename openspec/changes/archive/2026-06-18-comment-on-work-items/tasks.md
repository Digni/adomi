## 1. Test the work item comment contract

- [x] 1.1 Add CLI tests for `adomi ado comment <work-item-id> --message <text>` creating a work item comment, printing only the created comment ID, preserving empty stderr on success, and sending no network/credential calls for invalid IDs or invalid message sources. Verification: `go test ./internal/cli -run 'TestADOWorkItemComment'` fails before implementation for the missing command and passes after implementation.
- [x] 1.2 Add CLI tests for `--message-file`, `--json`, `adomi ado work-item comment` alias behavior, blank/empty message rejection, conflicting message sources, Azure DevOps write failure, malformed/empty response handling, missing comment ID, and mismatched positive `workItemId`. Verification: `go test ./internal/cli -run 'TestADOWorkItemComment|TestADOPullRequestComment|TestADOPullRequestReply'` passes and existing PR comment/reply tests remain green.
- [x] 1.3 Add Azure DevOps client tests for POSTing `{ "text": ... }` to the work item comments endpoint with auth and `api-version=7.0-preview.3`, accepting either `id` or `commentId` as the created ID, preserving returned URL, validating response work item ID when present, reporting missing comment ID before any work item ID mismatch, and surfacing non-2xx/decode/missing-ID errors. Verification: `go test ./internal/ado -run 'TestClientCreateWorkItemComment'` passes.

## 2. Implement Azure DevOps work item comment client support

- [x] 2.1 Add work item comment model/options types and a work item maintenance interface method, then extend CLI `ADOClient` and fake clients without changing fetch or PR maintenance contracts; the shared `fakeADOClient` and `fakePRMaintenanceClient` in `internal/cli/ado_test.go` must satisfy the new method. Verification: `go test ./internal/ado ./internal/cli` compiles to the next expected failing tests rather than interface/type errors.
- [x] 2.2 Implement the work item comments URL helper and `CreateWorkItemComment` method using the existing auth, JSON helper/error handling style, documented preview API version, and response ID normalization. Verification: `go test ./internal/ado -run 'TestClientCreateWorkItemComment|TestClientFetchWorkItem'` passes.

## 3. Implement CLI command, alias, and output behavior

- [x] 3.1 Add work item comment argument/result parsing for `--message`, `--message-file`, `--profile`, `--global`, and `--json`, reusing or extracting existing message-source validation without changing PR parser semantics. Verification: `go test ./internal/cli -run 'TestADOWorkItemComment|TestADOPullRequestComment|TestADOPullRequestReply'` passes.
- [x] 3.2 Wire `adomi ado comment` and `adomi ado work-item comment` through Cobra/runner dispatch, validate content before credentials, load the configured profile/PAT/proxy like `ado fetch`, call the new client method, print comment ID by default, and emit one compact JSON object for `--json`. Verification: `go test ./internal/cli -run 'TestADOWorkItemComment|TestADOFetch|TestADOPullRequest'` passes.
- [x] 3.3 Update help text, generated adomi skill guidance, README/docs/handover text, and related assertions so work item comment support is discoverable while unsupported work item mutation commands remain absent. Verification: `go test ./internal/cli -run 'TestADO.*Help|TestAgentSkill'` passes and `rg -n "ado comment|ado work-item comment|work item comment|comment on a work item" README.md docs internal/cli/agent.go` shows consistent wording.

## 4. Final verification

- [x] 4.1 Run targeted packages after implementation. Verification: `go test ./internal/ado ./internal/cli` passes.
- [x] 4.2 Run the full Go suite. Verification: `go test ./...` passes.
- [x] 4.3 Validate the OpenSpec change and review scope. Verification: `openspec validate comment-on-work-items --strict` passes, `openspec status --change comment-on-work-items` reports apply-ready artifacts, and `git diff --stat` shows only planned proposal/spec/docs/code changes plus any pre-existing unrelated working-tree changes kept separate.
