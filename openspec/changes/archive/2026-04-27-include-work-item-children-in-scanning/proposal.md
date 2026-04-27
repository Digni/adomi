## Why

Work item context currently follows only the selected item and its parent chain up to Epic, which can omit directly related child tasks or stories that explain implementation details, acceptance criteria, and current progress. Including the selected item's children gives agents a broader but still bounded context for planning and implementation.

## What Changes

- Extend Azure DevOps work item scanning/export to include direct children of the requested work item in addition to the requested item and its parent chain.
- Keep parent-chain traversal to Epic unchanged so hierarchy context remains available.
- Ensure child work items are fetched, exported as JSON/HTML, included in `tree.json`, represented in `index.json`, and have attachments downloaded through the existing export pipeline.
- Preserve existing CLI invocation and stdout behavior; no new user-facing command flags are introduced.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `cli-command-surface`: `adomi ado fetch <work-item-id>` will export direct child work items of the requested item as part of the fetched context while preserving the existing command surface and stream behavior.

## Impact

- Affected code: Azure DevOps work item relation modeling and traversal in `internal/ado`, export/index generation that consumes `WorkItemTree`, and tests for fetch traversal and CLI fetch integration.
- APIs/dependencies: No public CLI or configuration changes; Azure DevOps work item fetches continue using the existing work item API with expanded relations.
- Systems: Export contents under `.adomi/azure-devops/<profile>/<project>/work-items/<root-id>/` will contain additional child work item artifacts when children exist.
