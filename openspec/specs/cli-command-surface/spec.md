# cli-command-surface Specification

## Purpose
Define the public command layout and stream behavior for the `adomi` CLI.
## Requirements
### Requirement: Cobra-backed root command

The system SHALL expose the `adomi` CLI through a Cobra command tree while preserving the programmatic `Run(args, stdin, stdout, stderr)` execution boundary.

#### Scenario: Root command without arguments
- **WHEN** the user runs `adomi` without a command
- **THEN** the command exits non-zero and reports usage for the available top-level commands on stderr

#### Scenario: Unknown top-level command
- **WHEN** the user runs `adomi unknown`
- **THEN** the command exits non-zero and reports that the command is unknown

### Requirement: Top-level config command

The system SHALL support configuration initialization through `adomi config init`.

#### Scenario: Initialize repository config
- **WHEN** the user runs `adomi config init` inside a repository without an existing repo config
- **THEN** the command creates the same repository config template currently produced by config initialization and prints the created config path

#### Scenario: Initialize global config
- **WHEN** the user runs `adomi config init --global`
- **THEN** the command creates the same global config template currently produced by global config initialization and prints the created config path

#### Scenario: Refuse to overwrite config
- **WHEN** the user runs `adomi config init` for a target config path that already exists
- **THEN** the command exits non-zero and does not modify the existing file

### Requirement: Hidden compatibility for old config route

The system SHALL keep `adomi ado config init` as a hidden compatibility alias for `adomi config init` during this migration.

#### Scenario: Legacy repository config command
- **WHEN** the user runs `adomi ado config init`
- **THEN** the command behaves the same as `adomi config init`

#### Scenario: Legacy global config command
- **WHEN** the user runs `adomi ado config init --global`
- **THEN** the command behaves the same as `adomi config init --global`

#### Scenario: Help output omits legacy config route
- **WHEN** the user requests help for `adomi ado`
- **THEN** the help output does not list `config` as an Azure DevOps subcommand

### Requirement: Agent skill command namespace

The system SHALL expose agent-oriented skill creation through the top-level `adomi agent skill` command, SHALL provide named coding-agent provider selection, and SHALL preserve the existing `Run(args, stdin, stdout, stderr)` execution boundary.

#### Scenario: Agent namespace appears in top-level help
- **WHEN** the user runs `adomi` without a command
- **THEN** the command exits non-zero and reports usage that includes the `agent` top-level command on stderr

#### Scenario: Agent skill help describes scope and provider flags
- **WHEN** the user requests help for `adomi agent skill`
- **THEN** the help output describes the default global scope, the `--global` scope, the `--project` scope, the `--provider` selector with all supported values, the shared `.agents/skills` mapping, and the backward-compatible `--claude` target override

#### Scenario: Named provider is accepted
- **WHEN** the user runs `adomi agent skill --provider <provider>` with a supported provider and otherwise valid arguments
- **THEN** the command resolves the documented provider target and preserves the existing success stream behavior

#### Scenario: Unsupported provider is rejected through the command boundary
- **WHEN** the user runs `adomi agent skill --provider <unknown>`
- **THEN** the command exits non-zero, reports the unsupported provider through the error or stderr path, and leaves stdout empty

#### Scenario: Explicit and legacy provider selectors conflict
- **WHEN** the user runs `adomi agent skill --provider <provider> --claude`
- **THEN** the command exits non-zero, reports the conflicting selectors through the error or stderr path, and leaves stdout empty

#### Scenario: Existing Azure DevOps namespace remains available
- **WHEN** the user runs `adomi ado` without an Azure DevOps subcommand
- **THEN** the command exits non-zero and reports usage for the existing Azure DevOps subcommands

#### Scenario: Existing config namespace remains available
- **WHEN** the user runs `adomi config init` with valid arguments
- **THEN** the command behavior remains the same as before the agent command was added

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
- **THEN** the help output lists the supported `fetch`, `ensure`, `comment`, `reply`, `resolve`, and `reopen` operations

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

