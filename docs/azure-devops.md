# Adomi Azure DevOps reference

Adomi turns Azure DevOps work items, pull requests, wiki pages, and pipeline status into local, agent-readable context. This document is the detailed user reference for the supported command surface and its safety boundaries.

- [Back to the README](../README.md)
- [Getting started](getting-started.md)

## Operating model

Adomi separates two kinds of operation:

- **Context and status reads** fetch Azure DevOps data into the current repository or return compact JSON.
- **Explicit maintenance writes** add a text comment, create or update the active branch PR, or maintain a named PR review thread.

Every network-backed Azure DevOps command runs from a Git repository. `--global` selects user-level configuration; it does not change that repository requirement or move exported context outside the checkout.

## Configuration and profiles

### Config locations

```text
<repo>/.adomi/config.yaml
~/.config/adomi/config.yaml
```

Create a repository template from inside a Git checkout:

```bash
adomi config init
```

Create a global template from any directory:

```bash
adomi config init --global
```

The command prints only the created path and refuses to overwrite an existing file. The generated YAML is fully commented; uncomment it before use.

Without `--global`, Adomi reads repository config first and falls back to global config only when the repository file is absent. The files are not merged. `--global` forces the global file.

### Profile format

```yaml
azureDevOps:
  defaultProfile: company-cloud

  profiles:
    company-cloud:
      patRef: ""
      baseUrl: https://dev.azure.com/my-org
      organization: my-org
      project: MyProject
      apiVersion: "7.1"
      proxy: ""

    company-onprem:
      patRef: shared-ado-pat
      baseUrl: https://tfs.company.local/tfs/DefaultCollection
      project: MyProject
      apiVersion: "7.0"
      proxy: http://proxy.company.local:8080
```

Profile behavior:

- `baseUrl` and `project` are required.
- `apiVersion` defaults to `"7.1"` when omitted.
- `proxy` is optional. A configured proxy takes precedence; otherwise Adomi uses environment proxy settings.
- `patRef` selects the OS-keyring entry. When empty or omitted, the profile name is used.
- `defaultProfile` is used when a command omits `--profile`.
- Profile names are exact and case-sensitive.
- `organization` is included in generated cloud profiles, while API requests are built from `baseUrl` and `project`.

Select a configured profile explicitly with `--profile <profile-name>`. Use `--global` when the global file must be selected even though repository config exists.

List profile names in sorted order:

```bash
adomi ado profiles list
adomi ado profiles list --global
```

### Credentials

Store or remove the PAT referenced by a profile:

```bash
adomi ado login --profile company-cloud
adomi ado logout --profile company-cloud

adomi ado login --profile company-cloud --global
adomi ado logout --profile company-cloud --global
```

For a direct shared credential reference:

```bash
adomi ado login --pat-ref shared-ado-pat
adomi ado logout --pat-ref shared-ado-pat
```

Login prompts through stderr, hides terminal input where supported, stores the PAT in the operating system keyring, and confirms the stored credential reference on stderr. Default stdout remains empty. Add `--json` to login or logout for a compact stdout result containing `action` and `credentialRef`. The YAML schema contains only `patRef`, never the PAT itself.

Read operations require permission to read their requested Azure DevOps resource. Work item comments require work item write permission; linking a work item to a PR requires code read and work item write permissions; other PR maintenance requires appropriate PR and review-thread write permissions. Pipeline inspection has one repository-confirmed exact scope requirement: `vso.build` read.

## Output and stream contract

Adomi reserves stdout for successful command data:

- Fetch commands print only the exported directory path by default. With `--json`, they print one compact result object instead.
- Config initialization and agent-skill creation print only the created path.
- Profile listing prints one sorted profile name per line.
- Login and logout leave stdout empty by default. With `--json`, they print one compact result object.
- Plain maintenance commands print only the created or maintained ID. Successful PR linking prints every requested work item ID in input order, one per line.
- Maintenance `--json` forms print one compact JSON object followed by a newline.
- Pipeline commands print one compact JSON value followed by a newline.

Successful login/logout confirmations, fetch progress, and fetch summaries use stderr. Help, prompts, validation errors, diagnostics, and Azure DevOps or network errors also use stderr. Determine success from the exit status rather than whether stderr is empty.

Because fetch result data remains isolated on stdout, path capture stays safe even while progress is visible in the terminal:

