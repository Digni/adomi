# repository-documentation Specification

## Purpose
Define the repository's public documentation hierarchy, supported onboarding path, discoverable workflows, and verifiable user-facing claims for Adomi.

## Requirements

### Requirement: Public README explains Adomi's value and audience
The repository SHALL provide a scannable `README.md` that identifies Adomi as an agent-focused Azure DevOps CLI, explains how repository-local context helps developers and coding agents work from shared source material, and distinguishes context gathering from Adomi's narrow, explicit maintenance operations.

#### Scenario: New visitor understands the product
- **WHEN** a developer opens the GitHub repository without prior Adomi knowledge
- **THEN** the README explains the problem Adomi solves, who it is for, what is written below `.adomi/context`, and how that context supports agent-assisted Azure DevOps work

#### Scenario: Product description does not overstate automation
- **WHEN** the README summarizes Adomi's maintenance capabilities
- **THEN** it makes clear that supported writes are explicit and bounded and does not claim work item editing, PR approval or merge, deployment control, pipeline mutation, or other unsupported automation

### Requirement: Installation and first-use path are complete and reproducible
The documentation SHALL provide a supported installation path, prerequisites, binary verification, configuration, secure credential setup, optional agent-skill installation, and a first repository-local context workflow using commands supported by the current repository.

#### Scenario: Install from the Go module
- **WHEN** a user follows the documented Go-module installation path on a supported development machine
- **THEN** the `adomi` binary is installed without requiring an unpublished package-manager formula, installer script, or release artifact and `adomi --help` succeeds

#### Scenario: Build and install from source
- **WHEN** a contributor follows the documented source installation path from a clone of the repository
- **THEN** the documented command targets `./cmd/adomi`, produces a runnable binary, and includes the necessary PATH or output-location guidance

#### Scenario: Configure a first Azure DevOps profile
- **WHEN** a user follows the getting-started guide inside a Git repository
- **THEN** the guide covers repository or global config initialization, activating the generated YAML example, choosing a default or named profile, storing the PAT through `adomi ado login`, and selecting `--profile` or `--global` when needed

#### Scenario: Complete a first context fetch
- **WHEN** a configured user follows the quick start
- **THEN** the guide offers an optional `adomi agent skill` step, runs a supported work item or pull request fetch, identifies the repository-local output directory, and explains what the user or agent should inspect next

#### Scenario: Onboarding examples protect credentials
- **WHEN** installation or configuration documentation discusses Azure DevOps authentication
- **THEN** it uses placeholders, directs PAT values through secure login and OS keyring storage, describes relevant permissions without embedding a secret in config or command history, and never presents a real credential

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

### Requirement: Documentation has a clear source hierarchy
The repository SHALL use the README as the concise GitHub landing page, `docs/getting-started.md` as the complete onboarding guide, and `docs/azure-devops.md` as the detailed Azure DevOps command and behavior reference, with descriptive links between them and without presenting the obsolete implementation handover as current user guidance.

#### Scenario: README links to deeper guidance
- **WHEN** a reader needs more detail than the README provides
- **THEN** relative links lead to the getting-started guide and Azure DevOps reference and each linked file links back to the relevant entry point where useful

#### Scenario: Legacy handover is retired
- **WHEN** the documentation change is complete
- **THEN** `docs/adomi_azure_devops_handover.md` no longer exists as a competing, implementation-era source and its still-valid user guidance is incorporated into the current documentation hierarchy

#### Scenario: Repository documentation links resolve
- **WHEN** repository-local Markdown links in the README and maintained user guides are checked
- **THEN** every relative file link and section anchor resolves to an existing target with matching casing

### Requirement: User-facing claims remain verifiable
The documentation SHALL derive commands, flags, config locations, output paths, credential requirements, and safety boundaries from the current CLI help, implementation, and canonical OpenSpec specifications, and SHALL include a concise contributor workflow using the repository's actual build and test entry points.

#### Scenario: Command examples match the CLI
- **WHEN** user-facing command examples are compared with command help and argument validation
- **THEN** every command name, required argument, material flag, and documented stdout or output-path behavior matches the current executable contract

#### Scenario: Installation example is smoke tested
- **WHEN** the documentation implementation is verified
- **THEN** the documented install or build command is run with an isolated temporary binary destination and the produced `adomi --help` command succeeds

#### Scenario: Contributor workflow matches the repository
- **WHEN** a contributor follows the README's development section
- **THEN** it identifies the repository's current Go prerequisite and the commands for formatting, testing, and building without claiming nonexistent release or packaging automation

#### Scenario: Documentation change preserves runtime behavior
- **WHEN** the change is applied
- **THEN** no Go source, CLI behavior, config schema, Azure DevOps request, or generated contract is changed as part of the documentation work
