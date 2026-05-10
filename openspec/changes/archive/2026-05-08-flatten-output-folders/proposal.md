## Why

The exported Azure DevOps context directories are deeper than the way users and agents consume them. Paths such as `.adomi/azure-devops/<profile>/<project>/work-items/<id>/work-items/<id>.json` duplicate concepts and make the output harder to scan even though the command already prints the final export directory and `index.json` records profile/project metadata.

## What Changes

- **BREAKING**: Change work item export directories from `.adomi/azure-devops/<profile>/<project>/work-items/<id>/` to `.adomi/context/work-items/<id>/`.
- **BREAKING**: Change pull request export directories from `.adomi/azure-devops/<profile>/<project>/pull-requests/<id>/` to `.adomi/context/pull-requests/<id>/`.
- Rename the per-work-item JSON subdirectory inside a work item export bundle from `work-items/` to `items/`, avoiding nested `work-items/<id>/work-items/<id>.json` paths.
- Keep successful CLI stdout path-only: `adomi ado fetch` and `adomi ado pr` continue to print exactly the generated export directory path followed by a newline.
- Keep provider/profile/project metadata in `index.json` rather than in the primary directory hierarchy.
- Keep repository-local `.adomi/config.yaml` and credential/config behavior unchanged.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `cli-command-surface`: Azure DevOps fetch and pull request commands will export context under the flatter `.adomi/context/<kind>/<id>/` layout while preserving the command surface and stdout/stderr contract.

## Impact

- Affected code: work item output path generation and index relative paths in `internal/ado/export.go`, attachment relative path behavior in `internal/ado/attachments.go`, pull request output path generation in `internal/ado/pullrequest.go`, removal of obsolete profile/project path sanitization if unused, and CLI tests that assert printed export paths.
- Affected docs/specs: `openspec/specs/cli-command-surface/spec.md`, `docs/adomi_azure_devops_handover.md`, and any existing planning documentation that states the old layout.
- APIs/dependencies: No new external dependencies and no CLI flag changes.
- Systems: Existing exports under `.adomi/azure-devops/...` are not migrated or deleted; future runs write the new `.adomi/context/...` bundle and replace only the matching new bundle path.
