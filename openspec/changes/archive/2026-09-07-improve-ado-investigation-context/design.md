## Context

See [proposal.md](proposal.md) for motivation and delivery scope. These artifacts propose a standard-risk implementation; this session changes planning files only.

Current implementation facts, checked in this checkout:

| Boundary | Evidence | Consequence |
| --- | --- | --- |
| PR discovery | `internal/ado/client_pr.go:75` applies source/target/status filters but issues one request; `internal/ado/pullrequest.go:216` has no paging options. | Reuse the existing list method with optional page controls. |
| Existing list consumer | `internal/cli/ado_pr_ensure.go:72` uses list results to decide whether to create/update a PR. | Keep its default request and return behavior unchanged. |
| Repository/ref selection | `internal/cli/ado_pr_ensure.go:169` selects a repository; `:307` normalizes short names to `refs/heads/` and preserves full refs. | Reuse repository selection and normalization without ensure's current/default-branch inference. |
| Pipeline summaries | `internal/cli/ado_pipeline.go:14` selects in-progress versus recent mode; `internal/ado/pipeline.go:115` already traverses continuation tokens. | Thread the branch into both URL builders, preserving modes and output projection. |
| Pipeline transport | `internal/ado/pipeline_transport.go:13`, `:36`, `:68` enforce HTTPS/direct loopback, safe diagnostics, no redirects, and 8 MiB responses. | New inspection reads use these policies without changing other clients. |
| Work-item selection | `internal/ado/fetch.go:36` fetches root, parent chain through Epic, and root's direct children with ID deduplication. | Comments cover precisely these items. |
| Comment gap | `internal/ado/models.go:23` models only creation-response identity; `internal/ado/client_workitem.go:41` only posts comments. | Add a read model and paged reader; preserve comment creation. |
| Existing exports | `internal/ado/export.go:52` removes the old bundle before downloading attachments; `:129` escapes HTML. | Fetch comments before entering export; do not claim existing exports are atomic. |
| Stdout/counts | `internal/cli/ado_fetch.go:62` counts exporter progress callbacks as attachments; `:90` defines `{path,workItems,attachments}`. | Keep that callback attachment-only and retain the JSON shape even with comments enabled. |
| Guidance overlap | `enable-complete-pr-flow` replaces generated-skill and workflow-documentation requirements. | Use a distinct added skill requirement; preserve its governance wording in the documentation delta. |

Obsidian `1.Projects/adomi/1. Daily Work/2026/07/2026-07-15.md` confirms the existing one-shot/transport/output decisions. The existing run-list lack of an aggregate-byte ceiling is a recorded trade-off; the new inspection budget below applies only to inspection.

## Goals / Non-Goals

**Goals:** Reuse existing command/client/export seams; make selection and evidence coverage explicit; retain enough identity to connect records, attempts, logs, and tests; preserve existing invocations.

**Non-Goals:** An investigation dashboard, a generic ADO query layer, local filtering of a project-wide recent window, implicit current-branch filtering, work-item traversal expansion, work-item revision history/reactions/comment attachments, raw test-report attachments or coverage, complete historical retry reconstruction, or deployment/runtime verification. No global skill installation or publishing is part of this proposal.

## Decisions

### 1. PR listing is a small JSON discovery command

Syntax: `adomi ado pr list [--source <branch>] [--target <branch>] [--status active|completed|abandoned|all] [--repository <name-or-id>] [--profile <name>] [--global]`.

Default status is `active`; omitted source/target means no corresponding filter. Repository selection follows `selectPRRepository`, including its origin tie-break and explicit override. No branch lookup is needed, so detached HEAD works. Trim branch values, reject empty values, and reuse `normalizeBranchRef`; do not interpret `origin/foo` as a local remote-tracking shortcut. Full refs remain exact.

Add optional `$top`/`$skip` controls to `PullRequestListOptions`, applied only when requested. The discovery operation requests pages of 100, advances skip by the number received, and continues until an empty page, including after short pages. This avoids assuming the server honors the requested page size. Preserve server order; reject duplicates/non-progress, invalid identity, missing collection, and safety-limit exhaustion. Bound discovery at 1,000 pages, 100,000 PRs, and 8 MiB per page. There is no silent result limit and no caller-managed pagination flag.

Stdout is always one compact `{ "pullRequests": [...] }` object plus newline, matching pipeline list's JSON-first convention. Each entry has `pullRequestId`, `title`, `status`, `isDraft`, `repositoryId`, `repositoryName`, `sourceRefName`, `targetRefName`, `creationDate`, `closedDate`, and `url` (the API URL). Optional dates/link are null when absent; available dates use UTC RFC3339Nano. Discovery omits descriptions and discussion; `pr fetch` remains the detailed follow-up. An empty array means no matches in the completed observed traversal, not a transactional snapshot.

Alternatives: exposing the raw response couples users to large payloads; a default top-N window cannot reliably establish absence. Making every existing `ListPullRequests` invocation paginate would also change the mutation preflight used by `pr ensure`.

