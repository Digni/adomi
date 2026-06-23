## Context

Adomi's work item surface is currently read-oriented. `adomi ado fetch <work-item-id>` is wired as a direct `ado` subcommand in `internal/cli/root.go:113-141`, and its runner loads repo/global config, reads the PAT, builds the ADO client, fetches the work item tree, exports context, and prints only the export path (`internal/cli/ado.go:22-76`). Work item ID validation already exists for fetch (`internal/cli/ado.go:822-852`).

The CLI already has conservative write patterns for pull requests: the `ado pr` dispatcher recognizes explicit verbs (`internal/cli/ado.go:80-98`), PR comments validate message input and load message files before credentials/network calls (`internal/cli/ado.go:225-241`), JSON output is emitted through a single-line helper (`internal/cli/ado.go:1235-1241`), and parser rules enforce exactly one `--message` or `--message-file` (`internal/cli/ado.go:1013-1078`, `internal/cli/ado.go:1080-1148`). File-backed messages are already rejected when empty or blank (`internal/cli/ado.go:1210-1232`).

The ADO client already centralizes auth, request errors, JSON POST/PATCH handling, and URL construction. Work item fetch uses `GET .../_apis/wit/workitems/{id}?$expand=all&api-version={profile-api-version}` (`internal/ado/client.go:74-100`, `internal/ado/client.go:436-444`), while PR write methods use `doJSON` for authenticated JSON requests and shared non-2xx/decode handling (`internal/ado/client.go:301-392`). The current `ADOClient` interface bundles work item fetching/attachments and PR maintenance (`internal/cli/root.go:35-40`), so adding work item comments will require an additive interface method and corresponding fake-client test updates.

Microsoft documents the Work Item Tracking comment creation API as `POST https://dev.azure.com/{organization}/{project}/_apis/wit/workItems/{workItemId}/comments?api-version=7.0-preview.3` with request body `{ "text": "..." }`, a Comment response, and `vso.work_write` permission. The fetched documentation's sample response uses `commentId`, while the response definition also lists `id`, so the client must normalize either positive field as the created comment ID.

Relevant Obsidian project memory was checked: `1.Projects/adomi/1. Daily Work/2026/06/2026-06-01.md` records the earlier PR comment OpenSpec/implementation and its data-only stdout pattern, and `1.Projects/adomi/1. Daily Work/2026/06/2026-06-02.md` records the later inline PR comment extension. The user chose the command-shape decision for this proposal: canonical flat command plus a work-item namespace alias.

## Goals / Non-Goals

**Goals:**

- Add a narrow work item comment creation capability through `adomi ado comment <work-item-id>` and alias `adomi ado work-item comment <work-item-id>`.
- Reuse the existing profile/global/PAT/proxy client setup used by work item fetch.
- Reuse existing PR message-source semantics: exactly one `--message` or `--message-file`, non-blank content, validation before credentials or network writes.
- Print only the created comment ID on plain stdout and one compact JSON object when `--json` is supplied.
- Validate enough of the Azure DevOps response to avoid reporting a successful comment without a usable comment ID.

**Non-Goals:**

- No work item field updates, state transitions, relation changes, assignment changes, attachment upload, comment update, comment delete, or comment reaction support.
- No generalized `work-item` command namespace beyond the requested `work-item comment` alias.
- No fetch/export behavior changes.
- No interactive editor/stdin message source; keep the existing explicit inline/file message UX.
- No idempotency or duplicate-comment suppression; repeating the command intentionally sends another Azure DevOps comment.

## Decisions

### Decision 1: canonical flat command with a namespace alias

Use:

```bash
adomi ado comment <work-item-id> --message "text"
adomi ado comment <work-item-id> --message-file comment.md
adomi ado work-item comment <work-item-id> --message "text"
```

Rationale: `adomi ado fetch <work-item-id>` is already flat under the ADO namespace (`internal/cli/root.go:133-141`), while PR-specific operations are namespaced under `adomi ado pr` (`internal/cli/root.go:144-159`). The user explicitly selected flat canonical command plus alias. The alias gives discoverability for users who expect a work-item noun without forcing an immediate broader namespace redesign.

Alternative considered: only `adomi ado work-item comment`. Rejected because it diverges from the existing flat work item fetch command and requires more CLI surface to document for a single operation.

### Decision 2: add a work item maintenance client method instead of prefetching work items

Add focused types such as `WorkItemCommentCreateOptions` and `WorkItemComment`, plus a `CreateWorkItemComment(ctx, opts)` method on a new or existing work item maintenance interface. The runner should create the client the same way as fetch (`internal/cli/ado.go:28-56`) and post directly to the comment endpoint.

Rationale: the comment API itself validates the work item exists and that the PAT can write to it. Prefetching the work item would add an extra read request and would not remove the need to handle comment POST errors. The current client already returns rich non-2xx error context via `responseError` (`internal/ado/client.go:549-562`).

Alternative considered: call `FetchWorkItem` first to verify existence. Rejected as redundant and slower; tests can still validate that invalid CLI IDs fail before client creation.

### Decision 3: use the documented preview API version for comment creation

Build the comment URL from the existing base URL and project, but use `api-version=7.0-preview.3` for this endpoint unless later implementation research proves Azure DevOps accepts the normal profile API version. Keep the route construction consistent with the existing lowercase `workitems` helper path in `internal/ado/client.go:436-444`; Azure DevOps routes are case-insensitive, but tests should assert the project/base path and comments suffix rather than introducing a second casing convention.

Rationale: the fetched Microsoft Learn page for Comments - Add says the API version parameter should be `7.0-preview.3`. Existing profiles default to `7.1` (`internal/config/config.go:15`, `internal/config/config.go:80-83`), but applying that default to this endpoint would contradict the endpoint documentation.

