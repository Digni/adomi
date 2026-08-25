## ADDED Requirements

### Requirement: Pull request lifecycle and vote command syntax
The system SHALL expose each supported pull request lifecycle transition and authenticated-user vote as an explicit `adomi ado pr` subcommand, SHALL accept completion preferences only on completion commands, and SHALL reject malformed invocations before credential or network access.

#### Scenario: Immediate completion command syntax
- **WHEN** the user runs `adomi ado pr complete <pull-request-id>` with optional `--merge-strategy <strategy>`, `--delete-source-branch <true|false>`, `--transition-work-items <true|false>`, `--merge-commit-message <text>`, `--profile <name>`, `--global`, or `--json`
- **THEN** the command accepts exactly one positive pull request ID and sends the validated completion request through the selected Azure DevOps profile

#### Scenario: Auto-completion command syntax
- **WHEN** the user runs `adomi ado pr auto-complete <pull-request-id>` with optional `--merge-strategy <strategy>`, `--delete-source-branch <true|false>`, `--transition-work-items <true|false>`, `--merge-commit-message <text>`, `--profile <name>`, `--global`, or `--json`
- **THEN** the command accepts exactly one positive pull request ID and sends the validated auto-completion request through the selected Azure DevOps profile

#### Scenario: State-only lifecycle command syntax
- **WHEN** the user runs `adomi ado pr cancel-auto-complete <pull-request-id>` or `adomi ado pr abandon <pull-request-id>` with optional `--profile <name>`, `--global`, or `--json`
- **THEN** the command accepts exactly one positive pull request ID and does not accept completion preference flags

#### Scenario: Authenticated-user vote command syntax
- **WHEN** the user runs `adomi ado pr approve <pull-request-id>`, `adomi ado pr approve-with-suggestions <pull-request-id>`, or `adomi ado pr reject <pull-request-id>` with optional `--profile <name>`, `--global`, or `--json`
- **THEN** the command accepts exactly one positive pull request ID and does not accept a reviewer identity or raw vote value

#### Scenario: Invalid lifecycle or vote invocation
- **WHEN** a lifecycle or vote command receives a missing, zero, negative, non-numeric, or extra pull request ID; an unknown flag; a completion preference on an unsupported command; or conflicting profile selection flags
- **THEN** the command exits non-zero before loading credentials or making an Azure DevOps request and leaves stdout empty

### Requirement: Pull request lifecycle and vote output
The system SHALL preserve Adomi's script-safe stdout contract for pull request governance commands and SHALL expose the returned state needed to distinguish completion, scheduling, cancellation, abandonment, voting, and idempotent outcomes in JSON mode.

#### Scenario: Plain lifecycle or vote success
- **WHEN** a supported pull request lifecycle or vote command succeeds without `--json`
- **THEN** stdout contains only the pull request ID followed by a newline

#### Scenario: JSON lifecycle success
- **WHEN** a completion, auto-completion, cancellation, or abandonment command succeeds with `--json`
- **THEN** stdout contains exactly one compact JSON object followed by a newline with `pullRequestId`, `action`, and `status`, plus returned `mergeStatus` or `autoCompleteEnabled` fields when they apply

#### Scenario: JSON vote success
- **WHEN** an approve, approve-with-suggestions, or reject command succeeds with `--json`
- **THEN** stdout contains exactly one compact JSON object followed by a newline with `pullRequestId`, `action`, `status`, `reviewerId`, and the exact numeric `vote`

#### Scenario: Idempotent JSON success
- **WHEN** the requested lifecycle state or authenticated-user vote already exists and the command succeeds with `--json`
- **THEN** the JSON result reports action `unchanged` and the fetched current state without claiming that a write occurred

#### Scenario: Governance command failure
- **WHEN** argument validation, configuration, authentication, preflight, Azure DevOps mutation, or response-state validation fails
- **THEN** the command exits non-zero, reports the error through the error or stderr path, and leaves stdout empty

### Requirement: Pull request lifecycle and vote help
The system SHALL provide side-effect-free help for every supported pull request lifecycle and vote command and SHALL identify the commands as explicit Azure DevOps writes.

#### Scenario: Lifecycle command help
- **WHEN** the user requests help for `complete`, `auto-complete`, `cancel-auto-complete`, or `abandon`
- **THEN** the help names the required pull request ID, supported flags, resulting state, asynchronous behavior where applicable, policy-respecting boundary, and relevant terminal-state restrictions without loading credentials or contacting Azure DevOps

#### Scenario: Vote command help
- **WHEN** the user requests help for `approve`, `approve-with-suggestions`, or `reject`
- **THEN** the help states that the command votes only as the authenticated user, identifies the vote meaning, and does not offer arbitrary reviewer selection without loading credentials or contacting Azure DevOps

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

