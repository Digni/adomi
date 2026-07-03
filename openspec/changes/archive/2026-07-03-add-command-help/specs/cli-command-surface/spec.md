## ADDED Requirements

### Requirement: Command-specific action help
The system SHALL provide side-effect-free command-specific help for public action commands so users and AI agents can inspect supported usage, required arguments, option constraints, output contracts, and conservative write boundaries before executing an operation.

#### Scenario: PR ensure help
- **WHEN** the user requests help for `adomi ado pr ensure`
- **THEN** the command exits successfully, leaves stdout empty, writes help to stderr, and describes creating/updating the active pull request, required `--title` behavior for creates, optional source/target/repository/profile/global/json/description flags, plain stdout result shape, and the absence of approval/merge/reviewer-governance actions

#### Scenario: PR comment help
- **WHEN** the user requests help for `adomi ado pr comment`
- **THEN** the command exits successfully, leaves stdout empty, writes help to stderr, and describes PR-level versus inline comment thread creation, the exactly-one message source rule, paired `--file` and `--line` inline target flags, optional profile/global/json flags, and the created-thread stdout contract

#### Scenario: PR thread help
- **WHEN** the user requests help for `adomi ado pr reply`, `adomi ado pr resolve`, or `adomi ado pr reopen`
- **THEN** each command exits successfully, leaves stdout empty, writes operation-specific help to stderr, and describes required pull request/thread identifiers, valid message-source rules where applicable, optional profile/global/json flags, and the plain stdout result shape

#### Scenario: Work item command help
- **WHEN** the user requests help for `adomi ado comment` or `adomi ado work-item comment`
- **THEN** the command exits successfully, leaves stdout empty, writes help to stderr, and describes the positive work item ID requirement, exactly-one message source rule, optional profile/global/json flags, created-comment stdout contract, and the absence of unsupported work item mutations

#### Scenario: Fetch, credential, and config command help
- **WHEN** the user requests help for `adomi ado fetch`, `adomi ado pr fetch`, `adomi ado login`, `adomi ado logout`, `adomi ado profiles list`, `adomi config init`, or the hidden compatibility alias `adomi ado config init`
- **THEN** the command exits successfully, leaves stdout empty, writes command-specific help to stderr, and describes the command's required arguments, relevant flags, and stdout behavior without loading credentials or contacting Azure DevOps

#### Scenario: Help is side-effect free
- **WHEN** a command-specific help request includes `--help` or `-h`
- **THEN** the command returns help before reading message or description files, resolving Git repository state, loading configuration, reading or writing credentials, creating HTTP clients, or making Azure DevOps requests

#### Scenario: Non-help invalid arguments preserve existing validation
- **WHEN** a public action command receives invalid arguments that are not a help request
- **THEN** the command exits non-zero, keeps stdout empty, reports the validation error on stderr or through the returned error as before, and does not make an Azure DevOps write request
