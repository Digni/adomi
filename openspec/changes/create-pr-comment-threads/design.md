## Context

Adomi already has a narrow Azure DevOps PR maintenance surface. The current `adomi ado pr` dispatcher recognizes `fetch`, `ensure`, `reply`, `resolve`, and `reopen` in `internal/cli/ado.go:79-94`, and the Cobra help text mirrors that list in `internal/cli/root.go:146-149`. Reply/status commands already share a maintenance path that loads config/credentials and resolves the PR repository ID before writing to Azure DevOps (`internal/cli/ado.go:222-264`, `internal/cli/ado.go:266-294`).

The domain types already model PR threads, comments, thread context, and file positions (`internal/ado/pullrequest.go:38-71`), but the maintainer interface currently exposes only reply-to-existing-thread and update-thread-status writes (`internal/ado/pullrequest.go:125-130`). The HTTP client already has the PR thread collection URL needed for creating a thread (`internal/ado/client.go:410-417`) and a separate thread-comments URL used by replies (`internal/ado/client.go:430-438`).

Azure DevOps represents a top-level PR comment as a new pull request thread whose first comment contains the text. Inline file comments are also new threads, but they introduce file positions and pull request iteration/change tracking context.

## Goals / Non-Goals

**Goals:**

- Phase 1: add a PR-level thread creation command for standalone top-level PR comments.
- Print the created thread ID on plain stdout, because it is the identifier needed by existing `reply`, `resolve`, and `reopen` commands.
- Include both `threadId` and the initial `commentId` in JSON output.
- Reuse existing message-source behavior from `reply`: exactly one of `--message` or `--message-file`, non-blank content, and validation before credential loading.
- Reuse existing PR maintenance safety boundaries: fetch the PR to resolve repository ID, keep stdout data-only, and avoid governance actions.

**Non-Goals:**

- Phase 2 inline file/line review threads are explicitly out of scope for this change.
- No support for approval, rejection, merge/complete, abandon, auto-complete, policy bypass, or reviewer management.
- No generalized Azure DevOps thread editing/deletion API.

## Decisions

### Decision 1: expose a first-class `comment` verb for Phase 1

Use:

```bash
adomi ado pr comment <pull-request-id> --message "text"
adomi ado pr comment <pull-request-id> --message-file comment.md
```

Rationale: the existing PR namespace is verb-oriented (`ensure`, `reply`, `resolve`, `reopen`) as shown by the dispatcher in `internal/cli/ado.go:79-94`. `comment` is user-facing language for creating a top-level PR comment while the specs/design can be precise that this creates a new PR-level thread.

Alternative considered: `thread create`. This is closer to Azure DevOps terminology, but it would introduce a nested command shape unlike the existing flat PR maintenance verbs.

### Decision 2: create only PR-level threads in Phase 1

The request body should contain one text comment and active status, without `threadContext` or `pullRequestThreadContext`:

```json
{
  "comments": [
    { "parentCommentId": 0, "content": "...", "commentType": "text" }
  ],
  "status": "active"
}
```

Rationale: the current code already has a PR thread collection URL (`internal/ado/client.go:410-417`) and response types for `PullRequestThread` and comments (`internal/ado/pullrequest.go:38-71`). Avoiding inline context keeps Phase 1 aligned with the existing conservative maintenance boundary and avoids unresolved Azure DevOps iteration/change tracking behavior.

Alternative considered: implement inline comments immediately with `--file` and `--line`. Deferred to Phase 2 because robust inline comments need decisions about right/left side, line ranges, latest iteration, and `changeTrackingId`.

### Decision 3: plain stdout prints thread ID, JSON prints thread and comment IDs

Plain success output for `comment` should be:

```text
147
```

where `147` is the created thread ID. JSON should include:

```json
{"pullRequestId":42,"threadId":147,"commentId":1,"action":"commented"}
```

Rationale: `reply` currently prints a created comment ID (`internal/cli/ado.go:258-262`), while `resolve`/`reopen` print the thread ID (`internal/cli/ado.go:289-293`). For new thread creation, the thread ID is the useful handle for subsequent thread maintenance.

Alternative considered: print the initial comment ID to mirror `reply`; rejected because it is less useful for follow-up maintenance commands.

### Decision 4: reuse and generalize existing message parsing patterns

`parsePRThreadArgs` currently handles `--message`, `--message-file`, `--profile`, `--global`, and `--json` for commands that operate on an existing thread (`internal/cli/ado.go:844-912`). Phase 1 should either introduce a sibling parser for PR-level comments or factor reusable message-source parsing without changing existing validation semantics.

Rationale: invalid arguments and empty messages should still fail before credentials or network writes, matching existing `reply` behavior.

Alternative considered: accept stdin for message content. Deferred because the existing PR maintenance UX uses inline/file message sources only.

## Risks / Trade-offs

- **Risk: the Azure DevOps response omits either thread ID or initial comment ID.** → Treat missing thread ID as an error because plain stdout depends on it; JSON can only include `commentId` when the first returned comment has a positive ID.
- **Risk: shared parsing changes break `reply`, `resolve`, or `reopen`.** → Prefer additive parser helpers with tests for existing commands; run targeted CLI tests after each parser change.
- **Risk: users expect `comment` to support file/line review comments immediately.** → Document Phase 1 as PR-level only, reject inline context flags as unsupported before credentials/network writes, and keep Phase 2 inline comments explicit in proposal/design.
- **Risk: help text or agent skill docs drift from the actual command surface.** → Update help tests and generated skill/docs references alongside CLI changes.

## Phase 2 Sketch: Inline File/Line Threads

Phase 2 can add optional inline parameters after Phase 1 is stable. In Phase 1 these inline context flags are intentionally unsupported and should be rejected rather than silently ignored:

```bash
adomi ado pr comment <pull-request-id> \
  --file /cmd/adomi/main.go \
  --line 123 \
  --message-file comment.md
```

Before implementing Phase 2, investigate Azure DevOps iteration APIs and changed-file metadata so inline threads can supply correct `threadContext` and, when required, `pullRequestThreadContext.changeTrackingId` and iteration context.

## Planning Verification

- [x] Every file/line reference was read directly by me.
- [x] I ran diagnostic commands myself for facts in the plan (`openspec list --json`, `find openspec -maxdepth 3 -type f`, `rg`/`nl` for command and client patterns).
- [x] Each implementation chunk in `tasks.md` has a verification checkpoint with concrete commands and expected outcomes.
- [x] I searched for existing patterns before proposing new ones (`runADOPullRequestReply`, `parsePRThreadArgs`, client URL helpers, active specs/docs).
- [x] I checked current filesystem state for specs and existing changes.
- [x] Blast radius is listed in the proposal impact section and tasks.
- [x] Edge cases are documented in specs and risks.

## Pre-Mortem

1. The plan could fail because the created thread response shape differs from existing `PullRequestThread` assumptions in `internal/ado/pullrequest.go:38-71`, especially if `comments` is empty or the initial comment ID is absent. The design now requires treating missing thread ID as an error and making comment ID conditional/validated in tests.
2. The plan could break existing commands if shared message parsing changes alter `reply` validation in `internal/cli/ado.go:844-912`. The tasks therefore call for preserving existing tests and adding focused regression coverage.
3. The plan could overreach into inline review comments without the necessary iteration/change-tracking context. The design explicitly constrains this change to PR-level threads and leaves inline comments to Phase 2.