```bash
CONTEXT=$(adomi ado fetch 12345)
```

Successful context fetches replace the previous bundle for the same work item, pull request, or canonical wiki. Treat `.adomi/context/` as refreshable local data and avoid editing files there as durable source.

## Work items

### Fetch context

```bash
adomi ado fetch <work-item-id> [--profile <profile-name>] [--global] [--json]
```

The positive work item ID is required. Adomi fetches:

- the requested work item;
- its parent chain until an Epic, no parent, or an already visited item;
- the requested item's direct children; and
- attached files for every included item.

It does not recursively fetch every descendant.

Without `--json`, success prints a path below:

```text
.adomi/context/work-items/<work-item-id>/
```

With `--json`, success instead prints:

```json
{"path":".adomi/context/work-items/12345","workItems":3,"attachments":2}
```

Progress is reported on stderr as each work item is fetched and immediately after each attachment file is written. A final stderr summary reports the exported work-item and attachment counts.

The bundle contains:

```text
index.json
tree.json
items/<included-work-item-id>.json
html/<included-work-item-id>.html
attachments/<included-work-item-id>/...   # when attachments exist
```

Start with `index.json` and `tree.json`, then inspect the relevant item and attachment files.

### Add a text comment

Canonical command:

```bash
adomi ado comment <work-item-id> --message <text>
adomi ado comment <work-item-id> --message-file <path>
```

Namespace alias:

```bash
adomi ado work-item comment <work-item-id> --message-file <path>
```

Provide exactly one of `--message` or `--message-file`. Add `--profile`, `--global`, or `--json` as needed. Plain success output is the created comment ID; `--json` emits one compact result object.

Apart from explicit work-item-to-PR linking, work item maintenance is text-comment only. Adomi does not update fields, state, assignment, generic relations, attachments, existing comments, or reactions.

## Pull requests

### Fetch context

```bash
adomi ado pr fetch <pull-request-id> [--profile <profile-name>] [--global] [--json]
```

The compatibility form `adomi ado pr <pull-request-id>` performs the same fetch.

Without `--json`, success prints a path below:

```text
.adomi/context/pull-requests/<pull-request-id>/
```

With `--json`, success instead prints `path`, `threadCount`, and `commentCount`:

```json
{"path":".adomi/context/pull-requests/42","threadCount":3,"commentCount":11}
```

Pull-request bundle progress and the final export summary are written to stderr.

The bundle contains:

```text
index.json
pull-request.json
threads.json
comments.md
threads/<thread-id>.json
```

Fetch and inspect the current PR context before answering or changing review threads unless the relevant thread details are already known.

### Link work items

```bash
adomi ado pr link <pull-request-id> \
  --work-item <work-item-id> \
  [--work-item <work-item-id>...] \
  [--profile <profile-name>] [--global] [--json]
```

The PR ID and at least one work item ID must be positive. Repeat `--work-item` to link multiple work items; duplicate IDs are rejected before configuration, credentials, or network access. Adomi first fetches the PR, preflights every requested work item, and then links only the missing items sequentially in input order. An exact existing PR link is a successful no-op, so rerunning the command is safe.

Plain success prints all requested work item IDs in input order, one per line, including IDs that were already linked. JSON success distinguishes newly linked and unchanged items:

```json
{"pullRequestId":42,"workItemIds":[101,102],"linkedWorkItemIds":[101],"alreadyLinkedWorkItemIds":[102],"action":"linked"}
```

When every requested item was already linked, `action` is `"unchanged"`. Adomi buffers output until the whole request succeeds. Multiple links are not atomic: if a later Azure DevOps update fails, earlier links remain persisted, the command stops, stdout stays empty, and the error identifies both the failed item and the IDs linked earlier in the invocation. Rerun the same command to finish safely.

The selected PAT needs code read and work item write permissions. This command supports only explicit work-item-to-PR linking: it does not provide generic relation editing, unlink/list operations, work-item field/state mutation, auto-discovery, or automatic linking from `pr ensure`.

### Ensure the active branch PR

```bash
adomi ado pr ensure \
  [--title <title>] \
  [--description-file <path>] \
  [--source <branch>] \
  [--target <branch>] \
  [--repository <name-or-id>] \
  [--profile <profile-name>] [--global] [--json]
```

