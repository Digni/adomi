## Context

Adomi creates its production Azure DevOps HTTP client in `internal/ado/client.go:35-53`. The client configures proxy behavior and a timeout but leaves `CheckRedirect` unset, so Go applies its default redirect-following policy. The CLI wires this factory into all production Azure DevOps command paths through `internal/cli/root.go:602-623` and calls it from work item fetch, PR ensure, shared maintenance-client construction, and PR fetch (`internal/cli/ado.go:22-64`, `internal/cli/ado.go:154-220`, `internal/cli/ado.go:428-465`, `internal/cli/ado.go:554-596`).

Each request adds PAT Basic authentication in `internal/ado/client.go:574-576`. JSON read methods and the shared write helper only inspect the final response status before decoding (`internal/ado/client.go:77-103`, `internal/ado/client.go:124-235`, `internal/ado/client.go:390-423`). When an invalid PAT produces a redirect to interactive sign-in, the final `200 text/html` page reaches those JSON decoders and is reported as a decode failure instead of an authentication or HTTP failure.

The same redirect policy affects `Download` (`internal/ado/client.go:425-452`). Because attachment export writes every byte returned by `Download` (`internal/ado/attachments.go:39-72`), a followed sign-in redirect can be persisted as if it were the requested attachment. Microsoft documents a successful Work Item Tracking attachment response as `200` with `application/octet-stream` or `application/zip`, not as a redirect.

Existing non-2xx handling is centralized in `responseError` (`internal/ado/client.go:589-601`), and existing tests cover direct 401/500 responses and malformed or empty successful JSON (`internal/ado/client_test.go:62-155`, `internal/ado/pullrequest_test.go:81-98`). There is no redirect regression test and the only `NewHTTPClient` tests currently cover invalid proxy configuration (`internal/ado/client_test.go:438-451`).

Relevant Obsidian project notes were checked. `1.Projects/adomi/1. Daily Work/2026/06/2026-06-12.md` records the deliberate strict-JSON response handling added during work item comments, but no Adomi note establishes redirect or invalid-PAT behavior.

## Goals / Non-Goals

**Goals:**

- Make every production `adomi ado` network operation stop at the Azure DevOps redirect response instead of following it to interactive HTML.
- Route the resulting non-2xx response through a concise, status-bearing error path without decoding or persisting the redirected page.
- Preserve existing direct 401/403/other non-2xx behavior, proxy configuration, timeout behavior, successful JSON decoding, successful attachment downloads, stdout/stderr contracts, and PAT secrecy.
- Add regression tests that prove the redirect target is never requested and representative read, write, and download paths fail correctly.

**Non-Goals:**

- No token refresh, PAT validation request, automatic logout, interactive reauthentication, or credential-store changes.
- No command syntax, configuration schema, API version, retry policy, or external dependency changes.
- No automatic correction of a base URL that redirects to a canonical URL; users must configure the final Azure DevOps API base URL.
- No broad content-type validation for arbitrary `200` responses. A genuinely successful but malformed JSON response remains a decode error, as it does today.
- No behavioral guarantee for callers that construct `ado.Client` with an arbitrary injected `*http.Client`; this change governs the production client created by `NewHTTPClient` and used by the CLI.

## Decisions

### Decision 1: stop redirects in the shared production HTTP client

Set `CheckRedirect` on the `http.Client` returned by `NewHTTPClient` (`internal/ado/client.go:35-53`) so it returns `http.ErrUseLastResponse`. For malformed `Location` values, which Go parses before calling `CheckRedirect`, wrap the production transport with a narrow guard that removes only an unparseable redirect location from 301/302/303/307/308 responses. Go then returns the original redirect response without requesting a target, allowing every existing status check to treat it as non-success while retaining the original status.

Rationale: all production Azure DevOps CLI paths share this factory (`internal/cli/root.go:602-623`), so one transport policy covers work item reads, PR reads, JSON writes through `doJSON`, and attachment downloads. The official Go contract for `http.ErrUseLastResponse` preserves the last response for valid redirect locations, while the transport guard closes the malformed-location gap before Go can turn it into a location-leaking transport error.

Alternative considered: add `Content-Type` checks only before JSON decoding. Rejected because it duplicates checks across several read methods and `doJSON`, and it does not protect `Download` from returning sign-in HTML as attachment bytes.

Alternative considered: mutate or replace redirect behavior inside `NewClient`. Rejected because `NewClient` intentionally accepts an injected `*http.Client` (`internal/ado/client.go:56-74`); silently overriding caller policy would broaden the package contract and complicate tests beyond the production CLI bug.

### Decision 2: treat every API redirect as a failed response, not only recognized sign-in URLs

Do not follow same-origin or cross-origin 301, 302, 303, 307, or 308 responses from Azure DevOps API and attachment requests. Report the original status through the existing action-specific error path. For redirect responses, keep the diagnostic concise and do not append the HTML body or expose the `Location` URL; include guidance that the PAT or configured base URL may be invalid.

Rationale: the client is calling REST endpoints, not navigating a browser. Following a 302/303 can also turn a JSON write into a GET before the client sees the final response. The configured base URL is already required to be absolute (`internal/ado/client.go:56-69`), attachment URLs are already restricted to the configured Azure DevOps origin before authentication (`internal/ado/client.go:578-586`), and Microsoft documents attachment success as a direct `200` response. Failing closed also avoids sending any follow-up request to an interactive identity endpoint.

Alternative considered: recognize a fixed list of Microsoft sign-in hosts or URL paths. Rejected because Azure DevOps Services, Azure DevOps Server, proxies, and identity front doors can produce different redirect destinations; a host allow/deny list would be brittle and would still leave unrecognized HTML redirects.

