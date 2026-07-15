# ado-pipeline-runs Specification

## Purpose
Define read-only inspection of running Azure DevOps Build pipeline runs and their current results.

## Requirements

### Requirement: List in-progress Azure DevOps pipeline runs
The system SHALL request YAML and classic Build pipeline runs from the configured Azure DevOps project's Build API with the exact status filter `inProgress`, SHALL traverse the continuation-token sequence observed during that invocation before succeeding within fixed ceilings of 1,000 response pages and 100,000 accumulated runs, and SHALL preserve the order in which valid runs are received across pages. The result is a best-effort one-shot view and SHALL NOT be represented as a transactionally complete point-in-time snapshot. Classic Release deployments are not Build API runs and SHALL remain excluded.

#### Scenario: Project has in-progress runs
- **WHEN** the user lists pipeline runs with valid configuration and credentials and Azure DevOps returns one or more `inProgress` builds
- **THEN** the system returns each valid run received during the observed continuation sequence with its run and pipeline identity, current status, source revision, available timestamps, and web link in received order

#### Scenario: Project has no in-progress runs
- **WHEN** Azure DevOps returns a present top-level `value` property containing either null or an empty array
- **THEN** the system succeeds with an empty non-null run collection

#### Scenario: Running runs span continuation pages
- **WHEN** Azure DevOps returns exactly one non-blank continuation-header value with a page of `inProgress` builds
- **THEN** the system treats the token as opaque, requests the next page with that exact token, and succeeds only after the service omits the continuation header

#### Scenario: Project state changes during pagination
- **WHEN** runs start, finish, or move between pages while the continuation sequence is being traversed
- **THEN** the system completes the observed continuation sequence without restarting, polling, or reconciling it and returns the valid runs received as a best-effort one-shot view that may omit concurrent changes

#### Scenario: Pipeline list pagination is invalid
- **WHEN** Azure DevOps returns an empty or whitespace-only continuation value, multiple continuation-header values, any continuation token already observed during the invocation, a non-`inProgress` item, or the same in-range run ID more than once across pages
- **THEN** the list fails instead of looping, omitting data, or returning an ambiguous collection

#### Scenario: Pipeline list reaches its safety ceilings
- **WHEN** the final allowed page has no continuation header and the accumulated collection contains at most 100,000 runs
- **THEN** the list may succeed, including when it contains exactly 1,000 response pages or exactly 100,000 runs

#### Scenario: Pipeline list exceeds its safety ceilings
- **WHEN** the 1,000th response page contains another continuation token or appending a page would accumulate more than 100,000 runs
- **THEN** the list fails without requesting an additional page or writing success data to stdout

#### Scenario: Pipeline list response is invalid
- **WHEN** a list request returns an HTTP error, authorization redirect, network error, malformed/empty/multiple JSON values, omits the top-level `value` property, or contains an item without run and pipeline IDs from `1` through `2147483647`, a non-blank pipeline name, run number, or status
- **THEN** the complete list fails and no success data is written to stdout

### Requirement: Retrieve one Azure DevOps pipeline run
The system SHALL retrieve one Build API run by decimal run ID from `1` through `2147483647`, matching Azure DevOps's positive signed-int32 `buildId` range, from the configured Azure DevOps project and SHALL return its current overall status and terminal result when available.

#### Scenario: Retrieve an in-progress run
- **WHEN** Azure DevOps returns the requested run with a non-terminal status and no result or result `none`
- **THEN** the system returns the run with its current status and a null result

#### Scenario: Retrieve a completed run
- **WHEN** Azure DevOps returns the requested run with status `completed` and a non-blank terminal result
- **THEN** the system returns the exact overall result reported by Azure DevOps

#### Scenario: Completed run omits its result
- **WHEN** Azure DevOps returns status `completed` but omits the result or reports result `none`
- **THEN** the lookup fails because the response cannot answer how the run completed

#### Scenario: Non-completed run reports a terminal result
- **WHEN** Azure DevOps returns any status other than exact `completed` together with a non-blank result other than `none`
- **THEN** the response fails as inconsistent instead of returning contradictory status data

#### Scenario: Pipeline run lookup response is invalid
- **WHEN** the lookup returns an HTTP error, authorization redirect, network error, malformed/empty/multiple JSON values, a missing required field, an invalid date-time field, a run or pipeline ID outside `1` through `2147483647`, or a run ID different from the requested ID
- **THEN** the lookup fails and no success data is written to stdout

### Requirement: Project pipeline runs into stable compact status data
The system SHALL project Build API responses into a stable JSON shape containing `id`, `pipelineId`, `pipelineName`, `runNumber`, `status`, `result`, `sourceBranch`, `sourceVersion`, `queueTime`, `startTime`, `finishTime`, and `webUrl` without exposing unrelated Build API fields. Missing, null, empty, or whitespace-only `sourceBranch`, `sourceVersion`, and `webUrl` inputs SHALL project as JSON null; other values SHALL have surrounding whitespace removed. Every available timestamp SHALL be normalized to UTC and formatted with RFC3339Nano using `Z`; unavailable timestamps SHALL be JSON null.

#### Scenario: Single-run JSON projection
- **WHEN** one pipeline run is retrieved successfully
- **THEN** the system returns one compact JSON object with every documented key and represents each unavailable result, source, timestamp, or web-link value as JSON null

