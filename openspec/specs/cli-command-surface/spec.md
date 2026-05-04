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
- **THEN** the command fetches and exports work item context for the requested work item, its parent chain, and any direct children of the requested work item, and prints only the exported directory path to stdout

#### Scenario: Fetch includes direct child work items
- **WHEN** the requested Azure DevOps work item has direct child relations
- **THEN** each direct child work item is fetched and included in the exported JSON, HTML, tree, index, and attachment context alongside the requested item and parent chain

#### Scenario: Fetch pull request comments
- **WHEN** the user runs `adomi ado pr <pull-request-id>` with valid configuration and credentials
- **THEN** the command fetches the pull request, downloads all comment threads, exports them under the repository-local `.adomi/azure-devops/<profile>/<project>/pull-requests/<id>/` directory, and prints only the exported directory path to stdout

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

#### Scenario: Agent skill success stdout
- **WHEN** the user successfully runs `adomi agent skill`
- **THEN** stdout contains only the created skill directory path and stderr does not contain success data

#### Scenario: Agent skill validation error leaves stdout empty
- **WHEN** the user runs `adomi agent skill --claude --unknown .`
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Secret prompt
- **WHEN** `adomi ado login` prompts for a PAT
- **THEN** the prompt is written to stderr and not stdout

#### Scenario: Validation error
- **WHEN** a command receives invalid arguments
- **THEN** the command exits non-zero and reports the validation error on stderr