Adomi infers the source branch from the current branch, the Azure DevOps repository from matching Git remotes, and the target branch from the selected remote's default branch when possible. Use the explicit source, target, or repository flags when inference is unavailable or ambiguous.

For the resolved source/target pair:

- If no active PR exists, `--title` is required and Adomi creates it.
- If exactly one active PR exists, Adomi updates only the title or description explicitly provided.
- If no update field is provided for an existing PR, it leaves the PR unchanged and returns its ID.
- Multiple matching PRs or identical source and target refs fail before a write.

Plain success output is the PR ID. `--json` also reports the action and resolved refs.

### Create a PR comment thread

Create a PR-level thread:

```bash
adomi ado pr comment <pull-request-id> --message <text>
adomi ado pr comment <pull-request-id> --message-file <path>
```

Create a right-side, single-line inline thread on a changed file in the latest PR version:

```bash
adomi ado pr comment <pull-request-id> \
  --file <changed-file-path> \
  --line <positive-line-number> \
  --message-file <path>
```

`--file` and `--line` must be provided together. Adomi does not support left-side or deleted-file targets, line ranges, explicit offsets, suggestions, or manual iteration overrides.

Plain success output is the created thread ID. Add `--json` for a compact result object.

### Reply, resolve, or reopen

```bash
adomi ado pr reply <pull-request-id> \
  --thread <thread-id> \
  --message-file <path>

adomi ado pr resolve <pull-request-id> --thread <thread-id>
adomi ado pr reopen <pull-request-id> --thread <thread-id>
```

`reply` accepts exactly one of `--message` or `--message-file`. Resolve marks the explicit thread `fixed`; reopen marks it `active`. Plain output is the created comment ID for a reply and the thread ID for resolve or reopen. Every command supports `--profile`, `--global`, and `--json`.

### PR write boundary

Adomi can ensure title/description for the active branch PR, link explicitly named work items, create a PR-level or supported inline text thread, reply to a thread, and mark a thread fixed or active.

PR linking does not provide generic relation editing, unlink/list operations, or work-item field/state mutation. Adomi also does not approve, reject, merge, complete, abandon, set auto-complete, bypass policies, or manage reviewers.

## Wikis

```bash
adomi ado wiki fetch <wiki-id-or-name> \
  --page <absolute-wiki-page-path> \
  [--recursive] \
  [--profile <profile-name>] [--global] [--json]
```

The wiki identifier and an absolute page path beginning with `/` are required. By default Adomi fetches only that page. `--recursive` includes the page and all descendants returned below it.

Without `--json`, success prints the exact bundle path below `.adomi/context/wikis/`. The final directory component is a safe encoded representation of the canonical wiki ID, so use stdout rather than constructing it yourself. With `--json`, stdout instead contains `path` and `pages`:

```json
{"path":".adomi/context/wikis/<safe-wiki-id>","pages":5}
```

Page progress and the final export summary are written to stderr.

The bundle contains:

```text
index.json
wiki.json
pages.json
pages/<safe-page-file>.md
```

`index.json` maps original Azure DevOps wiki paths to the generated Markdown filenames.

Wiki fetch is read-only. It does not perform indexed search, mutate a wiki, override versions, download linked attachments or other resources, or reconstruct historical archives. Links in fetched Markdown remain remote links.

## Pipeline runs

```bash
adomi ado pipeline list [--last <N>] [--profile <profile-name>] [--global]
adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]
```

`pipeline list` without `--last` requests exact `inProgress` runs across YAML and classic Build pipelines and follows continuation pages. With `--last <N>`, it requests the `N` most recently queued runs in the project across any status, with `N` from `1` through `200`. Both modes return a best-effort one-shot view, not a transactional snapshot; runs can change while pages are being read.

To check CI for a just-pushed change, list recent runs and match your branch or commit against each run's `sourceBranch` and `sourceVersion`, then re-check a known run with `pipeline get`.

`pipeline get` accepts a decimal Build run ID from `1` through `2147483647` and returns its current overall status and terminal result when available.

List success shape:

```json
{"runs":[]}
```

Each projected run contains every key below:

