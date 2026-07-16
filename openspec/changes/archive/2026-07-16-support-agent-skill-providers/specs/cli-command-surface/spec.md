## MODIFIED Requirements

### Requirement: Agent skill command namespace

The system SHALL expose agent-oriented skill creation through the top-level `adomi agent skill` command, SHALL provide named coding-agent provider selection, and SHALL preserve the existing `Run(args, stdin, stdout, stderr)` execution boundary.

#### Scenario: Agent namespace appears in top-level help
- **WHEN** the user runs `adomi` without a command
- **THEN** the command exits non-zero and reports usage that includes the `agent` top-level command on stderr

#### Scenario: Agent skill help describes scope and provider flags
- **WHEN** the user requests help for `adomi agent skill`
- **THEN** the help output describes the default global scope, the `--global` scope, the `--project` scope, the `--provider` selector with all supported values, the shared `.agents/skills` mapping, and the backward-compatible `--claude` target override

#### Scenario: Named provider is accepted
- **WHEN** the user runs `adomi agent skill --provider <provider>` with a supported provider and otherwise valid arguments
- **THEN** the command resolves the documented provider target and preserves the existing success stream behavior

#### Scenario: Unsupported provider is rejected through the command boundary
- **WHEN** the user runs `adomi agent skill --provider <unknown>`
- **THEN** the command exits non-zero, reports the unsupported provider through the error or stderr path, and leaves stdout empty

#### Scenario: Explicit and legacy provider selectors conflict
- **WHEN** the user runs `adomi agent skill --provider <provider> --claude`
- **THEN** the command exits non-zero, reports the conflicting selectors through the error or stderr path, and leaves stdout empty

#### Scenario: Existing Azure DevOps namespace remains available
- **WHEN** the user runs `adomi ado` without an Azure DevOps subcommand
- **THEN** the command exits non-zero and reports usage for the existing Azure DevOps subcommands

#### Scenario: Existing config namespace remains available
- **WHEN** the user runs `adomi config init` with valid arguments
- **THEN** the command behavior remains the same as before the agent command was added
