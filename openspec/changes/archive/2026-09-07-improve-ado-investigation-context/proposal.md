## Why

ADO investigations currently need custom API helpers to discover unlinked pull requests, find branch-specific pipeline history, and recover work-item discussion. Overall pipeline success also does not explain which stages, jobs, tasks, and tests actually ran.

## What Changes

Deliver two increments in one change:

1. Add `adomi ado pr list --source <branch> --status all`, with optional target/repository filters, complete bounded pagination, and compact JSON discovery output. Add server-side `--branch <branch>` filtering to `adomi ado pipeline list`, including `--last 10`. Add opt-in `adomi ado fetch <id> --include-comments` for discussion on every work item already included in the export.
2. Add `adomi ado pipeline inspect <run-id>` to export the run summary, stage/job/task timeline, failed-task logs, and published test runs/results into a repository-local context bundle.

Preserve current command defaults, profile/keyring handling, and stdout conventions. Document missing data and one-shot observation limits; failed retrievals must not masquerade as empty or complete evidence. Update CLI help, public guidance, and generated agent skills with each increment.

## Capabilities

### New Capabilities

- `ado-pr-discovery`: Repository-scoped PR discovery with source, target, status, and pagination handling.
- `ado-work-item-comments`: Opt-in paginated discussion export for the existing work-item tree.
- `ado-pipeline-inspection`: Local export of execution records, failed-task logs, and published test results for one Build run.

### Modified Capabilities

- `ado-pipeline-runs`: Server-side branch filtering and an explicit distinction between summary commands and detailed inspection.
- `cli-command-surface`: Parsing, routing, side-effect-free help, and deterministic output for the additions.
- `agent-skill-creation`: Agent guidance for discovering and reading investigation evidence.
- `repository-documentation`: Discoverable investigation workflows, permissions, output paths, and evidence limitations.

## Impact

Implementation is **standard risk**: bounded additive reads and local exports through existing CLI/configuration/client seams. Consumers are CLI users, agent skills, scripts reading JSON, and context-bundle readers. No new dependency, configuration schema, ADO write operation, or change to shared authentication/transport policy is proposed.

Affected areas include `internal/ado/client_pr.go`, pipeline readers and transport URL builders, work-item fetch/export models, `internal/cli/ado_*.go`, command routing/help, `internal/cli/agent.go`, and `README.md`/`docs/azure-devops.md`. PR discovery will preserve the existing `ListPullRequests` caller used by `pr ensure`.

The second increment uses Build timeline/log and Test read APIs and requires test-read permission in addition to build-read permission. It covers YAML and classic Build runs; classic Release deployments, polling, mutation, arbitrary artifacts, test attachments, and deployment-environment verification remain outside scope. The existing completed but unarchived `enable-complete-pr-flow` change must retain its governance and host-execution guidance when these deltas are eventually synchronized.
