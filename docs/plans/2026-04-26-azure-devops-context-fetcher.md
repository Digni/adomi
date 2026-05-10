# Azure DevOps Context Fetcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go MVP CLI described in `adomi_azure_devops_handover.md` so `adomi ado fetch <id>` writes Azure DevOps work item context under the current repository's `.adomi` directory and prints only the generated folder path.

**Architecture:** Keep the CLI thin and split behavior into focused internal packages: workspace discovery, YAML configuration, secure PAT storage, Azure DevOps API access, traversal, attachment handling, and export. This MVP plan originally used standard-library command parsing; the post-MVP CLI has since moved to Cobra for command hierarchy while preserving the same internal package boundaries.

**Tech Stack:** Go 1.26.2, `gopkg.in/yaml.v3`, `github.com/zalando/go-keyring`, `github.com/spf13/cobra`, standard `net/http`, `httptest`, and `testing`.

---

## Grounded Facts

- The repository is intentionally minimal: `README.md:1` contains only `# adomi`.
- Existing tracked files are `.gitignore` and `README.md`; `adomi_azure_devops_handover.md` is currently untracked.
- `.gitignore:1-31` already has a broad Go template; it includes `*.test` at `.gitignore:12`, but does not yet include `.adomi/`, `bin/`, or exact `coverage.out`.
- The handover requires repository-local `.adomi` storage, not home-directory storage, at `adomi_azure_devops_handover.md:50-68`.
- MVP commands are listed at `adomi_azure_devops_handover.md:72-87`.
- Config format and precedence are specified at `adomi_azure_devops_handover.md:91-132`.
- Output layout is specified at `adomi_azure_devops_handover.md:136-170`.
- Suggested package/file structure is specified at `adomi_azure_devops_handover.md:174-194`.
- Dependencies are specified at `adomi_azure_devops_handover.md:198-207`; the old MVP-only "no Cobra" constraint has been superseded by the post-MVP Cobra command tree.
- Azure DevOps URL, auth, proxy, traversal, attachment, export, and stdout contracts are specified at `adomi_azure_devops_handover.md:253-455`.
- Acceptance criteria and required tests are specified at `adomi_azure_devops_handover.md:459-507`.
- Current branch is `feature/ado-fetch`, while the handover suggested `feature/azure-devops-context-fetcher` from `main`. This plan assumes continuing on the current branch unless the user asks for the exact branch name before approval.

## Files

- Create: `go.mod` and `go.sum`
- Create: `cmd/adomi/main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/ado.go`
- Create: `internal/workspace/repo.go`
- Create: `internal/workspace/repo_test.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/securestore/keyring.go`
- Create: `internal/ado/models.go`
- Create: `internal/ado/client.go`
- Create: `internal/ado/client_test.go`
- Create: `internal/ado/fetch.go`
- Create: `internal/ado/fetch_test.go`
- Create: `internal/ado/attachments.go`
- Create: `internal/ado/attachments_test.go`
- Create: `internal/ado/export.go`
- Create: `internal/ado/export_test.go`
- Modify: `.gitignore:1-31`

## Task 1: Module, Ignore Rules, And CLI Shell

- [ ] Initialize the module as `github.com/Digni/adomi`.
  - Run: `go mod init github.com/Digni/adomi`
  - Expected: `go.mod` exists with module path `github.com/Digni/adomi`.
- [ ] Add dependencies.
  - Run: `go get github.com/zalando/go-keyring gopkg.in/yaml.v3`
  - Expected: `go.mod` and `go.sum` include both dependencies.
- [ ] Update `.gitignore` to include the handover entries that are missing: `.adomi/`, `bin/`, and `coverage.out`.
- [ ] Create `cmd/adomi/main.go` to call `cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)` and exit non-zero on errors.
- [ ] Create `internal/cli/root.go` with `Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error` and route only `ado`.
- [ ] Create `internal/cli/ado.go` with command parsing for `ado fetch`, `ado login`, `ado logout`, and `ado profiles list`.
- [ ] Verification checkpoint:
  - Run: `go test ./...`
  - Expected: packages compile; behavior tests may be added in later tasks.

## Task 2: Repository Root Detection

- [ ] Write tests in `internal/workspace/repo_test.go` for:
  - returning the directory that contains `.git` when called from a nested subdirectory.
  - returning an error when no parent contains `.git`.
  - accepting a `.git` file as valid so linked worktrees are handled like normal Git repositories.
- [ ] Implement `internal/workspace/repo.go` with `FindRepoRoot(start string) (string, error)` using `filepath.Abs`, upward walking, and `os.Stat`.
- [ ] Verification checkpoint:
  - Run: `go test ./internal/workspace`
  - Expected: all repo detection tests pass.

