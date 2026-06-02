# cli-command-surface Specification

## Purpose
Define the public command layout and stream behavior for the `adomi` CLI.
## Requirements
### Requirement: Cobra-backed root command

The system SHALL expose the `adomi` CLI through a Cobra command tree while preserving the programmatic `Run(args, stdin, stdout, stderr)` execution boundary.

#### Scenario: Root command without arguments
- **WHEN** the user runs `adomi` without a command
- **THEN** the command exits non-zero and reports usage for the available top-level commands on stderr

#### Scenario: Unknown top-level command
- **WHEN** the user runs `adomi unknown`
- **THEN** the command exits non-zero and reports that the command is unknown

### Requirement: Top-level config command

The system SHALL support configuration initialization through `adomi config init`.

#### Scenario: Initialize repository config
- **WHEN** the user runs `adomi config init` inside a repository without an existing repo config
- **THEN** the command creates the same repository config template currently produced by config initialization and prints the created config path

#### Scenario: Initialize global config
- **WHEN** the user runs `adomi config init --global`
- **THEN** the command creates the same global config template currently produced by global config initialization and prints the created config path

#### Scenario: Refuse to overwrite config
- **WHEN** the user runs `adomi config init` for a target config path that already exists
- **THEN** the command exits non-zero and does not modify the existing file

### Requirement: Hidden compatibility for old config route

The system SHALL keep `adomi ado config init` as a hidden compatibility alias for `adomi config init` during this migration.

#### Scenario: Legacy repository config command
- **WHEN** the user runs `adomi ado config init`
- **THEN** the command behaves the same as `adomi config init`

#### Scenario: Legacy global config command
- **WHEN** the user runs `adomi ado config init --global`
- **THEN** the command behaves the same as `adomi config init --global`

#### Scenario: Help output omits legacy config route
- **WHEN** the user requests help for `adomi ado`
- **THEN** the help output does not list `config` as an Azure DevOps subcommand

### Requirement: Agent skill command namespace

The system SHALL expose agent-oriented skill creation through the top-level `adomi agent skill` command while preserving the existing `Run(args, stdin, stdout, stderr)` execution boundary.

#### Scenario: Agent namespace appears in top-level help
- **WHEN** the user runs `adomi` without a command
- **THEN** the command exits non-zero and reports usage that includes the `agent` top-level command on stderr

#### Scenario: Agent skill help describes scope and target flags
- **WHEN** the user requests help for `adomi agent skill`
- **THEN** the help output describes the default global scope, the `--global` scope, the `--project` scope, and the `--claude` target override

#### Scenario: Existing Azure DevOps namespace remains available
- **WHEN** the user runs `adomi ado` without an Azure DevOps subcommand
- **THEN** the command exits non-zero and reports usage for the existing Azure DevOps subcommands

#### Scenario: Existing config namespace remains available
- **WHEN** the user runs `adomi config init` with valid arguments
- **THEN** the command behavior remains the same as before the agent command was added

### Requirement: Azure DevOps commands remain under ado namespace

The system SHALL preserve Azure DevOps-specific commands under `adomi ado`.

#### Scenario: Fetch work item context
- **WHEN** the user runs `adomi ado fetch <work-item-id>` with valid configuration and credentials
- **THEN** the command fetches and exports work item context for the requested work item, its parent chain, and any direct children of the requested work item under `.adomi/context/work-items/<work-item-id>/`, and prints only the exported directory path to stdout

#### Scenario: Fetch includes direct child work items
- **WHEN** the requested Azure DevOps work item has direct child relations
- **THEN** each direct child work item is fetched and included in the exported JSON, HTML, tree, index, and attachment context alongside the requested item and parent chain

#### Scenario: Fetch work item export uses flattened internal item paths
- **WHEN** `adomi ado fetch <work-item-id>` exports multiple work item JSON payloads
- **THEN** the payload files are written under `items/<id>.json` relative to the exported work item context directory and `index.json` references those relative `items/<id>.json` paths

#### Scenario: Fetch pull request comments
- **WHEN** the user runs `adomi ado pr fetch <pull-request-id>` with valid configuration and credentials
- **THEN** the command fetches the pull request, downloads all comment threads, exports them under the repository-local `.adomi/context/pull-requests/<pull-request-id>/` directory, and prints only the exported directory path to stdout

#### Scenario: Compatibility pull request fetch alias
- **WHEN** the user runs `adomi ado pr <pull-request-id>` with valid configuration and credentials
- **THEN** the command behaves the same as `adomi ado pr fetch <pull-request-id>`

#### Scenario: Pull request namespace help lists supported operations
- **WHEN** the user requests help for `adomi ado pr`
- **THEN** the help output lists the supported `fetch`, `ensure`, `comment`, `reply`, `resolve`, and `reopen` operations

#### Scenario: Pull request ensure command
- **WHEN** the user runs `adomi ado pr ensure` with valid inferred repository context, valid configuration, and valid credentials
- **THEN** the command creates or updates the active pull request for the resolved source and target branches

#### Scenario: Pull request comment command
- **WHEN** the user runs `adomi ado pr comment <pull-request-id>` with one valid message source, optional paired `--file <path> --line <line>` inline target flags, valid configuration, and valid credentials
- **THEN** the command creates a new comment thread on the requested pull request, targeting the PR level when no inline target is provided and the latest changed file version when an inline target is provided

#### Scenario: Pull request reply command
- **WHEN** the user runs `adomi ado pr reply <pull-request-id> --thread <thread-id>` with one valid message source, valid configuration, and valid credentials
- **THEN** the command creates a reply comment on the requested pull request thread

#### Scenario: Pull request resolve command
- **WHEN** the user runs `adomi ado pr resolve <pull-request-id> --thread <thread-id>` with valid configuration and credentials
- **THEN** the command marks the requested pull request thread fixed

#### Scenario: Pull request reopen command
- **WHEN** the user runs `adomi ado pr reopen <pull-request-id> --thread <thread-id>` with valid configuration and credentials
- **THEN** the command marks the requested pull request thread active

#### Scenario: Export metadata records Azure DevOps source context
- **WHEN** `adomi ado fetch <work-item-id>`, `adomi ado pr fetch <pull-request-id>`, or `adomi ado pr <pull-request-id>` succeeds
- **THEN** the exported `index.json` records the Azure DevOps source, profile, and project metadata used for the fetch

#### Scenario: Manage credentials
- **WHEN** the user runs `adomi ado login` or `adomi ado logout` with valid credential flags
- **THEN** the command stores or deletes credentials using the same profile and `patRef` behavior as before

#### Scenario: List profiles
- **WHEN** the user runs `adomi ado profiles list`
- **THEN** the command lists configured profile names in sorted order as before

### Requirement: CLI stream behavior

The system SHALL keep stdout reserved for successful command data and stderr reserved for prompts, help, usage, and errors.

#### Scenario: Successful fetch stdout
- **WHEN** `adomi ado fetch <work-item-id>` succeeds
- **THEN** stdout contains only the exported path followed by a newline

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

#### Scenario: Pull request write JSON stdout
- **WHEN** a pull request maintenance command succeeds with `--json`
- **THEN** stdout contains one JSON object followed by a newline and stderr does not contain success data

#### Scenario: Pull request write validation error leaves stdout empty
- **WHEN** a pull request maintenance command receives invalid arguments or cannot infer required repository context
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Pull request write network error leaves stdout empty
- **WHEN** a pull request maintenance command receives an Azure DevOps error response or network error
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Agent skill success stdout
- **WHEN** the user successfully runs `adomi agent skill`
- **THEN** stdout contains only the created skill directory path and stderr does not contain success data

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
