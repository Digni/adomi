## MODIFIED Requirements

### Requirement: Pipeline run inspection remains read-only and one-shot
The system SHALL use only Azure DevOps Build read operations for summary list/get commands and Build/Test read operations for detailed inspect exports. It SHALL NOT poll, wait, stream, mutate a pipeline, or call classic Release deployment APIs. Execution detail SHALL be fetched only by the explicit inspect command.

#### Scenario: List and get perform one-shot reads
- **WHEN** pipeline list or get succeeds
- **THEN** the system completes the finite list/get request sequence and returns the state received during that sequence without waiting for a later status

#### Scenario: Excluded deployment and execution surfaces are not invoked
- **WHEN** pipeline status is read through list or get
- **THEN** the system does not request stages, jobs, tasks, timelines, logs, artifacts, approvals, environments, deployment resources, releases, or classic Release deployments and does not queue, cancel, retry, or approve anything

#### Scenario: Explicit inspect reads execution evidence
- **WHEN** the user invokes `pipeline inspect <run-id>`
- **THEN** the finite read sequence includes the execution records, failed-task logs, and published test results defined by the pipeline-inspection capability without polling or mutation

## ADDED Requirements

### Requirement: Pipeline list filters branches before applying history limits
The system SHALL accept optional `--branch <branch>` on pipeline list and send the normalized full ref as `branchName` on every Builds list request. A short branch SHALL receive `refs/heads/`; full refs SHALL be preserved after trimming surrounding whitespace. Omission SHALL preserve the existing requests/output. Branch-only listing SHALL retain exact `inProgress` scope; combining branch with `--last N` SHALL return at most N most recently queued matching runs across statuses, retaining current ordering, pagination, limits, and JSON projection. Filtering SHALL remain project-scoped and SHALL NOT infer PR validation refs or strip remote-name prefixes.

#### Scenario: Noisy unrelated runs do not consume the requested history
- **WHEN** a user runs `adomi ado pipeline list --branch feature/example --last 10` and many unrelated runs were queued more recently
- **THEN** every request has `branchName=refs/heads/feature/example`, no status filter, descending queue-time order, and the remaining requested count, so unrelated branches do not consume the ten-run allowance

#### Scenario: Branch filtering continues across pages
- **WHEN** matching runs require continuation pages
- **THEN** the normalized branch is identical on every request and traversal stops at N matches or token exhaustion using existing continuation validation

#### Scenario: Branch without last lists in-progress runs
- **WHEN** `pipeline list --branch feature/example` is invoked without `--last`
- **THEN** requests combine the branch filter with exact `statusFilter=inProgress`

#### Scenario: Branch has no matches
- **WHEN** the filtered request sequence returns a valid empty collection
- **THEN** the existing compact JSON shape contains `runs: []`

#### Scenario: Server returns a different or missing branch
- **WHEN** a filtered response contains a run whose source ref is missing or differs from the requested normalized ref
- **THEN** the entire operation fails with empty success stdout rather than hiding the mismatch through local filtering
