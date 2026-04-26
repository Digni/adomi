# Review Warning Fixes

Date: 2026-04-26

## Goal

Fix the accepted review warnings from the Azure DevOps context fetcher implementation:

- sanitize or escape Azure DevOps work item descriptions in generated HTML;
- avoid echoing PAT input in interactive terminals;
- prevent unbounded attachment buffering;
- harden attachment filename sanitization;
- add focused tests for these behaviors plus the high-value coverage gaps from review.

The export-directory deletion and concurrent same-output warnings are intentionally out of scope because this tool is intended for agent-run, worktree-local disposable output.

## Verified Context

- `internal/ado/export.go:138` renders generated HTML and currently injects `item.Description()` directly into the body at `internal/ado/export.go:144`.
- `internal/ado/export_test.go:101` currently expects raw description HTML to be preserved, so this test must change before production code changes.
- `internal/cli/ado.go:96` prompts for the PAT and `internal/cli/ado.go:97` reads it through `bufio.NewReader(stdin)`, which echoes in a real terminal.
- `internal/cli/root.go:37` already centralizes injectable dependencies, so PAT reading can be added there without coupling `runADOLogin` to global terminal state.
- `internal/ado/client.go:99` exposes `Download(ctx, rawURL) ([]byte, error)`, and `internal/ado/client.go:119` uses `io.ReadAll`.
- `internal/ado/attachments.go:14` defines the attachment downloader interface, and `internal/ado/attachments.go:51` expects a byte slice from `Download`.
- `internal/ado/attachments.go:98` sanitizes only whitespace, `/`, `\`, and leading/trailing dots/spaces.
- `internal/ado/client_test.go:142` already tests same-host attachment URL rejection.
- Official Azure DevOps docs list attachment downloads under `https://dev.azure.com/{organization}/{project}/_apis/wit/attachments/{id}` and attachment create responses return same-host `https://dev.azure.com/fabrikam/_apis/wit/attachments/{id}?fileName=...` URLs, so the current same-host restriction matches the documented REST shape.

## Plan

1. HTML description safety
   - Red: update/add `internal/ado/export_test.go` coverage so a description containing rich text plus script/event-handler markup is rendered as escaped text, not executable markup.
   - Green: change `renderHTML` in `internal/ado/export.go` to escape `item.Description()` with `html.EscapeString`.
   - Verification: `go test ./internal/ado` should fail before the change and pass after it.

2. Attachment download memory bound
   - Red: add `internal/ado/client_test.go` coverage for an attachment response over the configured max size returning an error.
   - Green: add a package constant max attachment bytes and read through `io.LimitReader`/limit detection instead of raw `io.ReadAll`.
   - Keep the public `Download(ctx, rawURL) ([]byte, error)` shape for now to avoid broad interface churn; the documented cloud default is 60 MB per attachment, so use a conservative cap that covers documented cloud attachments while preventing unlimited memory growth.
   - Verification: `go test ./internal/ado` should fail before the change and pass after it.

3. Filename sanitization
   - Red: add `internal/ado/attachments_test.go` coverage for NUL/control characters, reserved Windows device names, and very long filenames.
   - Green: harden `sanitizeFilename` to replace invalid/control characters, avoid reserved device names, and cap basename length while preserving extensions when possible.
   - Verification: `go test ./internal/ado` should fail before the change and pass after it.

4. Non-echo PAT input
   - Red: add CLI tests that prove `ado login` uses an injected secret reader and does not consume normal stdin when the dependency is present.
   - Green: add a `ReadSecret(prompt, stdin, stderr)` dependency in `internal/cli/root.go`; default implementation should use `golang.org/x/term.ReadPassword` when `stdin` is an `*os.File` attached to a terminal, and fall back to buffered stdin for non-terminal/test input.
   - Verification: `go test ./internal/cli` should fail before the change and pass after it. If a newer `golang.org/x/term` is needed, run `go get golang.org/x/term` and then `go mod tidy`.

5. Coverage gaps from review
   - Add tests for malformed/empty 200 responses in `internal/ado/client_test.go`.
   - Add tests for config missing in both repo/home in `internal/config/config_test.go`.
   - Add tests for invalid `ado fetch` args and `ado logout` missing profile in `internal/cli/ado_test.go`.
   - Add tests for `ExportContext` nil tree, zero `CreatedAt`, and nil downloader in `internal/ado/export_test.go`.
   - Add a subprocess-style test for `cmd/adomi/main.go` error stderr and non-zero exit.
   - Verification: targeted package tests should pass, then `go test ./...` should pass.

6. Final verification
   - Run `gofmt` on edited Go files.
   - Run `go test ./...`.
   - Run `go build -o /tmp/adomi-smoke ./cmd/adomi`.

## Edge Cases

- ADO description values may be empty, plain text, or HTML-like strings. Escaping makes all descriptions inert text in generated HTML. This trades rich formatting for safety.
- Attachment responses may omit `Content-Length`, lie about size, or stream more bytes than expected. The read cap must detect overflow while reading, not rely only on headers.
- Attachment names may contain separators, control characters, reserved device names, trailing dots/spaces, long stems, or no extension. Sanitization should produce a non-empty local filename and keep duplicate suffixing behavior.
- PAT input may come from a terminal, pipe, or tests. The default secret reader must preserve pipe/test behavior while using no-echo only for terminal files.
- Azure DevOps attachment URLs are documented as same `dev.azure.com/{organization}` REST URLs. If a real tenant returns a different same-service host, the existing same-host hardening may need relaxing, but I found no official-doc evidence requiring that.

## Blast Radius

- `internal/ado/client.go` changes affect all attachment downloads and client tests.
- `internal/ado/attachments.go` changes affect exported attachment filenames and duplicate suffixing.
- `internal/ado/export.go` changes affect generated HTML output only, not JSON exports.
- `internal/cli/root.go` and `internal/cli/ado.go` changes affect login only; fetch/logout/profiles should continue through existing dependency defaults.
- `go.mod`/`go.sum` may change if `golang.org/x/term` is added or upgraded for terminal password reading.

## Pre-Mortem

1. The HTML fix could break agents that expected rich description HTML. The plan chooses safety over fidelity because generated files are opened locally and the source content is user-controlled at `internal/ado/export.go:144`.
2. The attachment cap could reject legitimate oversized on-prem/server attachments. The cap should be a named constant and the error should explain the limit; configurability can be added later if real usage needs it.
3. Terminal secret reading can be brittle in tests and pipes. Adding an injectable `ReadSecret` dependency at `internal/cli/root.go:37` keeps the behavior testable and preserves non-terminal fallback.

## Planning Verification

- [x] Every file/line reference was read directly by me (not from subagent summary alone)
- [x] I ran diagnostic commands myself for facts in the plan
- [x] Each step has a verification checkpoint with concrete command and expected outcome
- [x] I searched for existing patterns before proposing new ones
- [x] I checked current filesystem state for counts, paths, and names
- [x] Blast radius listed if shared code is touched (all usages traced)
- [x] Edge cases documented for every integration point and data transformation