#### Scenario: Work item comment command
- **WHEN** the user runs `adomi ado comment <work-item-id>` with one valid message source, valid configuration, and valid credentials
- **THEN** the command adds a new text comment to the requested Azure DevOps work item

#### Scenario: Work item comment namespace alias
- **WHEN** the user runs `adomi ado work-item comment <work-item-id>` with one valid message source, valid configuration, and valid credentials
- **THEN** the command behaves the same as `adomi ado comment <work-item-id>`

#### Scenario: Work item namespace help lists supported operations
- **WHEN** the user requests help for `adomi ado work-item`
- **THEN** the help output lists `comment` as the supported work item operation and does not list unsupported work item mutation commands

#### Scenario: Fetch pull request comments
- **WHEN** the user runs `adomi ado pr fetch <pull-request-id>` with valid configuration and credentials
- **THEN** the command fetches the pull request, downloads all comment threads, exports them under the repository-local `.adomi/context/pull-requests/<pull-request-id>/` directory, and prints only the exported directory path to stdout

#### Scenario: Compatibility pull request fetch alias
- **WHEN** the user runs `adomi ado pr <pull-request-id>` with valid configuration and credentials
- **THEN** the command behaves the same as `adomi ado pr fetch <pull-request-id>`

#### Scenario: Pull request namespace help lists supported operations
- **WHEN** the user requests help for `adomi ado pr`
- **THEN** the help output lists the supported `fetch`, `ensure`, `link`, `comment`, `reply`, `resolve`, `reopen`, `complete`, `auto-complete`, `cancel-auto-complete`, `abandon`, `approve`, `approve-with-suggestions`, and `reject` operations

#### Scenario: Pull request ensure command
- **WHEN** the user runs `adomi ado pr ensure` with valid inferred repository context, valid configuration, and valid credentials
- **THEN** the command creates or updates the active pull request for the resolved source and target branches

#### Scenario: Pull request comment command
- **WHEN** the user runs `adomi ado pr comment <pull-request-id>` with one valid message source, optional paired `--file <path> --line <line>` inline target flags, valid configuration, and valid credentials
- **THEN** the command creates a new comment thread on the requested pull request, targeting the PR level when no inline target is provided and the latest changed file version when an inline target is provided

#### Scenario: Pull request reply command
- **WHEN** the user runs `adomi ado pr reply <pull-request-id> --thread <thread-id>` with one valid message source, valid configuration, and valid credentials
- **THEN** the command creates a reply comment on the requested pull request thread

#### Scenario: Pull request resolve command
- **WHEN** the user runs `adomi ado pr resolve <pull-request-id> --thread <thread-id>` with valid configuration and credentials
- **THEN** the command marks the requested pull request thread fixed

#### Scenario: Pull request reopen command
- **WHEN** the user runs `adomi ado pr reopen <pull-request-id> --thread <thread-id>` with valid configuration and credentials
- **THEN** the command marks the requested pull request thread active

#### Scenario: Complete pull request command
- **WHEN** the user runs `adomi ado pr complete <pull-request-id>` with valid arguments, configuration, credentials, preflight state, and permissions
- **THEN** the command requests immediate completion of that pull request through the repository-scoped Azure DevOps Git API

#### Scenario: Auto-complete pull request commands
- **WHEN** the user runs `adomi ado pr auto-complete <pull-request-id>` or `adomi ado pr cancel-auto-complete <pull-request-id>` with valid arguments, configuration, credentials, preflight state, and permissions
- **THEN** the command explicitly enables or cancels policy-gated auto-completion for that pull request

#### Scenario: Abandon pull request command
- **WHEN** the user runs `adomi ado pr abandon <pull-request-id>` with valid arguments, configuration, credentials, preflight state, and permissions
- **THEN** the command closes the pull request without merging it

#### Scenario: Authenticated-user pull request vote commands
- **WHEN** the user runs `adomi ado pr approve <pull-request-id>`, `adomi ado pr approve-with-suggestions <pull-request-id>`, or `adomi ado pr reject <pull-request-id>` with valid arguments, configuration, credentials, preflight state, and permissions
- **THEN** the command casts the corresponding vote only for the authenticated Azure DevOps user

#### Scenario: Export metadata records Azure DevOps source context
- **WHEN** `adomi ado fetch <work-item-id>`, `adomi ado pr fetch <pull-request-id>`, or `adomi ado pr <pull-request-id>` succeeds
- **THEN** the exported `index.json` records the Azure DevOps source, profile, and project metadata used for the fetch

#### Scenario: Manage credentials
- **WHEN** the user runs `adomi ado login` or `adomi ado logout` with valid credential flags
- **THEN** the command stores or deletes credentials using the same profile and `patRef` behavior as before

#### Scenario: List profiles
- **WHEN** the user runs `adomi ado profiles list`
- **THEN** the command lists configured profile names in sorted order as before
