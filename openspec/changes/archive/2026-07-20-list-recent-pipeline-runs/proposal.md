# Proposal: list-recent-pipeline-runs

## Why

`adomi ado pipeline list` only ever returns `inProgress` runs, and `adomi ado pipeline get <run-id>` requires a run ID the caller has no way to discover. The primary consumer — an agent that just finished an implementation and pushed — cannot answer "was CI good for my change?" because there is no read path for recent run history. Adding a bounded "last N runs" mode completes the pipeline capability: discovery via recent history, then cheap single-run re-checks via `get`.

## What Changes

- Add a `--last <N>` flag to `adomi ado pipeline list` (N from 1 through a fixed maximum) that returns the N most recently queued Build runs in the configured project, across any status, with `status` and `result` populated.
- Without `--last`, `pipeline list` keeps its current frozen behavior: exact `inProgress` runs only. No breaking change to the default contract.
- Reject invalid `--last` values (missing value, non-decimal, zero, negative, above the maximum, repeated flag) before repository, configuration, credential, or HTTP access, following the existing strict-parser pattern.
- The ADO client gains a recent-runs read path using the same Builds list endpoint with `$top` and descending queue-time order, inheriting the existing transport, pagination, confidentiality, and ceiling guarantees.
- Update command help and the generated agent skill to document the discovery-plus-`get` workflow.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `ado-pipeline-runs`: The list capability gains a recent-runs mode. List validation can no longer assume every returned run is `inProgress`; status/result consistency and shape guarantees now apply across statuses, and the requested mode determines which runs are acceptable.
- `cli-command-surface`: `adomi ado pipeline list` gains the `--last <N>` flag with early validation; help text describes both modes and their combination rules.

## Impact

- `internal/ado/pipeline.go`, `internal/ado/pipeline_transport.go`: conditional `statusFilter`, `$top`, mode-parameterized list validation, new reader method on `PipelineRunReader`.
- `internal/cli/ado_pipeline.go`, `internal/cli/commands.go`, `internal/cli/help.go`: flag parsing, runner wiring, help text.
- `internal/cli/agent.go`: generated skill content describing `--last` and the discovery workflow.
- No new dependencies, no new API scopes (existing `vso.build` read suffices), no mutation surface — the capability remains read-only.
- Tests: `internal/ado` client tests and `internal/cli` parser/wiring/real-`httptest` tests follow the patterns established by the original pipeline change.
