# Adomi Azure DevOps Context Fetcher Handover

## Purpose

Implement a Go CLI tool named `adomi` that can fetch Azure DevOps work item context for agents.

The main use case is:

```bash
adomi ado fetch 12345
```

The command should fetch the work item, walk up the parent tree to Epic, download attachments, store all context in the repository-local `.adomi` folder, and print the generated folder path.

---

## Repository

- Repository: `Digni/adomi`
- Default branch: `main`
- Starting point: blank slate / minimal repo

Create a new branch locally before starting:

```bash
git checkout main
git pull
git checkout -b feature/azure-devops-context-fetcher
```

---

## Core Requirements

The CLI must:

1. Accept a work item or task ID.
2. Query Azure DevOps via REST API.
3. Support Azure DevOps Services and Azure DevOps Server/on-prem.
4. Support multiple Azure DevOps profiles.
5. Use a PAT stored in OS secure storage.
6. Support proxies.
7. Walk up the parent work item tree until Epic.
8. Download attached images and documents.
9. Store all fetched context in `<repo-root>/.adomi`.
10. Print only the generated folder path when done.

---

## Important Design Decision

`.adomi` must be created inside the current Git repository.

When running from any subdirectory:

```bash
adomi ado fetch 12345
```

the tool should walk upward until it finds `.git`, then write to:

```text
<repo-root>/.adomi/
```

Do not store fetched context in the user home directory.

The user home directory may only be used as a fallback config location.

---

## Commands

Work item context, comments, and credentials:

```bash
adomi ado fetch <work-item-id>
adomi ado fetch <work-item-id> --profile <profile-name>
adomi ado comment <work-item-id> --message <text>
adomi ado comment <work-item-id> --message-file <path>
adomi ado work-item comment <work-item-id> --message-file <path> # namespace alias
adomi ado login --profile <profile-name>
adomi ado logout --profile <profile-name>
adomi ado profiles list
adomi config init
adomi config init --global
```

Pull request context and conservative PR maintenance:

```bash
adomi ado pr fetch <pull-request-id>
adomi ado pr <pull-request-id> # compatibility alias for fetch
adomi ado pr ensure --title <title>
adomi ado pr ensure --description-file <path>
adomi ado pr comment <pull-request-id> --message-file <path>
adomi ado pr comment <pull-request-id> --file <path> --line <line> --message-file <path>
adomi ado pr reply <pull-request-id> --thread <thread-id> --message-file <path>
adomi ado pr resolve <pull-request-id> --thread <thread-id>
adomi ado pr reopen <pull-request-id> --thread <thread-id>
```

Read-only pipeline run status:

```bash
adomi ado pipeline list [--profile <profile-name>] [--global]
adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]
```

Both pipeline commands run inside a Git repository, including with `--global`; use `--profile <profile-name>` to select a configured profile. `pipeline list` requests the exact `inProgress` runs across YAML and classic Build pipelines and follows the returned continuation pages as a best-effort one-shot view rather than a transactional snapshot. `pipeline get` accepts a decimal Build run ID from 1 through 2147483647. Success is compact JSON, with list returning an ordered `runs` array and get returning one run object; unavailable result, source, timestamp, and link fields are JSON null.

The PAT needs the `vso.build` read scope. Pipeline requests require HTTPS, except HTTP is allowed for exact localhost or a direct IPv4/IPv6 loopback address. Loopback HTTP requests bypass configured proxies so credentials remain on-machine. The commands do not poll, fetch stage/job/environment or other execution details, mutate pipelines, or inspect classic Release deployments.

Work item comment maintenance is intentionally narrow: `adomi ado comment <work-item-id>` and the `adomi ado work-item comment <work-item-id>` alias add a text-only comment from exactly one message source (`--message` or `--message-file`). Plain stdout returns only the created work item comment ID; `--json` returns one compact JSON object. No work item field updates, state transitions, assignment changes, relation edits, attachment uploads, comment updates/deletions, or reaction management are supported.

`adomi ado pr ensure` runs inside the current Git repository. It infers the Azure DevOps repository from matching git remotes, the source branch from the current branch, and the target branch from the selected remote default branch when possible. Use `--repository <name-or-id>`, `--source <branch>`, or `--target <branch>` when inference is ambiguous or unavailable.

PR maintenance is intentionally narrow: it can create/update title or description for the active branch PR, create a new PR-level comment thread, create a right-side single-line inline comment on the latest changed PR file version, reply to explicit thread IDs, and mark explicit threads `fixed` or `active`. Inline comments do not support deleted/left-side targets, ranges, explicit offsets, suggestions, or manual iteration overrides. It does not approve, reject, merge/complete, abandon, set auto-complete, bypass policies, or manage reviewers.

