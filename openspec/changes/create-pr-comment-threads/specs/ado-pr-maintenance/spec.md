## ADDED Requirements

### Requirement: PR-level thread creation
The system SHALL allow users to create a new top-level Azure DevOps pull request comment thread without changing pull request approval, completion, or reviewer state.

#### Scenario: Create PR-level thread from message file
- **WHEN** the user runs `adomi ado pr comment <pull-request-id> --message-file <path>` with a readable non-empty message file, valid configuration, and valid credentials
- **THEN** the system fetches the pull request to resolve its repository ID, creates a new active PR-level text comment thread using the file contents as the initial comment, and prints only the created thread ID followed by a newline to stdout

#### Scenario: Create PR-level thread from inline message
- **WHEN** the user runs `adomi ado pr comment <pull-request-id> --message <text>` with non-empty message text, valid configuration, and valid credentials
- **THEN** the system fetches the pull request to resolve its repository ID, creates a new active PR-level text comment thread using the message text as the initial comment, and prints only the created thread ID followed by a newline to stdout

#### Scenario: PR-level thread creation requires one message source
- **WHEN** the user runs `adomi ado pr comment <pull-request-id>` without exactly one of `--message-file` or `--message`
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps write request

#### Scenario: PR-level thread creation rejects inline context flags in Phase 1
- **WHEN** the user runs `adomi ado pr comment <pull-request-id> --file <path> --line <line> --message <text>` or provides another inline file/line context flag reserved for future inline review threads
- **THEN** the command rejects the unsupported argument, exits non-zero, and does so before loading credentials or making any Azure DevOps write request

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
