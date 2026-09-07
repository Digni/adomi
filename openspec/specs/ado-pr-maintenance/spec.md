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
The system SHALL expose pull request completion, auto-completion, auto-complete cancellation, abandonment, approval, approval with suggestions, and rejection only through their explicit commands and SHALL NOT expose arbitrary reviewer management, vote reset, wait-for-author voting, policy bypass, abandoned-pull-request reactivation, or completed-pull-request reversion behavior as part of PR maintenance.

#### Scenario: Supported lifecycle actions are discoverable
- **WHEN** the user requests help for `adomi ado pr`
- **THEN** the help output lists `complete`, `auto-complete`, `cancel-auto-complete`, `abandon`, `approve`, `approve-with-suggestions`, and `reject` alongside the existing supported PR operations and describes them as explicit governance writes

#### Scenario: Unsupported governance action is unavailable
- **WHEN** the user runs `adomi ado pr reset-vote`, `wait-for-author`, `reviewer`, `bypass`, `reactivate`, `revert`, or another unsupported PR governance command
- **THEN** the command exits non-zero and does not make any Azure DevOps write request

#### Scenario: Unknown PR maintenance command
- **WHEN** the user runs an unrecognized `adomi ado pr` operation that is not one of the supported context, maintenance, lifecycle, or vote commands
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

### Requirement: Immediate pull request completion
The system SHALL allow users to explicitly request immediate completion of an existing active, non-draft Azure DevOps pull request by ID, SHALL bind the request to the source commit returned by the preflight read, and SHALL report success only when Azure DevOps returns the requested pull request in the completed state.

#### Scenario: Complete an active pull request
- **WHEN** the user runs `adomi ado pr complete <pull-request-id>` for an active, non-draft pull request whose preflight response includes a repository ID and source commit ID, with valid configuration, credentials, and completion permissions
- **THEN** the system requests status `completed` for that repository-scoped pull request using the fetched source commit, preserves the supported completion preferences not overridden by the user, and prints only the pull request ID followed by a newline to stdout after the response reports status `completed`

#### Scenario: Completion is queued asynchronously
- **WHEN** Azure DevOps accepts `adomi ado pr complete <pull-request-id>`, returns pull request status `completed`, and reports a merge status that is not yet terminal
- **THEN** the command succeeds without polling, plain stdout contains only the pull request ID, and `--json` exposes the returned merge status without claiming that the merge has already succeeded

#### Scenario: Already completed pull request
- **WHEN** `adomi ado pr complete <pull-request-id>` preflights a pull request whose status is already `completed`
- **THEN** the command performs no update, treats the request as an idempotent success, and reports action `unchanged`

#### Scenario: Abandoned pull request cannot be completed
- **WHEN** `adomi ado pr complete <pull-request-id>` preflights a pull request whose status is `abandoned`
- **THEN** the command exits non-zero before making a pull request update and leaves stdout empty

#### Scenario: Draft pull request cannot be completed
- **WHEN** `adomi ado pr complete <pull-request-id>` preflights an active draft pull request
- **THEN** the command exits non-zero before making a pull request update and leaves stdout empty

#### Scenario: Completion preflight lacks required identity
- **WHEN** `adomi ado pr complete <pull-request-id>` receives a pull request without a repository ID or current source commit ID
- **THEN** the command exits non-zero before making a pull request update and leaves stdout empty

#### Scenario: Source commit changes before completion
- **WHEN** Azure DevOps rejects the completion update because the fetched source commit no longer matches the pull request source
- **THEN** the command exits non-zero, does not retry against the newer source commit, and leaves stdout empty

#### Scenario: Completion response does not prove the requested state
- **WHEN** Azure DevOps returns a successful HTTP response for completion but the response has a different pull request ID or does not report status `completed`
- **THEN** the command exits non-zero and does not print success data to stdout

### Requirement: Policy-gated pull request auto-completion
The system SHALL allow users to explicitly schedule an existing active, non-draft Azure DevOps pull request for auto-completion under its branch policies, SHALL identify the authenticated caller as the auto-complete setter, and SHALL distinguish an active scheduled pull request from one that Azure DevOps completes immediately.

#### Scenario: Enable auto-complete on an active pull request
- **WHEN** the user runs `adomi ado pr auto-complete <pull-request-id>` for an active, non-draft pull request that is not already scheduled, and the pull request and authenticated-user preflight responses contain their required IDs
- **THEN** the system sets auto-complete to the authenticated user's identity with policy-respecting completion preferences and succeeds only when the response either remains active with auto-complete set or is already completed

#### Scenario: Auto-complete finishes immediately
- **WHEN** Azure DevOps completes the pull request while processing `adomi ado pr auto-complete <pull-request-id>` because all requirements are already satisfied
- **THEN** the command treats the completed response as success, reports the returned completed and merge states in JSON, and does not imply that the pull request remains scheduled

#### Scenario: Auto-complete remains scheduled
- **WHEN** Azure DevOps accepts `adomi ado pr auto-complete <pull-request-id>` and returns an active pull request with an auto-complete setter
- **THEN** the command succeeds without polling for policy completion and `--json` reports that auto-complete is enabled

