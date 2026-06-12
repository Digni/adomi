## Context

`adomi ado fetch <work-item-id>` already builds a `WorkItemTree` from the requested item, its parent chain, and direct children (`internal/ado/fetch.go:21`, `internal/ado/fetch.go:47`, `internal/ado/fetch.go:61`). The CLI wires the same Azure DevOps client into both tree fetching and context export (`internal/cli/ado.go:22`, `internal/cli/ado.go:62`, `internal/cli/ado.go:66`), and the default dependency wiring points fetch/export calls at the ADO package (`internal/cli/root.go:251`, `internal/cli/root.go:254`).

Attachment support is already present in the export layer: `ExportContext` accepts an `AttachmentDownloader`, iterates over `tree.WorkItems`, and calls `DownloadAttachments` when a downloader is provided (`internal/ado/export.go:44`, `internal/ado/export.go:64`, `internal/ado/export.go:65`). The index includes an `attachments/<id>` path for every work item (`internal/ado/export.go:93`, `internal/ado/export.go:102`). `DownloadAttachments` writes files below the work item's attachment directory and returns download/write errors with work item context (`internal/ado/attachments.go:39`, `internal/ado/attachments.go:47`, `internal/ado/attachments.go:56`, `internal/ado/attachments.go:62`). The HTTP client downloads attachment URLs with authorization, same-host validation, and the existing 64 MiB size cap (`internal/ado/client.go:395`, `internal/ado/client.go:415`, `internal/ado/client.go:418`, `internal/ado/client.go:538`).

The existing test suite already covers real CLI wiring that downloads root and child attachments (`internal/cli/ado_test.go:2285`, `internal/cli/ado_test.go:2362`, `internal/cli/ado_test.go:2363`), duplicate filename handling (`internal/ado/attachments_test.go:126`), and nil-downloader export behavior (`internal/ado/export_test.go:158`). `go test ./...` passes on the current tree. Obsidian lookup found `1.Projects/adomi/1. Daily Work/2026/05/2026-05-08.md`, which documents the prior flatten-output-folder change and confirms the current `.adomi/context/work-items/<id>/` export shape; no conflicting attachment-specific project note was found.

## Goals / Non-Goals

**Goals:**

- Make attachment downloads an explicit `cli-command-surface` requirement for work item fetch exports.
- Harden implementation tests for attachment downloads across every exported work item category: requested item, parent chain, and direct children.
- Preserve current export layout and stdout contract.
- Preserve existing download safety checks: credentials, host validation, and attachment size limit.

**Non-Goals:**

- Add new CLI flags to enable/disable attachment downloads.
- Change pull request context export behavior.
- Change Azure DevOps API versioning, credential storage, or proxy behavior.
- Stream large attachments or introduce resumable/download-cache behavior.
- Guarantee cleanup of partial attachment files after a failed attachment download; failed exports may leave files downloaded earlier in the same attempt under the freshly recreated export directory.

## Decisions

1. **Keep attachment download in `ExportContext`, not `FetchTree`.**
   - Rationale: `FetchTree` is responsible for discovering work items (`internal/ado/fetch.go:21`) while `ExportContext` owns filesystem output (`internal/ado/export.go:44`) and already has the export root needed for `attachments/<id>` paths.
   - Alternative considered: fetch attachment bytes while walking the work item tree. Rejected because it would mix discovery and filesystem/export concerns and make non-export uses of `FetchTree` heavier.

2. **Use the full exported tree as the attachment download source.**
   - Rationale: `ExportContext` iterates over `tree.WorkItems` (`internal/ado/export.go:64`), which is the same collection used for JSON/HTML/index output. This keeps attachments aligned with every exported item, including parent-chain and direct-child work items.
   - Alternative considered: download attachments only for the requested root work item. Rejected because child and parent context can contain critical referenced files and the user requested attachments when getting WorkItems generally.

