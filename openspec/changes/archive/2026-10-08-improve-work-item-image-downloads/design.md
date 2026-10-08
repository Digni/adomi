## Context

See proposal.md for scope and approval. Risk: **high**, because text-derived URLs reach authenticated requests and a manifest adds an export contract.

Verified current boundaries:
- `internal/ado/models.go:74` selects only `AttachedFile`; `attachments.go:39` downloads each relation independently. Its filename allocator at line 154 does not reserve generated names.
- `internal/ado/export.go:46` owns replacement of the output directory, downloads before final metadata, and discards attachment summaries. It preserves raw JSON and escapes HTML (`renderHTML`, line 156).
- `internal/ado/workitem_comments.go:22` retains comment text and optional format; `internal/cli/ado_fetch.go:61` fetches discussion only with `--include-comments`.
- The CLI passes the configured client to export and counts each exporter progress event as one downloaded file (`ado_fetch.go:75`). The same downloader interface is embedded in `ADOClient` (`root.go:33`); preserve its byte-returning signature and the exporter return signature to avoid unrelated consumers.
- `Client.Download` enforces configured scheme/host and 64 MiB per file (`client.go:174`); `NewHTTPClient` rejects redirects and supplies a timeout (`client.go:58`). No HTML or Markdown parser currently exists in go.mod.
- Existing export/CLI tests cover full item scope, nil downloader, strict failure, partial file leftovers, and stdout. Help and generated guidance explicitly distinguish comment progress from attachment counts; wording must now explain that downloaded comment images count as files.
- The prior Obsidian note `1.Projects/adomi/1. Daily Work/2026/06/2026-06-03.md` and archived attachment design confirm strict failures and current output layout.

## Goals / Non-Goals

Produce complete, traceable local image evidence within the supported work-item attachment boundary. Preserve item/comment source data and existing HTML escaping. No raw HTML rendering, general web crawler, external-origin downloads, PR/wiki changes, persistent cache, concurrency, or atomic export replacement.

## Decisions

1. **Keep discovery and file output in the work-item export layer.** Process attachment relations first, then HTML image sources in string-valued fields in sorted field-name order, then images in the included comments. Use an HTML tokenizer and a CommonMark parser rather than regex; Markdown parsing must distinguish code examples from image references and resolve reference-style images. Honor known comment formats; for missing/unknown format use Markdown parsing with embedded HTML support. Parse HTML fields as HTML. Images referenced by ordinary hyperlinks, CSS, `srcset`, or embedded data are outside discovery support; discovered unsupported `src` values remain visible in the manifest.
2. **Resolve inline URLs against explicit configured context.** Add the configured base URL to export options, populated by CLI configuration rather than trusting item/comment URLs. Accept absolute, root-relative, and project-relative work-item attachment endpoints under that configured origin and organization/collection path. Reject userinfo, unsupported schemes, encoded path separators/traversal and non-attachment endpoints. Existing relation download behavior remains unchanged. Never send the PAT to arbitrary image URLs, follow redirects, or infer trusted hosts from content. Unsupported inline references have a skipped status and reason without making a request.
3. **Use one per-item asset map.** Deduplicate ordinary relations and inline references by normalized URL; for supported attachment endpoints, use endpoint identity without presentation-only `fileName`, `download`, `api-version`, or fragments. Preserve other query parameters. Keep all distinct provenance entries. Do not deduplicate across items, which would change their existing independent attachment folders.
4. **Make evidence addressable.** Write `assets/<id>.json` with `workItemId` and a non-null `assets` array; index each via additive `assetsPath`. Each asset has original `url`, `status` (`downloaded` or `skipped`), local `path` and `name` when downloaded, `reason` when skipped, and `sources` containing `kind` (`attachment`, `field`, `comment`), field name or comment ID and original reference URL. Raw source JSON remains unchanged. Nil-downloader exports retain metadata-only behavior and identify undispatched references as skipped.
5. **Preserve bytes and reliable filenames.** Prefer the relation name, then attachment `fileName` query value, then URL basename. Determine recognizable inline image types from bytes and append an appropriate extension when missing or misleading. Do not transform image data. An inline source returning non-image data is a failed supported download, not a successful screenshot. Ordinary attachments can contain any bytes. Reserve every final sanitized name case-insensitively, including generated suffixes; keep existing path/length/device-name protections.
6. **Preserve strict failure and output contracts.** A supported download failure, invalid image response, or filesystem error fails the fetch with empty success stdout and no final metadata. Earlier files from that attempt may remain, matching current behavior. Unsupported inline references are explicitly skipped rather than turning a ticket containing an external badge into a failed export. Emit exporter progress only after a physical file write; manifest entries and skipped references do not inflate counts. JSON stdout keeps exactly `path`, `workItems`, `attachments`, where attachments counts physical downloaded files, including inline images.

## Risks / Trade-offs

- **Credential leakage through inline content** → explicit configured context, attachment endpoint restriction, hostile URL fixtures, and existing redirect/auth regressions.
- **Silent evidence loss through duplicate names or aliases** → reserve final names, per-item identity deduplication, multiple source records, and byte-level assertions in export tests.
- **A successful HTTP response is a login page** → image-byte validation for inline references; do not relax existing HTTP failure policy.
- More image references increase bandwidth; retain sequential downloads and the existing per-file limit. No new retry or caching machinery.
- Live tenant URL forms are **UNVERIFIED**. Deterministic HTTP fixtures prove the specified supported forms, not compatibility with every Azure DevOps deployment. Unsupported forms remain explicit and can be assessed from a future user-provided example.

## Migration Plan

No server-side changes or migration. Fetch continues replacing the same local export directory. New manifests and additive index paths appear on the next fetch. Existing raw JSON, HTML, CLI flags, stdout keys, and attachment directories remain compatible; counts now include newly discovered downloaded images. Rollback reverts the code and refreshes the export. Do not publish or commit as part of this task.

## Verification

Use existing Go unit/export/HTTP CLI harnesses with durable behavior regressions and TDD per chunk, recorded in tasks.md. Test real configured-client requests through local HTTP servers, on-disk bytes and manifests, raw-content preservation, source provenance, unsupported URLs with zero requests, auth redirects, failure stdout, and counts. This deterministic integration gate proves the changed network-to-filesystem behavior; no live Azure DevOps claim is made. After high-risk agent review and fixes: `go test ./...`, `go vet ./...`, `go build ./...`, formatting and strict OpenSpec validation.

## Sources

- [Goldmark parser documentation](https://github.com/yuin/goldmark/tree/v1): CommonMark parsing and AST access.
- [Go HTML tokenizer documentation](https://pkg.go.dev/golang.org/x/net/html): HTML tokenization and decoded attributes.
