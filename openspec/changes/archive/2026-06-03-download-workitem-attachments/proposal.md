## Why

Work item context exports are most useful when they contain the referenced files alongside the work item JSON, HTML, tree, and index data. Attachment download behavior is already part of the fetch/export path, but it needs to be explicit in the OpenSpec contract and hardened with coverage for requested, parent-chain, and direct-child work items.

## What Changes

- Make `adomi ado fetch <work-item-id>` explicitly require attachment downloads for every exported work item: the requested work item, its parent chain, and direct children.
- Harden tests/specification around the existing attachment export behavior so regressions are caught, especially parent-chain attachment downloads and failure-path stdout behavior.
- Keep downloaded attachment files under each work item's `attachments/<work-item-id>/` directory inside the exported context directory.
- Ensure the export index exposes each work item's attachment directory path so downstream agents can discover downloaded files.
- Preserve existing stdout behavior: successful fetch prints only the exported context directory path.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `cli-command-surface`: Work item fetch exports must explicitly include attachment downloads for all exported work items and expose the attachment directory paths in export metadata.

## Impact

- Affected CLI/API behavior: `adomi ado fetch <work-item-id>` work item context export.
- Affected code areas: work item tree fetching, attachment download/export helpers, Azure DevOps client attachment download behavior, CLI wiring, and generated export index metadata.
- Affected tests: `internal/ado` export/attachment tests and `internal/cli` fetch wiring/real-wiring/failure-path tests.
- Existing spec overlap: `cli-command-surface` already says direct child work items are included in attachment context; this change makes attachment download behavior explicit and testable for all exported work item categories.
- External dependency: Azure DevOps attachment URLs are fetched with the configured credentials and must continue to respect host validation and size limits.