Alternative considered: always use the configured `profile.APIVersion`. Rejected for the proposal because the documented endpoint is preview-specific; if a server rejects the preview endpoint, the normal Azure DevOps error path should report that failure without printing stdout success data.

### Decision 4: normalize response ID fields defensively

Model both `id` and `commentId`, then expose a normalized created comment ID. Treat the POST as failed if neither field is positive. If `workItemId` is present and non-zero, require it to match the requested work item ID; if it is absent/zero, do not fail solely for that omission.

Rationale: the Microsoft sample response uses `commentId`, while the definition lists `id`. Plain stdout needs one stable numeric ID, and existing PR write methods already validate response IDs before returning success (`internal/ado/client.go:326-329`, `internal/ado/client.go:344-347`, `internal/ado/client.go:359-365`).

Alternative considered: trust only `id`. Rejected because it would fail against the documented sample shape.

### Decision 5: keep stdout semantics data-only and command-specific

Plain success output should be the created work item comment ID followed by one newline. JSON should be a single compact line like:

```json
{"workItemId":12345,"commentId":50,"action":"commented","url":"https://dev.azure.com/..."}
```

Rationale: existing fetch output is only the exported path (`internal/cli/ado.go:76`), PR comment output is only the created thread ID (`internal/cli/ado.go:295`), PR reply output is only the created comment ID (`internal/cli/ado.go:339`), and shared JSON output is a single `json.Marshal` line (`internal/cli/ado.go:1235-1241`).

Alternative considered: print the full Azure DevOps URL by default. Rejected because it would make shell usage harder and diverge from current data-only numeric IDs for write commands.

## Risks / Trade-offs

- **Risk: Azure DevOps Server/on-prem versions differ in work item comments API support.** → Use the documented endpoint and let non-2xx Azure DevOps responses surface as errors with stdout empty; avoid adding config complexity until a concrete incompatible server is found.
- **Risk: response shape differs between `id`, `commentId`, and `workItemId`.** → Normalize `id`/`commentId`, validate a positive normalized ID, and validate `workItemId` only when present.
- **Risk: shared parser refactoring breaks existing PR comment/reply commands.** → Prefer additive helpers or targeted extraction, keep existing test cases green, and run PR comment/reply parser tests alongside new work item comment tests.
- **Risk: the alias suggests a broader `work-item` namespace exists.** → Document only `work-item comment` as an alias in this change and reject unsupported `work-item` subcommands before credentials/network writes.
- **Risk: PAT scopes are insufficient.** → Document that work item comments require work item write permission (`vso.work_write` for OAuth-equivalent scope); Azure DevOps authorization failures remain non-zero with stdout empty.
- **Risk: agents retry and create duplicate comments.** → Treat POST as non-idempotent, document that re-running the command creates another comment, and avoid automatic retry loops unless the caller can prove the previous attempt failed before Azure DevOps accepted it.

## Edge Cases

- CLI parsing: missing ID, non-integer/non-positive ID, missing message source, both message sources, blank inline message, missing/empty/blank message file, missing `--profile` value, unsupported alias subcommands, and unknown flags all fail before credential lookup or network writes.
- Azure DevOps integration: network errors, non-2xx responses, oversized comment rejections, malformed JSON, empty response bodies, missing comment ID, and mismatched positive `workItemId` all fail without stdout success data. Oversized messages are not prevalidated locally; rely on Azure DevOps returning a non-2xx validation error.
- Data transformation: message file content is sent exactly as read after non-blank validation; inline message text is sent exactly as provided after trim-only blank validation; CRLF, Unicode, and trailing newlines are preserved with no normalization or trimming beyond blank checks; Azure DevOps is responsible for rendering the comment text/markdown. JSON output uses the normalized positive comment ID rather than leaking both raw response ID fields.
- Shared paths: existing fetch, PR fetch, PR comment, PR reply, PR resolve/reopen, login/logout, and profiles commands must keep current stdout/stderr behavior.

## Migration Plan

No migration is required. This is an additive CLI/API capability. Rollback is removing the new command/alias and client method before release; no local files or persistent schema are changed.

## Open Questions

None for the proposal. If implementation discovers Azure DevOps Server versions that require a different comment API version, stop and update this design/spec before working around it.

## Planning Verification

- [x] Every file/line reference was read directly by me.
- [x] I ran diagnostic commands myself for facts in the plan (`openspec list --json`, `openspec status --json`, `git status --short`, targeted code reads/greps, and Microsoft Learn lookup for the work item comment endpoint).
- [x] Each implementation chunk in `tasks.md` has a verification checkpoint with concrete command and expected outcome.
- [x] I searched for existing patterns before proposing new ones (`runADOFetch`, PR comment/reply parsing/output, client URL/JSON helpers, active OpenSpec specs, agent guidance).
- [x] I checked current filesystem state for specs, archived changes, and active changes.
- [x] I checked relevant Obsidian project notes for prior PR comment decisions and recorded the source paths in Context.
- [x] Blast radius is listed in the proposal Impact section and tasks.
- [x] Edge cases are documented above and in the spec scenarios.

## Pre-Mortem

1. The plan could fail if Azure DevOps returns only `commentId` while the implementation validates only `id`; Decision 4 now requires normalizing both fields.
2. The plan could fail for on-prem Azure DevOps if `7.0-preview.3` is not supported; Decision 3 makes this explicit and requires updating the design/spec instead of silently switching API versions during apply.
3. The plan could break existing PR parsing/output if message-source helpers are changed globally; the tasks require targeted PR regression tests alongside work item comment tests.
