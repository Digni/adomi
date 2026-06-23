# ado-work-item-maintenance Specification

## Purpose
Define conservative Azure DevOps work item maintenance behavior for adding text comments without exposing broader work item mutation actions.
## Requirements
### Requirement: Work item comment creation
The system SHALL allow users to add a text comment to an Azure DevOps work item using a conservative comment-only work item write operation.

#### Scenario: Create work item comment from inline message
- **WHEN** the user runs `adomi ado comment <work-item-id> --message <text>` with a positive work item ID, non-blank message text, valid configuration, and valid credentials
- **THEN** the system posts the text as a new Azure DevOps work item comment and prints only the created comment ID followed by a newline to stdout

#### Scenario: Create work item comment from message file
- **WHEN** the user runs `adomi ado comment <work-item-id> --message-file <path>` with a positive work item ID, a readable non-empty non-blank message file, valid configuration, and valid credentials
- **THEN** the system posts the file contents as a new Azure DevOps work item comment and prints only the created comment ID followed by a newline to stdout

#### Scenario: Create work item comment through namespace alias
- **WHEN** the user runs `adomi ado work-item comment <work-item-id> --message <text>` with a positive work item ID, non-blank message text, valid configuration, and valid credentials
- **THEN** the command behaves the same as `adomi ado comment <work-item-id> --message <text>`

#### Scenario: Work item comment requires one message source
- **WHEN** the user runs a work item comment command without exactly one of `--message` or `--message-file`
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps write request

#### Scenario: Work item comment rejects invalid identifiers
- **WHEN** the user runs a work item comment command with a missing, non-integer, or non-positive work item ID
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps write request

#### Scenario: Work item comment rejects blank content
- **WHEN** the user runs a work item comment command with blank inline message text or a message file whose contents are empty or whitespace-only
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps write request

#### Scenario: Work item comment JSON output
- **WHEN** `adomi ado comment <work-item-id> --message <text> --json` succeeds
- **THEN** stdout contains one JSON object describing the work item ID, created comment ID, action, and response URL when one is returned, followed by a newline

#### Scenario: Work item comment response missing comment ID
- **WHEN** Azure DevOps accepts a work item comment request but the response does not include a positive `id` or `commentId`
- **THEN** the command exits non-zero and does not print success data to stdout

#### Scenario: Work item comment response mismatches work item ID
- **WHEN** Azure DevOps accepts a work item comment request but the response includes a positive `workItemId` that differs from the requested work item ID
- **THEN** the command exits non-zero and does not print success data to stdout

#### Scenario: Work item comment write failure leaves stdout empty
- **WHEN** a work item comment command receives an Azure DevOps error response, authorization failure, oversized message rejection, malformed response, empty response, or network error
- **THEN** the command exits non-zero and stdout is empty

### Requirement: Conservative work item write boundary
The system SHALL NOT expose work item field mutation, state transitions, relation edits, attachment uploads, comment updates, comment deletion, or reaction management as part of work item maintenance.

#### Scenario: Unsupported work item write action is unavailable
- **WHEN** the user requests help for `adomi ado` or `adomi ado work-item`
- **THEN** the help output does not list commands for updating fields, changing state, assigning users, editing relations, uploading attachments, updating comments, deleting comments, or managing reactions

#### Scenario: Unknown work item maintenance command
- **WHEN** the user runs `adomi ado work-item update <work-item-id>` or another unsupported work item maintenance command
- **THEN** the command exits non-zero and does not make any Azure DevOps write request
