## ADDED Requirements

### Requirement: Generated skill documents pull request work-item linking
The system SHALL teach generated Adomi skills how to link explicitly named Azure DevOps work items to a pull request without suggesting broader relation or governance mutations.

#### Scenario: Generated skill documents link command
- **WHEN** the user runs `adomi agent skill`
- **THEN** the created `SKILL.md` documents `adomi ado pr link <pull-request-id> --work-item <work-item-id>` and explains that `--work-item` may be repeated for multiple distinct IDs

#### Scenario: Generated skill documents link safety behavior
- **WHEN** the user runs `adomi agent skill`
- **THEN** the created `SKILL.md` explains that already-linked work items are successful no-ops, multi-item writes are not atomic, and failed invocations are safe to re-run after the cause is addressed

#### Scenario: Generated skill documents permissions and boundary
- **WHEN** the user runs `adomi agent skill`
- **THEN** the created `SKILL.md` states that PR work-item linking requires code read and work-item write permission, keeps PAT values secret, and does not imply support for unlinking, generic work-item relation edits, work-item field/state changes, PR voting, approval, merge, completion, or reviewer management
