## MODIFIED Requirements

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