```json
{
  "id": 123,
  "pipelineId": 45,
  "pipelineName": "Build",
  "runNumber": "20260716.1",
  "status": "inProgress",
  "result": null,
  "sourceBranch": "refs/heads/main",
  "sourceVersion": "0123456789abcdef",
  "queueTime": "2026-07-16T10:00:00Z",
  "startTime": "2026-07-16T10:00:02Z",
  "finishTime": null,
  "webUrl": "https://dev.azure.com/example/project/_build/results?buildId=123"
}
```

Unavailable result, source, timestamp, and link values are JSON `null`. `pipeline get` returns one run object rather than a `runs` wrapper.

Pipeline requirements and boundaries:

- The PAT needs `vso.build` read scope.
- The configured endpoint must use HTTPS, except HTTP is allowed for exact localhost or a direct IPv4/IPv6 loopback address.
- Loopback HTTP bypasses configured and environment proxies so the authenticated request stays on-machine.
- The command does not poll or wait.
- It does not fetch stages, jobs, tasks, timelines, logs, artifacts, approvals, environments, deployment resources, or classic Release deployments.
- It does not queue, cancel, retry, approve, or otherwise mutate a pipeline.

The HTTPS-or-loopback restriction is specific to pipeline commands; other Azure DevOps commands currently require an absolute configured base URL and use their normal proxy behavior.

## Agent skill generation

Generate an Adomi skill for Codex, OpenCode, Pi, GitHub Copilot, Cursor, or Claude:

| Provider | `--provider` value | Skill root |
| --- | --- | --- |
| [Codex](https://developers.openai.com/codex/skills/) | `codex` | `.agents/skills` |
| [OpenCode](https://opencode.ai/docs/skills/) | `opencode` | `.agents/skills` |
| [Pi](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/README.md#skills) | `pi` | `.agents/skills` |
| [GitHub Copilot](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills) | `github-copilot` | `.agents/skills` |
| [Cursor](https://cursor.com/docs/skills) | `cursor` | `.agents/skills` |
| Claude | `claude` | `.claude/skills` |

The first five providers share one portable `.agents/skills/adomi/` installation. Selecting a different shared provider resolves to the same target and does not create a provider-specific duplicate.

| Command | Target |
| --- | --- |
| `adomi agent skill` | `~/.agents/skills/adomi/SKILL.md` |
| `adomi agent skill --provider codex --global` | `~/.agents/skills/adomi/SKILL.md` |
| `adomi agent skill --provider codex --project` | `<repo>/.agents/skills/adomi/SKILL.md` |
| `adomi agent skill --provider claude` | `~/.claude/skills/adomi/SKILL.md` |
| `adomi agent skill --provider claude --project` | `<repo>/.claude/skills/adomi/SKILL.md` |
| `adomi agent skill --claude` | `~/.claude/skills/adomi/SKILL.md` |
| `adomi agent skill --claude --project` | `<repo>/.claude/skills/adomi/SKILL.md` |

Global is the default scope; `--global` makes it explicit. The shared-agent location is the default target. Replace `codex` in the shared examples with `opencode`, `pi`, `github-copilot`, or `cursor` to select another supported provider. `--claude` remains a backward-compatible alias for `--provider claude`. `--project` requires a Git repository. Existing skill directories require interactive confirmation unless `--force` or `--yes` is provided; failed replacement attempts preserve the prior skill where possible.

The generated skill describes configuration discovery, context-before-maintenance workflow, command outputs, and safety boundaries. It guides an agent but does not automatically run Adomi, resolve profiles, or grant authorization.

## HTTP, proxy, and error behavior

- A profile's explicit `proxy` is used when set; otherwise standard environment proxy settings apply.
- Azure DevOps API redirects are not followed. A redirect is reported as an authentication or base-URL error without fetching the interactive destination.
- Pipeline loopback HTTP requests bypass proxies; this exception does not change the transport of other commands.
- PATs, authorization values, raw HTML error pages, and redirect locations must not appear in pipeline diagnostics.
- A failed command leaves stdout free of success data.

## Command discovery

Public action help is side-effect free and returns before configuration, credential, repository, or network work:

```bash
adomi --help
adomi config init --help
adomi agent skill --help
adomi ado fetch --help
adomi ado comment --help
adomi ado pr ensure --help
adomi ado pr comment --help
adomi ado wiki fetch --help
adomi ado pipeline --help
```

Use the [getting-started guide](getting-started.md) for a clean first setup or return to the [README](../README.md) for the overview.
