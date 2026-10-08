## MODIFIED Requirements

### Requirement: Work item fetch downloads attachments

The system SHALL download Azure DevOps work item attachments for every work item included in a successful `adomi ado fetch <work-item-id>` export and SHALL discover inline work-item images according to ado-work-item-assets. It SHALL expose each exported work item's attachment directory and asset manifest in the export index. Existing path-only stdout and the JSON keys path, workItems, attachments SHALL remain unchanged; attachments SHALL count physical downloaded files including inline images.

#### Scenario: Fetch downloads requested work item attachments
- **WHEN** the user runs `adomi ado fetch <work-item-id>` with valid configuration and credentials and the requested work item has attachment relations
- **THEN** the command downloads those attachments under `.adomi/context/work-items/<work-item-id>/attachments/<work-item-id>/` and prints only the exported directory path to stdout

#### Scenario: Fetch downloads parent-chain work item attachments
- **WHEN** the requested Azure DevOps work item has a parent chain and any fetched parent work item has attachment relations
- **THEN** the command downloads the parent work item's attachments under `.adomi/context/work-items/<requested-work-item-id>/attachments/<parent-work-item-id>/`

#### Scenario: Fetch downloads direct child work item attachments
- **WHEN** the requested Azure DevOps work item has direct child relations and any fetched direct child work item has attachment relations
- **THEN** the command downloads the child work item's attachments under `.adomi/context/work-items/<requested-work-item-id>/attachments/<child-work-item-id>/`

#### Scenario: Fetch index exposes attachment paths
- **WHEN** `adomi ado fetch <work-item-id>` exports work item context
- **THEN** `index.json` includes each exported work item's relative POSIX attachment directory path as `attachments/<exported-work-item-id>` with no trailing slash

#### Scenario: Work item without attachments does not require attachment files
- **WHEN** an exported work item has neither attachment relations nor supported inline images
- **THEN** the command does not create attachment files for that work item and the rest of the work item context export can still succeed

#### Scenario: Attachment downloads complete before final metadata is written
- **WHEN** `adomi ado fetch <work-item-id>` exports work item context for work items with attachment relations
- **THEN** the command completes attachment downloads before writing final item, HTML, tree, and index export artifacts for the successful fetch

#### Scenario: Attachment download failure leaves stdout empty
- **WHEN** `adomi ado fetch <work-item-id>` cannot download an attachment for any exported work item because Azure DevOps returns an error response or a network error occurs
- **THEN** the command exits non-zero and stdout is empty

#### Scenario: Fetch help explains image evidence
- **WHEN** work-item fetch help is requested
- **THEN** help explains inline image discovery, opt-in comment images, asset manifest paths, skipped unsupported references, and file-count semantics
