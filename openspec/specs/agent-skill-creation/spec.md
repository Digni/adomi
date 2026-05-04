# agent-skill-creation Specification

## Purpose
Define how `adomi` creates installable agent skill scaffolds for the default shared-agent target and Claude target, either globally for the current user or locally for the current repository.

## Requirements

### Requirement: Scoped agent skill installation
The system SHALL create an agent skill for the current repository without requiring a positional path, defaulting to the global shared-agent skill root unless the user selects a different scope or agent target.

#### Scenario: Create global default shared-agent skill
- **WHEN** the user runs `adomi agent skill` from a repository named `adomi`
- **THEN** the system creates `~/.agents/skills/adomi/SKILL.md`

#### Scenario: Create explicit global default shared-agent skill
- **WHEN** the user runs `adomi agent skill --global` from a repository named `adomi`
- **THEN** the system creates `~/.agents/skills/adomi/SKILL.md`

#### Scenario: Create global Claude skill
- **WHEN** the user runs `adomi agent skill --claude` from a repository named `adomi`
- **THEN** the system creates `~/.claude/skills/adomi/SKILL.md`

#### Scenario: Create project default shared-agent skill
- **WHEN** the user runs `adomi agent skill --project` from a repository named `adomi`
- **THEN** the system creates `.agents/skills/adomi/SKILL.md` in that repository

#### Scenario: Create project Claude skill
- **WHEN** the user runs `adomi agent skill --claude --project` from a repository named `adomi`
- **THEN** the system creates `.claude/skills/adomi/SKILL.md` in that repository

#### Scenario: Conflicting scopes
- **WHEN** the user runs `adomi agent skill --global --project`
- **THEN** the command exits non-zero and does not create a skill directory

#### Scenario: Positional path is rejected
- **WHEN** the user runs `adomi agent skill .`
- **THEN** the command exits non-zero and does not create a skill directory

### Requirement: Repository-based skill name inference
The system SHALL determine the installed skill name from the current repository root directory basename converted to kebab-case.

#### Scenario: Derive name from repository basename
- **WHEN** the user runs `adomi agent skill` from a repository whose root directory is named `My Agent Skill`
- **THEN** the installed skill directory is named `my-agent-skill`

#### Scenario: Invalid inferred skill name
- **WHEN** the user runs `adomi agent skill --claude` from a repository whose root directory name cannot be represented as a non-empty kebab-case identifier
- **THEN** the command exits non-zero and does not create a skill directory

### Requirement: Generated skill file format
The system SHALL create a `SKILL.md` entry file with valid skill front matter and Markdown body.

#### Scenario: Generated SKILL contains required front matter
- **WHEN** the user runs `adomi agent skill --claude` from a repository named `adomi`
- **THEN** the created `SKILL.md` begins with YAML front matter containing `name: adomi` and a non-empty `description`

#### Scenario: Generated SKILL contains editable instructions
- **WHEN** the user runs `adomi agent skill` from a repository named `adomi`
- **THEN** the created `SKILL.md` contains Markdown sections that explain the skill purpose, when to use it, and the basic process placeholder the user can edit

### Requirement: Existing skill preservation
The system SHALL refuse to overwrite an installed skill directory that already exists.

#### Scenario: Installed global Claude skill already exists
- **WHEN** the user runs `adomi agent skill --claude` and `~/.claude/skills/adomi/` already exists
- **THEN** the command exits non-zero and leaves the existing directory unchanged

#### Scenario: Installed global default skill already exists
- **WHEN** the user runs `adomi agent skill` and `~/.agents/skills/adomi/` already exists
- **THEN** the command exits non-zero and leaves the existing directory unchanged

#### Scenario: Installed project default skill already exists
- **WHEN** the user runs `adomi agent skill --project` and `.agents/skills/adomi/` already exists in the repository
- **THEN** the command exits non-zero and leaves the existing directory unchanged

### Requirement: Successful command output
The system SHALL print only the installed skill directory path to stdout when skill creation succeeds.

#### Scenario: Successful global default creation output
- **WHEN** the user successfully runs `adomi agent skill` from a repository named `adomi`
- **THEN** stdout contains only the created `~/.agents/skills/adomi` path followed by a newline

#### Scenario: Successful global Claude creation output
- **WHEN** the user successfully runs `adomi agent skill --claude` from a repository named `adomi`
- **THEN** stdout contains only the created `~/.claude/skills/adomi` path followed by a newline

#### Scenario: Creation error output
- **WHEN** the user runs `adomi agent skill` and skill creation fails validation
- **THEN** stdout is empty and the error is reported through the command error/stderr path
