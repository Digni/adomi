## Why

When Azure DevOps rejects an invalid or expired PAT by redirecting to an interactive sign-in page, Adomi follows that redirect and treats the final HTML response as successful API data. JSON endpoints then report a misleading decode error, while attachment downloads can mistake the sign-in page for file content, instead of returning a clear authentication or HTTP failure.

## What Changes

- Stop Adomi's production Azure DevOps HTTP client from following authentication redirects into interactive HTML pages.
- Surface a concise, status-bearing error for redirected or otherwise unsuccessful Azure DevOps API requests without attempting JSON decoding or returning redirected HTML as attachment data.
- Apply the behavior consistently to work item fetch/export, attachment downloads, work item comments, pull request fetch/export, and pull request maintenance commands.
- Preserve the existing CLI contract that failures exit non-zero, leave stdout empty, and report the error through stderr.
- Add redirect-chain regression coverage alongside the existing direct 401/500 and malformed-JSON tests.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `cli-command-surface`: Define consistent authentication-redirect failure behavior for every network-backed `adomi ado` command, including attachment downloads.

## Impact

- Shared Azure DevOps HTTP construction and response handling in `internal/ado/client.go`, including every request path that uses the client created by `NewHTTPClient`.
- Work item attachment export in `internal/ado/attachments.go`, which currently persists any successful download bytes without validating that an authentication redirect occurred.
- Azure DevOps client and CLI regression tests in `internal/ado/client_test.go`, `internal/ado/pullrequest_test.go`, and, if needed to verify stream behavior end to end, `internal/cli/ado_test.go`.
- No command syntax, configuration schema, credential storage, external dependency, or successful-response behavior changes.
