## Why

Adomi can fetch Azure DevOps work item context, but it cannot write a short follow-up comment back to the work item after an agent or user has acted on it. Adding a narrow comment-only work item write surface closes that loop without expanding into broader work item field/state mutation.

## What Changes

- Add a canonical `adomi ado comment <work-item-id>` command that posts a text comment to an Azure DevOps work item.
- Add `adomi ado work-item comment <work-item-id>` as a convenience alias for the same behavior.
- Reuse existing Azure DevOps profile, global config, PAT, proxy, message-source, JSON, and stdout/stderr conventions where they already exist for ADO commands.
- Support exactly one message source: `--message <text>` or `--message-file <path>`.
- Print only the created work item comment ID on plain stdout; with `--json`, print one compact JSON result containing the work item ID, comment ID, action, and response URL when Azure DevOps returns one.
- Keep work item comments intentionally comment-only: no work item field updates, state transitions, relation edits, attachment uploads, deletions, or comment updates in this change.

## Capabilities

### New Capabilities
- `ado-work-item-maintenance`: Define conservative Azure DevOps work item write behavior for adding comments while excluding broader work item mutation.

### Modified Capabilities
- `cli-command-surface`: Expose the new work item comment command and alias under `adomi ado`, and preserve data-only stdout / stderr-for-errors stream behavior.

## Impact

- CLI dispatch/help and argument parsing in `internal/cli/root.go` and `internal/cli/ado.go`.
- Azure DevOps client work item comment models, option types, interface methods, URL helpers, and POST handling in `internal/ado/models.go`, `internal/ado/client.go`, and any new focused work item maintenance file if useful.
- CLI and HTTP client coverage in `internal/cli/ado_test.go` and `internal/ado/client_test.go` or a new focused test file.
- Agent skill/help generation, README/docs, and the adomi skill guidance that describe supported Azure DevOps work item operations.
