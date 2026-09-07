## ADDED Requirements

### Requirement: Pull request governance documentation is operationally complete
The repository documentation SHALL describe the supported pull request lifecycle and authenticated-user voting workflows with commands, prerequisites, outputs, state restrictions, completion preferences, and remaining safety exclusions that match the executable contract.

#### Scenario: Lifecycle commands are documented
- **WHEN** a user reads the pull request reference
- **THEN** it documents `complete`, `auto-complete`, `cancel-auto-complete`, and `abandon`, including valid completion preferences, branch-policy enforcement, idempotent behavior, terminal-state restrictions, source-commit pinning for immediate completion, and the lack of completion polling

#### Scenario: Reviewer vote commands are documented
- **WHEN** a user reads the pull request reference
- **THEN** it documents `approve`, `approve-with-suggestions`, and `reject`, their Azure DevOps vote meanings, authenticated-user-only behavior, active-pull-request restriction, and the exclusion of arbitrary reviewer management

#### Scenario: Governance permissions and outputs are documented
- **WHEN** a user prepares to run a lifecycle or vote command
- **THEN** the documentation identifies the code write permission, explains `--profile`, `--global`, and `--json`, distinguishes plain ID output from structured state output, and does not expose PAT values

#### Scenario: Agent authorization boundary is documented
- **WHEN** a user delegates pull request work to a coding agent
- **THEN** the documentation states that the agent requires explicit authorization for the exact lifecycle or vote mutation and must not infer that authorization from a request to create, update, review, or discuss a pull request

## MODIFIED Requirements

### Requirement: Public README explains Adomi's value and audience
The repository SHALL provide a scannable `README.md` that identifies Adomi as an agent-focused Azure DevOps CLI, explains how repository-local context helps developers and coding agents work from shared source material, and distinguishes context gathering from Adomi's bounded, explicit maintenance and governance operations.

#### Scenario: New visitor understands the product
- **WHEN** a developer opens the GitHub repository without prior Adomi knowledge
- **THEN** the README explains the problem Adomi solves, who it is for, what is written below `.adomi/context`, and how that context supports agent-assisted Azure DevOps work

#### Scenario: Product description does not overstate automation
- **WHEN** the README summarizes Adomi's maintenance and governance capabilities
- **THEN** it makes clear that supported writes are explicit and bounded, names the authenticated-user and branch-policy constraints on pull request governance, and does not claim broader work item editing, arbitrary reviewer management, policy bypass, deployment control, pipeline mutation, or other unsupported automation

### Requirement: Implemented workflows are discoverable
The README and linked reference documentation SHALL make each implemented user workflow discoverable through its canonical command, outcome, credential or repository prerequisite, and material limitation.

#### Scenario: Work item workflows are documented
- **WHEN** a user looks for work item support
- **THEN** the documentation covers parent-context and attachment fetching through `adomi ado fetch`, text-only comment creation through the canonical comment command, repository-local export behavior, and the unsupported broader work item mutations

#### Scenario: Pull request workflows are documented
- **WHEN** a user looks for pull request support
- **THEN** the documentation covers context fetching, active-branch PR ensure, work-item linking, PR-level and supported inline comments, thread replies, resolve and reopen operations, immediate completion, policy-gated auto-completion and cancellation, abandonment, authenticated-user approval, approval with suggestions, and rejection; identifies repository and branch inference, code write permission, completion preferences, explicit authorization, asynchronous behavior, and output contracts; and preserves the boundary excluding arbitrary reviewer management, vote reset, wait-for-author voting, policy bypass, reactivation, and reversion

#### Scenario: Wiki workflows are documented
- **WHEN** a user looks for wiki support
- **THEN** the documentation covers the required absolute page path, optional recursive subtree fetch, repository-local Markdown and metadata output, and the exclusions of wiki search and attachment download

#### Scenario: Pipeline workflows are documented
- **WHEN** a user looks for pipeline support
- **THEN** the documentation covers one-shot listing of in-progress Build runs and retrieval of one run's overall status, compact JSON output, the required read scope, and the exclusions of polling, stage or job detail, logs, artifacts, mutation, and classic Release deployments

#### Scenario: Agent integration is documented
- **WHEN** a coding-agent user looks for Adomi integration
- **THEN** the documentation explains global and project-scoped `adomi agent skill` targets; names Codex, OpenCode, Pi, GitHub Copilot, Cursor, and Claude as supported providers; explains that the first five share `.agents/skills` while Claude uses `.claude/skills`; documents `--provider` and the backward-compatible `--claude` alias; covers safe replacement flags; and explains how the generated skill teaches agents to use Adomi, including the explicit authorization boundary for lifecycle and vote writes
