package cli

const adoPipelineInspectHelp = `Inspect one Azure DevOps pipeline run and export a local evidence snapshot.

Usage:
  adomi ado pipeline inspect <run-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <run-id>   decimal Build run ID in the range 1..2147483647

Flags:
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object with the snapshot path and record counts

Scope:
  Performs one read of the selected Build run, its current timelines, failed task logs, published Test runs, and all published Test results.
  The snapshot is a best-effort one-shot observation and is written below .adomi/context/pipelines/<run-id>/.
  It requires vso.build and vso.test read permission, a Git repository, and HTTPS or loopback HTTP.
  The inspection is bounded to 1000 HTTP requests, 128 MiB of successful response bodies, 8 MiB per response, and 100000 records per collection.

Exclusions:
  No polling, mutation, arbitrary artifacts, test attachments, environment or approval reads, or classic Release inspection is performed.

stdout:
  Success prints only the snapshot path plus a newline. With --json it prints {path,runId,timelineRecords,failedTaskLogs,testRuns,testResults} plus a newline.
  Errors leave stdout empty; safe diagnostics are returned as command errors.
`
