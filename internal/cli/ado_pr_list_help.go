package cli

const adoPRListHelp = `Discover Azure DevOps pull requests.

Usage:
  adomi ado pr list [--source <branch>] [--target <branch>] [--status active|completed|abandoned|all] [--repository <name-or-id>] [--profile <profile-name>] [--global]

Flags:
  --source <branch>           filter by source branch; short names receive refs/heads/
  --target <branch>           filter by target branch; short names receive refs/heads/
  --status <status>            active (default), completed, abandoned, or all
  --repository <name-or-id>   override repository inference
  --profile <profile-name>    select a configured Azure DevOps profile
  --global                    use user-level configuration

Rules:
  Omitted source and target filters remain unset, including on detached HEAD. Full refs are preserved after trimming whitespace; remote-name prefixes are not stripped.
  Repository selection follows matching Git remotes and prefers origin when needed. Use --repository when inference is unavailable or ambiguous.
  A Git repository is required. The PAT needs vso.code read permission. Discovery reads offset pages through an empty page, including after short pages, and preserves service order. Fixed bounds are 1,000 pages, 100,000 PRs, and 8 MiB per response.

stdout:
  Prints one compact JSON object with a pullRequests array and a trailing newline. Entries contain identity, branch, status, dates, and the API URL. Dates are UTC RFC3339Nano; absent dates and URLs are null.
  Use adomi ado pr fetch <pull-request-id> to read the description and discussion of a discovered PR.

Failure:
  Invalid responses, failed pages, duplicate identities, and exhausted bounds fail with empty success stdout. An empty array means no matches in the completed observed traversal.
`
