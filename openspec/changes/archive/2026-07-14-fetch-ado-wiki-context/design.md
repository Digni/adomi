## Context

Adomi's public command tree is rooted at `adomi`, with Azure DevOps operations registered below `adomi ado` (`internal/cli/root.go:74-91`, `internal/cli/root.go:118-137`). Network-backed commands load a repository/global profile, retrieve its PAT, construct the shared HTTP client and `ado.Client`, perform the remote operation, export repository-local context, and print the output path only after success (`internal/cli/ado.go:14-61`). The injectable `ADOClient` and `Dependencies` seams currently cover work items, attachments, and pull requests but not wikis (`internal/cli/root.go:35-61`, `internal/cli/root.go:602-624`).

The Azure DevOps client already owns authenticated requests, base URLs that may contain a collection path, project selection, API versioning, redirect rejection, and shared status errors (`internal/ado/client.go:22-32`, `internal/ado/client.go:59-114`, `internal/ado/client.go:117-142`, `internal/ado/client.go:430-463`, `internal/ado/client.go:506-534`). Wiki reads belong on this client rather than in CLI code.

Existing exports use deterministic repository-local roots, remove stale data from an earlier successful run, write resource data plus an `index.json`, and return an empty path on failure (`internal/ado/export.go:40-90`, `internal/ado/pullrequest.go:243-290`). Their tests establish relative slash-normalized index paths, UTC RFC3339 timestamps, stale-file removal, final-index behavior, and nil/input validation (`internal/ado/export_test.go:15-110`, `internal/ado/export_test.go:112-223`). CLI tests establish early argument rejection and empty stdout on failures (`internal/cli/ado_test.go:300-459`).

Adomi also generates its own agent skill, whose current context instructions enumerate work item and pull request fetch commands (`internal/cli/agent.go:123-211`) and whose tests assert those command strings (`internal/cli/agent_test.go:405-460`). Wiki context must be added there and to the concise public README command list rather than existing only in Cobra help.

