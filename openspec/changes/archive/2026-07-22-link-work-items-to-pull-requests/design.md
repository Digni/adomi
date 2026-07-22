## Context

The current pull request namespace is a manually parsed Cobra command whose dispatcher recognizes `fetch`, `ensure`, `comment`, `reply`, `resolve`, and `reopen` (`internal/cli/commands.go:172-190`, `internal/cli/ado_pr.go:14-31`). Existing PR mutations fetch the pull request first to resolve repository context (`internal/cli/ado_pr_comment.go:163-175`), and the shared client already exposes both PR fetching and work-item fetching through `ADOClient` (`internal/cli/root.go:30-38`).

Azure DevOps does not add work-item references through the normal PR update contract. Microsoft documents work-item updates as JSON Patch requests with media type `application/json-patch+json`, supports a `/rev` test before a relation update, and requires work-item write permission. Azure's own CLI constructs a PR artifact URI from the PR project GUID, repository GUID, and PR ID, then adds an `ArtifactLink` relation with the display name `Pull Request`.

The local models already retain work-item relations, and `FetchWorkItem` requests `$expand=all`, but `WorkItem` does not retain `rev` (`internal/ado/models.go:9-14`). `PullRequestRepo.Project` is an untyped map (`internal/ado/pullrequest.go:25-30`), and the generic JSON helper always emits `Content-Type: application/json` (`internal/ado/client.go:128-143`). Those are the only client-model gaps needed for the proposed operation.

External references:

