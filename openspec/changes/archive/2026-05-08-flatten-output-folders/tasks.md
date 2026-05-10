## 1. Work Item Export Layout

- [x] 1.1 Update `OutputPath` in `internal/ado/export.go` to return `.adomi/context/work-items/<root-id>` without profile/project path segments.
- [x] 1.2 Rename the internal per-work-item JSON directory from `work-items/` to `items/` in directory creation, item writes, and `IndexItem.Path` generation.
- [x] 1.3 Remove the now-obsolete profile/project path sanitization behavior from work item output paths, including `TestOutputPathSanitizesProfileAndProjectSegments`; remove `sanitizePathSegment` if no remaining production code uses it.
- [x] 1.4 Update work item export tests in `internal/ado/export_test.go` to assert the new bundle root, `items/<id>.json` files, `index.WorkItems[0].Path == "items/<id>.json"`, and nil-downloader behavior in `TestExportContextAllowsNilDownloader`.
- [x] 1.5 Verification checkpoint: run `go test ./internal/ado -run 'TestExport|TestOutputPath|TestAttachment'`; expected: targeted work item export and attachment tests pass.

## 2. Pull Request Export Layout

- [x] 2.1 Update `PullRequestOutputPath` in `internal/ado/pullrequest.go` to return `.adomi/context/pull-requests/<pull-request-id>` without profile/project path segments.
- [x] 2.2 Remove the now-obsolete profile/project path sanitization behavior from PR output paths, including `TestPullRequestOutputPathSanitizesProfileAndProject`; remove `sanitizePathSegment` if no remaining production code uses it after work item and PR path updates.
- [x] 2.3 Update pull request export tests in `internal/ado/pullrequest_test.go` to assert the new bundle root while preserving existing bundle files and thread relative paths.
- [x] 2.4 Verification checkpoint: run `go test ./internal/ado -run 'TestPullRequestOutputPath|TestExportPullRequest'`; expected: targeted PR output path and export tests pass.

## 3. CLI Wiring And Integration Tests

- [x] 3.1 Update mocked CLI tests in `internal/cli/ado_test.go` so successful fetch and PR commands print only the new `.adomi/context/...` paths, including the global-scope stub return paths in `TestADOFetchGlobalUsesGlobalConfigWithoutRepoConfig` and `TestADOPullRequestGlobalUsesGlobalConfigWithoutRepoConfig`.
- [x] 3.2 Update real wiring tests in `internal/cli/ado_test.go` to assert generated files under `.adomi/context/work-items/<id>/` and `.adomi/context/pull-requests/<id>/`, including work item JSON under `items/`.
- [x] 3.3 Verification checkpoint: run `go test ./internal/cli`; expected: CLI tests pass and stdout remains path-only.

## 4. Documentation And Spec Alignment

- [x] 4.1 Update the current canonical `openspec/specs/cli-command-surface/spec.md` requirement for Azure DevOps commands so work item and PR export scenarios match this change's delta spec.
- [x] 4.2 Update `docs/adomi_azure_devops_handover.md` output layout, `index.json` shape example, and CLI output contract examples to use `.adomi/context/<kind>/<id>/` and `items/<id>.json`.
- [x] 4.3 Review existing docs/plans references to the old layout and update only active/user-facing guidance needed for this change; archived historical plans may remain historical unless they are used as current handover guidance.
- [x] 4.4 Confirm `README.md` needs no output-layout update and `.gitignore` already ignores all `.adomi/` content.
- [x] 4.5 Verification checkpoint: run `openspec validate flatten-output-folders --strict`; expected: OpenSpec validation passes.

## 5. Final Verification

- [x] 5.1 Run `gofmt -w internal cmd`; expected: no output and Go files are formatted.
- [x] 5.2 Run `go test ./...`; expected: full test suite passes.
