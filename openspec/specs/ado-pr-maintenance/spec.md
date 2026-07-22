# ado-pr-maintenance Specification

## Purpose
Define conservative Azure DevOps pull request maintenance behavior for creating or updating the active branch pull request and explicitly maintaining existing pull request review threads without exposing approval, merge, completion, or reviewer-governance actions.
## Requirements
### Requirement: Repository-inferred PR context
The system SHALL resolve Azure DevOps PR maintenance context from the current Git repository before making any PR write request.

#### Scenario: Infer source branch and repository from Git
- **WHEN** the user runs `adomi ado pr ensure` inside a Git repository with an Azure DevOps remote, an attached current branch, valid Azure DevOps configuration, and valid credentials
- **THEN** the system uses the current Git branch as the PR source branch and the Azure DevOps remote repository as the PR repository for the ensure operation

#### Scenario: Require repository execution
- **WHEN** the user runs a PR maintenance command outside a Git repository
- **THEN** the command exits non-zero before making any Azure DevOps write request

#### Scenario: Detached source branch requires override
- **WHEN** the user runs `adomi ado pr ensure` while Git cannot report a current branch and no `--source` value is provided
- **THEN** the command exits non-zero before making any Azure DevOps write request

#### Scenario: Ambiguous repository inference requires override
- **WHEN** multiple Azure DevOps remotes could match the loaded profile and no `--repository` value is provided
- **THEN** the command exits non-zero before making any Azure DevOps write request and reports that the repository is ambiguous

#### Scenario: Target branch fallback unavailable
- **WHEN** the user runs `adomi ado pr ensure` without `--target` and the target branch cannot be inferred from Git remote default branch metadata
- **THEN** the command exits non-zero before making any Azure DevOps write request and reports that `--target` is required

### Requirement: Idempotent PR ensure
The system SHALL provide an idempotent PR ensure operation that creates an active Azure DevOps pull request for the resolved source/target branch pair only when one does not already exist.

#### Scenario: Create missing PR
- **WHEN** `adomi ado pr ensure --title <title>` resolves a repository, source branch, target branch, optional description, valid configuration, and valid credentials, and Azure DevOps returns no active PR matching the source and target refs
- **THEN** the system creates a pull request with those source and target refs and prints only the created pull request ID followed by a newline to stdout

#### Scenario: Missing title for create
- **WHEN** `adomi ado pr ensure` resolves a repository, source branch, target branch, valid configuration, and valid credentials, Azure DevOps returns no active PR matching the source and target refs, and the user did not provide `--title`
- **THEN** the command exits non-zero before creating a pull request

#### Scenario: Update existing PR title
- **WHEN** `adomi ado pr ensure --title <title>` finds exactly one active PR matching the resolved source and target refs
- **THEN** the system updates that pull request title and prints only the pull request ID followed by a newline to stdout

#### Scenario: Update existing PR description
- **WHEN** `adomi ado pr ensure --description-file <path>` finds exactly one active PR matching the resolved source and target refs and the description file is readable and non-empty
- **THEN** the system updates that pull request description from the file contents and prints only the pull request ID followed by a newline to stdout

#### Scenario: Existing PR without update fields
- **WHEN** `adomi ado pr ensure` finds exactly one active PR matching the resolved source and target refs and the user provided no updatable fields
- **THEN** the system leaves the pull request unchanged and prints only the pull request ID followed by a newline to stdout

#### Scenario: Multiple matching active PRs
- **WHEN** `adomi ado pr ensure` finds more than one active PR matching the resolved source and target refs
- **THEN** the command exits non-zero before creating or updating a pull request

#### Scenario: Source and target refs match
- **WHEN** `adomi ado pr ensure` resolves the same normalized branch ref for source and target
- **THEN** the command exits non-zero before listing, creating, or updating pull requests

#### Scenario: Ensure JSON output
- **WHEN** `adomi ado pr ensure --json` succeeds
- **THEN** stdout contains one JSON object describing the pull request ID, action, repository, source ref, and target ref followed by a newline

