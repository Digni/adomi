## MODIFIED Requirements

### Requirement: CLI stream behavior

The system SHALL keep stdout reserved for successful command data and stderr reserved for prompts, help, usage, and errors.

#### Scenario: Successful fetch stdout
- **WHEN** `adomi ado fetch <work-item-id>` succeeds
- **THEN** stdout contains only the exported path followed by a newline

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
