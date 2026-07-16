## MODIFIED Requirements

### Requirement: Implemented workflows are discoverable
The README and linked reference documentation SHALL make each implemented user workflow discoverable through its canonical command, outcome, credential or repository prerequisite, and material limitation.

#### Scenario: Work item workflows are documented
- **WHEN** a user looks for work item support
- **THEN** the documentation covers parent-context and attachment fetching through `adomi ado fetch`, text-only comment creation through the canonical comment command, repository-local export behavior, and the unsupported broader work item mutations

#### Scenario: Pull request workflows are documented
- **WHEN** a user looks for pull request support
- **THEN** the documentation covers context fetching, active-branch PR ensure, PR-level and supported inline comments, thread replies, resolve and reopen operations, repository and branch inference, and the conservative boundary excluding approvals, merges, policy bypass, and reviewer management

#### Scenario: Wiki workflows are documented
- **WHEN** a user looks for wiki support
- **THEN** the documentation covers the required absolute page path, optional recursive subtree fetch, repository-local Markdown and metadata output, and the exclusions of wiki search and attachment download

#### Scenario: Pipeline workflows are documented
- **WHEN** a user looks for pipeline support
- **THEN** the documentation covers one-shot listing of in-progress Build runs and retrieval of one run's overall status, compact JSON output, the required read scope, and the exclusions of polling, stage or job detail, logs, artifacts, mutation, and classic Release deployments

#### Scenario: Agent integration is documented
- **WHEN** a coding-agent user looks for Adomi integration
- **THEN** the documentation explains global and project-scoped `adomi agent skill` targets; names Codex, OpenCode, Pi, GitHub Copilot, Cursor, and Claude as supported providers; explains that the first five share `.agents/skills` while Claude uses `.claude/skills`; documents `--provider` and the backward-compatible `--claude` alias; covers safe replacement flags; and explains how the generated skill teaches agents to use Adomi
