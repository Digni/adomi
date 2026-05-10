## Context

`adomi ado fetch` and `adomi ado pr` currently export repository-local context under provider/profile/project-qualified directories. The current work item output path is built by `OutputPath` in `internal/ado/export.go:41-50` as `.adomi/azure-devops/<profile>/<project>/work-items/<root-id>`, and `ExportContext` then creates an inner `work-items` directory at `internal/ado/export.go:65-67` and writes each item to `work-items/<id>.json` at `internal/ado/export.go:80-83`. The resulting path repeats the work item concept: `.../work-items/<id>/work-items/<id>.json`.

Pull request exports use the same provider/profile/project prefix via `PullRequestOutputPath` in `internal/ado/pullrequest.go:138-147`, then write the PR bundle files under that directory at `internal/ado/pullrequest.go:158-193`. Attachments are written relative to the export bundle by `DownloadAttachments` in `internal/ado/attachments.go:39-72`, so they can remain bundle-local if the bundle root changes.

The existing OpenSpec CLI surface currently requires PR exports under `.adomi/azure-devops/<profile>/<project>/pull-requests/<id>/` in `openspec/specs/cli-command-surface/spec.md:84-86`. CLI tests also assert old printed paths for work items and pull requests in `internal/cli/ado_test.go:335-346`, `internal/cli/ado_test.go:440-459`, `internal/cli/ado_test.go:846-857`, `internal/cli/ado_test.go:900-919`, `internal/cli/ado_test.go:1006-1011`, and `internal/cli/ado_test.go:1091-1097`.

## Goals / Non-Goals

**Goals:**

- Flatten future context exports to `.adomi/context/work-items/<id>/` and `.adomi/context/pull-requests/<id>/`.
- Keep `adomi ado fetch` and `adomi ado pr` stdout path-only with a single trailing newline.
- Keep provider/profile/project/source metadata available in `index.json` instead of encoding it in the primary directory hierarchy.
- Remove the duplicated inner `work-items/` subdirectory by storing per-work-item JSON under `items/` inside work item bundles.
- Preserve existing bundle-local subdirectories that still carry useful meaning, such as `html/`, `attachments/<work-item-id>/`, and `threads/`.

**Non-Goals:**

- Do not change command names, flags, configuration loading, PAT storage, Azure DevOps API calls, or repository root detection.
- Do not migrate, delete, or read existing `.adomi/azure-devops/...` exports.
- Do not add compatibility flags or duplicate writes to both old and new layouts.
- Do not change the shape of Azure DevOps JSON payloads, rendered HTML content, PR comments markdown, or attachment filename sanitization.

## Decisions

1. **Use `.adomi/context/<kind>/<id>/` as the canonical export bundle root.**
   - Work item root: `.adomi/context/work-items/<id>/`.
   - Pull request root: `.adomi/context/pull-requests/<id>/`.
   - Rationale: The exported bundle is consumed via the printed path, so provider/profile/project are better treated as metadata than navigation. The current `Index` and `PullRequestIndex` already include profile/project/source fields in `internal/ado/export.go:22-29` and `internal/ado/pullrequest.go:216-228`.
   - Alternative considered: `.adomi/ado/<kind>/<id>/`; rejected because `context` describes the exported artifact's purpose rather than the upstream provider.
   - Alternative considered: `.adomi/context/<profile>-<project>/<kind>/<id>/`; rejected because the user selected Option A and values human/agent readability over path-level collision isolation.

2. **Rename inner per-work-item JSON directory from `work-items/` to `items/`.**
   - Rationale: The outer `work-items/<id>` identifies the bundle kind and identity; the inner directory only contains item JSON payloads for the root, parents, and children. `items/12345.json` avoids `work-items/12345/work-items/12345.json` while preserving a grouped home for multiple item JSON files.
   - Alternative considered: Store item JSON files directly at bundle root. Rejected because root files already contain `index.json` and `tree.json`, and child/parent item files would clutter the navigational entry point.

3. **Keep bundle-local derived-data directories unchanged where they are not redundant.**
   - `html/<id>.html`, `attachments/<work-item-id>/<filename>`, and `threads/<thread-id>.json` remain descriptive and avoid mixing generated views, binary attachments, and thread payloads into the root.
   - Rationale: This reduces only the confusing hierarchy while preserving useful internal grouping.

4. **Treat this as a breaking output-layout change without migration.**
   - Rationale: The CLI contract prints the generated directory path, so scripts using `LOCATION=$(adomi ado fetch 12345)` continue to work. Scripts that hardcode `.adomi/azure-devops/...` are intentionally broken and must use the printed path or new layout.
   - Existing old exports are not removed because `ExportContext` and `ExportPullRequest` remove only the exact output directory they are about to write (`internal/ado/export.go:61-64`, `internal/ado/pullrequest.go:158-160`).

## Risks / Trade-offs

