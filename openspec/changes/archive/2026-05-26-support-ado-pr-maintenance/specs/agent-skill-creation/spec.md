## ADDED Requirements

### Requirement: Generated skill documents PR maintenance commands
The system SHALL update the generated `adomi` agent skill so agents can use conservative Azure DevOps pull request maintenance commands after the PR maintenance capability is available.

#### Scenario: Generated SKILL documents explicit PR fetch command
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` documents `adomi ado pr fetch <pull-request-id>` as the canonical command for gathering pull request context and notes that `adomi ado pr <pull-request-id>` remains available as a compatibility alias

#### Scenario: Generated SKILL documents PR ensure command
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` documents `adomi ado pr ensure` as the command for creating or updating the active pull request for the current repository branch

#### Scenario: Generated SKILL documents thread reply command
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` documents `adomi ado pr reply <pull-request-id> --thread <thread-id>` for responding to existing pull request review threads

#### Scenario: Generated SKILL documents thread status commands
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` documents `adomi ado pr resolve <pull-request-id> --thread <thread-id>` and `adomi ado pr reopen <pull-request-id> --thread <thread-id>` for explicit thread status maintenance

#### Scenario: Generated SKILL describes PR maintenance safety boundary
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` tells agents that `adomi` PR maintenance does not approve, reject, merge, complete, abandon, set auto-complete, bypass policies, or manage reviewers

#### Scenario: Generated SKILL instructs context-before-maintenance workflow
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` instructs agents to fetch and inspect pull request context before replying to or resolving review threads unless the user has already provided the relevant thread details

#### Scenario: Generated SKILL mentions required write scopes without exposing secrets
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` notes that PR maintenance requires Azure DevOps credentials with appropriate PR/thread write permissions while preserving the existing instruction never to print, log, echo, commit, or expose PAT values
