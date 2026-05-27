## ADDED Requirements

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
