# Design: cli-feedback-output

## Context

The CLI's consumers are primarily AI agents, secondarily humans. Today, `ado login` and `ado logout` complete silently, and long multi-request fetches emit nothing until they return. stdout is already a stable result-data stream: paths and IDs in plain mode, or one JSON value for commands that support `--json`. Commands parse their own arguments because they use `DisableFlagParsing: true`.

The useful change is bounded feedback at the operation boundary. A global HTTP request logger would require root-level argument pre-scanning and transport-specific wrapping, including special handling for pipeline loopback proxy safety. That complexity is deliberately excluded from this change.

## Goals / Non-Goals

**Goals:**

- Confirm successful credential storage and deletion.
- Emit plain, line-based progress during multi-request fetches.
- Report each attachment immediately after its file is written.
- Preserve existing non-JSON stdout byte-for-byte.
- Add structured `--json` results to login, logout, and fetch commands.
- Keep external text from forging extra feedback lines.

**Non-Goals:**

- No global `--verbose` flag or HTTP request logging.
- No transport wrapper or transport-error formatting changes.
- No spinners, colors, progress bars, TTY detection, or log-level system.
- No `--quiet` flag.

## Decisions

### D1: Feedback uses stderr; stdout remains result data

Status, progress, confirmations, and summaries go to stderr. stdout continues to carry exactly one result value: the existing path or ID, or one compact object when `--json` is passed.

This preserves shell path capture such as `CONTEXT=$(adomi ado fetch 12345)`. Scripts that display stderr will see new success feedback; this is intentional and documented.

### D2: Progress callbacks stay inside the ADO operation boundary

`ado.FetchTree`, `ExportContext`, `FetchWikiContext`, and `FetchPullRequestBundle` accept an optional `ProgressFunc`; nil remains silent. The CLI supplies a callback that writes escaped messages to stderr.

Attachment progress is emitted by `DownloadAttachments` immediately after `os.WriteFile` succeeds. It is not reconstructed from a completed attachment batch in `ExportContext`. Therefore a completed attachment remains observable even if the next download is slow or fails.

The callback reports completion, not merely download start, so the fetch attachment count remains the number of successfully written files.

### D3: Extend the existing `--json` convention

The new result objects are:

- login: `{"action":"stored","credentialRef":"..."}`
- logout: `{"action":"deleted","credentialRef":"..."}`
- work item fetch: `{"path":"...","workItems":12,"attachments":4}`
- wiki fetch: `{"path":"...","pages":5}`
- pull request fetch: `{"path":"...","threadCount":3,"commentCount":11}`

Pull-request counts match the exported index: all threads count toward `threadCount`; non-deleted comments across all threads count toward `commentCount`.

### D4: Feedback records are single physical lines

The shared CLI feedback writer escapes control characters in external values before writing a newline. Progress producers pass plain messages; the writer treats callback messages as data, not format strings.

### D5: Plain feedback remains deliberately small

Confirmation and summary messages are short imperative lines without prefixes or timestamps. Fetch progress is one line per fetched item, page, attachment, or pull-request bundle step. There is no HTTP request-level output.

## Risks / Trade-offs

- [Scripts observe new stderr lines] → stdout remains unchanged and the new stderr contract is documented in command help, user docs, and the generated agent skill.
- [A later attachment fails] → progress is emitted after each successful write, so completed files remain visible before the final error.
- [External text contains controls] → the single feedback boundary escapes controls and guarantees one physical line per event.
- [Callback signatures affect internal callers] → the package is internal, callbacks are nil-safe, and all callers are covered by the Go test suite.

## Migration Plan

Purely additive CLI behavior with no persisted-state migration. Rollback is a code revert. Existing non-JSON stdout and exit-code contracts remain unchanged.

## Open Questions

- Should progress become opt-in if consumers report that default stderr is too noisy? Not in this change; usage feedback should drive that decision.
