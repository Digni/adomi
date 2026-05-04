## MODIFIED Requirements

### Requirement: Scoped agent skill installation
The system SHALL create an `adomi` agent skill without requiring a positional path, defaulting to the global shared-agent skill root unless the user selects a different scope or agent target.

#### Scenario: Create global default shared-agent skill
- **WHEN** the user runs `adomi agent skill` from any directory
- **THEN** the system creates `~/.agents/skills/adomi/SKILL.md`

#### Scenario: Create explicit global default shared-agent skill
- **WHEN** the user runs `adomi agent skill --global` from any directory
- **THEN** the system creates `~/.agents/skills/adomi/SKILL.md`

#### Scenario: Create global Claude skill
- **WHEN** the user runs `adomi agent skill --claude` from any directory
- **THEN** the system creates `~/.claude/skills/adomi/SKILL.md`

#### Scenario: Create project default shared-agent skill
- **WHEN** the user runs `adomi agent skill --project` from a repository
- **THEN** the system creates `.agents/skills/adomi/SKILL.md` in that repository

#### Scenario: Create project Claude skill
- **WHEN** the user runs `adomi agent skill --claude --project` from a repository
- **THEN** the system creates `.claude/skills/adomi/SKILL.md` in that repository

#### Scenario: Conflicting scopes
- **WHEN** the user runs `adomi agent skill --global --project`
- **THEN** the command exits non-zero and does not create a skill directory

#### Scenario: Positional path is rejected
- **WHEN** the user runs `adomi agent skill .`
- **THEN** the command exits non-zero and does not create a skill directory

### Requirement: Repository-based skill name inference
The system SHALL no longer infer the installed skill name from the current repository root directory and SHALL use `adomi` as the generated skill name.

#### Scenario: Repository basename is ignored for skill name
- **WHEN** the user runs `adomi agent skill` from a repository whose root directory is named `My Agent Skill`
- **THEN** the installed skill directory is named `adomi`

#### Scenario: Generated skill front matter uses fixed name
- **WHEN** the user runs `adomi agent skill --project` from a repository whose root directory is named `my-app`
- **THEN** the created `SKILL.md` front matter contains `name: adomi`

### Requirement: Generated skill file format
The system SHALL create a `SKILL.md` entry file with valid skill front matter and an `adomi`-specific Markdown body that teaches agents how to use `adomi` for Azure DevOps work.

#### Scenario: Generated SKILL contains required front matter
- **WHEN** the user runs `adomi agent skill --claude` from any repository
- **THEN** the created `SKILL.md` begins with YAML front matter containing `name: adomi` and a non-empty `description`

#### Scenario: Generated SKILL description targets Azure DevOps work
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` description mentions Azure DevOps and ADO work so agents can select it for Azure DevOps-related user requests

#### Scenario: Generated SKILL documents Azure DevOps context commands
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` documents `adomi ado fetch <work-item-id>` and `adomi ado pr <pull-request-id>` as the primary commands for gathering work item and pull request context

#### Scenario: Generated SKILL documents configuration commands
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` documents available configuration and credential commands including `adomi config init`, `adomi config init --global`, `adomi ado profiles list`, `adomi ado login`, and `adomi ado logout`

#### Scenario: Generated SKILL instructs config and project inference
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` tells agents to inspect repository/global config and use repository or folder path structure as hints to resolve Azure DevOps project/profile context before asking the user

#### Scenario: Generated SKILL protects PAT values
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` tells agents never to print, log, or expose PAT values

### Requirement: Existing skill preservation
The system SHALL require explicit confirmation or `--force`/`--yes` before replacing an installed skill directory that already exists and SHALL preserve the old skill if replacement fails.

#### Scenario: Installed global Claude skill already exists and user declines replacement
- **WHEN** the user runs `adomi agent skill --claude`, `~/.claude/skills/adomi/` already exists, and the user does not confirm replacement
- **THEN** the command exits non-zero and leaves the existing directory unchanged

#### Scenario: Installed global default skill already exists and user confirms replacement
- **WHEN** the user runs `adomi agent skill`, `~/.agents/skills/adomi/` already exists, and the user confirms replacement
- **THEN** the command replaces the existing directory with a newly generated skill and prints the skill directory path to stdout

#### Scenario: Installed global default skill already exists and user forces replacement
- **WHEN** the user runs `adomi agent skill --force` and `~/.agents/skills/adomi/` already exists
- **THEN** the command replaces the existing directory with a newly generated skill without prompting and prints the skill directory path to stdout

#### Scenario: Installed global default skill already exists and user uses yes replacement
- **WHEN** the user runs `adomi agent skill --yes` and `~/.agents/skills/adomi/` already exists
- **THEN** the command replaces the existing directory with a newly generated skill without prompting and prints the skill directory path to stdout

#### Scenario: Installed project default skill already exists and user declines replacement
- **WHEN** the user runs `adomi agent skill --project`, `.agents/skills/adomi/` already exists in the repository, and the user does not confirm replacement
- **THEN** the command exits non-zero and leaves the existing directory unchanged

#### Scenario: Confirmed replacement write fails
- **WHEN** the user confirms replacement and writing the new `SKILL.md` fails
- **THEN** the command exits non-zero, keeps stdout empty, and restores the previous skill directory where possible