- [Microsoft Work Items - Update](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/work-items/update?view=azure-devops-rest-7.1)
- [Microsoft Pull Requests - Get Pull Request](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/get-pull-request?view=azure-devops-rest-7.1)
- [Azure DevOps CLI PR work-item implementation](https://github.com/Azure/azure-devops-cli-extension/blob/master/azure-devops/azext_devops/dev/repos/pull_request.py#L583-L622)

## Goals / Non-Goals

**Goals:**

- Link one or more existing work items to one existing pull request through a focused PR command.
- Validate the full request before the first write and make repeated invocations safe.
- Preserve work-item concurrency through revision-tested relation updates.
- Keep stdout deterministic and machine-readable, including mixed new/already-linked results.
- Reuse current profile, PAT, proxy, Azure DevOps error, and strict JSON response behavior.

**Non-Goals:**

- No unlink, list, generic relation editing, work-item field/state/assignment changes, or automatic work-item discovery from branches, commits, titles, or descriptions.
- No automatic linking as part of `adomi ado pr ensure`; the caller must name the target PR and work items explicitly.
- No PR approval, voting, reviewer management, completion, merge, abandonment, auto-complete, or policy behavior.
- No cross-profile or cross-organization linking. The initial contract covers work items accessible through the selected profile and configured project.
- No transactional guarantee across multiple work items; Azure DevOps updates one work item per request.

## Decisions

### Decision 1: add a direct `pr link` verb with repeatable singular flags

Use:

```bash
adomi ado pr link <pull-request-id> \
  --work-item <work-item-id> \
  [--work-item <work-item-id>...] \
  [--profile <profile-name>] [--global] [--json]
```

This matches the existing direct PR verbs and keeps every linked ID explicit for the manual flag parser. Parse the complete argument set first, require positive IDs and at least one `--work-item`, preserve input order, and reject duplicate requested IDs before configuration, credentials, or network access.

Alternative considered: `adomi ado pr work-item add` with a space-separated `--work-items` list, mirroring Azure CLI. Rejected because adomi's PR namespace currently uses direct verbs, and a value-consuming list is more ambiguous under `DisableFlagParsing: true`.

Alternative considered: extend `pr ensure` with work-item flags. Rejected because linking an existing PR should not require repository/branch inference or couple two independently retryable mutations.

### Decision 2: add the relation through the Work Item Tracking API

Fetch the pull request by ID and require both `repository.project.id` and `repository.id`. Construct the exact Azure DevOps artifact URI:

```text
vstfs:///Git/PullRequestId/{projectGuid}%2F{repositoryGuid}%2F{pullRequestId}
```

For each missing relation, send this JSON Patch to the configured project's work-item endpoint:

```json
[
  {"op":"test","path":"/rev","value":7},
  {
    "op":"add",
    "path":"/relations/-",
    "value":{
      "rel":"ArtifactLink",
      "url":"vstfs:///Git/PullRequestId/{projectGuid}%2F{repositoryGuid}%2F{pullRequestId}",
      "attributes":{"name":"Pull Request"}
    }
  }
]
```

Use the selected profile's configured API version, request `$expand=relations` on the update response, and send `Content-Type: application/json-patch+json`. Extend the existing request options with a content type override, defaulting to the current `application/json`, so authentication, redirect handling, non-2xx diagnostics, and strict single-JSON response decoding remain centralized.

Alternative considered: update `workItemRefs` through the PR PATCH method. Rejected because Azure DevOps does not list it among mutable PR fields and may reject or ignore it.

Alternative considered: invent a write operation under the PR work-items endpoint. Rejected because the documented Git endpoint is list-only; the supported write is the work-item `ArtifactLink` relation.

### Decision 3: preflight all work items, then apply revision-aware writes sequentially

Add the PR project ID to the decoded PR model without discarding existing project metadata, and add `Rev int` to `WorkItem`. Before the first write:

1. Fetch the PR and validate project/repository identities.
2. Fetch every requested work item with its current revision and expanded relations.
3. Fail if any requested item is missing/inaccessible or returns a non-positive/mismatched ID or non-positive revision.
4. Classify exact matching `ArtifactLink` URLs as already linked.

Then update only missing links in input order. The `/rev` test makes a concurrent work-item change fail instead of applying against a stale representation. Validate that each expanded update response returns the requested work-item ID, a positive revision, and the exact new artifact relation. Do not require a specific revision increment beyond positivity because the successful relation in the response is the direct postcondition this command needs to prove.

Already-linked items are successful no-ops. A fully or partially completed invocation can therefore be re-run safely. Do not retry revision conflicts automatically because a fresh read may reveal a materially changed work item that the caller should reconsider.

Alternative considered: optimistically PATCH every requested ID and suppress Azure DevOps's duplicate-relation error, as Azure CLI does. Rejected because preflight provides deterministic new/already-linked output, catches inaccessible IDs before writes, and avoids coupling idempotency to an error-message string.

### Decision 4: make partial multi-item behavior explicit

Azure DevOps exposes individual work-item updates, so a multi-item command cannot be atomic. If a later PATCH fails, stop immediately; earlier links remain, later links are not attempted, stdout stays empty, and the returned error identifies the failed work item plus any IDs linked earlier in this invocation. The user can fix the cause and re-run the same command; preflight then treats earlier links as already linked.

This retains the repository's existing empty-stdout-on-failure contract while making durable partial side effects visible.

### Decision 5: return ordered data-only results

On plain success, print every requested work-item ID in original input order, one per line, regardless of whether it was newly or previously linked. With `--json`, emit one compact object containing:

```json
{
  "pullRequestId":42,
  "workItemIds":[101,102],
  "linkedWorkItemIds":[101],
  "alreadyLinkedWorkItemIds":[102],
  "action":"linked"
}
```

Use `action: "unchanged"` when every requested item was already linked. Buffer output until the whole operation succeeds so failures never leak partial success data to stdout.

## Risks / Trade-offs

- **A later work-item update can fail after earlier links succeeded.** → Preflight all items, stop on the first write failure, report completed IDs in the error, leave stdout empty, and make re-runs idempotent.
- **A work item can change between preflight and PATCH.** → Include a `/rev` test and fail on conflict instead of overwriting or automatically retrying.
- **PR artifact URLs require encoded separators and stable GUID identities.** → Construct the URI only from the fetched PR's project/repository GUIDs, test the exact `%2F` form, and fail before work-item writes when either GUID is absent.
- **Existing Azure DevOps Server versions may vary in JSON Patch behavior.** → Use the profile's configured API version and established error path; do not claim compatibility beyond tested versions or add fallback formats without evidence.
- **Adding a content-type option touches a shared request helper.** → Preserve `application/json` as the default and add regression tests for existing PR/work-item methods alongside the new JSON Patch test.
- **The command crosses the PR and work-item domains.** → Keep it under the PR namespace and document that generic work-item relation editing remains unsupported; the PAT needs code read and work-item write permission.

## Migration Plan

No data or configuration migration is required. This is an additive CLI/API capability. Rollback removes the `link` dispatcher/help/guidance and its client methods; links already created in Azure DevOps remain and can be removed through Azure DevOps itself.

## Open Questions

None required for implementation. Cross-project work items and older Azure DevOps Server compatibility remain deliberately unpromised until exercised against a concrete environment.
