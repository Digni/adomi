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
