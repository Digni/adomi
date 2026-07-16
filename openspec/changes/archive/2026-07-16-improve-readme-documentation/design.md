## Context

`README.md` is currently a 46-line command summary. It covers work item comments, wiki fetch, and pipeline status, but it omits installation, configuration, agent-skill generation, pull request workflows, work item attachment behavior, repository-local output, contributor entry points, and an explanation of why these capabilities matter together.

The only broad document under `docs/` is `adomi_azure_devops_handover.md`. It contains useful configuration, command, output, and safety details, but it is structured as an implementation handover with historical MVP instructions and stale non-goals. The current CLI and canonical specifications are more authoritative: the root exposes `config`, `agent`, and `ado`; Azure DevOps support covers work items, PRs, wikis, and pipeline runs; profiles can be repository-local or global; PATs are stored through the OS keyring; and supported writes are deliberately narrow.

There are currently no release tags, installer scripts, package-manager manifests, or GitHub release workflows in the repository. Installation guidance therefore has to describe only a Go/source path that can be reproduced from the current repository. Direct remote `go install ...@latest` availability is **UNVERIFIED** and must not be presented as supported unless the implementation smoke verifies it.

## Goals / Non-Goals

**Goals:**

- Make the README answer, in order: what Adomi is, why an agent-assisted developer would use it, what it can do, how to install and configure it, and where to learn more.
- Give a new user a short, secret-safe path from clone or Go installation to a first repository-local context export.
- Keep detailed command behavior and safety boundaries discoverable without overwhelming the GitHub landing page.
- Replace the implementation-era handover with a maintained user reference whose claims are traceable to current help, source, and specifications.
- Give contributors the actual formatting, test, and build entry points without implying release infrastructure that does not exist.

**Non-Goals:**

- Add or change CLI behavior, configuration, Azure DevOps API support, generated agent-skill content, or output formats.
- Create Homebrew, package-manager, binary-release, installer, container, or CI/release automation.
- Publish hosted documentation, a documentation generator, screenshots, badges, benchmarks, or marketing claims that require external evidence.
- Turn the README into an exhaustive flag reference or retain internal implementation history in the primary user journey.

## Decisions

### Use a three-level documentation hierarchy

`README.md` will be the scannable GitHub landing page, `docs/getting-started.md` will contain the reproducible onboarding flow, and `docs/azure-devops.md` will contain the detailed command, output, authentication, and safety reference. The README will link directly to both guides, and the guides will provide useful navigation back to the README or one another.

This separates orientation, onboarding, and reference concerns while keeping the documentation set small. Putting every flag and exclusion in the README would bury the value proposition; creating one page per command would add unnecessary maintenance overhead for the current command surface.

### Organize the README around the agent workflow

The opening will describe the core loop: configure an Azure DevOps profile, securely store credentials, fetch source context into the current repository, let a coding agent inspect the exported artifacts, and perform only user-authorized maintenance. A compact capability table will map each job to its canonical command and result, followed by a minimal quick start and links to detailed guidance.

This tells users how the features work together instead of presenting an alphabetic command inventory. Safety boundaries will appear beside the relevant write capability so they are visible before a user follows a maintenance example.

### Document only install paths proven from the current repository

The primary path will use the Go toolchain and `./cmd/adomi`, with separate guidance for installing to a binary destination and building from a clone. The implementation will smoke-test the copied command with isolated temporary Go cache and binary directories, then run the resulting binary's help command. A direct remote module command may be documented only if it can be verified without relying on a nonexistent release tag; otherwise the clone-based Go path remains the supported route.

This is less convenient than a package manager but accurately reflects the current distribution state. Adding packaging belongs in a separate change because it would introduce release behavior rather than documentation.

### Treat current contracts as the source of truth

Documentation claims will be reconciled in this order: current CLI help and argument parsing for commands and flags, canonical OpenSpec specifications for behavioral boundaries, implementation for paths and generated artifacts, and the handover only as a source of still-valid explanatory material. The old handover will be replaced by `docs/azure-devops.md`; it will not be copied wholesale or kept as a competing reference.

This prevents stale MVP instructions and non-goals from being promoted into public documentation. It also makes future documentation reviews traceable to concrete repository sources.

### Keep examples copyable, data-oriented, and secret-safe

Examples will use placeholders, prefer canonical command names, show that fetch commands return paths while pipeline commands return JSON, and direct PAT entry through `adomi ado login`. The guides will explain repository versus global config and note required Azure DevOps permissions at the workflow level without showing raw credentials or encouraging PATs in YAML or shell history.

This matches Adomi's existing stdout and credential-handling contracts and makes examples suitable for both humans and agents.

## Risks / Trade-offs

- **Documentation duplicates CLI help and can drift** → Keep the README summary-level, centralize detailed behavior in `docs/azure-devops.md`, compare examples with current help/specs during implementation, and include a final link and command audit.
- **Installation guidance may imply a distribution channel that is not actually available** → Smoke-test an isolated Go/source installation first and omit remote `@latest`, package-manager, or release-asset claims unless they are proven by current repository state.
- **A broad capability overview may hide important write boundaries** → Pair every maintenance summary with its exclusions and link to the full reference before presenting advanced examples.
- **Replacing the handover can lose still-useful implementation knowledge** → Inventory its config, output-layout, authentication, proxy, and API-boundary content before removal, carry forward user-relevant facts, and rely on source/specs rather than preserving obsolete implementation instructions.
- **The three-file structure introduces some repeated onboarding text** → Keep the README quick start deliberately minimal and make the getting-started guide the single complete setup flow.
