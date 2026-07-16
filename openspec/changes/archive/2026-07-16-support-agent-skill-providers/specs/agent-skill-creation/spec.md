## ADDED Requirements

### Requirement: Named agent provider installation
The system SHALL accept `codex`, `opencode`, `pi`, `github-copilot`, `cursor`, and `claude` as named `--provider` values for `adomi agent skill`, SHALL map providers that support the shared Agent Skills directory to `.agents/skills`, and SHALL validate provider selection before changing the filesystem.

#### Scenario: Create a global shared-provider skill
- **WHEN** the user runs `adomi agent skill --provider <provider>` with `<provider>` equal to `codex`, `opencode`, `pi`, `github-copilot`, or `cursor`
- **THEN** the system creates `~/.agents/skills/adomi/SKILL.md` and no provider-specific duplicate

#### Scenario: Create a project shared-provider skill
- **WHEN** the user runs `adomi agent skill --provider <provider> --project` from a repository with `<provider>` equal to `codex`, `opencode`, `pi`, `github-copilot`, or `cursor`
- **THEN** the system creates `<repo>/.agents/skills/adomi/SKILL.md` and no provider-specific duplicate

#### Scenario: Create a global Claude provider skill
- **WHEN** the user runs `adomi agent skill --provider claude` from any directory
- **THEN** the system creates `~/.claude/skills/adomi/SKILL.md`

#### Scenario: Create a project Claude provider skill
- **WHEN** the user runs `adomi agent skill --provider claude --project` from a repository
- **THEN** the system creates `<repo>/.claude/skills/adomi/SKILL.md`

#### Scenario: Legacy Claude selector remains compatible
- **WHEN** the user runs `adomi agent skill --claude` with any otherwise valid scope and replacement flags
- **THEN** the command selects the same target and preserves the same behavior as `--provider claude`

#### Scenario: Unknown provider is rejected
- **WHEN** the user runs `adomi agent skill --provider <unknown>` with a value outside the supported provider set
- **THEN** the command exits non-zero, leaves stdout empty, and does not create or replace a skill directory

#### Scenario: Provider and legacy Claude selectors conflict
- **WHEN** the user combines an explicit `--provider` value with `--claude`
- **THEN** the command exits non-zero, leaves stdout empty, and does not create or replace a skill directory
