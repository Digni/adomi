## ADDED Requirements

### Requirement: Pull request work-item linking
The system SHALL allow users to explicitly link one or more existing Azure DevOps work items to an existing pull request without exposing generic work-item relation editing.

#### Scenario: Link one work item
- **WHEN** the user runs `adomi ado pr link <pull-request-id> --work-item <work-item-id>` with positive IDs, valid configuration, valid credentials, a pull request with project and repository identities, and an accessible work item that is not linked to the pull request
- **THEN** the system adds a `Pull Request` `ArtifactLink` relation from the work item to the requested pull request and reports the work item as newly linked

#### Scenario: Link multiple work items
- **WHEN** the user repeats `--work-item <work-item-id>` with distinct positive IDs and every requested item passes preflight
- **THEN** the system processes the requested work items in input order and links each item that does not already contain the pull request artifact relation

#### Scenario: Already-linked work item is unchanged
- **WHEN** a requested work item already contains an `ArtifactLink` whose URL exactly matches the requested pull request artifact URL
- **THEN** the system makes no update request for that work item, reports it as already linked, and treats it as a successful result

#### Scenario: Preflight completes before writes
- **WHEN** any requested work item is missing, inaccessible, has a mismatched response ID, or lacks a positive revision during preflight
- **THEN** the command exits non-zero before updating any requested work item

#### Scenario: Pull request identity is incomplete
- **WHEN** the fetched pull request does not contain both a project GUID and repository GUID
- **THEN** the command exits non-zero before fetching or updating any requested work item and reports the missing pull request identity

#### Scenario: Work-item revision changed before update
- **WHEN** a work item revision changes after preflight and Azure DevOps rejects the revision-tested relation update
- **THEN** the command exits non-zero without retrying or overwriting the concurrent change

#### Scenario: Later work-item update fails
- **WHEN** a multi-item invocation links one or more earlier work items and a later work-item update fails
- **THEN** the command stops without attempting subsequent items, leaves the earlier links in place, identifies the failed item and earlier linked IDs in the error, and remains safe to re-run

#### Scenario: Link response does not prove the relation
- **WHEN** Azure DevOps accepts a relation update but returns a missing or mismatched work-item ID, a non-positive revision, or an expanded response without the exact requested artifact relation
- **THEN** the command exits non-zero without printing success data

### Requirement: Conservative pull request work-item link boundary
The system SHALL limit this capability to adding explicitly requested pull request artifact relations and SHALL NOT expose broader work-item or pull request governance mutations.

#### Scenario: Unsupported relation action remains unavailable
- **WHEN** the user requests help for `adomi ado pr`
- **THEN** the help output lists work-item linking but does not list unlinking, generic relation editing, work-item field/state/assignment changes, PR voting, approval, reviewer management, completion, merge, abandonment, auto-complete, or policy bypass

#### Scenario: No automatic work-item discovery
- **WHEN** the user runs `adomi ado pr link` without at least one explicit `--work-item <work-item-id>`
- **THEN** the command exits non-zero and does not infer work items from the branch, commits, pull request title, or pull request description
