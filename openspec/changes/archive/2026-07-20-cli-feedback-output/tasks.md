# Tasks: cli-feedback-output

Verification conventions: behavior changes use the existing Go test harness and test-first fixes where practical. Final gate: `go test ./...`, `go vet ./...`, `go build ./...`, and strict OpenSpec validation.

## 1. Reduce the change to operation-level feedback

- [x] 1.1 Remove the experimental global `--verbose` argument pre-scan, HTTP logging transport, pipeline wrapper handling, transport-error rewriting, and their dedicated tests. Restore the pre-change transport behavior while preserving unrelated pipeline safety. *Immediate: `go test ./internal/ado ./internal/cli` passed; scoped `rg` found no retained implementation references.*

## 2. Auth credential feedback

- [x] 2.1 `runADOLogin` / `runADOLogout`: write confirmation lines with credential ref and action to stderr on success. Tests cover confirmation on stderr, stdout empty, and no confirmation on failure. *Deferred: final full test gate necessarily covers the existing CLI tests.*
- [x] 2.2 Add `--json` to login/logout parsers and emit `action` plus `credentialRef` through `writeJSONLine`. *Deferred: final full test gate necessarily covers the existing exact-output tests.*

## 3. Progress hooks in the ADO package

- [x] 3.1 `FetchTree`: optional nil-safe progress callback with one event per fetched work item. *Deferred: final full test gate necessarily covers the existing fetch tests.*
- [x] 3.2 Emit attachment progress immediately after each successful file write, including completed attachments before a later failure. Add the regression test first. *Immediate: the regression test failed with no progress under batch-delayed reporting; after moving the callback behind the file write, `go test ./internal/ado -run 'Attachment|Export' -count=1` passed.*
- [x] 3.3 `FetchWikiContext`: optional progress event per retrieved page. *Deferred: final full test gate necessarily covers the existing wiki tests.*
- [x] 3.4 `FetchPullRequestBundle`: progress events for bundle fetch steps. *Deferred: final full test gate necessarily covers the existing pull-request tests.*

## 4. CLI progress, summaries, and JSON fetch results

- [x] 4.1 Shared stderr feedback writer keeps each event on one physical line and treats callback messages as data. *Deferred: final full test gate necessarily covers the existing feedback tests.*
- [x] 4.2 `ado fetch`: progress and summary on stderr; path-only default stdout; `--json` result with `path`, `workItems`, and `attachments`. *Deferred: final full test gate necessarily covers the existing fetch tests.*
- [x] 4.3 `ado wiki fetch`: progress and summary on stderr; `--json` result with `path` and `pages`. *Deferred: final full test gate necessarily covers the existing wiki tests.*
- [x] 4.4 `ado pr fetch`: progress and summary on stderr; `--json` result matching exported thread/comment count semantics. *Deferred: final full test gate necessarily covers the existing pull-request tests.*

## 5. User and agent documentation

- [x] 5.1 Complete command help for auth and fetch `--json` forms plus stderr confirmations, progress, and summaries; add help-output assertions. *Immediate: `go test ./internal/cli -run 'Help|AgentSkill' -count=1` passed.*
- [x] 5.2 Update `docs/azure-devops.md` and check `docs/getting-started.md` so path capture, JSON results, and stderr feedback are described accurately. *Not applicable: prose-only contract update; direct review confirmed remaining path-only claims apply only to default stdout or unrelated file-creation commands.*
- [x] 5.3 Update the generated agent skill and its tests with the same stdout/stderr and `--json` contract. *Immediate: `go test ./internal/cli -run 'Help|AgentSkill' -count=1` passed.*

## 6. Final gates

- [x] 6.1 Run `go test ./...`, `go vet ./...`, `go build ./...`, and `openspec validate cli-feedback-output --strict --no-interactive`. *Immediate: tests, vet, strict OpenSpec validation, and the host-permitted full build all succeeded; `git diff --check` was clean.*
- [x] 6.2 Run a focused implementation review, apply justified fixes, then rerun affected checks and the full build. *Immediate: the high-risk review found auth-confirmation, feedback-formatting, usage-discoverability, and regression-coverage gaps; all were fixed. No critical/warning findings remain unresolved. `go test ./...`, `go vet ./...`, `go build ./...`, strict OpenSpec validation, and `git diff --check` all passed after the fixes.*
