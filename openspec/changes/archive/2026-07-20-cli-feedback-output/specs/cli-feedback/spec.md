# cli-feedback spec delta

## ADDED Requirements

### Requirement: Authentication credential feedback

The system SHALL report the outcome of credential storage operations. `adomi ado login` and `adomi ado logout` SHALL write a confirmation line containing the credential ref and the action taken (stored or deleted) to stderr on success. Both commands SHALL support `--json`, in which case stdout SHALL contain exactly one JSON object with `action` and `credentialRef` fields instead of the default empty stdout.

#### Scenario: Login confirms PAT storage
- **WHEN** `adomi ado login --profile <profile>` successfully stores the PAT in the keyring
- **THEN** stderr contains a confirmation line naming the credential ref and that it was stored, and stdout is empty

#### Scenario: Logout confirms PAT deletion
- **WHEN** `adomi ado logout --profile <profile>` successfully deletes the PAT from the keyring
- **THEN** stderr contains a confirmation line naming the credential ref and that it was deleted, and stdout is empty

#### Scenario: Login JSON result
- **WHEN** `adomi ado login --profile <profile> --json` succeeds
- **THEN** stdout contains exactly one JSON object with `action` `"stored"` and the resolved `credentialRef`, and stderr contains no result data

#### Scenario: Logout JSON result
- **WHEN** `adomi ado logout --profile <profile> --json` succeeds
- **THEN** stdout contains exactly one JSON object with `action` `"deleted"` and the resolved `credentialRef`, and stderr contains no result data

#### Scenario: Failed login stores nothing and confirms nothing
- **WHEN** `adomi ado login` fails because the keyring rejects the PAT or the PAT is empty
- **THEN** the command exits non-zero, no confirmation line is written, and stdout is empty

### Requirement: Long-running operation progress

The system SHALL emit line-based progress messages to stderr during multi-request fetch operations: work item tree fetches, recursive wiki fetches, attachment downloads, and pull request bundle fetches. Progress messages SHALL be plain single lines (no TTY control sequences, spinners, or carriage-return updates) so agents and log capture can consume them in non-interactive contexts. Progress output SHALL NOT be gated on TTY detection. External text interpolated into feedback lines (wiki page paths, work item titles, attachment file names) SHALL be control-character-escaped at the feedback boundary so that every event remains exactly one physical line.

#### Scenario: Work item tree fetch progress
- **WHEN** `adomi ado fetch <work-item-id>` fetches a work item tree with a parent chain and child work items
- **THEN** stderr contains one progress line per work item fetched, naming the work item ID

#### Scenario: Attachment download progress
- **WHEN** `adomi ado fetch <work-item-id>` downloads attachments
- **THEN** stderr contains one progress line immediately after each attachment file is written, including completed attachments before a later attachment fails

#### Scenario: Wiki fetch progress
- **WHEN** `adomi ado wiki fetch` retrieves multiple wiki pages recursively
- **THEN** stderr contains progress lines naming the wiki pages retrieved

#### Scenario: Pull request fetch progress
- **WHEN** `adomi ado pr fetch <pull-request-id>` assembles the pull request bundle across multiple requests
- **THEN** stderr contains progress lines for the fetch steps

#### Scenario: Progress never appears on stdout
- **WHEN** any fetch operation emits progress messages
- **THEN** stdout contains only the command's result data and no progress lines

#### Scenario: External text in progress lines is escaped
- **WHEN** a wiki page path or attachment file name contains a newline or terminal control character
- **THEN** the progress line containing it remains a single physical line with control characters escaped

### Requirement: Fetch result summaries

The system SHALL write a human-readable summary line to stderr after successful fetch operations, reporting what was exported: `adomi ado fetch` SHALL report work item and attachment counts, `adomi ado wiki fetch` SHALL report the page count, and `adomi ado pr fetch` SHALL report the exported bundle contents. Summary lines SHALL appear on stderr only.

#### Scenario: Work item fetch summary
- **WHEN** `adomi ado fetch <work-item-id>` completes successfully
- **THEN** stderr contains a summary line with the number of work items and attachments exported and the export path

#### Scenario: Wiki fetch summary
- **WHEN** `adomi ado wiki fetch` completes successfully
- **THEN** stderr contains a summary line with the number of pages exported and the export path

#### Scenario: Pull request fetch summary
- **WHEN** `adomi ado pr fetch <pull-request-id>` completes successfully
- **THEN** stderr contains a summary line describing the exported pull request bundle

### Requirement: Structured JSON results for fetch commands

`adomi ado fetch`, `adomi ado wiki fetch`, and `adomi ado pr fetch` SHALL support `--json`. When present, stdout SHALL contain exactly one JSON object describing the result. `fetch` SHALL emit `path` (string), `workItems` (int), and `attachments` (int). `wiki fetch` SHALL emit `path` (string) and `pages` (int). `pr fetch` SHALL emit `path` (string), `threadCount` (int), and `commentCount` (int), where `threadCount` counts all threads including deleted threads and `commentCount` counts non-deleted comments summed across all threads, matching the exported pull request index semantics. The default non-JSON stdout output SHALL remain the single export path line.

#### Scenario: Fetch JSON result
- **WHEN** `adomi ado fetch <work-item-id> --json` succeeds
- **THEN** stdout contains exactly one JSON object with the export path, work item count, and attachment count

#### Scenario: Wiki fetch JSON result
- **WHEN** `adomi ado wiki fetch --json` succeeds
- **THEN** stdout contains exactly one JSON object with the export path and page count

#### Scenario: Pull request fetch JSON result
- **WHEN** `adomi ado pr fetch <pull-request-id> --json` succeeds
- **THEN** stdout contains exactly one JSON object with `path`, `threadCount`, and `commentCount`, matching the thread and comment counts written to the exported pull request index

#### Scenario: Pull request fetch JSON counts deleted threads
- **WHEN** `adomi ado pr fetch <pull-request-id> --json` succeeds and the pull request has a deleted thread with a non-deleted comment
- **THEN** `threadCount` includes the deleted thread and `commentCount` includes the non-deleted comment

#### Scenario: Non-JSON fetch stdout unchanged
- **WHEN** `adomi ado fetch <work-item-id>` succeeds without `--json`
- **THEN** stdout contains only the exported dir path followed by a newline