## Task 3: YAML Configuration Loading

- [ ] Write tests in `internal/config/config_test.go` for:
  - repository config at `<repo-root>/.adomi/config.yaml` overriding home fallback config.
  - fallback to `$HOME/.config/adomi/config.yaml` when repo config is missing.
  - defaulting `apiVersion` to `7.1`.
  - selecting explicit profile or `azureDevOps.defaultProfile`.
  - returning useful errors for missing profile, missing `baseUrl`, or missing `project`.
- [ ] Implement `internal/config/config.go` with structs matching the handover YAML shape and a loader that accepts `repoRoot`, `homeDir`, and requested profile.
- [ ] Verification checkpoint:
  - Run: `go test ./internal/config`
  - Expected: config precedence, defaults, and validation tests pass.

## Task 4: Secure PAT Storage

- [ ] Create `internal/securestore/keyring.go` with a small interface:
  - `Get(profile string) (string, error)`
  - `Set(profile, pat string) error`
  - `Delete(profile string) error`
- [ ] Use keyring service name `adomi.azure-devops` and username equal to the profile name.
- [ ] Wire `adomi ado login --profile <profile>` to prompt `Azure DevOps PAT for <profile>:` on stderr, read from stdin, trim the trailing newline, and store through the interface.
- [ ] Wire `adomi ado logout --profile <profile>` to delete the stored PAT.
- [ ] Verification checkpoint:
  - Run: `go test ./internal/cli ./internal/securestore`
  - Expected: CLI login/logout tests use a fake store and do not touch the OS keyring.

## Task 5: Azure DevOps HTTP Client And Models

- [ ] Create `internal/ado/models.go` with `WorkItem`, `Relation`, and helper methods for ID, type, title, description, parent relation, and attachment relations. Keep raw fields as `map[string]any` for forward compatibility.
- [ ] Create `internal/ado/client.go` with:
  - `NewHTTPClient(proxyURL string) (*http.Client, error)`
  - `Client.FetchWorkItem(ctx context.Context, id int) (*WorkItem, error)`
  - `Client.Download(ctx context.Context, rawURL string) ([]byte, error)`
- [ ] Build work item URLs as `{baseUrl}/{project}/_apis/wit/workitems/{id}?$expand=all&api-version={apiVersion}`.
- [ ] Apply `req.SetBasicAuth("", pat)` for work items and attachment downloads.
- [ ] Return errors on non-2xx status codes with enough context for stderr.
- [ ] Write `internal/ado/client_test.go` with `httptest.Server` coverage for URL generation, auth header presence, JSON decoding, non-2xx errors, and proxy parse validation.
- [ ] Verification checkpoint:
  - Run: `go test ./internal/ado`
  - Expected: client/model tests pass.

## Task 6: Parent Traversal

- [ ] Define a fetcher interface in `internal/ado/fetch.go` so traversal can be tested without HTTP.
- [ ] Write `internal/ado/fetch_test.go` for:
  - input task -> parent user story -> Epic chain.
  - no-parent item returns a one-item chain.
  - repeated parent ID stops traversal.
  - fetch errors are returned.
- [ ] Implement traversal using relation `System.LinkTypes.Hierarchy-Reverse`; parse parent IDs from relation URLs robustly by reading the last numeric path segment.
- [ ] Preserve item order from input item upward to Epic in `FetchTree`.
- [ ] Verification checkpoint:
  - Run: `go test ./internal/ado -run 'TestFetchTree|TestParent'`
  - Expected: traversal tests pass.

## Task 7: Attachment Handling

- [ ] Write `internal/ado/attachments_test.go` for filename selection:
  - relation attribute `name` wins.
  - URL basename is used when `name` is empty.
  - `attachment-<n>` is used when neither source has a valid basename.
  - unsafe path separators are reduced to safe local filenames.
- [ ] Implement `FilenameForAttachment(relation Relation, ordinal int) string`.
- [ ] Implement download orchestration that stores bytes under `attachments/<work-item-id>/<filename>` using the same authenticated `Client.Download`.
- [ ] Verification checkpoint:
  - Run: `go test ./internal/ado -run 'TestAttachment|TestFilename'`
  - Expected: attachment tests pass.

## Task 8: Exporter

- [ ] Write `internal/ado/export_test.go` for:
  - output path `<repo-root>/.adomi/context/work-items/<root-id>`.
  - `index.json`, `tree.json`, per-item JSON, and per-item HTML are written.
  - `index.json` contains `source`, `profile`, `project`, `rootWorkItemId`, `epicWorkItemId`, `createdAt`, and relative paths.
  - HTML escapes title/type and includes the raw Azure DevOps description inside the `<div>` as required by the handover's minimal HTML contract.
