## Why

Adomi can inspect Azure DevOps work items, pull requests, and wikis, but it cannot report whether a project pipeline is currently running or how a completed run ended. Agents therefore lack a direct, scriptable way to answer basic build/deployment readiness questions from the same authenticated project context.

## What Changes

- Add read-only `adomi ado pipeline list` support that traverses the Build API continuation sequence and returns a best-effort one-shot view of the project's currently in-progress runs as compact JSON, including both YAML and classic Build pipelines.
- Add read-only `adomi ado pipeline get <run-id>` support that returns one run's overall status, terminal result when available, pipeline identity, source revision, timestamps, and links as compact JSON.
- Reuse the configured Azure DevOps profile, PAT, proxy, project, API version, redirect protection, and empty-stdout-on-error conventions while requiring HTTPS except for exact localhost/loopback HTTP endpoints, bypassing configured proxies for loopback HTTP so credentials remain on-machine, and suppressing credentials, raw response bodies, and redirect locations from pipeline HTTP diagnostics.
- Keep the first slice to one-shot overall run inspection. Polling/waiting, stage/job/task detail, logs, artifacts, environment status, pipeline mutation, and classic Release deployments are out of scope.

## Capabilities

### New Capabilities

- `ado-pipeline-runs`: List in-progress Azure DevOps Build API runs, including YAML and classic Build pipelines, and retrieve the overall status/result of one run through the Build REST API.

### Modified Capabilities

- `cli-command-surface`: Add the `adomi ado pipeline list` and `adomi ado pipeline get` commands, validation/help behavior, compact JSON success output, and failure-stream guarantees.

## Impact

- Adds pipeline-run models, a read-only client interface, authenticated Build REST calls, bounded pagination and response reads, response validation, and output projection under `internal/ado`.
- Extends the `adomi ado` command tree, dependency seam, parsers/help, JSON result output, and production-wiring tests under `internal/cli`.
- Updates the generated Adomi agent skill, README, and Azure DevOps handover documentation so agents and users can discover the status workflow and required read scope.
- Uses the existing standard-library HTTP stack and Azure DevOps profile/PAT configuration; no new dependency or configuration field is expected. Pipeline requests add a capability-specific transport check without changing existing commands.
