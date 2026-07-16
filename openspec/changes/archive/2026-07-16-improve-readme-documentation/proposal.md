## Why

Adomi's public README currently presents a few disconnected Azure DevOps commands but does not explain the agent-focused workflow, how to install and configure the CLI, or the full set of supported capabilities and safety boundaries. A newcomer cannot quickly decide why Adomi is useful or complete a first repository-local context workflow without reading source code and an implementation-era handover document.

## What Changes

- Rework `README.md` into the GitHub landing page for Adomi: explain the problem it solves, the intended agent-assisted development workflow, its repository-local context model, and its conservative write boundaries.
- Add a verified installation and quick-start path covering prerequisites, installation from the Go module or source repository, configuration, secure PAT login, optional agent-skill generation, and a first context fetch.
- Present a scannable capability map for work items, pull requests, wikis, pipeline runs, credentials/profiles, and agent integration, with links to deeper guidance instead of turning the README into an exhaustive command reference.
- Add a focused getting-started guide and replace the implementation-era Azure DevOps handover with a current user-facing Azure DevOps reference that documents commands, output locations, credential requirements, limitations, and safe maintenance boundaries.
- Add a concise contributor section with the verified local build and test entry points, while keeping release packaging and package-manager instructions out of scope until those distribution mechanisms exist.
- Verify documented commands, paths, relative links, and capability claims against the current CLI help, source, and canonical OpenSpec specifications.

## Capabilities

### New Capabilities
- `repository-documentation`: Define the public README, onboarding guide, Azure DevOps reference, discoverability, accuracy, and documentation-verification requirements for Adomi's GitHub repository.

### Modified Capabilities

None. This change documents existing behavior and does not alter any CLI or Azure DevOps contract.

## Impact

- Rewrites `README.md` as the primary public entry point.
- Adds a user-focused getting-started document under `docs/` and replaces `docs/adomi_azure_devops_handover.md` with a current Azure DevOps reference at a descriptive path.
- Adds no executable behavior, API changes, dependencies, configuration fields, release automation, or package-manager support.
- Requires documentation checks plus command/install smoke verification so copied examples remain accurate and secret-safe.
