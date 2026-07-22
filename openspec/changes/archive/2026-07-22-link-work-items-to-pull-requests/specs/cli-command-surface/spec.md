## ADDED Requirements

### Requirement: Pull request work-item link command
The system SHALL expose deterministic CLI parsing, help, output, and failure behavior for `adomi ado pr link`.

#### Scenario: Link command is discoverable
- **WHEN** the user requests help for `adomi ado pr`
- **THEN** the help output lists `link` alongside the existing `fetch`, `ensure`, `comment`, `reply`, `resolve`, and `reopen` operations

#### Scenario: Link command accepts repeatable work-item flags
- **WHEN** the user runs `adomi ado pr link <pull-request-id>` with one or more `--work-item <work-item-id>` flags plus optional `--profile`, `--global`, and `--json`
- **THEN** the command preserves the requested work-item IDs in input order and uses the selected configuration and credential behavior shared by PR maintenance commands

#### Scenario: Link command validates identifiers before side effects
- **WHEN** the pull request ID or any work-item ID is missing, non-integer, or non-positive
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps request

#### Scenario: Link command rejects duplicate work-item IDs
- **WHEN** the same work-item ID is supplied more than once
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps request and identifies the duplicate ID

#### Scenario: Link command rejects unknown arguments
- **WHEN** the user supplies an unsupported flag, a missing flag value, or an unexpected positional argument
- **THEN** the command exits non-zero before loading credentials or making any Azure DevOps request

#### Scenario: Link command help describes its contract
- **WHEN** the user requests help for `adomi ado pr link`
- **THEN** the command exits successfully, leaves stdout empty, and writes operation-specific help to stderr describing repeatable work-item flags, profile/global/JSON flags, plain and JSON output, idempotent already-linked behavior, work-item write permission, and non-atomic multi-item failures

#### Scenario: Plain link success output
- **WHEN** `adomi ado pr link` succeeds without `--json`
- **THEN** stdout contains every requested work-item ID in original input order, one ID per line, including IDs that were already linked, and contains no other result data

#### Scenario: JSON link success output
- **WHEN** `adomi ado pr link` succeeds with `--json`
- **THEN** stdout contains one compact JSON object with the pull request ID, all requested work-item IDs, newly linked IDs, already-linked IDs, and action `linked` or `unchanged`, followed by a newline

#### Scenario: Link failure leaves stdout empty
- **WHEN** link validation, preflight, authentication, network access, decoding, revision testing, or any relation update fails
- **THEN** the command exits non-zero and stdout is empty even if an earlier work-item link from the same invocation was already persisted