#### Scenario: Auto-complete is already enabled without preference or policy-safety changes
- **WHEN** `adomi ado pr auto-complete <pull-request-id>` preflights an active, non-draft pull request with auto-complete already set, the user supplied no completion preference flags, and the fetched completion options contain neither policy bypass nor optional-policy-ignore IDs
- **THEN** the command performs no update and reports action `unchanged`

#### Scenario: Sanitize policy overrides on an existing auto-complete schedule
- **WHEN** `adomi ado pr auto-complete <pull-request-id>` preflights an active, non-draft pull request with auto-complete already set and fetched completion options that enable policy bypass or contain optional-policy-ignore IDs
- **THEN** the system performs a preference-only update that explicitly disables bypass and clears optional-policy-ignore IDs, preserves the supported completion preferences, and does not resolve or replace the existing setter

#### Scenario: Update preferences for an already scheduled pull request
- **WHEN** `adomi ado pr auto-complete <pull-request-id>` preflights an active, non-draft pull request with auto-complete already set and the user supplies supported completion preferences that differ from the fetched values
- **THEN** the system updates only the supported completion preferences and reports the returned lifecycle state

#### Scenario: Auto-complete identity is unavailable
- **WHEN** the authenticated-user preflight response does not contain a non-empty identity ID for a new auto-complete request
- **THEN** the command exits non-zero before setting auto-complete and leaves stdout empty

#### Scenario: Terminal or draft pull request cannot be scheduled
- **WHEN** `adomi ado pr auto-complete <pull-request-id>` preflights a completed, abandoned, or draft pull request
- **THEN** the command exits non-zero before making a pull request update and leaves stdout empty

#### Scenario: Auto-complete response does not prove scheduling or completion
- **WHEN** Azure DevOps returns a successful HTTP response whose pull request ID differs from the request, or whose state is neither active with auto-complete set nor completed
- **THEN** the command exits non-zero and does not print success data to stdout

### Requirement: Auto-complete cancellation
The system SHALL allow users to explicitly cancel auto-complete for an existing active Azure DevOps pull request and SHALL report success only when the returned pull request no longer has auto-complete set.

#### Scenario: Cancel scheduled auto-complete
- **WHEN** the user runs `adomi ado pr cancel-auto-complete <pull-request-id>` for an active pull request with auto-complete set and a valid repository ID
- **THEN** the system clears the auto-complete setter on that repository-scoped pull request and prints only the pull request ID followed by a newline after the response proves auto-complete is no longer set

#### Scenario: Auto-complete is already disabled
- **WHEN** `adomi ado pr cancel-auto-complete <pull-request-id>` preflights an active pull request without an auto-complete setter
- **THEN** the command performs no update, treats the request as an idempotent success, and reports action `unchanged`

#### Scenario: Terminal pull request cannot cancel auto-complete
- **WHEN** `adomi ado pr cancel-auto-complete <pull-request-id>` preflights a completed or abandoned pull request
- **THEN** the command exits non-zero before making a pull request update and leaves stdout empty

#### Scenario: Cancellation response does not prove the requested state
- **WHEN** Azure DevOps returns a successful HTTP response whose pull request ID differs from the request, whose status is not active, or whose auto-complete setter remains present
- **THEN** the command exits non-zero and does not print success data to stdout

### Requirement: Pull request abandonment
The system SHALL allow users to explicitly abandon an existing active Azure DevOps pull request by ID without merging it and SHALL report success only when the returned pull request state proves abandonment.

#### Scenario: Abandon an active pull request
- **WHEN** the user runs `adomi ado pr abandon <pull-request-id>` for an active pull request whose preflight response includes a repository ID, with valid configuration, credentials, and permissions
- **THEN** the system updates that repository-scoped pull request to status `abandoned` and prints only the pull request ID followed by a newline after the response reports status `abandoned`

#### Scenario: Abandon a pull request with auto-complete enabled
- **WHEN** the user runs `adomi ado pr abandon <pull-request-id>` for an active pull request that is scheduled for auto-completion
- **THEN** the abandonment closes the pull request without merging, and the command does not issue a separate auto-complete cancellation request

#### Scenario: Already abandoned pull request
- **WHEN** `adomi ado pr abandon <pull-request-id>` preflights a pull request whose status is already `abandoned`
- **THEN** the command performs no update, treats the request as an idempotent success, and reports action `unchanged`

#### Scenario: Completed pull request cannot be abandoned
- **WHEN** `adomi ado pr abandon <pull-request-id>` preflights a pull request whose status is `completed`
- **THEN** the command exits non-zero before making a pull request update and leaves stdout empty

#### Scenario: Abandonment response does not prove the requested state
- **WHEN** Azure DevOps returns a successful HTTP response for abandonment but the response has a different pull request ID or does not report status `abandoned`
- **THEN** the command exits non-zero and does not print success data to stdout

