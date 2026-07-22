## Why

Adomi can create and maintain Azure DevOps pull requests, but it cannot associate the work items that explain and track the change. Users must currently leave the CLI and link those items in Azure DevOps manually.

## What Changes

- Add `adomi ado pr link <pull-request-id> --work-item <work-item-id>` for linking an existing work item to an existing pull request; `--work-item` may be repeated to link multiple items in one invocation.
- Make repeated runs safe by treating already-linked work items as successful no-ops and rejecting duplicate or invalid requested IDs before any write.
- Resolve the pull request's repository and artifact identity from Azure DevOps, then add revision-aware `ArtifactLink` relations to the requested work items.
- Preserve data-only output: plain success output lists the requested work item IDs, while `--json` distinguishes newly linked and already-linked IDs.
- Document non-atomic multi-item failure behavior, required Azure DevOps work-item write permission, and the existing conservative PR governance boundary.
- Update generated agent guidance so agents can discover the command without inferring broader work-item or PR-governance powers.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ado-pr-maintenance`: Add explicit, idempotent linking of one or more existing Azure DevOps work items to an existing pull request.
- `cli-command-surface`: Add the `adomi ado pr link` operation, argument validation, output, help, and failure contracts.
- `agent-skill-creation`: Teach generated Adomi guidance when and how to link work items to a pull request and which write boundary still applies.

## Impact

- Affected code: `internal/cli/commands.go`, `internal/cli/ado_pr.go` or a focused PR-link command file, `internal/cli/help.go`, `internal/cli/agent.go`, `internal/ado/pullrequest.go`, `internal/ado/models.go`, `internal/ado/client.go`, `internal/ado/client_pr.go`, `internal/ado/client_workitem.go`, and their tests.
- Affected documentation/specs: `docs/azure-devops.md`, `openspec/specs/ado-pr-maintenance`, `openspec/specs/cli-command-surface`, and `openspec/specs/agent-skill-creation` after implementation and archive.
- Azure DevOps integration: reads the pull request, reads requested work-item revisions/relations, and updates work items with JSON Patch `ArtifactLink` relations. The PAT therefore needs code read access plus work-item write access; no new Go dependency is expected.
- Compatibility: additive CLI behavior only. Existing fetch, ensure, comment, reply, resolve, reopen, work-item comment, output, profile, and credential behavior remains unchanged.
