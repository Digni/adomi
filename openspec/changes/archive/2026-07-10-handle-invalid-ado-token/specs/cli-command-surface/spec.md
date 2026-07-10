## ADDED Requirements

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
