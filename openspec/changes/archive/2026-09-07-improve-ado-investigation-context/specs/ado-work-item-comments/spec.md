## Purpose

Preserve work-item discussion as optional repository-local investigation context alongside the existing work-item export.

## ADDED Requirements

### Requirement: Work-item comment export is opt-in and follows existing item scope
`adomi ado fetch <id> --include-comments` SHALL retrieve current non-deleted comments for every distinct item in the existing exported root/parent/direct-child set. It SHALL NOT expand work-item traversal or change comment creation. Without the flag, comment reads and comment artifacts SHALL be absent and existing output SHALL remain unchanged.

#### Scenario: Root and related items have discussion
- **WHEN** comments are requested for a fetch containing a root, its parents, and direct children
- **THEN** each exported item receives its own discussion context and no item is queried twice

#### Scenario: Default fetch remains compatible
- **WHEN** the flag is omitted
- **THEN** no comment endpoint is called and existing JSON, HTML, tree, index, attachment, and stdout contracts are retained

### Requirement: Work-item comments are retrieved through complete bounded paging
The system SHALL request ascending non-deleted comments through the Comments read API, following opaque body continuation tokens until exhausted. It SHALL retain comment identity, item identity, current text/version, and available author, created/modified time, format, and source URL metadata. Optional unavailable metadata SHALL be null. It SHALL reject conflicting IDs, wrong item identity, duplicate comments, malformed responses, and repeated or invalid non-terminal tokens, and SHALL enforce 8 MiB per response and 1,000 pages/100,000 comments per item without silent truncation.

#### Scenario: Handover comment is on a later page
- **WHEN** the relevant comment is returned after the first response
- **THEN** it is included in the item's chronological discussion with its reported author and timestamps

#### Scenario: Comment was edited or deleted
- **WHEN** the service supplies an edited current version or a deleted comment
- **THEN** current non-deleted text/version is retained and deleted text is excluded without reconstructing historical revisions

#### Scenario: Pagination gives a remote next-page URL
- **WHEN** a response includes a continuation token and a `nextPage` URL
- **THEN** the next request uses the token with the configured endpoint and does not follow the returned URL

#### Scenario: Comment retrieval fails
- **WHEN** any item's comment read fails, returns inconsistent data, or exceeds its limits
- **THEN** the command exits nonzero with empty stdout before replacing the prior work-item export

### Requirement: Comment artifacts distinguish omission from empty discussion
With comments enabled, the bundle SHALL include `comments/<item-id>.json` containing `workItemId` and a non-null `comments` array, an index `commentsPath` for every exported item, and an HTML-escaped discussion section in that item's existing HTML file. Existing item payloads and tree structure SHALL remain unchanged. Plain stdout SHALL remain path-only and JSON stdout SHALL remain exactly the existing `path`, `workItems`, and `attachments` shape in both modes. Comment progress SHALL NOT change attachment counts.

#### Scenario: Item has no comments
- **WHEN** a complete successful read returns no non-deleted comments
- **THEN** the item has an explicit empty comment array and a discussion section indicating no comments were returned

#### Scenario: Comment contains HTML or terminal controls
- **WHEN** remote comment text contains markup or control characters
- **THEN** JSON preserves the data, HTML displays escaped text, and progress does not echo remote comment bodies to the terminal

#### Scenario: Default refresh replaces a previous enriched bundle
- **WHEN** a successful fetch without `--include-comments` refreshes an earlier enriched export
- **THEN** the refreshed bundle has no stale comment files, index fields, or discussion sections