### Requirement: Authenticated-user pull request votes
The system SHALL allow users to cast an Azure DevOps reviewer vote as the authenticated caller through explicit approve, approve-with-suggestions, and reject commands, SHALL preserve the caller's existing required-reviewer designation, and SHALL NOT accept an arbitrary reviewer identity.

#### Scenario: Approve a pull request
- **WHEN** the user runs `adomi ado pr approve <pull-request-id>` for an active pull request with valid repository and authenticated-user identities
- **THEN** the system casts vote `10` for the authenticated user and prints only the pull request ID followed by a newline after the reviewer response contains the same identity and vote

#### Scenario: Approve a pull request with suggestions
- **WHEN** the user runs `adomi ado pr approve-with-suggestions <pull-request-id>` for an active pull request with valid repository and authenticated-user identities
- **THEN** the system casts vote `5` for the authenticated user and prints only the pull request ID followed by a newline after the reviewer response contains the same identity and vote

#### Scenario: Reject a pull request
- **WHEN** the user runs `adomi ado pr reject <pull-request-id>` for an active pull request with valid repository and authenticated-user identities
- **THEN** the system casts vote `-10` for the authenticated user and prints only the pull request ID followed by a newline after the reviewer response contains the same identity and vote

#### Scenario: Caller is not yet a reviewer
- **WHEN** the authenticated user casts a supported vote but is not present in the fetched pull request reviewer list
- **THEN** the system uses the Azure DevOps cast-vote operation to add only that authenticated caller as a non-required reviewer with the requested vote

#### Scenario: Caller is a required reviewer
- **WHEN** the authenticated user already appears as a required reviewer on the pull request and casts a supported vote
- **THEN** the vote update preserves the user's required-reviewer designation

#### Scenario: Requested vote is already set
- **WHEN** the authenticated user already has the vote represented by the invoked approve, approve-with-suggestions, or reject command
- **THEN** the command performs no vote update, treats the request as an idempotent success, and reports action `unchanged`

#### Scenario: Terminal pull request cannot be voted on
- **WHEN** a supported vote command preflights a completed or abandoned pull request
- **THEN** the command exits non-zero before making a reviewer update and leaves stdout empty

#### Scenario: Authenticated voter identity is unavailable
- **WHEN** the authenticated-user preflight response does not contain a non-empty identity ID
- **THEN** the vote command exits non-zero before making a reviewer update and leaves stdout empty

#### Scenario: Vote response does not prove the requested vote
- **WHEN** Azure DevOps returns a successful HTTP response whose reviewer identity differs from the authenticated user or whose vote differs from the requested value
- **THEN** the command exits non-zero and does not print success data to stdout

#### Scenario: Vote does not complete or abandon the pull request
- **WHEN** an approve, approve-with-suggestions, or reject command succeeds
- **THEN** the command changes only the authenticated caller's reviewer vote and does not request pull request completion, abandonment, auto-complete, policy bypass, or another reviewer's mutation

### Requirement: Policy-respecting completion preferences
The system SHALL let users explicitly control supported Azure DevOps completion preferences for immediate and automatic completion, SHALL preserve fetched supported values for omitted preferences, and SHALL NOT request policy bypass or invent a local default merge strategy.

#### Scenario: Supply supported completion preferences
- **WHEN** the user runs `adomi ado pr complete <pull-request-id>` or `adomi ado pr auto-complete <pull-request-id>` with `--merge-strategy <strategy>`, `--delete-source-branch <true|false>`, `--transition-work-items <true|false>`, or `--merge-commit-message <text>`
- **THEN** the system applies only the explicitly supplied values over the fetched supported completion preferences before sending the lifecycle update

#### Scenario: Accepted merge strategies
- **WHEN** the user supplies `--merge-strategy` with `no-fast-forward`, `squash`, `rebase`, or `rebase-merge`
- **THEN** the command maps the value to the corresponding Azure DevOps merge strategy without changing any other omitted preference

#### Scenario: Invalid completion preference
- **WHEN** the user supplies an unsupported merge strategy, a non-boolean value for a completion boolean flag, an empty merge commit message, or a completion preference flag to `adomi ado pr abandon`
- **THEN** the command exits non-zero before loading credentials or making an Azure DevOps request

#### Scenario: Omitted completion preferences
- **WHEN** a complete or auto-complete command omits one or more supported completion preference flags
- **THEN** the system preserves the corresponding fetched supported values when present and otherwise leaves them unspecified for Azure DevOps to resolve under repository policy

#### Scenario: Preserve a legacy squash preference
- **WHEN** a complete or auto-complete command omits `--merge-strategy` and the fetched completion options contain deprecated `squashMerge: true` without an explicit merge strategy
- **THEN** the system preserves the user-visible intent by sending explicit merge strategy `squash` and does not send the deprecated field

#### Scenario: Existing bypass preference is not used
- **WHEN** a pull request was previously configured outside Adomi with policy-bypass or optional-policy-ignore completion settings
- **THEN** an Adomi complete or auto-complete request sends `bypassPolicy: false` and an empty `autoCompleteIgnoreConfigIds` list in a freshly constructed completion-options object, does not copy the stored bypass reason, and remains subject to all branch policies
