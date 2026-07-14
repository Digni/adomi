## ADDED Requirements

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