#### Scenario: Export metadata records Azure DevOps source context
- **WHEN** `adomi ado fetch <work-item-id>`, `adomi ado pr fetch <pull-request-id>`, or `adomi ado pr <pull-request-id>` succeeds
- **THEN** the exported `index.json` records the Azure DevOps source, profile, and project metadata used for the fetch

#### Scenario: Manage credentials
- **WHEN** the user runs `adomi ado login` or `adomi ado logout` with valid credential flags
- **THEN** the command stores or deletes credentials using the same profile and `patRef` behavior as before

#### Scenario: List profiles
- **WHEN** the user runs `adomi ado profiles list`
- **THEN** the command lists configured profile names in sorted order as before

### Requirement: Work item fetch downloads attachments

The system SHALL download Azure DevOps work item attachments for every work item included in a successful `adomi ado fetch <work-item-id>` export and SHALL expose each exported work item's attachment directory in the export index.

#### Scenario: Fetch downloads requested work item attachments
- **WHEN** the user runs `adomi ado fetch <work-item-id>` with valid configuration and credentials and the requested work item has attachment relations
- **THEN** the command downloads those attachments under `.adomi/context/work-items/<work-item-id>/attachments/<work-item-id>/` and prints only the exported directory path to stdout

#### Scenario: Fetch downloads parent-chain work item attachments
- **WHEN** the requested Azure DevOps work item has a parent chain and any fetched parent work item has attachment relations
- **THEN** the command downloads the parent work item's attachments under `.adomi/context/work-items/<requested-work-item-id>/attachments/<parent-work-item-id>/`

#### Scenario: Fetch downloads direct child work item attachments
- **WHEN** the requested Azure DevOps work item has direct child relations and any fetched direct child work item has attachment relations
- **THEN** the command downloads the child work item's attachments under `.adomi/context/work-items/<requested-work-item-id>/attachments/<child-work-item-id>/`

#### Scenario: Fetch index exposes attachment paths
- **WHEN** `adomi ado fetch <work-item-id>` exports work item context
- **THEN** `index.json` includes each exported work item's relative POSIX attachment directory path as `attachments/<exported-work-item-id>` with no trailing slash

#### Scenario: Work item without attachments does not require attachment files
- **WHEN** an exported work item has no attachment relations
- **THEN** the command does not create attachment files for that work item and the rest of the work item context export can still succeed

#### Scenario: Attachment downloads complete before final metadata is written
- **WHEN** `adomi ado fetch <work-item-id>` exports work item context for work items with attachment relations
- **THEN** the command completes attachment downloads before writing final item, HTML, tree, and index export artifacts for the successful fetch

#### Scenario: Attachment download failure leaves stdout empty
- **WHEN** `adomi ado fetch <work-item-id>` cannot download an attachment for any exported work item because Azure DevOps returns an error response or a network error occurs
- **THEN** the command exits non-zero and stdout is empty

### Requirement: CLI stream behavior

The system SHALL keep stdout reserved for successful command result data. stderr SHALL carry prompts, help, usage, and errors, and MAY additionally carry status, progress, confirmation, and summary lines as defined by the `cli-feedback` capability. Result data (paths, IDs, JSON result objects, profile lists) SHALL never be written to stderr; feedback lines SHALL never be written to stdout.

#### Scenario: Successful fetch stdout
- **WHEN** `adomi ado fetch <work-item-id>` succeeds
- **THEN** stdout contains only the exported path followed by a newline

#### Scenario: Work item comment success stdout
- **WHEN** `adomi ado comment <work-item-id>` succeeds without `--json`
- **THEN** stdout contains only the created work item comment ID followed by a newline

#### Scenario: Work item comment alias success stdout
- **WHEN** `adomi ado work-item comment <work-item-id>` succeeds without `--json`
- **THEN** stdout contains only the created work item comment ID followed by a newline

#### Scenario: Pull request ensure success stdout
- **WHEN** `adomi ado pr ensure` succeeds without `--json`
- **THEN** stdout contains only the pull request ID followed by a newline

