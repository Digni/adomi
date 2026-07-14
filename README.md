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