#### Scenario: In-progress-list JSON projection
- **WHEN** the in-progress run list is retrieved successfully
- **THEN** the system returns one compact JSON object whose `runs` array contains the ordered projected run objects and is an empty JSON array when no runs exist

#### Scenario: Azure DevOps adds a status or result value
- **WHEN** a valid Build response contains an unknown non-blank status with an unavailable result or exact status `completed` with an unknown non-blank result
- **THEN** the system preserves the unknown string instead of replacing it with an inferred value

#### Scenario: Result contains surrounding whitespace
- **WHEN** an otherwise consistent available result contains surrounding whitespace
- **THEN** the system trims the result before comparison and projection

#### Scenario: Timestamp has an offset or fractional seconds
- **WHEN** an available queue, start, or finish time contains a non-UTC offset or fractional seconds
- **THEN** the projected timestamp represents the same instant in UTC with `Z`, uses RFC3339Nano precision, and removes insignificant trailing fractional zeros

#### Scenario: Timestamp has whole-second precision or is unavailable
- **WHEN** an available timestamp has no fractional seconds or a timestamp is unavailable
- **THEN** the available value uses `YYYY-MM-DDTHH:MM:SSZ` and the unavailable value is JSON null

#### Scenario: Optional source or link string is unavailable
- **WHEN** `sourceBranch`, `sourceVersion`, or `webUrl` is missing, explicit null, empty, or whitespace-only
- **THEN** the corresponding documented key is present with a JSON null value

#### Scenario: Optional source or link string is available
- **WHEN** `sourceBranch`, `sourceVersion`, or `webUrl` contains non-whitespace text with optional surrounding whitespace
- **THEN** the corresponding documented key contains the text with surrounding whitespace removed

### Requirement: Pipeline run inspection remains read-only and one-shot
The system SHALL use only Azure DevOps Build read operations for this capability and SHALL NOT poll, wait, stream, mutate a pipeline, fetch execution detail, or call classic Release deployment APIs.

#### Scenario: List and get perform one-shot reads
- **WHEN** either pipeline command succeeds
- **THEN** the system completes the finite list/get request sequence and returns the state received during that sequence without waiting for a later status

#### Scenario: Excluded deployment and execution surfaces are not invoked
- **WHEN** pipeline status is inspected successfully
- **THEN** the system does not request stages, jobs, tasks, timelines, logs, artifacts, approvals, environments, deployment resources, releases, or classic Release deployments and does not queue, cancel, retry, or approve anything

### Requirement: Pipeline API failures suppress confidential and untrusted data
The system SHALL NOT include the configured PAT, any `Authorization` header or value, any raw response-body content, HTML fragments, or any redirect `Location` value in a pipeline client error.

#### Scenario: Build API returns an error response
- **WHEN** a pipeline list or lookup receives a non-success HTTP response containing unique text in its body
- **THEN** the error identifies the operation and status using safe guidance without containing the response-body text, PAT, or authorization value

#### Scenario: Build API redirects a pipeline request
- **WHEN** a pipeline list or lookup receives a redirect containing a unique `Location` value
- **THEN** the error reports the redirect using safe guidance without containing the redirect value, response body, PAT, or authorization value

### Requirement: Pipeline credentials use a safe transport
The system SHALL send pipeline requests only when the configured base URL uses HTTPS or uses HTTP with a hostname that is exactly `localhost` case-insensitively or is an IPv4/IPv6 loopback address. The system SHALL bypass configured and environment proxies for loopback HTTP pipeline requests so the authenticated request remains direct and on-machine. The system SHALL reject all other schemes and remote HTTP origins before constructing or sending an authenticated request, without changing transport behavior for existing non-pipeline commands.

#### Scenario: Pipeline request uses HTTPS
- **WHEN** the configured Azure DevOps base URL uses HTTPS
- **THEN** the pipeline client may construct and send the authenticated Build API request

#### Scenario: Pipeline request uses a loopback HTTP endpoint
- **WHEN** the configured base URL uses HTTP and its hostname is exactly `localhost` or parses directly as an IPv4/IPv6 loopback address, with or without a port
- **THEN** the pipeline client sends the authenticated Build API request directly to loopback without resolving any other hostname through DNS or routing the request through a configured or environment proxy

#### Scenario: Pipeline request uses an unsafe transport
- **WHEN** the configured base URL uses remote HTTP, a non-HTTP scheme, or an HTTP hostname other than exact `localhost` or a parsed loopback IP address
- **THEN** the pipeline operation fails before constructing a request, adding an `Authorization` header, contacting the endpoint, or writing success data

### Requirement: Pipeline success responses have a bounded size
The system SHALL limit each successful Build API list-page or get response to 8 MiB (`8 * 1024 * 1024` bytes) before JSON decoding and SHALL fail without exposing body content when either the declared or streamed body exceeds that limit.

#### Scenario: Pipeline response is within the size limit
- **WHEN** a successful list-page or get response contains at most 8 MiB, including a body exactly at the limit
- **THEN** the system reads the complete body and proceeds with strict JSON decoding and shape validation

#### Scenario: Pipeline response declares an oversized body
- **WHEN** a successful list-page or get response declares a `Content-Length` greater than 8 MiB
- **THEN** the operation fails before decoding and writes no success data

#### Scenario: Pipeline response streams an oversized body
- **WHEN** a chunked, missing-length, or incorrectly declared successful response contains a byte beyond 8 MiB
- **THEN** the operation fails without decoding or exposing the oversized body and writes no success data