#### Scenario: Pull request comment success stdout
- **WHEN** `adomi ado pr comment <pull-request-id>` succeeds without `--json`
- **THEN** stdout contains only the created thread ID followed by a newline

#### Scenario: Pull request reply success stdout
- **WHEN** `adomi ado pr reply <pull-request-id> --thread <thread-id>` succeeds without `--json`
- **THEN** stdout contains only the created comment ID followed by a newline

#### Scenario: Pull request resolve success stdout
- **WHEN** `adomi ado pr resolve <pull-request-id> --thread <thread-id>` succeeds without `--json`
- **THEN** stdout contains only the thread ID followed by a newline

#### Scenario: Pull request reopen success stdout
- **WHEN** `adomi ado pr reopen <pull-request-id> --thread <thread-id>` succeeds without `--json`
- **THEN** stdout contains only the thread ID followed by a newline

#### Scenario: Work item write JSON stdout
- **WHEN** a work item comment command succeeds with `--json`
- **THEN** stdout contains one JSON object followed by a newline and stderr contains no result data

#### Scenario: Pull request write JSON stdout
- **WHEN** a pull request maintenance command succeeds with `--json`
- **THEN** stdout contains one JSON object followed by a newline and stderr contains no result data

#### Scenario: Work item write validation error leaves stdout empty
- **WHEN** a work item comment command receives invalid arguments
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Pull request write validation error leaves stdout empty
- **WHEN** a pull request maintenance command receives invalid arguments or cannot infer required repository context
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Work item write network error leaves stdout empty
- **WHEN** a work item comment command receives an Azure DevOps error response or network error
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Pull request write network error leaves stdout empty
- **WHEN** a pull request maintenance command receives an Azure DevOps error response or network error
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Agent skill success stdout
- **WHEN** the user successfully runs `adomi agent skill`
- **THEN** stdout contains only the created skill directory path and stderr contains no result data

#### Scenario: Agent skill replacement prompt
- **WHEN** the user runs `adomi agent skill` and the target skill already exists
- **THEN** the replacement confirmation prompt is written to stderr and not stdout

#### Scenario: Agent skill force replacement has no prompt
- **WHEN** the user runs `adomi agent skill --force` and the target skill already exists
- **THEN** no replacement confirmation prompt is written and stdout contains only the replaced skill directory path on success

#### Scenario: Agent skill yes replacement has no prompt
- **WHEN** the user runs `adomi agent skill --yes` and the target skill already exists
- **THEN** no replacement confirmation prompt is written and stdout contains only the replaced skill directory path on success

#### Scenario: Agent skill validation error leaves stdout empty
- **WHEN** the user runs `adomi agent skill --claude --unknown .`
- **THEN** stdout is empty and the command exits non-zero

#### Scenario: Secret prompt
- **WHEN** `adomi ado login` prompts for a PAT
- **THEN** the prompt is written to stderr and not stdout

#### Scenario: Validation error
- **WHEN** a command receives invalid arguments
- **THEN** the command exits non-zero and reports the validation error on stderr

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

### Requirement: Azure DevOps API redirects fail without browser navigation

The system SHALL treat an HTTP redirect returned to a network-backed `adomi ado` request as a failed Azure DevOps API response, SHALL NOT request the redirect target, and SHALL report a concise status-bearing diagnostic through the normal CLI error path without decoding or persisting redirected HTML.

#### Scenario: Invalid token redirects a JSON read to sign-in
- **WHEN** an Azure DevOps JSON read returns an HTTP redirect to an interactive sign-in page because the configured PAT is invalid or expired
- **THEN** the command exits non-zero, leaves stdout empty, reports the original redirect status with authentication or base-URL guidance on stderr, does not report a JSON decode error, and does not request the sign-in target

#### Scenario: Invalid token redirects a JSON write to sign-in
- **WHEN** a work item or pull request maintenance request returns an HTTP redirect to an interactive sign-in page
- **THEN** the command exits non-zero, leaves stdout empty, reports the original redirect status on stderr, does not follow the redirect as a GET or another write, and does not print success data

