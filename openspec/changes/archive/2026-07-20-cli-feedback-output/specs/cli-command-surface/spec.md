# cli-command-surface spec delta

## MODIFIED Requirements

### Requirement: CLI stream behavior

The system SHALL keep stdout reserved for successful command result data. stderr SHALL carry prompts, help, usage, and errors, and MAY additionally carry status, progress, confirmation, and summary lines as defined by the `cli-feedback` capability. Result data (paths, IDs, JSON result objects, profile lists) SHALL never be written to stderr; feedback lines SHALL never be written to stdout.

#### Scenario: Successful fetch stdout
- **WHEN** `adomi ado fetch <work-item-id>` succeeds
- **THEN** stdout contains only the exported path followed by a newline

#### Scenario: Work item comment success stdout
- **WHEN** `adomi ado comment <work-item-id>` succeeds without `--json`
- **THEN** stdout contains only the created work item comment ID followed by a newline

#### Scenario: Work item comment alias success stdout
- **WHEN** `adomi ado work-item comment <work-item-id>` succeeds without `--json`
- **THEN** stdout contains only the created work item comment ID followed by a newline

#### Scenario: Pull request ensure success stdout
- **WHEN** `adomi ado pr ensure` succeeds without `--json`
- **THEN** stdout contains only the pull request ID followed by a newline

#### Scenario: Pull request comment success stdout
- **WHEN** `adomi ado pr comment <pull-request-id>` succeeds without `--json`
- **THEN** stdout contains only the created thread ID followed by a newline

#### Scenario: Pull request reply success stdout
- **WHEN** `adomi ado pr reply <pull-request-id> --thread <thread-id>` succeeds without `--json`
- **THEN** stdout contains only the created comment ID followed by a newline

#### Scenario: Pull request resolve success stdout
- **WHEN** `adomi ado pr resolve <pull-request-id> --thread <thread-id>` succeeds without `--json`
- **THEN** stdout contains only the thread ID followed by a newline

#### Scenario: Pull request reopen success stdout
- **WHEN** `adomi ado pr reopen <pull-request-id> --thread <thread-id>` succeeds without `--json`
- **THEN** stdout contains only the thread ID followed by a newline

#### Scenario: Work item write JSON stdout
- **WHEN** a work item comment command succeeds with `--json`
- **THEN** stdout contains one JSON object followed by a newline and stderr contains no result data

#### Scenario: Pull request write JSON stdout
- **WHEN** a pull request maintenance command succeeds with `--json`
- **THEN** stdout contains one JSON object followed by a newline and stderr contains no result data

#### Scenario: Work item write validation error leaves stdout empty
- **WHEN** a work item comment command receives invalid arguments
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Pull request write validation error leaves stdout empty
- **WHEN** a pull request maintenance command receives invalid arguments or cannot infer required repository context
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Work item write network error leaves stdout empty
- **WHEN** a work item comment command receives an Azure DevOps error response or network error
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Pull request write network error leaves stdout empty
- **WHEN** a pull request maintenance command receives an Azure DevOps error response or network error
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Agent skill success stdout
- **WHEN** the user successfully runs `adomi agent skill`
- **THEN** stdout contains only the created skill directory path and stderr contains no result data

#### Scenario: Agent skill replacement prompt
- **WHEN** the user runs `adomi agent skill` and the target skill already exists
- **THEN** the replacement confirmation prompt is written to stderr and not stdout

#### Scenario: Agent skill force replacement has no prompt
- **WHEN** the user runs `adomi agent skill --force` and the target skill already exists
- **THEN** no replacement confirmation prompt is written and stdout contains only the replaced skill directory path on success

#### Scenario: Agent skill yes replacement has no prompt
- **WHEN** the user runs `adomi agent skill --yes` and the target skill already exists
- **THEN** no replacement confirmation prompt is written and stdout contains only the replaced skill directory path on success

#### Scenario: Agent skill validation error leaves stdout empty
- **WHEN** the user runs `adomi agent skill --claude --unknown .`
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Secret prompt
- **WHEN** `adomi ado login` prompts for a PAT
- **THEN** the prompt is written to stderr and not stdout

#### Scenario: Validation error
- **WHEN** a command receives invalid arguments
- **THEN** the command exits non-zero and reports the validation error on stderr
