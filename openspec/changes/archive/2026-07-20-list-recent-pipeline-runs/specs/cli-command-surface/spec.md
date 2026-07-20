# cli-command-surface Delta

## MODIFIED Requirements

### Requirement: Azure DevOps pipeline run commands
The system SHALL expose one-shot read-only pipeline run inspection through `adomi ado pipeline list [--last <N>] [--profile <profile-name>] [--global]` and `adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]`. Without `--last`, `pipeline list` SHALL retain its exact existing behavior of listing `inProgress` runs; with `--last <N>`, it SHALL list the N most recently queued runs across any status.

#### Scenario: Pipeline namespace appears in Azure DevOps help
- **WHEN** the user requests help for `adomi ado`
- **THEN** the help output lists the `pipeline` namespace as read-only Azure DevOps status inspection

#### Scenario: Pipeline command help describes scope and output
- **WHEN** the user requests help for `adomi ado pipeline`, `adomi ado pipeline list`, or `adomi ado pipeline get`
- **THEN** help describes the exact `inProgress` default list scope and the `--last <N>` recent-runs mode across YAML and classic Build pipelines, their best-effort one-shot rather than transactional snapshot semantics, the `--last` range `1..200`, run-ID range `1..2147483647`, profile/global and repository behavior, HTTPS-or-loopback transport requirement, compact JSON fields and nullable result semantics, `vso.build` read scope, and the exclusion of polling, execution detail, mutation, and classic Release deployments

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
- **WHEN** either pipeline command runs outside a Git repository, including with `--global`
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
- **WHEN** repository resolution, configuration, credential lookup, HTTP client construction, Build API access, pagination, response validation, projection, or JSON encoding fails
- **THEN** the command exits non-zero and stdout contains no success data

#### Scenario: Pipeline HTTP failure keeps stderr confidential
- **WHEN** the real pipeline client returns an HTTP status or redirect error while its configured PAT, authorization value, response body, HTML, or redirect location contains unique marker text
- **THEN** the returned error and production stderr contain none of those markers and report only safe operation, status, and corrective guidance

#### Scenario: Pipeline command help is side-effect free
- **WHEN** a pipeline namespace or action help request includes `--help` or `-h`
- **THEN** the command exits successfully, leaves stdout empty, and writes help to stderr before resolving repository state, loading configuration, reading credentials, creating an HTTP client, or contacting Azure DevOps