#### Scenario: Attachment request redirects to sign-in
- **WHEN** an attachment download returns an HTTP redirect instead of attachment bytes
- **THEN** the fetch exits non-zero, leaves stdout empty, does not request the redirect target, and does not write the redirected page as an attachment or successful export artifact

#### Scenario: Redirect diagnostic excludes interactive response content
- **WHEN** an Azure DevOps API response redirects with an HTML body and a `Location` URL
- **THEN** the error includes the original HTTP status but does not include the HTML body or the redirect destination

#### Scenario: Direct authorization failure remains an HTTP error
- **WHEN** Azure DevOps returns a direct 401 or 403 response without a redirect
- **THEN** the command exits non-zero, leaves stdout empty, and reports the returned HTTP status through the existing action-specific error path

#### Scenario: Successful Azure DevOps response remains unchanged
- **WHEN** Azure DevOps returns a valid successful JSON response or documented successful attachment response without a redirect
- **THEN** the command preserves its existing decoding, download, and success-output behavior

### Requirement: Azure DevOps pipeline run commands
The system SHALL expose one-shot read-only pipeline run inspection through `adomi ado pipeline list [--last <N>] [--profile <profile-name>] [--global]` and `adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]`. Without `--last`, `pipeline list` SHALL retain its exact existing behavior of listing `inProgress` runs; with `--last <N>`, it SHALL list the N most recently queued runs across any status.

#### Scenario: Pipeline namespace appears in Azure DevOps help
- **WHEN** the user requests help for `adomi ado`
- **THEN** the help output lists the `pipeline` namespace as read-only Azure DevOps status inspection

#### Scenario: Pipeline command help describes scope and output
- **WHEN** the user requests help for `adomi ado pipeline`, `adomi ado pipeline list`, or `adomi ado pipeline get`
- **THEN** help describes the exact `inProgress` default list scope and the `--last <N>` recent-runs mode across YAML and classic Build pipelines, their best-effort one-shot rather than transactional snapshot semantics, the `--last` range `1..200`, run-ID range `1..2147483647`, profile/global and repository behavior, HTTPS-or-loopback transport requirement, compact JSON fields and nullable result semantics, `vso.build` read scope, and the exclusion of polling, execution detail, mutation, and classic Release deployments

#### Scenario: List in-progress pipeline runs
- **WHEN** the user runs `adomi ado pipeline list` without `--last` inside a repository with valid configuration and credentials
- **THEN** stdout contains exactly one compact JSON object with a `runs` array followed by one newline and stderr contains no success data

#### Scenario: List recent pipeline runs
- **WHEN** the user runs `adomi ado pipeline list --last <N>` with N from `1` through `200` inside a repository with valid configuration and credentials
- **THEN** stdout contains exactly one compact JSON object with a `runs` array of at most N runs across any status followed by one newline and stderr contains no success data

#### Scenario: Get one pipeline run
- **WHEN** the user runs `adomi ado pipeline get <run-id>` with a run ID from `1` through `2147483647` inside a repository with valid configuration and credentials
- **THEN** stdout contains exactly one compact projected pipeline-run JSON object followed by one newline and stderr contains no success data

#### Scenario: Pipeline commands select configuration scope
- **WHEN** the user supplies `--profile <profile-name>` and optionally `--global`
- **THEN** the command resolves the Azure DevOps profile and credential using the same repository/global scope rules as the existing network-backed `adomi ado` commands

#### Scenario: Pipeline commands require a repository
- **WHEN** either pipeline command runs outside a Git repository, including with `--global`
- **THEN** it exits non-zero before loading configuration or credentials

#### Scenario: Pipeline list rejects invalid arguments early
- **WHEN** `pipeline list` receives a positional argument, a value-less `--last` or profile flag, a non-decimal, zero, negative, or above-`200` `--last` value, a `--last` value too large for numeric parsing, a repeated `--last` or profile/global flag, or an unknown argument
- **THEN** the command exits non-zero before loading configuration or credentials and leaves stdout empty