### Requirement: PR thread replies
The system SHALL allow users to add an explicit text reply to an existing Azure DevOps pull request thread.

#### Scenario: Reply from message file
- **WHEN** the user runs `adomi ado pr reply <pull-request-id> --thread <thread-id> --message-file <path>` with a readable non-empty message file, valid configuration, and valid credentials
- **THEN** the system fetches the pull request to resolve its repository ID, creates a text comment on that pull request thread, and prints only the created comment ID followed by a newline to stdout

#### Scenario: Reply from inline message
- **WHEN** the user runs `adomi ado pr reply <pull-request-id> --thread <thread-id> --message <text>` with non-empty message text, valid configuration, and valid credentials
- **THEN** the system fetches the pull request to resolve its repository ID, creates a text comment on that pull request thread, and prints only the created comment ID followed by a newline to stdout

#### Scenario: Reply requires one message source
- **WHEN** the user runs `adomi ado pr reply <pull-request-id> --thread <thread-id>` without exactly one of `--message-file` or `--message`
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps write request

#### Scenario: Reply missing repository ID
- **WHEN** `adomi ado pr reply <pull-request-id> --thread <thread-id> --message <text>` fetches a pull request response that does not include a repository ID
- **THEN** the command exits non-zero before creating a comment

#### Scenario: Reply JSON output
- **WHEN** `adomi ado pr reply <pull-request-id> --thread <thread-id> --message <text> --json` succeeds
- **THEN** stdout contains one JSON object describing the pull request ID, thread ID, comment ID, and action followed by a newline

### Requirement: PR thread status maintenance
The system SHALL allow users to explicitly resolve or reopen an existing Azure DevOps pull request thread without changing pull request approval, completion, or reviewer state.

#### Scenario: Resolve thread as fixed
- **WHEN** the user runs `adomi ado pr resolve <pull-request-id> --thread <thread-id>` with valid configuration and valid credentials
- **THEN** the system fetches the pull request to resolve its repository ID, updates that pull request thread status to `fixed`, and prints only the thread ID followed by a newline to stdout

#### Scenario: Reopen thread as active
- **WHEN** the user runs `adomi ado pr reopen <pull-request-id> --thread <thread-id>` with valid configuration and valid credentials
- **THEN** the system fetches the pull request to resolve its repository ID, updates that pull request thread status to `active`, and prints only the thread ID followed by a newline to stdout

#### Scenario: Thread command rejects invalid IDs
- **WHEN** the user runs a PR thread status command with a non-positive pull request ID or non-positive thread ID
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps write request

#### Scenario: Thread status missing repository ID
- **WHEN** a PR thread status command fetches a pull request response that does not include a repository ID
- **THEN** the command exits non-zero before updating thread status

#### Scenario: Thread status JSON output
- **WHEN** `adomi ado pr resolve <pull-request-id> --thread <thread-id> --json` or `adomi ado pr reopen <pull-request-id> --thread <thread-id> --json` succeeds
- **THEN** stdout contains one JSON object describing the pull request ID, thread ID, status, and action followed by a newline

### Requirement: Conservative PR write boundary
The system SHALL NOT expose pull request approval, rejection, completion, abandonment, auto-complete, policy bypass, or reviewer-management behavior as part of PR maintenance.

#### Scenario: Unsupported governance action is unavailable
- **WHEN** the user requests help for `adomi ado pr`
- **THEN** the help output does not list commands for approving, rejecting, completing, merging, abandoning, bypassing policies, setting auto-complete, or managing reviewers

#### Scenario: Unknown PR maintenance command
- **WHEN** the user runs `adomi ado pr complete <pull-request-id>` or another unsupported PR maintenance command
- **THEN** the command exits non-zero and does not make any Azure DevOps write request

### Requirement: PR-level thread creation
The system SHALL allow users to create a new active Azure DevOps pull request comment thread either at the pull request level or on the latest version of a changed file, without changing pull request approval, completion, or reviewer state.

