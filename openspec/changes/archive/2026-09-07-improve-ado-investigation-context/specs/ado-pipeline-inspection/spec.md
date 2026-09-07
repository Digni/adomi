## Purpose

Export evidence about what a selected Azure DevOps Build run executed, including observed execution records, failed-task logs, and published tests.

## ADDED Requirements

### Requirement: Inspect the observed pipeline execution hierarchy
`adomi ado pipeline inspect <run-id>` SHALL read the selected Build run, its current root timeline, and referenced detail timelines once per distinct timeline. It SHALL preserve record and parent identity, type/name/order, state/result, available times, issues, task metadata, attempt identity, and log references. Unknown types/states and unavailable optional fields SHALL remain distinguishable. Earlier-attempt references SHALL be retained but timelines referenced solely as previous attempts SHALL NOT be fetched. The export SHALL describe this finite observation scope and SHALL NOT infer execution from overall success.

#### Scenario: Succeeded run contains skipped work
- **WHEN** a succeeded run's timeline includes succeeded, skipped, canceled, or pending records
- **THEN** their reported states/results are exported without upgrading them to succeeded or executed

#### Scenario: Classic Build has no stage layer
- **WHEN** the service returns jobs/tasks without stage records
- **THEN** the reported hierarchy is preserved without synthesizing stages

#### Scenario: Detail timeline or retry information exists
- **WHEN** records reference detail timelines or prior attempts
- **THEN** detail timelines are read without repeated traversal, attempt identities/references remain visible, and the manifest states that full historical attempt reconstruction was not requested

### Requirement: Export logs for observed failed tasks
The system SHALL retrieve full text logs for observed task records whose result is `failed` and whose log reference has a valid ID. Each distinct log SHALL be downloaded once and mapped back to its task records. Failed tasks without a log reference SHALL be marked `notPublished`; downloaded logs SHALL be marked `downloaded` with a relative path; other task logs SHALL be marked `notRequested`. Returned URLs SHALL NOT determine authenticated download destinations or local filenames.

#### Scenario: Several failed tasks reference a log
- **WHEN** observed failed tasks share a log ID
- **THEN** one local text file is written and every relevant record references it

#### Scenario: Failed task has no published log reference
- **WHEN** a failed task has no log reference
- **THEN** its result is exported with explicit `notPublished` availability rather than an invented empty log

#### Scenario: Referenced log cannot be retrieved
- **WHEN** a referenced log returns 403, 404, another HTTP error, or an oversized body
- **THEN** inspection fails with safe operation/status or limit guidance and no successful export output

### Requirement: Export published test results associated with the selected build
The system SHALL query Test runs using the selected Build response's build URI as a server-side filter and retrieve every observed run's result pages across all outcomes. It SHALL traverse offset pages until empty, advancing by returned item count, and reject duplicate identities or explicit build/test-run identity mismatches. Exported data SHALL preserve test-run/result IDs, available names, states/outcomes, durations, and failure messages/stack traces. Reported run totals and counts calculated from retrieved outcomes SHALL remain separately labeled.

#### Scenario: Tests span runs and result pages
- **WHEN** the selected build has multiple published test runs with paginated results
- **THEN** all observed result pages are exported under their corresponding test-run IDs without fetching unrelated builds' tests

#### Scenario: Build has no published tests
- **WHEN** the complete filtered Test run traversal is empty
- **THEN** the export contains an empty test-run collection and states that no published tests were returned, without asserting that no tests executed or that tests passed

#### Scenario: Test reads are unavailable
- **WHEN** the build URI is missing, test-read permission is absent, or any test page is invalid or unavailable
- **THEN** inspection fails rather than converting missing evidence into zero tests

### Requirement: Inspection exports are identifiable local observations
The system SHALL write each successful inspection into a unique snapshot directory under `.adomi/context/pipelines/<run-id>/`, containing `index.json`, `run.json`, `timeline.json`, `logs/<log-id>.txt` for downloaded logs, `tests/runs.json`, and `tests/<test-run-id>/results.json`. The index SHALL record source/profile/project/base URL, run identity, UTC observation start/finish times, counts, evidence scope, log availability, and relative artifact paths. It SHALL be finalized only after all requested reads and writes succeed. An unsuccessful invocation SHALL remove its unfinished snapshot without replacing existing successful snapshots.

#### Scenario: Inspect an active run twice
- **WHEN** two inspections observe different states of the same active run
- **THEN** they produce separate timestamped observations, retain non-terminal data without waiting, and do not combine evidence from the two invocations

#### Scenario: Successful export output
- **WHEN** inspection succeeds
- **THEN** default stdout is only the path plus newline, or with `--json` one compact object with `path`, `runId`, `timelineRecords`, `failedTaskLogs`, `testRuns`, and `testResults` plus newline, with counts describing exported records

#### Scenario: Local export fails
- **WHEN** writing an inspection artifact fails
- **THEN** no success data is printed, the unfinished snapshot is removed, and prior snapshots remain intact

### Requirement: Detailed inspection is bounded and read-only
Inspection SHALL perform only finite Build/Test GET requests with existing pipeline HTTPS/direct-loopback, proxy-bypass, and no-redirect protections. It SHALL limit each successful response/log to 8 MiB, the entire inspection to 1,000 HTTP requests and 128 MiB of successful response bodies, and each accumulated record collection to 100,000 entries. Exceeding a ceiling SHALL fail without truncation or partial-success output. Errors/progress SHALL exclude credentials, raw remote bodies, and redirect destinations. No polling, mutation, arbitrary artifacts, test attachments, or classic Release APIs SHALL be used.

#### Scenario: Payload or request budget is exhausted
- **WHEN** the next read or appended data would exceed a documented limit
- **THEN** the operation stops before further requests or excess allocation and reports a bounded failure rather than a successful subset

#### Scenario: Remote content includes unsafe links or active markup
- **WHEN** records/logs contain arbitrary URLs, markup, or path-like names
- **THEN** authenticated requests remain on constructed configured-project endpoints, artifact paths use validated IDs, log content is inert local text, and remote body content is not echoed in diagnostics
