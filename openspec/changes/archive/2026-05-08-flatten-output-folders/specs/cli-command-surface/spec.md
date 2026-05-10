## MODIFIED Requirements

### Requirement: Azure DevOps commands remain under ado namespace

The system SHALL preserve Azure DevOps-specific commands under `adomi ado`.

#### Scenario: Fetch work item context
- **WHEN** the user runs `adomi ado fetch <work-item-id>` with valid configuration and credentials
- **THEN** the command fetches and exports work item context for the requested work item, its parent chain, and any direct children of the requested work item under `.adomi/context/work-items/<work-item-id>/`, and prints only the exported directory path to stdout

#### Scenario: Fetch includes direct child work items
- **WHEN** the requested Azure DevOps work item has direct child relations
- **THEN** each direct child work item is fetched and included in the exported JSON, HTML, tree, index, and attachment context alongside the requested item and parent chain

#### Scenario: Fetch work item export uses flattened internal item paths
- **WHEN** `adomi ado fetch <work-item-id>` exports multiple work item JSON payloads
- **THEN** the payload files are written under `items/<id>.json` relative to the exported work item context directory and `index.json` references those relative `items/<id>.json` paths

#### Scenario: Fetch pull request comments
- **WHEN** the user runs `adomi ado pr <pull-request-id>` with valid configuration and credentials
- **THEN** the command fetches the pull request, downloads all comment threads, exports them under the repository-local `.adomi/context/pull-requests/<pull-request-id>/` directory, and prints only the exported directory path to stdout

#### Scenario: Export metadata records Azure DevOps source context
- **WHEN** `adomi ado fetch <work-item-id>` or `adomi ado pr <pull-request-id>` succeeds
- **THEN** the exported `index.json` records the Azure DevOps source, profile, and project metadata used for the fetch

#### Scenario: Manage credentials
- **WHEN** the user runs `adomi ado login` or `adomi ado logout` with valid credential flags
- **THEN** the command stores or deletes credentials using the same profile and `patRef` behavior as before

#### Scenario: List profiles
- **WHEN** the user runs `adomi ado profiles list`
- **THEN** the command lists configured profile names in sorted order as before