Alternative considered: preserve redirects that stay on the configured host. Rejected because a same-origin proxy or Azure DevOps Server sign-in route can still return HTML, and canonicalizing a misconfigured base URL is outside this change.

### Decision 3: keep direct HTTP errors and successful response handling unchanged

Continue using `responseError` for direct non-2xx responses (`internal/ado/client.go:589-601`) and preserve its bounded response-body context for non-redirect failures. Add redirect-specific formatting before body inclusion so a redirect cannot dump HTML into the CLI error. Existing `2xx` JSON validation and attachment size limits (`internal/ado/client.go:411-421`, `internal/ado/client.go:441-461`) remain unchanged.

Rationale: direct 401/403/500 errors already produce useful action and status context, and strict decoding intentionally catches malformed successful responses. The bug is the loss of the original redirect status, not the existing behavior for actual final responses.

Alternative considered: map every redirect to a synthetic 401. Rejected because Adomi should not invent a status the server did not return; a redirect may also signal a bad base URL rather than an invalid PAT.

### Decision 4: test the shared policy plus representative consumers

Add an `httptest` redirect chain using the actual `NewHTTPClient`. Assert that the redirect target receives zero requests and that the caller sees the original 3xx status. Cover representative JSON read, JSON write, and attachment download paths so the regression suite proves there is no decode attempt, no write success output, and no returned attachment bytes. Keep the existing direct 401/500 and malformed JSON cases green.

Rationale: a factory-only assertion proves configuration but not consumer behavior; exhaustive repetition across every method adds noise because the same `*http.Client` and status checks are shared. Representative tests exercise each materially different response path identified in `internal/ado/client.go:77-452`.

## Risks / Trade-offs

- **[Risk] A deployment currently relies on an HTTP-to-HTTPS, renamed-organization, or other canonical redirect.** -> Fail with the original status and PAT/base-URL guidance; require the final API base URL in configuration rather than silently changing request origin or method.
- **[Risk] A legitimate attachment deployment returns a redirect despite Microsoft's documented direct `200` response.** -> Cover current successful attachment tests, document the direct-response assumption, and stop implementation to update this design/spec if real supported Azure DevOps behavior contradicts it.
- **[Risk] Redirect errors still leak an HTML body or identity URL.** -> Give 3xx responses dedicated concise formatting that excludes both response body and `Location` value.
- **[Risk] Tests use `httptest.Server.Client()` directly and miss the production policy.** -> Construct redirect regression clients through `NewHTTPClient`; retain injected clients only for unrelated focused tests.
- **[Risk] Fixing JSON endpoints alone leaves attachment corruption possible.** -> Include `Download` in the acceptance tests and assert no bytes are returned from a redirected request.

## Edge Cases

- Redirect statuses: 301, 302, 303, 307, and 308 are all non-success and are not followed. Valid relative/same-origin/cross-origin locations reach `CheckRedirect`; a missing location is returned directly; and an unparseable location is removed by the production transport guard so the original response reaches shared status handling without leaking the location value. Other 3xx statuses such as 304 remain ordinary status-bearing failures and do not receive PAT/base-URL redirect guidance.
- Authentication and permission failures: direct 401 and 403 responses remain status-bearing failures; empty error bodies remain valid errors without a dangling separator; bounded non-HTML response context remains available.
- Unexpected response shape: a direct `2xx` HTML or malformed JSON response on a JSON endpoint remains a decode error because no redirect occurred; empty JSON bodies remain decode errors. A direct `2xx` attachment response remains subject to the existing size/read checks.
- Writes: a redirected POST/PATCH is not retried or converted into a GET, and no success data is printed.
- Attachments: a redirect returns no bytes to `Download`, so `DownloadAttachments` cannot write the identity page; existing same-origin prevalidation still runs before the request.
- Proxy and on-prem use: proxy-produced redirects and Azure DevOps Server redirects fail consistently; proxy selection and TLS/network errors retain their current behavior.
- Injected clients: tests or external package callers that bypass `NewHTTPClient` keep their supplied redirect policy and are outside the CLI contract.

## Migration Plan

No data or configuration migration is required. Deploy the client behavior with the CLI release. Rollback is reverting the redirect policy and redirect-specific error formatting; no persisted schema or credential data changes are involved. If rollout reveals a supported Azure DevOps endpoint that legitimately requires redirects, stop and revise the spec/design before adding a narrow redirect exception.

## Open Questions

None.

## Planning Verification

- [x] Every file/line reference was read directly by the main agent.
- [x] Diagnostic commands were run directly for repository facts, including focused searches, current OpenSpec status, existing tests, and `go test ./internal/ado ./internal/cli`.
- [x] Each implementation chunk in `tasks.md` has a concrete verification checkpoint and expected outcome.
- [x] Existing HTTP error, JSON decode, attachment, and client-construction patterns were searched before proposing the change.
- [x] Current filesystem state, active OpenSpec changes, and the pre-existing untracked wiki proposal were checked and preserved.
- [x] Relevant Adomi Obsidian notes were searched/read; no prior redirect decision was found.
- [x] Blast radius covers all `httpClient.Do` consumers plus attachment persistence.
- [x] Error, empty-body, missing-location, unexpected-shape, redirect-status, write, download, proxy, and injected-client edges are documented.

## Pre-Mortem

1. The change could appear fixed for fetch while invalid-token attachment requests still save HTML. Decision 4 now requires a redirected `Download` regression proving no bytes reach attachment persistence.
2. The change could break valid installations that rely on a canonical base-URL redirect. The design makes final API URLs an explicit requirement and requires a spec update rather than an unplanned redirect exception if supported Azure DevOps behavior contradicts the assumption.
3. The change could replace the JSON decode error with a large HTML error or identity URL. Decision 2 requires redirect-specific formatting that excludes the response body and `Location` value.
