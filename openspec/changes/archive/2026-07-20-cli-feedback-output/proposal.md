# Proposal: cli-feedback-output

## Why

The CLI gives almost no feedback on success or during execution. `ado login` / `ado logout` store and delete keyring credentials completely silently, and long multi-request operations (work item tree fetch, recursive wiki fetch, attachment downloads, PR bundle fetch) run with no progress indication. The primary consumers of this CLI are AI agents, which currently cannot confirm state changes or observe operation progress; humans get the same silence. Only failures produce useful output.

## What Changes

- **Auth confirmations**: `ado login` and `ado logout` report the credential ref and action taken (stored/deleted) on success.
- **Operation progress**: long-running fetch operations (work item tree fetch, wiki recursive fetch, attachment downloads, PR bundle fetch) emit line-based progress/status messages on stderr. Plain lines, not TTY spinners, so agents and logs can consume them in non-interactive contexts.
- **Result summaries**: fetch commands (`ado fetch`, `ado wiki fetch`, `ado pr fetch`) report human-readable summaries on stderr (e.g. counts of work items, attachments, wiki pages exported) in addition to the existing stdout path output.
- **Structured `--json` results**: extend the existing `writeJSONLine` convention to commands that lack it (`ado login`, `ado logout`, `ado fetch`, `ado wiki fetch`, `ado pr fetch`) so agents can parse results. stdout stays reserved for the single result value (ID, path, or JSON object); all feedback goes to stderr.

## Capabilities

### New Capabilities

- `cli-feedback`: Status, progress, and result summaries emitted on stderr; structured `--json` results for state-changing and fetch commands. Defines the feedback contract for both agent and human consumers.

### Modified Capabilities

- `cli-command-surface`: The `CLI stream behavior` requirement changes — stderr may now carry status, progress, confirmation, and summary lines in addition to prompts/help/errors. Scenarios that currently require stderr to be free of success output are refined to distinguish result *data* (stays on stdout only) from *status messages* (allowed on stderr).

## Impact

- **Code**: `internal/cli/ado_auth.go` (login/logout confirmations), `internal/cli/ado_fetch.go`, `ado_wiki.go`, `ado_pr.go` (progress, summaries, and `--json`), and `internal/ado/fetch.go`, `attachments.go`, `export.go`, `wiki_context.go`, `pullrequest.go` (progress hooks).
- **Docs**: `docs/azure-devops.md` (output and stream contract), `docs/getting-started.md` (stale-claim check), generated agent skill text in `internal/cli/agent.go`, command help texts.
- **Specs**: new `cli-feedback` capability spec; delta to `cli-command-surface` stream behavior scenarios.
- **Consumers**: stdout contracts are unchanged for existing non-JSON commands (additive only). stderr gains new output on success paths — agents/scripts that parse stderr may observe new lines.
- **No breaking changes** to exit codes or existing stdout data. New `--json` forms are additive; stderr gains documented success feedback.
