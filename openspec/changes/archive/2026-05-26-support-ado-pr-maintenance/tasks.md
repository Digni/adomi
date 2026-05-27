## 1. Repository Inference And Command Shape

- [x] 1.1 Add tests for Azure DevOps remote parsing that extracts organization, project, and repository from HTTPS, SSH, and `*.visualstudio.com` remote URL forms, including unsupported and ambiguous remote cases.
- [x] 1.2 Implement repository inference helpers, extending current remote parsing without changing config initialization output.
- [x] 1.3 Add tests for current source branch detection, detached HEAD failure, explicit `--source` override, target branch inference from the selected remote default branch, explicit `--target` override, and source-ref-equals-target-ref rejection.
- [x] 1.4 Implement Git branch/default target discovery behind dependency seams so CLI tests do not require real repository state except dedicated integration-style tests.
- [x] 1.5 Refactor `adomi ado pr` into a namespace with `fetch`, `ensure`, `reply`, `resolve`, and `reopen`, while preserving `adomi ado pr <pull-request-id>` as a compatibility alias.
- [x] 1.6 Verification checkpoint: run `go test ./internal/config ./internal/cli -run 'Test.*Remote|Test.*Branch|Test.*Repository|TestADOPullRequest'`; expected: repository inference, branch inference, PR namespace, and compatibility fetch tests pass.

## 2. Azure DevOps PR Maintenance Client

- [x] 2.1 Add request/response models and interfaces for listing active PRs by source/target refs, creating PRs, patching PR title/description, creating thread comments, and patching thread status.
- [x] 2.2 Add URL helper tests for repository-scoped PR list/create/update, thread comment create, and thread status update endpoints, preserving base URL project/path behavior for Azure DevOps Services and Server/on-prem.
- [x] 2.3 Implement client methods with existing PAT basic auth, API version query behavior, non-2xx response errors, and JSON decoding validation.
- [x] 2.4 Add client tests for method, URL, query parameters, auth header, request bodies, successful decoding, empty/malformed responses, and non-2xx errors for every new write operation.
- [x] 2.5 Verification checkpoint: run `go test ./internal/ado -run 'TestClient.*PullRequest|Test.*PullRequestMaintenance|Test.*Thread'`; expected: all PR maintenance client tests pass.

## 3. Ensure Workflow

- [x] 3.1 Add workflow tests for `ensure` creating a PR when no active PR exists and `--title` is supplied, rejecting missing title on create, updating title, updating description from file, leaving an existing PR unchanged when no update fields are supplied, rejecting same source/target refs, and rejecting multiple matches.
- [x] 3.2 Implement source/target ref normalization so short branch names become `refs/heads/<name>` and already-qualified refs are not double-prefixed.
- [x] 3.3 Implement `adomi ado pr ensure` orchestration: resolve repo/home/config/PAT/client, infer or parse repository/source/target, list active PRs, then create/update/unchanged according to the spec.
- [x] 3.4 Add `--json` support for `ensure` with compact JSON output and ID-only default stdout.
- [x] 3.5 Verification checkpoint: run `go test ./internal/ado ./internal/cli -run 'Test.*Ensure'`; expected: ensure create/update/unchanged/ambiguous/inference-failure scenarios pass and stdout remains data-only.

## 4. Thread Reply And Status Workflow

- [x] 4.1 Add parser tests for `reply`, `resolve`, and `reopen`, including non-positive IDs, missing `--thread`, conflicting/missing message sources, empty message values, and `--json`.
- [x] 4.2 Implement shared thread-command setup that fetches the pull request to resolve repository ID before comment or thread-status mutations, including missing-repository-ID failure tests.
- [x] 4.3 Implement `adomi ado pr reply <pull-request-id> --thread <thread-id>` with exactly one of `--message-file` or `--message`, creating a text comment and printing the comment ID or JSON.
- [x] 4.4 Implement `adomi ado pr resolve <pull-request-id> --thread <thread-id>` by patching the thread status to `fixed` and printing the thread ID or JSON.
- [x] 4.5 Implement `adomi ado pr reopen <pull-request-id> --thread <thread-id>` by patching the thread status to `active` and printing the thread ID or JSON.
- [x] 4.6 Verification checkpoint: run `go test ./internal/cli -run 'Test.*Reply|Test.*Resolve|Test.*Reopen'`; expected: thread command tests pass, invalid arguments make no network calls, missing PR repository IDs fail before mutation, and stdout remains ID-only or JSON-only.

## 5. Specs, Docs, And Agent Guidance

- [x] 5.1 Update current canonical `openspec/specs/cli-command-surface/spec.md` and `openspec/specs/agent-skill-creation/spec.md` after implementation so PR namespace, compatibility fetch alias, PR write stdout behavior, and generated skill PR command guidance match the change specs.
- [x] 5.2 Update generated `adomi` skill text and its tests so agents know when to use `adomi ado pr fetch`, `ensure`, `reply`, `resolve`, and `reopen`; fetch/read context before replying or resolving when needed; understand PR/thread write PAT-scope requirements; and know that approval/merge/reviewer management are out of scope.
- [x] 5.3 Update user-facing documentation that lists Azure DevOps commands, including required PAT scopes for PR write/thread operations and repository-inference behavior.
- [x] 5.4 Verification checkpoint: run `go test ./internal/cli -run TestAgentSkill`; expected: generated skill tests pass with PR maintenance command guidance and safety boundaries.
- [x] 5.5 Verification checkpoint: run `openspec validate support-ado-pr-maintenance --strict`; expected: change validates.

## 6. Final Verification

- [x] 6.1 Run `gofmt -w internal cmd`; expected: no output and all touched Go files are formatted.
- [x] 6.2 Run `go test ./...`; expected: the full test suite passes.
- [x] 6.3 Optionally run `go build -o bin/adomi ./cmd/adomi`; expected: the CLI builds successfully under ignored `bin/`.
