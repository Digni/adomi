## Why

Adomi can export Azure DevOps work items and pull requests as repository-local agent context, but it cannot fetch the Markdown documentation stored in Azure DevOps wikis. Agents therefore need a separate tool or manual copy step to use a wiki page or page subtree as local context.

## What Changes

- Add `adomi ado wiki fetch <wiki-id-or-name>` with a required absolute page path and optional recursive subtree selection.
- Resolve the requested wiki, fetch page metadata and Markdown through the authenticated Azure DevOps client, and export a deterministic bundle under `.adomi/context/wikis/<wiki-id>/`.
- Preserve the existing CLI contract: successful fetches print only the exported directory path to stdout; validation, API, decoding, and filesystem failures return an error with empty stdout.
- Keep the first slice read-only and context-oriented. Azure DevOps indexed wiki search, wiki mutation, version overrides, attachment mirroring, and archival/migration guarantees are out of scope.

## Capabilities

### New Capabilities

- `ado-wiki-context`: Resolve an Azure DevOps wiki and export one Markdown page or a recursive page subtree as repository-local agent context with discoverable metadata.

### Modified Capabilities

- `cli-command-surface`: Add the `adomi ado wiki fetch` namespace, arguments, help, success output, and failure-stream behavior.

## Impact

- Adds read-only wiki models, client interfaces, authenticated REST calls, orchestration, and export helpers under `internal/ado`.
- Extends the CLI command tree, dependency seams, argument parsing/help, and real-wiring coverage under `internal/cli`.
- Updates the generated Adomi agent skill and README so agents and users can discover the new context command.
- Adds wiki context files below the existing repository-local `.adomi/context` boundary without changing existing work item or pull request exports.
- Uses the existing standard-library HTTP stack and Azure DevOps profile/PAT configuration; no new dependency is expected.
