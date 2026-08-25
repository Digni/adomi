## ADDED Requirements

### Requirement: Generated skill governs pull request lifecycle and voting
The generated Adomi skill SHALL teach coding agents to treat completion, auto-completion, cancellation, abandonment, and reviewer voting as explicit external writes that require current pull request context and direct user authorization.

#### Scenario: Agent must inspect current pull request state
- **WHEN** a generated skill describes a supported lifecycle or vote command
- **THEN** it instructs the agent to fetch and inspect the pull request immediately before the mutation unless equivalent current state was supplied by the user

#### Scenario: Agent must have explicit authorization
- **WHEN** an agent considers invoking `complete`, `auto-complete`, `cancel-auto-complete`, `abandon`, `approve`, `approve-with-suggestions`, or `reject`
- **THEN** the generated skill instructs it to proceed only when the user explicitly requested that exact governance outcome and not infer authorization from a request to create, update, review, or discuss a pull request

#### Scenario: Agent understands completion behavior
- **WHEN** the generated skill documents immediate or automatic completion
- **THEN** it explains supported completion preference flags, branch-policy enforcement, source-commit pinning for immediate completion, the possibility that auto-complete finishes immediately, and that commands return without polling for final merge success

#### Scenario: Agent understands authenticated-user voting
- **WHEN** the generated skill documents approval, approval with suggestions, and rejection
- **THEN** it explains that the vote is cast only as the authenticated Azure DevOps user and that arbitrary reviewer management and raw vote selection remain unavailable

#### Scenario: Agent understands auto-complete cancellation
- **WHEN** the generated skill documents auto-completion
- **THEN** it also documents `adomi ado pr cancel-auto-complete <pull-request-id>` and explains that cancellation applies only while the pull request remains active

## MODIFIED Requirements

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
- **THEN** the created `SKILL.md` documents `adomi ado fetch <work-item-id>` and `adomi ado pr fetch <pull-request-id>` as the primary commands for gathering work item and pull request context, and notes that `adomi ado pr <pull-request-id>` remains available as a compatibility alias

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
- **THEN** the created `SKILL.md` documents the explicit completion, auto-completion, auto-complete cancellation, abandonment, approval, approval-with-suggestions, and rejection commands while stating that arbitrary reviewer management, vote reset, wait-for-author voting, policy bypass, abandoned-PR reactivation, and completed-PR reversion remain unavailable

#### Scenario: Generated SKILL instructs context-before-maintenance workflow
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` instructs agents to fetch and inspect pull request context before replying to or resolving review threads or performing a lifecycle or vote mutation unless the user has already provided the relevant current details

#### Scenario: Generated SKILL mentions required write scopes without exposing secrets
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` notes that PR maintenance, lifecycle, and reviewer-vote operations require Azure DevOps credentials with code write permission while preserving the existing instruction never to print, log, echo, commit, or expose PAT values

#### Scenario: Generated SKILL documents configuration commands
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` documents available configuration and credential commands including `adomi config init`, `adomi config init --global`, `adomi ado profiles list`, `adomi ado login`, and `adomi ado logout`

#### Scenario: Generated SKILL instructs config and project inference
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` tells agents to inspect repository/global config and use repository or folder path structure as hints to resolve Azure DevOps project/profile context before asking the user

#### Scenario: Generated SKILL protects PAT values
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` tells agents never to print, log, or expose PAT values