3. **Keep strict attachment failures as fetch failures, while accepting partial attachment leftovers.**
   - Rationale: `DownloadAttachments` returns errors for download/write failures (`internal/ado/attachments.go:56`, `internal/ado/attachments.go:62`) and `runADOFetch` only prints stdout after export succeeds (`internal/cli/ado.go:66`). `ExportContext` downloads attachments before writing final item, HTML, tree, and index artifacts, so a failed attachment prevents successful metadata from being written. However, `DownloadAttachments` writes file-by-file, so if a later attachment fails, earlier files from the same failed export attempt can remain in `attachments/<id>/`.
   - Alternative considered: delete partial attachment files on every failure. Rejected for this change because it would add filesystem rollback complexity beyond making the existing behavior explicit and test-hardened. A future change can introduce all-or-cleanup semantics if partial failed-export directories become a problem.

4. **Continue using `Client.Download` safety boundaries.**
   - Rationale: the client validates attachment URLs against the configured Azure DevOps host (`internal/ado/client.go:538`) and enforces the existing max attachment size (`internal/ado/client.go:415`, `internal/ado/client.go:418`).
   - Alternative considered: accept any attachment URL from the work item relation. Rejected because relation data is external input and same-host validation reduces credential exfiltration risk.

## Risks / Trade-offs

- **Attachment downloads increase fetch latency and bandwidth** → keep behavior scoped to existing fetch exports and rely on the current per-attachment size cap.
- **One failed attachment blocks the successful export** → preserve trustworthy success output and final metadata; document that earlier attachment files from the failed attempt may remain.
- **Spec may lag existing implementation because much of the behavior already exists** → implementation should first add/adjust tests for the explicit parent-chain/direct-child attachment requirements, then only change code if a gap appears.
- **Azure DevOps may return attachment relations with unexpected names or URLs** → keep filename sanitization and same-host URL validation in the attachment/client layers.

## Migration Plan

No user-facing migration is required. The export directory remains `.adomi/context/work-items/<work-item-id>/`, stdout remains the exported path only, and attachment files remain under `attachments/<work-item-id>/`.

Rollback is straightforward: revert any code/test/spec changes from this change. No persisted schema migration is introduced.

## Open Questions

None.

## Planning Verification

- [x] Every file/line reference was read directly by me (not from subagent summary alone).
- [x] I ran diagnostic commands myself for facts in the plan: `go test ./...` passed.
- [x] Each implementation step in `tasks.md` will include a verification checkpoint with concrete command and expected outcome.
- [x] I searched for existing patterns before proposing new ones: existing work item fetch/export/attachment code and specs were inspected.
- [x] I checked current filesystem state for counts, paths, and names: existing OpenSpec specs and change directories were listed before creating this change.
- [x] I checked relevant Obsidian project notes/Granola meeting summaries: searched `adomi attachment`, `adomi work item attachment`, and `Azure DevOps attachments adomi`; read `1.Projects/adomi/1. Daily Work/2026/05/2026-05-08.md`.
- [x] Blast radius listed if shared code is touched: CLI fetch flow, ADO export helpers, attachment downloader, client download safety, and tests.
- [x] Edge cases documented for every integration point and data transformation: download failure, write failure, unsafe URLs, oversized attachments, missing/no attachment relations, duplicate/unsafe filenames, and partial attachment leftovers on failed export attempts.

## Pre-Mortem

1. **Production failure: a parent-chain attachment was not downloaded.** Likely cause: tests only asserted root/direct-child attachments while `FetchTree` parent traversal (`internal/ado/fetch.go:47`) was not covered by an attachment fixture. Mitigation: add a real-wiring or export test fixture that includes root, parent, and child attachment relations.
2. **Production failure: fetch succeeds but silently omits attachments after an Azure DevOps error.** Likely cause: implementation swallows `DownloadAttachments` errors around `internal/ado/export.go:65`. Mitigation: assert fetch/export returns an error and keeps stdout empty when `AttachmentDownloader.Download` fails.
3. **Production failure: malicious or cross-host attachment URL receives credentials.** Likely cause: bypassing `Client.Download` host validation (`internal/ado/client.go:538`) or adding a downloader that does not validate. Mitigation: keep CLI wiring passing the existing client as the downloader and preserve client download tests.
