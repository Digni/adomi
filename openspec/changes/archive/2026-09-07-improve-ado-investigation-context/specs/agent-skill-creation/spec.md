## ADDED Requirements

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
