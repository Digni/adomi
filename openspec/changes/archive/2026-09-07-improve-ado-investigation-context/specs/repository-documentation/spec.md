## MODIFIED Requirements

### Requirement: Implemented workflows are discoverable
The README and linked reference documentation SHALL make each implemented user workflow discoverable through its canonical command, outcome, credential or repository prerequisite, and material limitation.

#### Scenario: Work item workflows are documented
- **WHEN** a user looks for work item support
- **THEN** the documentation covers parent-context and attachment fetching through `adomi ado fetch`, opt-in paginated non-deleted discussion for all exported items through `--include-comments`, text-only comment creation through the canonical comment command, repository-local export behavior, and the unsupported broader work item mutations

#### Scenario: Pull request workflows are documented
- **WHEN** a user looks for pull request support
- **THEN** the documentation covers repository/source/target/status PR discovery through `adomi ado pr list` with automatic bounded paging, JSON output and read permission, context fetching, active-branch PR ensure, work-item linking, PR-level and supported inline comments, thread replies, resolve and reopen operations, immediate completion, policy-gated auto-completion and cancellation, abandonment, authenticated-user approval, approval with suggestions, and rejection; identifies repository and branch inference, code write permission, completion preferences, explicit authorization, asynchronous behavior, and output contracts; and preserves the boundary excluding arbitrary reviewer management, vote reset, wait-for-author voting, policy bypass, reactivation, and reversion

#### Scenario: Wiki workflows are documented
- **WHEN** a user looks for wiki support
- **THEN** the documentation covers the required absolute page path, optional recursive subtree fetch, repository-local Markdown and metadata output, and the exclusions of wiki search and attachment download

#### Scenario: Pipeline workflows are documented
- **WHEN** a user looks for pipeline support
- **THEN** the documentation covers one-shot in-progress and recent Build-run listing with optional server-side branch filtering, summary get with stable JSON, and inspect exports of observed stage/job/task records, failed-task logs, and published tests; explains Build/Test read scopes, unique snapshot paths, optional JSON output, current-attempt/one-shot limits, empty versus unavailable evidence, and fixed budgets; and excludes polling, arbitrary artifacts, test attachments, mutation, and classic Release deployments

#### Scenario: Agent integration is documented
- **WHEN** a coding-agent user looks for Adomi integration
- **THEN** the documentation explains global and project-scoped `adomi agent skill` targets; names Codex, OpenCode, Pi, GitHub Copilot, Cursor, and Claude as supported providers; explains that the first five share `.agents/skills` while Claude uses `.claude/skills`; documents `--provider` and the backward-compatible `--claude` alias; covers safe replacement flags; and explains how the generated skill teaches agents to use Adomi, including the explicit authorization boundary for lifecycle and vote writes
