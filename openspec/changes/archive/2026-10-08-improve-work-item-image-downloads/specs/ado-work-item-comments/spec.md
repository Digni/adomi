## MODIFIED Requirements

### Requirement: Comment artifacts distinguish omission from empty discussion
With comments enabled, the bundle SHALL include `comments/<item-id>.json` containing `workItemId` and a non-null `comments` array, an index `commentsPath` for every exported item, and an HTML-escaped discussion section in that item's existing HTML file. Existing item payloads and tree structure SHALL remain unchanged. Plain stdout SHALL remain path-only and JSON stdout SHALL remain exactly the existing `path`, `workItems`, and `attachments` shape in both modes. Comment-read progress SHALL NOT change attachment counts. Downloaded inline comment images SHALL count as downloaded files and SHALL be mapped to their comment IDs in the asset manifest; comment text SHALL remain unchanged.

#### Scenario: Item has no comments
- **WHEN** a complete successful read returns no non-deleted comments
- **THEN** the item has an explicit empty comment array and a discussion section indicating no comments were returned

#### Scenario: Comment contains HTML or terminal controls
- **WHEN** remote comment text contains markup or control characters
- **THEN** JSON preserves the data, HTML displays escaped text, and progress does not echo remote comment bodies to the terminal

#### Scenario: Default refresh replaces a previous enriched bundle
- **WHEN** a successful fetch without `--include-comments` refreshes an earlier enriched export
- **THEN** the refreshed bundle has no stale comment files, index fields, or discussion sections

#### Scenario: Included comment contains images
- **WHEN** discussion is requested and a comment contains supported HTML or Markdown image references
- **THEN** the export downloads those images according to ado-work-item-assets and records their comment provenance
