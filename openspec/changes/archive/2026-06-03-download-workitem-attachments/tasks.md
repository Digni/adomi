## 1. Test Coverage

- [x] 1.1 Extend `TestADOFetchWithRealWiringWritesContextAndPrintsOnlyPath` or add an equivalent CLI integration test so the exported tree includes requested, parent-chain, and direct-child work items with attachments; verify files are written under `attachments/<exported-work-item-id>/` for all three categories. Checkpoint: `go test ./internal/cli -run TestADOFetchWithRealWiringWritesContextAndPrintsOnlyPath` passes and fails if the parent attachment assertion is removed.
- [x] 1.2 Add or update export/index assertions so every exported work item has the exact relative POSIX `attachments/<id>` path in `index.json` with no trailing slash, including work items without attachment relations. Checkpoint: `go test ./internal/ado -run 'TestExportContext|TestOutputPath'` passes.
- [x] 1.3 Add a CLI failure-path test, for example `TestADOFetchAttachmentDownloadFailureLeavesStdoutEmpty`, proving an attachment download error during `adomi ado fetch` makes the command fail and leaves stdout empty. Checkpoint: the relevant `go test ./internal/cli -run 'TestADOFetch.*Attachment.*Failure|TestADOFetch.*LeavesStdoutEmpty'` failure-path test passes.
- [x] 1.4 Add or update export-order/failure assertions proving attachment downloads complete before final item, HTML, tree, and index artifacts are written for a successful fetch; on attachment download failure, successful metadata artifacts are not written, while partial attachment files from earlier downloads in the same failed attempt are acceptable. Checkpoint: `go test ./internal/ado -run 'TestExportContext|TestDownloadAttachments'` passes.

## 2. Implementation Alignment

- [x] 2.1 Verify `runADOFetch` passes the authenticated Azure DevOps client as both `WorkItemFetcher` and `AttachmentDownloader`; adjust wiring only if the new tests reveal a gap. Checkpoint: `go test ./internal/cli -run TestADOFetchPrintsOnlyExportedPath` passes.
- [x] 2.2 Verify `ExportContext` downloads attachments for every `tree.WorkItems` entry before writing final success metadata and keeps no-attachment work items exportable; adjust only if the new tests reveal a gap. Checkpoint: `go test ./internal/ado -run 'TestExportContext|TestDownloadAttachments'` passes.
- [x] 2.3 Preserve `Client.Download` host validation, authorization, and size-limit behavior while satisfying the explicit attachment requirements. Checkpoint: `go test ./internal/ado -run 'TestClient.*Download|TestDownloadAttachments'` passes.

## 3. Specification and Full Verification

- [x] 3.1 Validate the OpenSpec change. Checkpoint: `openspec validate download-workitem-attachments --strict` exits 0.
- [x] 3.2 Run focused package tests. Checkpoint: `go test ./internal/ado ./internal/cli` exits 0.
- [x] 3.3 Run the full repository test suite. Checkpoint: `go test ./...` exits 0.
