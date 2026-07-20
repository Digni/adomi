# ado-pipeline-runs Delta

## ADDED Requirements

### Requirement: List recent Azure DevOps pipeline runs
When the user supplies `--last <N>` with N from `1` through `200`, the system SHALL request the N most recently queued YAML and classic Build pipeline runs from the configured Azure DevOps project's Build API without a status filter, using descending queue-time order and a `$top` of N on the first request, and SHALL traverse any continuation-token sequence observed during that invocation with `$top` decremented to the number of runs still needed on each subsequent request, stopping successfully once N runs are accumulated. The existing page and response-size ceilings apply, and the system SHALL preserve the order in which valid runs are received. Runs of any status, including non-terminal statuses without results, SHALL be accepted and returned as a best-effort one-shot view; the result SHALL NOT be represented as a transactionally complete point-in-time snapshot. The universal status/result consistency rules apply: a `completed` run SHALL have a non-blank result other than `none`, and a run with any other status SHALL NOT report a terminal result.

#### Scenario: Project has recent runs across statuses
- **WHEN** the user lists pipeline runs with `--last 10`, valid configuration and credentials, and Azure DevOps returns completed, in-progress, and not-started builds
- **THEN** the system returns each valid run in received order with its run and pipeline identity, status, terminal result when available, source revision, available timestamps, and web link

#### Scenario: Recent list request is bounded
- **WHEN** the user lists pipeline runs with `--last <N>`
- **THEN** the initial Builds list request uses `$top` equal to N, descending queue-time order, and no status filter

#### Scenario: Fewer runs exist than requested
- **WHEN** the user requests `--last 10` and Azure DevOps returns fewer than 10 runs with no continuation header
- **THEN** the system succeeds with exactly the runs received, including an empty non-null collection

#### Scenario: Recent runs span continuation pages
- **WHEN** Azure DevOps pages below the requested count and returns exactly one non-blank continuation-header value per page
- **THEN** the system treats the token as opaque, requests the next page with that exact token and a `$top` of the number of runs still needed, and succeeds once N runs are accumulated or the service omits the continuation header

#### Scenario: Reaching the requested count ends traversal
- **WHEN** a page brings the accumulated valid runs to exactly N and the response still carries a continuation header
- **THEN** the system succeeds with the N accumulated runs without requesting another page

#### Scenario: Recent list pagination is invalid
- **WHEN** Azure DevOps returns an empty or whitespace-only continuation value, multiple continuation-header values, any continuation token already observed during the invocation, or the same in-range run ID more than once across pages
- **THEN** the list fails instead of looping, omitting data, or returning an ambiguous collection

#### Scenario: A page exceeds its requested `$top`
- **WHEN** a single response page contains more runs than the `$top` sent with that request
- **THEN** the list fails as a service-contract violation without requesting an additional page or writing success data to stdout

#### Scenario: Recent list contains an inconsistent run
- **WHEN** a returned run reports status `completed` without a non-blank result other than `none`, or reports a terminal result together with any status other than exact `completed`
- **THEN** the complete list fails and no success data is written to stdout

#### Scenario: Recent list response is invalid
- **WHEN** a list request returns an HTTP error, authorization redirect, network error, malformed/empty/multiple JSON values, omits the top-level `value` property, or contains an item without run and pipeline IDs from `1` through `2147483647`, a non-blank pipeline name, run number, or status
- **THEN** the complete list fails and no success data is written to stdout

#### Scenario: Recent-list JSON projection
- **WHEN** the recent run list is retrieved successfully
- **THEN** the system returns one compact JSON object whose `runs` array contains the ordered projected run objects under the same stable projection rules as the in-progress list, with unavailable results, sources, timestamps, and web links as JSON null
