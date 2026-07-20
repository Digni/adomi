# Design: list-recent-pipeline-runs

## Context

`adomi ado pipeline list` requests `{base}/{project}/_apis/build/builds` with a hardcoded `statusFilter=inProgress` and `queryOrder=queueTimeDescending`, traverses continuation pages under fixed ceilings (1,000 pages / 100,000 runs / 8 MiB per response), rejects any non-`inProgress` item, and returns compact JSON (`internal/ado/pipeline_transport.go:102-108`, `internal/ado/pipeline.go:62-122`). `pipeline get <run-id>` already validates and normalizes runs of any status, including completed runs with terminal results (`internal/ado/pipeline.go:124-155`). The motivating workflow is an agent that just pushed an implementation asking "was CI good?": it needs a discovery path to recent runs (matching its branch or commit client-side via the already-projected `sourceBranch`/`sourceVersion`), then cheap single-run re-checks through the existing `get`.

## Goals / Non-Goals

**Goals:**

- `adomi ado pipeline list --last <N>` returns the N most recently queued Build runs (YAML and classic) in the configured project, any status, in received descending-queue-time order, as one-shot compact JSON identical in shape to today's list output.
- Bare `pipeline list` keeps its exact current `inProgress` behavior and contract; no existing consumer observes a change.
- All existing safety guarantees carry over unchanged: HTTPS-or-loopback transport, proxy bypass for loopback, 8 MiB response ceiling, continuation-token cycle guards, confidential-diagnostic suppression, empty stdout on failure, early argument validation.

**Non-Goals:**

- No server-side `--branch`, `--pipeline`/`--definition`, `--status`, or `--result` filters; client-side matching on the projected fields covers the motivating workflow. Deferred until project noise proves the need.
- No polling, waiting, or watch mode; the agent re-invokes when a run is still `inProgress`.
- No per-pipeline "last N" semantics; `--last` is project-wide recency.
- No classic Release deployment surface, no execution detail (stages/jobs/logs/artifacts), no mutation.

## Decisions

### `--last <N>` flag on `list`, default mode untouched

Add an optional `--last <N>` flag to the existing `list` command rather than a new `pipeline runs`/`history` subcommand. The query is the same Builds list endpoint with a different filter; a flag keeps the command tree and the generated agent skill small, and help for one command documents both modes. Bare `list` remains bit-for-bit the frozen `inProgress` contract, so existing skill documentation and agent prompts stay true.

**Alternative considered:** a separate subcommand with frozen semantics per command. Rejected: doubles the conceptual surface for one endpoint, and "flag present ⇒ recent mode, absent ⇒ in-progress" is unambiguous and trivially validated.

### Maximum N of 200, single bounded request sequence

`--last` accepts a decimal value from 1 through 200. Values missing, non-decimal, zero, negative, above 200, or a repeated `--last` flag fail before repository, configuration, credential, or HTTP access, following the strict pipeline/wiki parser pattern (`internal/cli/ado_pipeline.go:97`, `internal/cli/ado_wiki.go:93`). The 200 cap keeps the operation a bounded one-shot consistent with the tool's philosophy and comfortably covers "recent CI for my change" even in busy projects.

**Alternative considered:** reuse the 100,000-run in-progress ceiling. Rejected: an effectively unbounded history walk defeats the purpose of the flag and offers no realistic agent value.

### Sibling reader method instead of generalizing the existing one

Add `ListRecentPipelineRuns(ctx context.Context, n int) ([]PipelineRun, error)` to `PipelineRunReader` alongside the untouched `ListInProgressPipelineRuns`. The interface contract stays "return only fully validated/normalized models or an error". The production implementation shares URL construction, transport guards, bodyless GET, continuation handling, decoding, and `normalizePipelineRun` with the in-progress path; only the query parameters, the per-invocation run ceiling (N instead of 100,000), and the status acceptability rule differ.

**Alternative considered:** change `ListInProgressPipelineRuns` to take filter parameters. Rejected: it churns the existing interface implementations, the shared CLI fake, and its documented contract for zero behavioral gain; a sibling method leaves the frozen path literally untouched.

### Recent mode = no `statusFilter`, decrementing `$top`, keep `queryOrder=queueTimeDescending`

"Last N runs" means the N most recently queued runs regardless of status. Omitting `statusFilter` returns all statuses; the existing descending queue-time order is exactly "most recent first". Runs still `inProgress` (or `notStarted`, `cancelling`, `postponed`) are returned as-is — the agent needs to see them to know CI is still running rather than absent.

`$top` is a per-request maximum, not a total-result bound: the service may legitimately page below it and return a continuation token. The first request uses `$top=N`; each continuation request uses `$top=N-len(runs)` so the remaining budget shrinks with what was already received. Accumulating exactly N runs ends the traversal successfully even when another continuation header is present — the requested history is complete, so no further request is made. A single page delivering more runs than the decrementing `$top` requested violates the service contract and fails. Duplicate run IDs and invalid/cyclic tokens fail exactly as today, and the existing page and response-size ceilings apply unchanged.

**Alternative considered:** `finishTimeDescending` so "last" means "most recently finished". Rejected: it hides queued/running runs, which misleads the motivating workflow into concluding CI never started.

### Validation follows the mode, consistency rules stay universal

In recent mode the client accepts any non-blank status instead of rejecting non-`inProgress` items. The universal `normalizePipelineRun` consistency rules continue to apply in both modes, as they already do for `get`: `completed` requires a non-blank result other than `none`; any other status rejects a terminal result; unknown-but-consistent status/result strings are preserved. Required-shape validation (IDs in signed-int32 range, non-blank pipeline name, run number, status, valid timestamps) is unchanged.

## Risks / Trade-offs

- **A busy project makes project-wide `--last 10` miss the agent's branch** → Acceptable for the motivating small-team workflow; the projected `sourceBranch`/`sourceVersion` let the agent match client-side, and `--branch` is a documented future extension if noise proves the need.
- **Azure DevOps pages below `$top`** → Continuation requests decrement `$top` by the runs already received and traversal stops at exactly N, so valid paged responses never trip an overflow guard; only a page exceeding its requested `$top` (a service-contract violation) fails.
- **A run transitions between `--last` discovery and `get` re-check** → Inherent to one-shot semantics and already documented as best-effort; the agent re-invokes `get` for the current state.
- **Interface growth on the shared CLI fake** → One added fake method and wiring, following the established pattern from the original pipeline change; existing fake behavior for other commands is unaffected.

## Open Questions

(none — scope, cap, ordering, and validation semantics were settled during exploration)