- [ ] Implement `ExportContext` in `internal/ado/export.go` with deterministic directory creation and pretty JSON.
- [ ] Verification checkpoint:
  - Run: `go test ./internal/ado -run 'TestExport|TestOutputPath'`
  - Expected: export tests pass.

## Task 9: End-To-End CLI Wiring

- [ ] Add CLI tests using fake config/PAT/fetch/export collaborators where possible to verify:
  - `adomi ado fetch 12345 --profile company-cloud` prints exactly one path plus newline to stdout.
  - errors are returned and printed through `cmd/adomi/main.go` to stderr, not stdout.
  - `adomi ado profiles list` prints configured profile names.
  - missing profile flag for login/logout is rejected.
- [ ] Wire real `fetch` command:
  - find current working directory.
  - resolve repo root with `workspace.FindRepoRoot`.
  - load config with repo preference and home fallback.
  - read PAT from secure storage.
  - construct Azure DevOps client with proxy.
  - fetch parent tree.
  - export JSON/HTML and attachments under repo-local `.adomi`.
  - print only final folder path to stdout.
- [ ] Verification checkpoint:
  - Run: `go test ./internal/cli ./cmd/adomi`
  - Expected: CLI command behavior tests pass.

## Task 10: Full Verification

- [ ] Run formatter.
  - Run: `gofmt -w cmd internal`
  - Expected: no output; files formatted.
- [ ] Run all tests.
  - Run: `go test ./...`
  - Expected: all packages pass.
- [ ] Optional build smoke test.
  - Run: `go build -o bin/adomi ./cmd/adomi`
  - Expected: binary builds successfully under ignored `bin/`.

## Edge Cases And Failure Modes

- Config integration:
  - Missing repo config should fall back to home config.
  - Missing both configs should return a clear error.
  - Missing `apiVersion` should become `7.1`.
  - Empty `baseUrl` or `project` should stop before any network request.
- Keyring integration:
  - Missing PAT should stop with an error naming the profile but not printing secrets.
  - Login must not echo or log the PAT.
- HTTP integration:
  - Invalid proxy URL should fail client creation.
  - Non-2xx Azure DevOps responses should fail the command and avoid writing partial success as stdout.
  - Empty or malformed work item JSON should fail decoding.
- Traversal transformation:
  - Parent URL parsing assumes Azure DevOps relation URLs contain a numeric work item ID; if not, traversal stops with a clear parse error.
  - Cycles are stopped by a visited-ID set.
  - If no Epic exists before the root, the available chain is still exported.
- Attachment transformation:
  - Attachment names may contain path separators; filenames must be sanitized before writing.
  - Duplicate names under one work item need deterministic collision handling, such as suffixing `-2`.
  - Attachment download failures should fail the fetch command rather than silently producing incomplete context.
- Export integration:
  - Output paths include profile and project, so profile/project names must be sanitized or validated before path joining to avoid path traversal.
  - Existing output for the same work item may be overwritten deterministically.
  - CLI stdout must remain path-only for successful `fetch`; diagnostics go to stderr.

## Blast Radius

- This is a new CLI and new internal package tree in a minimal repo, so no existing runtime behavior is shared.
- Existing `.gitignore` behavior changes only by adding `.adomi/`, `bin/`, and `coverage.out`; existing Go ignore patterns remain.

## Planning Verification

- [x] Every file/line reference was read directly by me.
- [x] I ran diagnostic commands myself for facts in the plan: `rg --files`, `nl -ba README.md`, `nl -ba .gitignore`, `nl -ba adomi_azure_devops_handover.md`, `find`, `git status --short --branch`, `git branch --show-current`, `git branch --list`, `git remote -v`, `go version`.
- [x] Each step has a verification checkpoint with concrete command and expected outcome.
- [x] I searched for existing patterns before proposing new ones.
- [x] I checked current filesystem state for counts, paths, and names.
- [x] Blast radius listed; no shared code exists beyond `.gitignore`.
- [x] Edge cases documented for every integration point and data transformation.

## Pre-Mortem

1. The plan could fail if the current branch mismatch matters: `adomi_azure_devops_handover.md:23-29` says to create `feature/azure-devops-context-fetcher`, but the repository is already on `feature/ado-fetch`. Mitigation: confirm branch choice before code execution.
2. The plan could fail on keyring tests if CLI code directly imports the concrete keyring implementation. Mitigation: keep PAT storage behind an interface and test CLI with a fake store.
3. The plan could fail on Azure DevOps Server/on-prem URLs if URL joining strips collection path segments from `baseUrl`. Mitigation: use parsed URL path joining that preserves existing base path and add tests with `/tfs/DefaultCollection`.

## Approval Gate

No implementation code should be written until this plan is approved.
