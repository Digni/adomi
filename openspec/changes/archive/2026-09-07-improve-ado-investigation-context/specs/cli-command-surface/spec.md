## MODIFIED Requirements

### Requirement: Azure DevOps pipeline run commands
The system SHALL expose one-shot read-only pipeline run inspection through `adomi ado pipeline list [--branch <branch>] [--last <N>] [--profile <profile-name>] [--global]` and `adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]`, plus detailed repository-local export through `adomi ado pipeline inspect <run-id> [--profile <profile-name>] [--global] [--json]`. Without `--last`, `pipeline list` SHALL list `inProgress` runs; with `--last <N>`, it SHALL list the N most recently queued runs across any status. Optional branch filtering SHALL constrain either mode server-side; omission SHALL preserve existing behavior.

#### Scenario: Pipeline namespace appears in Azure DevOps help
- **WHEN** the user requests help for `adomi ado`
- **THEN** the help output lists the `pipeline` namespace as read-only Azure DevOps status inspection

#### Scenario: Pipeline command help describes scope and output
- **WHEN** the user requests help for `adomi ado pipeline`, `adomi ado pipeline list`, `adomi ado pipeline get`, or `adomi ado pipeline inspect`
- **THEN** help describes the exact `inProgress` default list scope and the `--last <N>` recent-runs mode across YAML and classic Build pipelines, their best-effort one-shot rather than transactional snapshot semantics, the `--last` range `1..200`, run-ID range `1..2147483647`, profile/global and repository behavior, HTTPS-or-loopback transport requirement, compact JSON fields and nullable result semantics, `vso.build` read scope, and branch filtering, list/get exclusion of execution detail, inspect bundle scope and path/JSON output, its additional `vso.test` read scope and fixed budgets, and the exclusion of polling, mutation, arbitrary artifacts, test attachments, and classic Release deployments

#### Scenario: List in-progress pipeline runs
- **WHEN** the user runs `adomi ado pipeline list` without `--last` inside a repository with valid configuration and credentials
- **THEN** stdout contains exactly one compact JSON object with a `runs` array followed by one newline and stderr contains no success data

#### Scenario: List recent pipeline runs
- **WHEN** the user runs `adomi ado pipeline list --last <N>` with N from `1` through `200` inside a repository with valid configuration and credentials
- **THEN** stdout contains exactly one compact JSON object with a `runs` array of at most N runs across any status followed by one newline and stderr contains no success data

#### Scenario: Get one pipeline run
- **WHEN** the user runs `adomi ado pipeline get <run-id>` with a run ID from `1` through `2147483647` inside a repository with valid configuration and credentials
- **THEN** stdout contains exactly one compact projected pipeline-run JSON object followed by one newline and stderr contains no success data

#### Scenario: Pipeline commands select configuration scope
- **WHEN** the user supplies `--profile <profile-name>` and optionally `--global`
- **THEN** the command resolves the Azure DevOps profile and credential using the same repository/global scope rules as the existing network-backed `adomi ado` commands

#### Scenario: Pipeline commands require a repository
- **WHEN** any pipeline command runs outside a Git repository, including with `--global`
- **THEN** it exits non-zero before loading configuration or credentials

#### Scenario: Pipeline list rejects invalid arguments early
- **WHEN** `pipeline list` receives a positional argument, a value-less `--last` or profile flag, a non-decimal, zero, negative, or above-`200` `--last` value, a `--last` value too large for numeric parsing, a repeated `--last` or profile/global flag, or an unknown argument
- **THEN** the command exits non-zero before loading configuration or credentials and leaves stdout empty

#### Scenario: Pipeline list accepts the `--last` boundaries
- **WHEN** the `--last` value is exactly `1` or exactly `200` and all other arguments are valid
- **THEN** argument validation succeeds with that exact value

#### Scenario: Pipeline get rejects invalid arguments early
- **WHEN** the run ID is missing, non-decimal, zero, negative, greater than `2147483647`, or too large for numeric parsing, a profile flag lacks a value, a profile/global flag is repeated, or an unknown argument is supplied
- **THEN** the command exits non-zero before loading configuration or credentials and leaves stdout empty

#### Scenario: Pipeline get accepts the maximum run ID
- **WHEN** the run ID is exactly `2147483647` and all other arguments are valid
- **THEN** argument validation succeeds with that exact ID independently of machine word size

#### Scenario: Pipeline command failure leaves stdout empty
- **WHEN** repository resolution, configuration, credential lookup, HTTP client construction, Build/Test API access, pagination, response validation, projection, export, or JSON encoding fails
- **THEN** the command exits non-zero and stdout contains no success data

#### Scenario: Pipeline HTTP failure keeps stderr confidential
- **WHEN** the real pipeline client returns an HTTP status or redirect error while its configured PAT, authorization value, response body, HTML, or redirect location contains unique marker text
- **THEN** the returned error and production stderr contain none of those markers and report only safe operation, status, and corrective guidance

#### Scenario: Pipeline command help is side-effect free
- **WHEN** a pipeline namespace or action help request includes `--help` or `-h`
- **THEN** the command exits successfully, leaves stdout empty, and writes help to stderr before resolving repository state, loading configuration, reading credentials, creating an HTTP client, or contacting Azure DevOps

#### Scenario: Pipeline branch arguments are validated early
- **WHEN** `pipeline list` receives a missing, blank, or repeated `--branch` value, or branch is supplied to get or inspect
- **THEN** it exits nonzero with empty stdout before configuration, credentials, or network access

#### Scenario: Inspect validates identifier and flags early
- **WHEN** `pipeline inspect` receives an invalid run ID under the same range rules as get, unexpected positionals, missing option values, repeated flags, or unknown flags
- **THEN** it exits nonzero before loading credentials or contacting Azure DevOps and leaves stdout empty

#### Scenario: Inspect help is sufficient without credentials
- **WHEN** a user requests `adomi ado pipeline inspect --help`
- **THEN** help describes run-ID validation, profile/global scope, repository requirement, export paths, plain/JSON output, current-attempt scope, failed-task-log selection, published tests, permissions, limits, and failure behavior before any credential or network access

## ADDED Requirements

### Requirement: Investigation additions have explicit parsing and help
The system SHALL route `adomi ado pr list` as a read-only action and accept `--include-comments` on work-item fetch. PR list SHALL support only optional `--source`, `--target`, `--status`, `--repository`, `--profile`, and `--global` flags and SHALL emit compact JSON without requiring a JSON flag. Existing profile/global selection, repository requirements, path-only fetch output, and fetch JSON shape SHALL be preserved. Help SHALL return successfully to stderr before repository/configuration/credential/file/network access.

#### Scenario: PR list is discoverable and validates before execution
- **WHEN** a user requests PR namespace/list help or invokes list with invalid status, blank/missing flag value, repeated flag, positional argument, or unsupported flag
- **THEN** help lists discovery alongside existing operations and explains defaults, normalization, paging, repository selection, read permission, and JSON output, while invalid invocations fail before credentials/network with empty stdout

#### Scenario: Fetch help explains comment inclusion
- **WHEN** a user requests work-item fetch help
- **THEN** help describes opt-in comments on every exported item, non-deleted current-version scope, artifact paths, unchanged stdout, read permission, and retrieval-failure behavior

#### Scenario: Duplicate comment inclusion is rejected
- **WHEN** work-item fetch receives `--include-comments` more than once or an unsupported assigned value
- **THEN** parsing fails before credential/network access with empty stdout