`adomi ado config init` is retained only as a hidden compatibility alias; prefer `adomi config init` in user-facing docs and scripts.

---

## Config

Use YAML.

Preferred config location:

```text
<repo-root>/.adomi/config.yaml
```

Fallback config location:

```text
~/.config/adomi/config.yaml
```

Example config:

```yaml
azureDevOps:
  defaultProfile: company-cloud

  profiles:
    company-cloud:
      patRef: shared-ado-pat
      baseUrl: https://dev.azure.com/my-org
      organization: my-org
      project: MyProject
      apiVersion: "7.1"
      proxy: ""

    company-onprem:
      patRef: shared-ado-pat
      baseUrl: https://tfs.company.local/tfs/DefaultCollection
      project: MyProject
      apiVersion: "7.0"
      proxy: http://proxy.company.local:8080
```

Notes:

- `organization` is useful for cloud profiles but should not be required for on-prem if `baseUrl` already contains the collection.
- `patRef` is optional. When set, the profile reads its PAT from that shared keyring reference; when omitted, the profile name is used as the keyring reference.
- `apiVersion` should default to `"7.1"` when omitted.
- `proxy` should be optional.

---

## Output Layout

For work item `12345`:

```text
<repo-root>/.adomi/
  context/
    work-items/
      12345/
        index.json
        tree.json
        items/
          12345.json
          12001.json
          10000.json
        html/
          12345.html
          12001.html
          10000.html
        attachments/
          12345/
            screenshot.png
            spec.pdf
    pull-requests/
      42/
        index.json
        pull-request.json
        threads.json
        comments.md
        threads/
          1.json
```

Add this `.gitignore`:

```gitignore
.adomi/
bin/
*.test
coverage.out
```

---

## Suggested File Structure

```text
go.mod
cmd/adomi/main.go

internal/cli/root.go
internal/cli/ado.go

internal/workspace/repo.go

internal/config/config.go

internal/securestore/keyring.go

internal/ado/client.go
internal/ado/models.go
internal/ado/fetch.go
internal/ado/attachments.go
internal/ado/export.go
```

---

## Dependencies

Use:

```bash
go get github.com/zalando/go-keyring
go get gopkg.in/yaml.v3
go get github.com/spf13/cobra
```

The MVP originally avoided Cobra, but the post-MVP CLI now uses Cobra for command hierarchy, help, and usage behavior while keeping command actions in `internal/cli` testable.

---

## Repo Root Detection

Implement:

```go
func FindRepoRoot(start string) (string, error)
```

Behavior:

1. Start from the current working directory.
2. Walk upward.
3. Return the first directory containing `.git`.
4. Return an error if not inside a Git repo.

Example implementation shape:

```go
func FindRepoRoot(start string) (string, error) {
    dir, err := filepath.Abs(start)
    if err != nil {
        return "", err
    }

    for {
        gitPath := filepath.Join(dir, ".git")
        if _, err := os.Stat(gitPath); err == nil {
            return dir, nil
        }

        parent := filepath.Dir(dir)
        if parent == dir {
            return "", errors.New("not inside a git repository")
        }

        dir = parent
    }
}
```

---

## Secure PAT Storage

Use:

```go
github.com/zalando/go-keyring
```

Store PAT as:

```text
service: adomi.azure-devops
user: <profile-name>
```

`adomi ado login --profile company-cloud` should prompt for a PAT and save it.

`adomi ado logout --profile company-cloud` should delete it.

Hidden input is preferred, but normal stdin prompt is acceptable for the first MVP.

---

## Azure DevOps Auth

Azure DevOps PAT uses Basic Auth with an empty username:

```go
req.SetBasicAuth("", pat)
```

Read-only context fetch commands require credentials that can read the requested work items or pull requests. Work item comment commands require work item write permission (the OAuth-equivalent `vso.work_write` scope). PR maintenance commands require credentials with appropriate PR/thread write permissions, such as code write permission for creating/updating PRs and thread/comment write permission for review thread maintenance. Never print, log, echo, commit, or otherwise expose PAT values.

---

## HTTP Client

Support profile proxy and environment proxy.

Rules:

1. If profile config has `proxy`, use it.
2. Otherwise use `http.ProxyFromEnvironment`.
3. Timeout: `60s`.

Example shape:

```go
func NewHTTPClient(proxyURL string) (*http.Client, error) {
    transport := &http.Transport{}

    if proxyURL != "" {
        parsed, err := url.Parse(proxyURL)
        if err != nil {
            return nil, err
        }
        transport.Proxy = http.ProxyURL(parsed)
    } else {
        transport.Proxy = http.ProxyFromEnvironment
    }

    return &http.Client{
        Transport: transport,
        Timeout:   60 * time.Second,
    }, nil
}
```

