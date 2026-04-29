## ADDED Requirements

### Requirement: Agent skill command namespace
The system SHALL expose agent-oriented skill creation through the top-level `adomi agent skill` command while preserving the existing `Run(args, stdin, stdout, stderr)` execution boundary.

#### Scenario: Agent namespace appears in top-level help
- **WHEN** the user runs `adomi` without a command
- **THEN** the command exits non-zero and reports usage that includes the `agent` top-level command on stderr

#### Scenario: Agent skill help describes default and Claude targets
- **WHEN** the user requests help for `adomi agent skill`
- **THEN** the help output describes that omitting a target writes to the default shared-agent root, describes the `--claude` target override, and describes the required source path argument

#### Scenario: Existing Azure DevOps namespace remains available
- **WHEN** the user runs `adomi ado` without an Azure DevOps subcommand
- **THEN** the command exits non-zero and reports usage for the existing Azure DevOps subcommands

#### Scenario: Existing config namespace remains available
- **WHEN** the user runs `adomi config init` with valid arguments
- **THEN** the command behavior remains the same as before the agent command was added

### Requirement: Agent command stream behavior
The system SHALL keep stdout reserved for successful `adomi agent` command data and stderr reserved for help, usage, and errors.

#### Scenario: Agent skill success stdout
- **WHEN** the user successfully runs `adomi agent skill .`
- **THEN** stdout contains only the created skill directory path and stderr does not contain success data

#### Scenario: Agent skill validation error leaves stdout empty
- **WHEN** the user runs `adomi agent skill --claude --unknown .`
- **THEN** stdout is empty and the command exits non-zero
