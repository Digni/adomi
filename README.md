# adomi

`adomi` is a Go CLI for Azure DevOps context workflows.

## Azure DevOps work items

Fetch work item context into the repository-local `.adomi/context` directory:

```bash
adomi ado fetch <work-item-id>
```

Comment on a work item with exactly one message source:

```bash
adomi ado comment <work-item-id> --message <text>
adomi ado comment <work-item-id> --message-file <path>
adomi ado work-item comment <work-item-id> --message-file <path>
```

Plain work item comment output prints only the created comment ID. `--json` prints one compact JSON object. Work item maintenance is comment-only; field updates, state transitions, relation edits, attachment uploads, comment updates/deletions, and reactions are not supported.

## Azure DevOps wikis

Fetch repository-local wiki context as Markdown and metadata:

```bash
adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive]
```

Only the requested page is fetched by default. Add `--recursive` to include every descendant page below it. Wiki fetch does not perform wiki search or download wiki attachments; remote links in Markdown remain unchanged.

## Azure DevOps pipeline status

Inspect running pipelines or one Build run from inside a Git repository:

```bash
adomi ado pipeline list [--profile <profile-name>] [--global]
adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]
```

`pipeline list` returns the exact `inProgress` YAML and classic Build pipeline runs observed across all continuation pages. Pagination is a best-effort one-shot view, not a transactional snapshot. `pipeline get` accepts a decimal Build run ID from 1 through 2147483647 and returns its current overall status and terminal result when available.

Success is compact JSON: list returns an ordered `runs` array and get returns one run object, with unavailable result, source, timestamp, and link fields represented as JSON null. The PAT needs the `vso.build` read scope. Pipeline requests require HTTPS, except HTTP is allowed for exact localhost or a direct IPv4/IPv6 loopback address. Loopback HTTP requests bypass configured proxies so credentials remain on-machine.

These commands do not poll, fetch stage/job/environment or other execution details, mutate pipelines, or inspect classic Release deployments.