#### Scenario: Create PR-level thread from message file
- **WHEN** the user runs `adomi ado pr comment <pull-request-id> --message-file <path>` with a readable non-empty message file, valid configuration, and valid credentials
- **THEN** the system fetches the pull request to resolve its repository ID, creates a new active PR-level text comment thread using the file contents as the initial comment, and prints only the created thread ID followed by a newline to stdout

#### Scenario: Create PR-level thread from inline message
- **WHEN** the user runs `adomi ado pr comment <pull-request-id> --message <text>` with non-empty message text, valid configuration, and valid credentials
- **THEN** the system fetches the pull request to resolve its repository ID, creates a new active PR-level text comment thread using the message text as the initial comment, and prints only the created thread ID followed by a newline to stdout

#### Scenario: Create inline thread on latest changed file version
- **WHEN** the user runs `adomi ado pr comment <pull-request-id> --file <path> --line <line> --message <text>` with a positive line number, non-empty message text, valid configuration, valid credentials, and the path matching a supported changed file in the latest pull request file version
- **THEN** the system fetches the pull request to resolve its repository ID, resolves the latest pull request iteration and matching change tracking ID for that path, creates a new active right-side single-line text comment thread anchored at that line, and prints only the created thread ID followed by a newline to stdout

#### Scenario: Create inline thread from message file
- **WHEN** the user runs `adomi ado pr comment <pull-request-id> --file <path> --line <line> --message-file <path-to-message>` with a positive line number, a readable non-empty message file, valid configuration, valid credentials, and the path matching a supported changed file in the latest pull request file version
- **THEN** the system uses the file contents as the initial comment and creates the same right-side single-line inline thread as the inline message variant

#### Scenario: Inline thread creation requires file and line together
- **WHEN** the user runs `adomi ado pr comment <pull-request-id>` with only one of `--file` or `--line`
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps write request

#### Scenario: Inline thread creation rejects invalid line
- **WHEN** the user runs `adomi ado pr comment <pull-request-id> --file <path> --line <line> --message <text>` with a non-integer or non-positive line value
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps write request

#### Scenario: Inline thread creation rejects unsupported target shape
- **WHEN** the user runs `adomi ado pr comment <pull-request-id> --file <path> --line <line> --message <text>` and Azure DevOps iteration changes do not contain a supported right-side changed file matching the path
- **THEN** the command exits non-zero before creating a thread and reports that inline comments can only target supported changed files in the latest pull request version

#### Scenario: PR-level thread creation requires one message source
- **WHEN** the user runs `adomi ado pr comment <pull-request-id>` without exactly one of `--message-file` or `--message`
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps write request

#### Scenario: PR-level thread creation rejects existing-thread flag
- **WHEN** the user runs `adomi ado pr comment <pull-request-id> --thread <thread-id> --message <text>`
- **THEN** the command rejects the unsupported argument, exits non-zero, and does so before loading credentials or making any Azure DevOps write request

#### Scenario: PR-level thread creation missing repository ID
- **WHEN** `adomi ado pr comment <pull-request-id> --message <text>` fetches a pull request response that does not include a repository ID
- **THEN** the command exits non-zero before creating a thread

#### Scenario: PR-level thread creation response missing thread ID
- **WHEN** Azure DevOps accepts a PR-level thread creation request but the response does not include a positive thread ID
- **THEN** the command exits non-zero without printing success data to stdout

#### Scenario: PR-level thread creation JSON output
- **WHEN** `adomi ado pr comment <pull-request-id> --message <text> --json` succeeds
- **THEN** stdout contains one JSON object describing the pull request ID, created thread ID, initial comment ID when available, and action followed by a newline

#### Scenario: Inline thread creation JSON output
- **WHEN** `adomi ado pr comment <pull-request-id> --file <path> --line <line> --message <text> --json` succeeds
- **THEN** stdout contains one JSON object describing the pull request ID, created thread ID, initial comment ID when available, target file path, target line, and action followed by a newline

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
