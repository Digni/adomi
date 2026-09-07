# ado-pr-discovery Specification

## Purpose
Let users and agents discover pull requests directly from repository and branch criteria without depending on work-item links.

## Requirements

### Requirement: Discover repository pull requests with explicit filters
The system SHALL expose `adomi ado pr list` with optional source, target, status, and repository filters. Status SHALL default to `active` and accept `active`, `completed`, `abandoned`, and `all`. Omitted branch filters SHALL remain unset. The command SHALL select the configured project's repository using existing remote inference and explicit override rules and SHALL send all supplied filters to Azure DevOps on every page.

#### Scenario: Discover an unlinked branch PR
- **WHEN** a user runs `adomi ado pr list --source feature/example --status all`
- **THEN** the query uses source `refs/heads/feature/example` and status `all` and can return active, completed, or abandoned matches regardless of work-item links

#### Scenario: Target and repository narrow discovery
- **WHEN** explicit `--repository`, `--source`, and `--target` values are supplied
- **THEN** the selected repository and both normalized branch filters constrain every request, with no dependency on the current checkout branch

#### Scenario: Omitted branch filters include all matching branches
- **WHEN** `pr list` is invoked without source or target, including on detached HEAD
- **THEN** no source or target filter is inferred and active PRs in the selected repository are queried

#### Scenario: Full refs are preserved
- **WHEN** a branch value already starts with `refs/`
- **THEN** it is used exactly after trimming surrounding whitespace, while a short branch receives `refs/heads/` and remote-name prefixes are not stripped

### Requirement: PR discovery completes bounded pagination before output
The system SHALL traverse offset pages until an empty collection is received, advancing by the number of items received even after short pages, and preserve received order. Discovery SHALL allow at most 1,000 pages, 100,000 accumulated PRs, and 8 MiB per response. It SHALL fail on invalid identity, missing/malformed collections, duplicate PR IDs, pagination non-progress, HTTP/decode errors, or an exhausted ceiling without a terminal empty page. It SHALL describe results as a best-effort observed traversal.

#### Scenario: Matches extend past the first page
- **WHEN** a matching PR exists after one or more nonempty pages, including a short page
- **THEN** it appears in the successful final result and no success output is emitted before traversal finishes

#### Scenario: No matches exist
- **WHEN** Azure DevOps returns a valid empty collection on the first page
- **THEN** stdout is exactly one compact object containing `pullRequests: []` followed by a newline

#### Scenario: Later page fails or repeats
- **WHEN** a later page fails, repeats a PR ID, or would exceed a documented ceiling
- **THEN** the command exits nonzero with empty success stdout rather than returning a truncated list

### Requirement: PR discovery has stable read-only output
Successful discovery SHALL write one compact JSON object with a `pullRequests` array and trailing newline. Each entry SHALL include `pullRequestId`, `title`, `status`, `isDraft`, `repositoryId`, `repositoryName`, `sourceRefName`, `targetRefName`, `creationDate`, `closedDate`, and `url`; absent optional dates/URL SHALL be null and available dates SHALL use UTC RFC3339Nano. `url` SHALL denote the API URL. Discovery SHALL issue only read requests and SHALL NOT change the request or result behavior of existing PR maintenance invocations.

#### Scenario: Discovery precedes detailed fetch
- **WHEN** a PR is returned by listing
- **THEN** its ID can be passed to `pr fetch` for description and discussion, without discovery downloading that additional context

#### Scenario: Existing ensure behavior is retained
- **WHEN** `pr ensure` is run after discovery support is added
- **THEN** its existing active-PR lookup, ambiguity handling, create/update decisions, and output contract remain unchanged