The documented [PR list API](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/get-pull-requests?view=azure-devops-rest-7.1) supports these filters and offset paging; its description is truncated, reinforcing the separate fetch workflow.

### 2. Branch filtering composes with both pipeline list modes

Add optional `--branch` to list only. Normalize it like PR refs and add `branchName` to every Builds list request. Without `--last`, retain `statusFilter=inProgress`; with `--last N`, retain `queryOrder=queueTimeDescending`, no status filter, and a decreasing `$top`. No branch flag means the existing requests and output stay unchanged. Reject a returned mismatched or missing source branch on a filtered request instead of silently filtering an incorrect server response.

Filtering remains project-scoped: identical refs in multiple repositories can match, just as the current command spans project pipelines. Full `refs/pull/.../merge` refs can be requested explicitly; a head-branch search does not infer associated PR validation refs. No repository flag is added to pipeline list in this change.

This uses the [Builds list server-side branch parameter](https://learn.microsoft.com/en-us/rest/api/azure/devops/build/builds/list?view=azure-devops-rest-7.1). Filtering locally after fetching 200 runs would preserve the reported bug.

### 3. Comments enrich the existing work-item bundle only when requested

After `FetchTree`, fetch comments once per distinct exported item, before calling `ExportContext`. Use a separate read model so the comment-create response/output remains stable. The [Comments list API](https://learn.microsoft.com/en-us/rest/api/azure/devops/wit/comments/get-comments?view=azure-devops-rest-7.1) documents `7.1-preview.4`, a body continuation token, sort order, and deleted-comment selection. Pin that version for this new read endpoint only; do not change the existing write endpoint version.

Request ascending order and `includeDeleted=false`. Follow opaque body tokens by rebuilding the configured endpoint, never requesting the returned `nextPage` URL. Keep current comment version, normalized ID (`id` or `commentId`, rejecting conflicts), work-item ID, text, reported format if present, author identity/display name, created/modified times, version, and source URL. Missing optional metadata stays null; deleted bodies are omitted. Fail on malformed collections, mismatched item IDs, duplicate comment IDs, repeated/blank non-terminal tokens, or the new reader's bounds of 1,000 pages/100,000 comments per item and 8 MiB per response.

Write `comments/<id>.json` with `{workItemId,comments:[...]}` and add `commentsPath` to that item's index entry. Append an HTML-escaped discussion section to `html/<id>.html`. Empty collections produce an explicit empty file/section; flag omission creates no comment fields/files or requests. Existing `items` JSON and `tree.json` stay unchanged. Plain stdout remains the path; JSON remains `{path,workItems,attachments}` in both modes. Report comment progress through the fetch feedback writer, not the attachment-count callback.

A comment-read failure leaves the previous export intact because replacement has not begun. Existing attachment/filesystem export-failure semantics remain unchanged. A later refresh without the flag removes old comment artifacts through the existing replacement behavior. This is narrower than converting all exports to an atomic publication mechanism.

### 4. Inspection exports a finite set of evidence for one Build run

Syntax: `adomi ado pipeline inspect <run-id> [--profile <name>] [--global] [--json]`. Accept the same signed-int32 run-ID range and repository requirement as `get`. Read the run using its existing normalization, then fetch the root timeline and referenced detail timelines. Preserve observed record IDs, parent IDs, type/name/order, state/result, times, attempt identity, issues, task identity, and log references. Keep unknown record types/status strings and missing optional values; do not manufacture stage nodes for classic Build runs. Record previous-attempt references, but do not fetch timelines solely to reconstruct earlier attempts. The manifest states this scope.

Use validated timeline/log identifiers to construct endpoints under the configured project. Deduplicate visited timeline IDs and log IDs. For every observed task with result `failed` and a log reference, request its entire log as `text/plain` once. A failed task without a log reference gets `logAvailability: "notPublished"`; other task logs get `"notRequested"`. Retrieved logs get `"downloaded"` and a relative path. Skipped/canceled tasks remain visible in the timeline; their logs and successful-task logs are outside this increment.

The [Build timeline API](https://learn.microsoft.com/en-us/rest/api/azure/devops/build/timeline/get?view=azure-devops-rest-7.1) exposes records and detail references. [Build log reads](https://learn.microsoft.com/en-us/rest/api/azure/devops/build/builds/get-build-log?view=azure-devops-rest-7.1) support a text representation. Do not follow their returned URLs or download a log archive.

Retrieve Test runs using the selected Build response's `uri` as the server-side `buildUri` filter, `includeRunDetails=true`, and offset paging. Retain that URI internally for inspection without adding it to the established `list`/`get` JSON. Inspection fails if the required build URI is unavailable. Reject explicit test-run build-identity mismatches from the documented `build` shallow reference and available `buildConfiguration` identity/URI. Shallow-reference IDs may be decimal strings. Do not use a project-wide list or a recent-date search window: [Test Runs list](https://learn.microsoft.com/en-us/rest/api/azure/devops/test/runs/list?view=azure-devops-rest-7.1) supports the build filter directly.

For each discovered test run, retrieve all outcome categories via [Test Results list](https://learn.microsoft.com/en-us/rest/api/azure/devops/test/results/list?view=azure-devops-rest-7.1), with pages of 1,000 and no extra-detail expansion. Both offset readers continue until an empty page, advancing by items received. Result identity is scoped by test-run ID: reject duplicates within a run while allowing the same result ID in different runs. Preserve test-run/result identity, title or automated test name, state/outcome, duration, and available failure message/stack trace; record reported run totals and totals calculated from retrieved outcomes separately. An empty test list means no published tests were returned for this build. It says nothing about unpublished test execution. Build/Test API versions otherwise follow the configured API version.

### 5. Inspection publication and failures are explicit

Create a unique snapshot directory under `.adomi/context/pipelines/<run-id>/` using the standard library. This avoids destructive replacement or mixing observations from repeated inspections. Each successful directory contains `index.json`, `run.json`, `timeline.json`, `logs/<log-id>.txt`, `tests/runs.json`, and `tests/<test-run-id>/results.json`. The index contains source/profile/project/base URL, run ID, UTC observation start/finish times, the one-shot/attempt scope, counts, log availability, and relative artifact paths. Only write its final index after all requested reads and artifact writes succeed; remove this invocation's unfinished directory on failure. Existing snapshots remain untouched. No automatic retention manager is added.

Default stdout is only the snapshot path; `--json` yields `{path,runId,timelineRecords,failedTaskLogs,testRuns,testResults}` plus newline. JSON counts describe exported records, not test outcomes. Diagnostics/progress go to stderr without remote content. All HTTP errors, including 401/403 and missing/retained-away 404 resources, decoding errors, non-progress, and budget overflow fail with nonzero exit and empty success stdout. No best-effort partial-success flag is introduced. A valid empty timeline/test collection and an absent log reference are data states; an error response is not.

Reuse pipeline transport restrictions for Build and Test reads. Successful log text is exported as data, never printed or rendered as active HTML. Bound each response/log at 8 MiB, the inspection at 1,000 HTTP requests and 128 MiB of total successful response bodies, and each accumulated record collection at 100,000 entries. Check bounds before further requests/allocation; exceedance fails rather than truncates. These are fixed limits, not new user configuration. The existing summary-list budgets stay unchanged.

### 6. Incremental integration preserves existing contracts

Wire each command through current routing/help and dependency seams. Reuse profile and OS-keyring behavior, and run all real `adomi` invocations through host-capable execution. Update `README.md`, `docs/azure-devops.md`, help, and `generateSkillContent` with each increment so agents can discover what has shipped. Skill guidance distinguishes Build-read (`vso.build`) and Test-read (`vso.test`) permission, PR-read (`vso.code`), and work-item-read (`vso.work`).

Add a distinct investigation requirement to `agent-skill-creation`; do not replace the large generated-format requirement. The workflow-documentation replacement includes the already implemented governance wording from `enable-complete-pr-flow`. Archive/synchronize that completed predecessor before this change is archived so later synchronization cannot restore obsolete pipeline exclusions. Do not edit or archive the predecessor during proposal creation.

## Risks / Trade-offs

- Concurrent changes during pagination or inspection → preserve observed order/identity and observation timestamps; no claim of an atomic or final snapshot. Duplicate/non-progress detection fails conservatively.
- A green build can contain skipped work or no published tests → export actual states and all published outcome categories; never synthesize test success from the build result.
- Missing Test permission, retained-away logs, or large evidence prevents a complete export → fail explicitly with safe operation/status guidance. The user can still use `pipeline get` independently. UNVERIFIED: live tenant retention behavior, permission combinations, and representative payload sizes; future runtime checks record the observed cases.
- Expanded local context contains remote discussion/log content → escape HTML, use validated IDs for filenames, and preserve safe transport/error handling. Arbitrary-secret detection/redaction in successful remote logs is not promised.
- New read interfaces affect test doubles and default wiring → cover both injected-client tests and real HTTP-to-CLI-to-filesystem fixtures. Existing ensure/comment-write and fetch/list/get regressions remain gates.

## Migration Plan

Implement increment 1 as PR discovery, branch history, then comment export, with guidance beside each chunk. Implement inspection as increment 2. No config or stored-data migration is needed; existing invocations remain valid. Rolling back the binary removes the new commands/flag support and leaves local evidence snapshots readable. Installation/publishing requires a later explicit request.

During apply, use the existing Go/httptest harness with TDD for the new behavior. Record each chunk's actual verification result in `tasks.md`; the final gates include focused review, full tests/vet/build, strict OpenSpec validation, and host-side read-only runtime checks. No code tests or builds are required to validate this proposal-only change.