---

## Azure DevOps API

Fetch a work item with relations:

```http
GET {baseUrl}/{project}/_apis/wit/workitems/{id}?$expand=all&api-version={apiVersion}
```

Base URL examples:

```text
https://dev.azure.com/my-org
https://tfs.company.local/tfs/DefaultCollection
```

Build the URL as:

```text
{baseUrl}/{project}/_apis/wit/workitems/{id}
```

Create a work item comment with the documented preview API:

```http
POST {baseUrl}/{project}/_apis/wit/workitems/{workItemId}/comments?api-version=7.0-preview.3
Content-Type: application/json

{ "text": "..." }
```

The response may identify the created comment as either `id` or `commentId`; normalize either positive value for stdout and JSON output.

---

## Parent Traversal

Parent relation:

```text
System.LinkTypes.Hierarchy-Reverse
```

Algorithm:

1. Fetch initial work item.
2. Store it.
3. Find parent relation.
4. Fetch parent.
5. Repeat until:
   - `System.WorkItemType == "Epic"`
   - no parent exists
   - parent was already visited

The returned tree should preserve the chain from input item up to Epic.

---

## Attachments

Attachment relation:

```text
AttachedFile
```

For each attachment:

1. Download the relation URL using the same auth.
2. Determine filename from relation attributes `name`.
3. Fallback to URL filename.
4. Final fallback: `attachment-{n}`.
5. Store under:

```text
attachments/<work-item-id>/<filename>
```

---

## Export Format

Canonical format: JSON.

Write:

```text
items/<id>.json
tree.json
index.json
```

Also generate simple HTML files:

```html
<h1>{id}: {title}</h1>
<p><strong>Type:</strong> {type}</p>
<div>{description}</div>
```

Keep HTML minimal. JSON is the primary agent-readable format.

---

## `index.json` Shape

Example:

```json
{
  "source": "azure-devops",
  "profile": "company-cloud",
  "project": "MyProject",
  "rootWorkItemId": 12345,
  "epicWorkItemId": 10000,
  "createdAt": "2026-04-26T12:00:00Z",
  "workItems": [
    {
      "id": 12345,
      "type": "Task",
      "title": "Implement checkout validation",
      "path": "items/12345.json",
      "htmlPath": "html/12345.html",
      "attachmentsPath": "attachments/12345"
    }
  ]
}
```

---

## CLI Output Contract

`adomi ado fetch` and `adomi ado pr fetch` should print only the final context folder path to stdout:

```text
/path/to/repo/.adomi/context/work-items/12345
```

Successful work item comment commands (`adomi ado comment` and `adomi ado work-item comment`) keep stdout data-only: plain output prints the created work item comment ID and `--json` prints one compact JSON object.

Successful PR maintenance commands keep stdout data-only: `ensure` prints the PR ID, `comment` prints the created thread ID, `reply` prints the created comment ID, `resolve`/`reopen` print the thread ID, and `--json` prints one compact JSON object. Errors, prompts, diagnostics, validation failures, and Azure DevOps/network failures should go to stderr and leave stdout empty.

This should work:

```bash
LOCATION=$(adomi ado fetch 12345)
cat "$LOCATION/index.json"
```

---

## Acceptance Criteria

Running:

```bash
adomi ado fetch 12345 --profile company-cloud
```

inside any subfolder of a Git repo should:

1. Resolve the Git repo root.
2. Read config from `<repo-root>/.adomi/config.yaml` or `~/.config/adomi/config.yaml`.
3. Read PAT from OS secure storage.
4. Fetch work item `12345`.
5. Fetch parents up to Epic.
6. Download attachments.
7. Write all files under `<repo-root>/.adomi/...`.
8. Print only the generated folder path.

---

## Tests

Add tests for:

1. Repo root detection.
2. Config loading with repo config preferred over home config.
3. Parent traversal using fake client responses.
4. Output path generation.
5. Attachment filename fallback logic.

---

## Implementation Order

1. Initialize Go module.
2. Add `.gitignore`.
3. Add CLI skeleton.
4. Add repo root detection.
5. Add YAML config loader.
6. Add keyring PAT storage.
7. Add Azure DevOps HTTP client.
8. Add single work item fetch.
9. Add parent traversal up to Epic.
10. Add JSON export into repo-local `.adomi`.
11. Add attachment download.
12. Add minimal HTML export.
13. Add tests.
14. Run `go test ./...`.

---

## Non-goals for MVP

Do not implement:

- Full Azure DevOps WIQL search.
- Bidirectional sync.
- Work item field/state/relation mutation beyond text-only comment creation.
- Rich HTML rendering.
- Background daemon.
- Agent-specific prompt generation.

The MVP is a local context fetcher.
