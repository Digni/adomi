## ADDED Requirements

### Requirement: Targeted agent skill installation
The system SHALL create an agent skill under the default shared-agent user skill root when the user runs `adomi agent skill` with a local source directory path, and SHALL create it under the Claude user skill root when the user adds `--claude`.

#### Scenario: Create default shared-agent skill from current directory
- **WHEN** the user runs `adomi agent skill .` from a directory named `adomi` and no `SKILL.md` exists in that source directory
- **THEN** the system creates `~/.agents/skills/adomi/SKILL.md`

#### Scenario: Create Claude skill from current directory
- **WHEN** the user runs `adomi agent skill --claude .` from a directory named `adomi` and no `SKILL.md` exists in that source directory
- **THEN** the system creates `~/.claude/skills/adomi/SKILL.md`

#### Scenario: Unsupported target flag
- **WHEN** the user runs `adomi agent skill --default .`
- **THEN** the command exits non-zero and does not create a skill directory

### Requirement: Source path validation
The system SHALL validate that the positional source path exists and is a directory before creating an installed skill.

#### Scenario: Source path does not exist
- **WHEN** the user runs `adomi agent skill --claude ./missing-path`
- **THEN** the command exits non-zero and does not create a skill directory

#### Scenario: Source path is a file
- **WHEN** the user runs `adomi agent skill ./README.md` and `README.md` is a file
- **THEN** the command exits non-zero and does not create a skill directory

#### Scenario: Missing source path
- **WHEN** the user runs `adomi agent skill --claude` without a positional path
- **THEN** the command exits non-zero and does not create a skill directory

### Requirement: Skill metadata inference
The system SHALL determine the installed skill name from an existing source `SKILL.md` when present, otherwise from the source directory basename converted to kebab-case.

#### Scenario: Derive name from directory basename
- **WHEN** the user runs `adomi agent skill ./My Agent Skill` and the source directory does not contain `SKILL.md`
- **THEN** the installed skill directory is named `my-agent-skill`

#### Scenario: Use existing SKILL metadata name
- **WHEN** the user runs `adomi agent skill .` and the source `SKILL.md` front matter contains `name: custom-skill`
- **THEN** the installed skill directory is named `custom-skill`

#### Scenario: Invalid inferred skill name
- **WHEN** the user runs `adomi agent skill --claude .` and the resulting skill name cannot be represented as a non-empty kebab-case identifier
- **THEN** the command exits non-zero and does not create a skill directory

### Requirement: Generated skill file format
The system SHALL create a `SKILL.md` entry file with valid skill front matter and Markdown body when the source directory does not already provide a skill file.

#### Scenario: Generated SKILL contains required front matter
- **WHEN** the user runs `adomi agent skill --claude .` from a source directory named `adomi` without an existing `SKILL.md`
- **THEN** the created `SKILL.md` begins with YAML front matter containing `name: adomi` and a non-empty `description`

#### Scenario: Generated SKILL contains editable instructions
- **WHEN** the user runs `adomi agent skill .` from a source directory named `adomi` without an existing `SKILL.md`
- **THEN** the created `SKILL.md` contains Markdown sections that explain the skill purpose, when to use it, and the basic process placeholder the user can edit

### Requirement: Existing skill preservation
The system SHALL refuse to overwrite an installed skill directory that already exists.

#### Scenario: Installed Claude skill already exists
- **WHEN** the user runs `adomi agent skill --claude .` and `~/.claude/skills/adomi/` already exists
- **THEN** the command exits non-zero and leaves the existing directory unchanged

#### Scenario: Installed default skill already exists
- **WHEN** the user runs `adomi agent skill .` and `~/.agents/skills/adomi/` already exists
- **THEN** the command exits non-zero and leaves the existing directory unchanged

### Requirement: Successful command output
The system SHALL print only the installed skill directory path to stdout when skill creation succeeds.

#### Scenario: Successful default creation output
- **WHEN** the user successfully runs `adomi agent skill .` from a directory named `adomi`
- **THEN** stdout contains only the created `~/.agents/skills/adomi` path followed by a newline

#### Scenario: Successful Claude creation output
- **WHEN** the user successfully runs `adomi agent skill --claude .` from a directory named `adomi`
- **THEN** stdout contains only the created `~/.claude/skills/adomi` path followed by a newline

#### Scenario: Creation error output
- **WHEN** the user runs `adomi agent skill .` and skill creation fails validation
- **THEN** stdout is empty and the error is reported through the command error/stderr path