Azure DevOps REST 7.1 exposes wiki resolution at `GET .../_apis/wiki/wikis/{wikiIdentifier}` and page reads at `GET .../_apis/wiki/wikis/{wikiIdentifier}/pages`. Page reads support `path`, `recursionLevel`, and `includeContent`, and return Markdown content in JSON when requested. The documented read scope is `vso.wiki`. Sources: [Wikis REST API](https://learn.microsoft.com/en-us/rest/api/azure/devops/wiki/wikis?view=azure-devops-rest-7.1) and [Get Page REST API](https://learn.microsoft.com/en-us/rest/api/azure/devops/wiki/pages/get-page?view=azure-devops-rest-7.1).

The earlier stub was not grounded in this codebase and mixed this feature with Azure DevOps indexed search. An Obsidian search for `adomi wiki` found only the current invalid-token work log, with no prior wiki product decision to preserve.

## Goals / Non-Goals

**Goals:**

- Add an explicit, read-only `adomi ado wiki fetch` command for one page or an explicitly recursive subtree.
- Reuse current profile, PAT, proxy, redirect, HTTP-error, repository, stdout, and dependency-injection conventions.
- Fetch all remote data before replacing a previous local bundle.
- Export canonical wiki metadata, page metadata/content, Markdown files, and a deterministic discoverability index below `.adomi/context/wikis/<wiki-id>/`.
- Make filesystem mapping safe for arbitrary page paths and detect unexpected duplicate paths.
- Cover the feature through vertical TDD at client, orchestration/export, and real CLI wiring boundaries.

**Non-Goals:**

- Azure DevOps Search or local full-text search.
- Wiki create, update, delete, rename, or ordering operations.
- Branch, tag, or commit version overrides; requests use the wiki's server-selected default version.
- Downloading wiki attachments or rewriting Markdown links.
- Producing a lossless archival/migration clone of the backing Git repository.
- Changing existing work item, attachment, pull request, configuration, or authentication behavior.

## Decisions

### 1. Use an explicit wiki namespace and required page path

Register `wiki` under the existing `ado` command, with `fetch` as its only operation:

`adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive] [--profile <profile-name>] [--global]`

The page path is required and must begin with `/`. Recursive traversal is opt-in. Every wiki-fetch flag is single-use: repeated `--page`, `--profile`, `--recursive`, or `--global` is rejected even when the repeated value is identical. This gives accidental duplicate input one deterministic failure contract instead of introducing last-value-wins behavior. Argument validation occurs before repository/config/PAT resolution, following existing fetch tests (`internal/cli/ado_test.go:433-459`). Both `--profile` and `--global` reuse the existing configuration semantics, but even global configuration still requires a repository because output is repository-local (`internal/cli/ado.go:20-31`).

Alternative considered: default to fetching `/` recursively. Rejected because a wiki may be very large and recursion must be a deliberate request.

### 2. Extend the existing client and dependency seams

Add wiki models and a `WikiFetcher` interface under `internal/ado`, then embed it in `internal/cli.ADOClient`. Add orchestration/export functions to `Dependencies` and wire defaults beside `FetchTree`, `ExportContext`, `FetchPullRequest`, and `ExportPullRequest` (`internal/cli/root.go:43-61`, `internal/cli/root.go:584-595`, `internal/cli/root.go:619-623`). Tests can inject fake fetch/export functions, while a real-wiring CLI test uses the production client against `httptest.Server`.

Client methods will:

1. Resolve the supplied ID/name with `GET {base}/{project}/_apis/wiki/wikis/{identifier}?api-version=<profile-version>`.
2. Fetch a page as JSON with `path=<absolute-path>&includeContent=true`.
3. For recursive selection, first request metadata using `recursionLevel=full&includeContent=false`, flatten the returned `subPages` tree, validate unique absolute paths, and then fetch each path individually with content.

All URLs are built from the existing parsed base URL and project, use normal query encoding, call the existing authorization helper, preserve the shared non-2xx status-error format while omitting HTML bodies only for wiki calls, and reject malformed, empty, or multiple-value JSON through the same strict decode shape used by `doJSON` (`internal/ado/client.go:430-463`). Wiki reads use a wiki-specific bodyless GET path that omits both a request body and `Content-Type`; the shared `doJSON` helper retains its existing nil-body wire behavior for pull-request iteration calls. Existing work-item, pull-request, and attachment request/error behavior remains unchanged. Wiki identifiers remain path segments and page paths remain query values so slashes and special characters cannot alter the API route. Every page response must contain the exact absolute path requested; an absolute but different response path is an error so a single or recursive fetch cannot silently substitute another page.

Alternative considered: rely on `includeContent=true` in one recursive response for every descendant. Rejected because the documented response guarantees page content but does not clearly guarantee populated content for every nested `subPages` entry; per-page reads make completeness testable.

### 3. Fetch a complete in-memory context before touching disk

Add an orchestration function that resolves the wiki, validates its canonical ID, fetches the selected page/tree, flattens it in stable path order, and returns a `WikiContext` containing the wiki, requested path, recursive flag, and fetched pages. Every recursive metadata path must be either the requested path or a true descendant beginning with the requested path plus `/`; similarly prefixed sibling paths are not descendants. The root path `/` is the explicit exception to that prefix construction: recursive root selection includes every absolute page path in the returned tree. It fails on a missing canonical wiki ID, missing/non-absolute/out-of-subtree page path, response/request path mismatch, duplicate page path, or any remote/decode error. Empty Markdown content is valid.

Remote calls are sequential. This is slower for large subtrees but avoids unbounded concurrency and makes request/failure behavior deterministic. Because orchestration completes before export starts, a remote failure leaves any previous successful bundle unchanged.

Alternative considered: stream each page directly to disk while fetching. Rejected because a late API failure would destroy or mix with the previous successful context.

### 4. Export an indexed bundle keyed by canonical wiki ID

Use `.adomi/context/wikis/<safe-canonical-wiki-id>/` and write:

- `wiki.json`: resolved canonical wiki metadata.
- `pages.json`: stable path-ordered fetched page models, including content and metadata.
- `pages/<encoded-page-path>.md`: the exact returned Markdown content.
- `index.json`: source, profile, project, wiki ID/name, requested path, recursive flag, UTC creation time, and one original-path-to-relative-file mapping per page.

Map each original wiki page path to a fixed-length lowercase SHA-256 hexadecimal Markdown filename. The standard-library hash is deterministic, contains no directory separators, and stays below common filesystem component limits even for the longest valid Azure DevOps wiki paths. `index.json` retains the exact original-path-to-file mapping, so filename reversibility is unnecessary. Detect duplicate output targets before writing so even a hash collision fails safely rather than overwriting another page. Independently validate that each computed target remains below `pages/`. Encode the canonical wiki ID as an unpadded URL-safe Base64 single path component; do not concatenate untrusted identifiers into filesystem paths. Before removing or writing the bundle, reject any existing symbolic link in its `.adomi/context/wikis/<safe-canonical-wiki-id>` path below the repository root so lexical containment cannot be redirected outside the repository.

Follow existing export behavior by removing the previous bundle only after the complete context is available, creating the `pages` directory, and writing data files (`internal/ado/export.go:52-89`). Serialize `index.json` to a temporary sibling, close it successfully, and atomically rename it into place only as the last step. A filesystem failure returns no output path and leaves no regular final success index; partial data files may remain until the next successful refetch, matching existing export failure behavior (`internal/ado/export_test.go:137-190`).

Alternatives considered: mirror the wiki hierarchy as directories, or encode the complete path with URL-safe Base64. Mirroring is rejected because traversal segments, case-insensitive filesystems, platform-reserved names, and file/directory collisions would require a larger cross-platform naming policy. Base64 is rejected for page filenames because its expansion can exceed common 255-byte filename-component limits for valid Azure DevOps wiki paths. The index retains human-readable wiki paths.

### 5. Preserve existing security and output boundaries

The CLI prints the bundle path only after export succeeds. All earlier failures leave stdout empty. Wiki requests use the shared production `NewHTTPClient`, so authentication redirects remain blocked and PAT/base-URL guidance is retained (`internal/ado/client.go:59-94`). Error text must not include the PAT, redirect target, or response HTML.

Markdown is stored as inert bytes and is not rendered to HTML. Remote attachment/resource links remain unchanged and are not fetched. The client never sends the PAT to URLs returned inside page content.

## Risks / Trade-offs

- **Large recursive subtrees cause many sequential requests and memory use** → recursion is explicit, page requests are sequential, and the scope is documented as context export rather than bulk archival. A future change may add limits or streaming after real usage data exists.
- **Azure DevOps returns unexpected or duplicate nested page metadata** → validate canonical wiki ID, absolute page paths, and uniqueness before export; fail rather than produce an ambiguous bundle.
- **A local write fails after the old bundle is removed** → write `index.json` last so partial data cannot look successful; report the error with empty stdout and replace partial data on the next successful run.
- **Page Markdown references attachments that are absent locally** → preserve links and state attachment mirroring as an explicit non-goal in command help and specs.
- **A page changes between tree discovery and its content request** → the bundle is a best-effort point-in-time context snapshot, not an archival transaction; store fetched metadata/content and make no consistency guarantee across requests.

## Migration Plan

This is additive and requires no data migration. Implement behind the new command, update README and generated agent-skill command documentation, and run the full Go test/build gates. Rollback removes the command and wiki-specific code; existing `.adomi/context/wikis/` output is inert repository-local data and can remain.

## Planning Verification

- [x] Every file/line reference was read directly in the main context.
- [x] Diagnostic commands were run directly for OpenSpec state, current files, existing patterns, and validation behavior.
- [x] Each implementation chunk will have a concrete test/build verification checkpoint in `tasks.md`.
- [x] Existing command, client, orchestration, export, failure-stream, and dependency-injection patterns were searched before proposing new ones.
- [x] Current filesystem state, active changes, canonical capabilities, and existing wiki code absence were checked.
- [x] Obsidian project notes were searched; no prior wiki decision was found.
- [x] Blast radius is listed: shared ADO client interface and implementation, CLI dependencies/command tree/help/parser, new wiki orchestration/export code, tests, specs, README, and generated agent-skill content.
- [x] Integration/data edge cases are specified: missing/invalid/mismatched identifiers and paths, HTTP/network/auth/decode failures, empty content, nested/empty/duplicate page trees, unsafe paths, collisions, stale exports, partial write failures, remote links, and mid-fetch changes.

## Pre-Mortem

1. **Recursive exports silently omit descendants.** Likely cause: assuming `includeContent=true` populates every nested page or flattening only one `subPages` level. Mitigation: recursively flatten `subPages`, fetch every unique path explicitly, and test a multi-level tree through the client/orchestrator.
2. **A crafted wiki/page identifier writes outside `.adomi/context/wikis`.** Likely cause: using raw IDs or page paths with `filepath.Join`. Mitigation: encode identifiers as single safe components, validate containment, detect collisions before writes, and add traversal/platform-name regression cases.
3. **The command reports success for an incomplete bundle.** Likely cause: printing before export finishes or writing `index.json` before all page files. Mitigation: preserve the `runADOFetch` success-output ordering (`internal/cli/ado.go:43-60`), write the index last, and test API/export failures with empty stdout and no final index.