- **Cross-profile/project ID collisions overwrite the same new bundle path** → Accept for Option A. The most recent export for a given repository/kind/id wins, and `index.json` records the source/profile/project for inspection. If this becomes a real workflow issue later, add a separate namespacing proposal.
- **Downstream users may have hardcoded old paths** → Mark the proposal as breaking, update docs/specs/tests, and preserve stdout path-only so command-substitution workflows remain stable.
- **Partial path updates could leave stale index references** → Update path generation and `index.json` relative paths together, then test both filesystem existence and index values.
- **Old `.adomi/azure-devops` data remains on disk** → Do not delete it automatically to avoid surprising data loss; document that future exports use `.adomi/context`.
- **Inner directory rename misses attachment summaries** → Attachment summaries already build paths relative to the bundle root in `internal/ado/attachments.go:59-68`, so they should stay unchanged; tests should confirm attachment files remain under `attachments/<work-item-id>/`.

## Edge Cases And Failure Modes

- Exporting with a nil work item tree or PR bundle must continue to fail before creating paths, preserving the validation currently in `internal/ado/export.go:53-56` and `internal/ado/pullrequest.go:150-153`.
- If `os.RemoveAll` fails for the new bundle root, export must return an error and avoid writing partial success, matching current removal behavior in `internal/ado/export.go:61-64` and `internal/ado/pullrequest.go:158-160`.
- If creating `items/`, `html/`, `attachments/`, or `threads/` fails, export must return an error and stdout must remain empty through the existing CLI error path.
- Empty or sanitized profile/project values no longer affect the output path, but must remain available in `index.json` exactly as loaded so users can identify source metadata.
- Removing profile/project path sanitization is safe for the new output root because the only dynamic path segment under `.adomi/context/<kind>/` is an integer ID and `<kind>` is a hardcoded constant.
- Existing old-layout directories may coexist with new-layout directories. The implementation must not read old exports, delete old exports, or assume they are absent.
- Fetching the same work item ID from two configured ADO projects in the same repository overwrites `.adomi/context/work-items/<id>/`; this is an accepted Option A trade-off.

## Migration Plan

- Update work item and PR output path helpers to return `.adomi/context/<kind>/<id>`.
- Update work item bundle internals from `work-items/<id>.json` to `items/<id>.json`, including `index.json` relative paths.
- Update tests, docs, and OpenSpec requirements that assert old paths.
- No persisted data migration is required. Rollback is a code revert; old export folders remain untouched.

## Verification Checkpoints

- After updating work item export path/layout: run `go test ./internal/ado -run 'TestExport|TestOutputPath|TestAttachment'`; expected: work item export and attachment tests pass with `.adomi/context/work-items/<id>` and `items/<id>.json`.
- After updating pull request export path/layout: run `go test ./internal/ado -run 'TestPullRequestOutputPath|TestExportPullRequest'`; expected: PR export tests pass with `.adomi/context/pull-requests/<id>`.
- After updating CLI expectations and real wiring tests: run `go test ./internal/cli`; expected: stdout remains path-only and points at the new layout.
- After docs/spec updates: run `openspec validate flatten-output-folders --strict`; expected: change validates.
- Final verification: run `go test ./...`; expected: full suite passes.

## Blast Radius

- `internal/ado/export.go`: shared work item output path, directory creation, per-item JSON writes, and index relative paths.
- `internal/ado/pullrequest.go`: shared PR output path and tests that assert the resulting path.
- `internal/ado/attachments.go`: no intended logic change, but attachment path assertions are affected by the changed bundle root.
- `internal/cli/ado_test.go`: mocked and real wiring tests assert printed paths and file locations.
- `openspec/specs/cli-command-surface/spec.md`: public behavior for export layout.
- `docs/adomi_azure_devops_handover.md`: handover output tree, `index.json` shape example, and CLI output contract.

## Pre-Mortem

1. The change failed because a hardcoded `work-items/` relative path remained in `newIndex` (`internal/ado/export.go:101-112`), causing `index.json` to point to files that no longer exist. Mitigation: make index path assertions part of the first verification checkpoint.
2. The change failed because CLI tests only checked stdout formatting and not real file locations. Mitigation: update the real wiring tests in `internal/cli/ado_test.go:1006-1027` and `internal/cli/ado_test.go:1091-1110` to assert new file paths.
3. The change failed because profile/project collisions unexpectedly mattered for a repository. Mitigation: explicitly document the Option A trade-off in design and keep source metadata in `index.json` for diagnosis.

## Open Questions

- None for this proposal. The accepted direction is Option A: flatten to `.adomi/context/<kind>/<id>/` and keep profile/project as metadata.

## Planning Verification

- [x] Every file/line reference was read directly by me.
- [x] I ran diagnostic commands myself for facts in the plan: `openspec list --json`, `grep`/`find` searches for output-path references, and `nl -ba` reads for referenced code/spec/test lines.
- [x] Each step has a verification checkpoint with concrete command and expected outcome.
- [x] I searched for existing patterns before proposing new ones.
- [x] I checked current filesystem state for counts, paths, and names.
- [x] Blast radius listed if shared code is touched, with usages traced.
- [x] Edge cases documented for every integration point and data transformation.
