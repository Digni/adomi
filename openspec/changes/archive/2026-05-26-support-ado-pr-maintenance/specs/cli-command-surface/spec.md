## ADDED Requirements

### Requirement: Pull request command namespace
The system SHALL expose Azure DevOps pull request context and maintenance operations through an `adomi ado pr` command namespace while preserving the existing numeric fetch alias.

#### Scenario: Explicit pull request fetch command
- **WHEN** the user runs `adomi ado pr fetch <pull-request-id>` with valid configuration and credentials
- **THEN** the command fetches the pull request, downloads all comment threads, exports them under the repository-local `.adomi/context/pull-requests/<pull-request-id>/` directory, and prints only the exported directory path to stdout

#### Scenario: Compatibility pull request fetch alias
- **WHEN** the user runs `adomi ado pr <pull-request-id>` with valid configuration and credentials
- **THEN** the command behaves the same as `adomi ado pr fetch <pull-request-id>`

#### Scenario: Pull request namespace help lists supported operations
- **WHEN** the user requests help for `adomi ado pr`
- **THEN** the help output lists the supported `fetch`, `ensure`, `reply`, `resolve`, and `reopen` operations

#### Scenario: Pull request ensure command
- **WHEN** the user runs `adomi ado pr ensure` with valid inferred repository context, valid configuration, and valid credentials
- **THEN** the command creates or updates the active pull request for the resolved source and target branches according to the PR maintenance requirements

#### Scenario: Pull request reply command
- **WHEN** the user runs `adomi ado pr reply <pull-request-id> --thread <thread-id>` with one valid message source, valid configuration, and valid credentials
- **THEN** the command creates a reply comment on the requested pull request thread according to the PR maintenance requirements

#### Scenario: Pull request resolve command
- **WHEN** the user runs `adomi ado pr resolve <pull-request-id> --thread <thread-id>` with valid configuration and credentials
- **THEN** the command marks the requested pull request thread fixed according to the PR maintenance requirements

#### Scenario: Pull request reopen command
- **WHEN** the user runs `adomi ado pr reopen <pull-request-id> --thread <thread-id>` with valid configuration and credentials
- **THEN** the command marks the requested pull request thread active according to the PR maintenance requirements

### Requirement: Pull request write stream behavior
The system SHALL keep pull request maintenance command stdout reserved for successful command data and stderr reserved for help, usage, diagnostics, prompts, and errors.

#### Scenario: Ensure success stdout
- **WHEN** `adomi ado pr ensure` succeeds without `--json`
- **THEN** stdout contains only the pull request ID followed by a newline

#### Scenario: Reply success stdout
- **WHEN** `adomi ado pr reply <pull-request-id> --thread <thread-id>` succeeds without `--json`
- **THEN** stdout contains only the created comment ID followed by a newline

#### Scenario: Resolve success stdout
- **WHEN** `adomi ado pr resolve <pull-request-id> --thread <thread-id>` succeeds without `--json`
- **THEN** stdout contains only the thread ID followed by a newline

#### Scenario: Reopen success stdout
- **WHEN** `adomi ado pr reopen <pull-request-id> --thread <thread-id>` succeeds without `--json`
- **THEN** stdout contains only the thread ID followed by a newline

#### Scenario: PR write JSON stdout
- **WHEN** a pull request maintenance command succeeds with `--json`
- **THEN** stdout contains one JSON object followed by a newline and stderr does not contain success data

#### Scenario: PR write validation error leaves stdout empty
- **WHEN** a pull request maintenance command receives invalid arguments or cannot infer required repository context
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: PR write network error leaves stdout empty
- **WHEN** a pull request maintenance command receives an Azure DevOps error response or network error
- **THEN** stdout is empty and the command exits non-zero
