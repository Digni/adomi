# agent-skill-creation Specification

## Purpose
Define how `adomi` creates installable `adomi` agent skill scaffolds for the default shared-agent target and Claude target, either globally for the current user or locally for the current repository.

## Requirements

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

#### Scenario: Generated SKILL requires host keyring access
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` tells agents to run every `adomi` command outside the agent sandbox through an approved host-capable execution path because Adomi uses the host operating system keyring, and to retry sandbox keyring failures outside the sandbox before reporting missing credentials

#### Scenario: Generated SKILL instructs config and project inference
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` tells agents to inspect repository/global config and use repository or folder path structure as hints to resolve Azure DevOps project/profile context before asking the user

#### Scenario: Generated SKILL protects PAT values
- **WHEN** the user runs `adomi agent skill` from any repository
- **THEN** the created `SKILL.md` tells agents never to print, log, or expose PAT values

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

### Requirement: Successful command output
The system SHALL print only the installed skill directory path to stdout when skill creation succeeds.

#### Scenario: Successful global default creation output
- **WHEN** the user successfully runs `adomi agent skill`
- **THEN** stdout contains only the created `~/.agents/skills/adomi` path followed by a newline

#### Scenario: Successful global Claude creation output
- **WHEN** the user successfully runs `adomi agent skill --claude`
- **THEN** stdout contains only the created `~/.claude/skills/adomi` path followed by a newline

#### Scenario: Creation error output
- **WHEN** the user runs `adomi agent skill` and skill creation fails validation
- **THEN** stdout is empty and the error is reported through the command error/stderr path

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

### Requirement: Generated skill documents investigation discovery and evidence exports
The generated skill SHALL teach agents to use PR discovery when work-item links are insufficient, server-side pipeline branch history when project-wide recent runs are noisy, opt-in work-item comment export for handover context, and pipeline inspect for execution evidence. It SHALL distinguish selection, output, read permissions, and evidence limitations while preserving existing governance authorization and host-keyring execution guidance.

#### Scenario: Agent discovers a branch PR and pipeline history
- **WHEN** the generated skill is read for a branch investigation
- **THEN** it documents `adomi ado pr list --source <branch> --status all` with repository/target options and bounded complete pagination, and `adomi ado pipeline list --branch <branch> --last 10` with server-side filtering and unchanged default list scope

#### Scenario: Agent gathers discussion and execution evidence
- **WHEN** the agent needs handover context or to explain what a run executed
- **THEN** the skill documents `adomi ado fetch <id> --include-comments` for current discussion on all exported items and `adomi ado pipeline inspect <run-id>` for its execution/log/test bundle, then instructs the agent to read the resulting files

#### Scenario: Agent interprets evidence conservatively
- **WHEN** inspection returns skipped work, no published tests, or a retrieval error
- **THEN** the skill explains that overall success is not proof every task/test ran, no published tests does not mean no tests executed, and failed retrieval is not empty evidence; it documents additional Test-read permission and one-shot/current-attempt scope

#### Scenario: Existing agent boundaries remain intact
- **WHEN** the generated skill includes the new commands
- **THEN** it retains host-capable execution for every adomi invocation, PAT confidentiality, and the explicit authorization boundaries for PR/work-item writes
