## ADDED Requirements

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

## MODIFIED Requirements

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