#### Scenario: Pipeline list accepts the `--last` boundaries
- **WHEN** the `--last` value is exactly `1` or exactly `200` and all other arguments are valid
- **THEN** argument validation succeeds with that exact value

#### Scenario: Pipeline get rejects invalid arguments early
- **WHEN** the run ID is missing, non-decimal, zero, negative, greater than `2147483647`, or too large for numeric parsing, a profile flag lacks a value, a profile/global flag is repeated, or an unknown argument is supplied
- **THEN** the command exits non-zero before loading configuration or credentials and leaves stdout empty

#### Scenario: Pipeline get accepts the maximum run ID
- **WHEN** the run ID is exactly `2147483647` and all other arguments are valid
- **THEN** argument validation succeeds with that exact ID independently of machine word size

#### Scenario: Pipeline command failure leaves stdout empty
- **WHEN** repository resolution, configuration, credential lookup, HTTP client construction, Build API access, pagination, response validation, projection, or JSON encoding fails
- **THEN** the command exits non-zero and stdout contains no success data

#### Scenario: Pipeline HTTP failure keeps stderr confidential
- **WHEN** the real pipeline client returns an HTTP status or redirect error while its configured PAT, authorization value, response body, HTML, or redirect location contains unique marker text
- **THEN** the returned error and production stderr contain none of those markers and report only safe operation, status, and corrective guidance

#### Scenario: Pipeline command help is side-effect free
- **WHEN** a pipeline namespace or action help request includes `--help` or `-h`
- **THEN** the command exits successfully, leaves stdout empty, and writes help to stderr before resolving repository state, loading configuration, reading credentials, creating an HTTP client, or contacting Azure DevOps

### Requirement: Azure DevOps wiki fetch command
The system SHALL expose read-only wiki context fetching through `adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive] [--profile <profile-name>] [--global]`.

#### Scenario: Wiki namespace appears in Azure DevOps help
- **WHEN** the user requests help for `adomi ado`
- **THEN** the help output lists the `wiki` namespace as a read-only Azure DevOps context command

#### Scenario: Wiki fetch help describes selection and output
- **WHEN** the user requests help for `adomi ado wiki` or `adomi ado wiki fetch`
- **THEN** help describes the required wiki identifier and absolute page path, optional recursive selection, profile/global configuration behavior, repository requirement, repository-local output bundle, and path-only stdout contract

#### Scenario: Fetch wiki page context
- **WHEN** the user runs `adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path>` inside a repository with valid configuration and credentials
- **THEN** the command exports only the requested wiki page context and prints only the exported directory path followed by a newline to stdout

#### Scenario: Fetch recursive wiki context
- **WHEN** the user adds `--recursive` to a valid wiki fetch command
- **THEN** the command exports the requested page and all descendant pages and prints only the exported directory path followed by a newline to stdout

#### Scenario: Wiki fetch selects configuration scope
- **WHEN** the user supplies `--profile <profile-name>` and optionally `--global`
- **THEN** the command resolves the Azure DevOps profile and credential using the same repository/global scope rules as existing network-backed `adomi ado` context commands

#### Scenario: Wiki fetch requires a repository
- **WHEN** the command runs outside a Git repository, including with `--global`
- **THEN** it exits non-zero before loading configuration or credentials because the context bundle has no repository-local destination

#### Scenario: Wiki fetch rejects invalid arguments early
- **WHEN** the wiki identifier is missing or blank, `--page` is missing, blank, or not an absolute wiki path, a value-taking flag lacks a value, any flag is supplied more than once, or an unknown argument is supplied
- **THEN** the command exits non-zero before loading configuration or credentials and leaves stdout empty

#### Scenario: Wiki fetch failure leaves stdout empty
- **WHEN** configuration, credential lookup, HTTP client construction, wiki resolution, page fetching, response validation, or bundle export fails
- **THEN** the command exits non-zero and stdout is empty

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
