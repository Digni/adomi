# adomi

Adomi brings Azure DevOps context into the repository where you and your coding agent are already working.

The name blends **ADO** with the French word *ami* (“friend”): Adomi is a friendly companion for your Azure DevOps work.

Instead of copying source material into a chat, use the `adomi` CLI to export work item, pull request, and wiki context below `.adomi/context/`, or to return pipeline status as compact JSON. Your agent can inspect those local artifacts before planning or changing code. When you explicitly ask it to report back, Adomi also provides a small set of bounded work item and pull request maintenance commands.

## Why Adomi

- **Context before code.** Fetch the work item hierarchy, direct children, attachments, pull request discussions, or selected wiki pages into the current Git repository.
- **One workflow for humans and agents.** Commands keep success output data-only, so paths, IDs, and JSON can be passed directly into scripts or agent workflows.
- **Credentials stay out of the repository.** Profiles live in YAML; PAT values are entered through `adomi ado login` and stored in the operating system keyring.
- **Writes stay explicit.** Adomi can add text comments, create or update the active branch PR, link explicitly named work items to a PR, and maintain selected review threads. It does not edit work item fields, approve or merge PRs, or control deployments.

The core loop is simple:

1. Configure an Azure DevOps profile and store its credential securely.
2. Fetch the source context into `.adomi/context/`.
3. Let your coding agent inspect those files before it plans or edits.
4. Use a maintenance command only when you want a specific result written back to Azure DevOps.

## What it can do

| Workflow | Primary command | Result and boundary |
| --- | --- | --- |
| Configuration and credentials | `adomi config init`, `adomi ado login` | Repository or global profiles with PAT values kept in the OS keyring. |
| Agent integration | `adomi agent skill --provider <name>` | Installs an Adomi skill globally or in the current project for Codex, OpenCode, Pi, GitHub Copilot, Cursor, or Claude. Existing skills require confirmation or `--force`/`--yes` to replace. |
| Work item context | `adomi ado fetch <work-item-id>` | Exports the parent chain, direct children, JSON/HTML, and attachments under `.adomi/context/work-items/`. |
| Work item comments | `adomi ado comment <work-item-id> ...` | Adds one text comment. It cannot update fields, state, assignment, generic relations, attachments, or existing comments. |
| Pull request context | `adomi ado pr fetch <pull-request-id>` | Exports PR metadata, review threads, and readable comments under `.adomi/context/pull-requests/`. |
| PR work-item links | `adomi ado pr link <pull-request-id> --work-item <work-item-id>` | Links explicitly named work items to a PR. Repeat `--work-item` for more IDs; no unlink, list, generic relation edit, or work-item field/state mutation is provided. |
| Pull request maintenance | `adomi ado pr ensure`, `comment`, `reply`, `resolve`, `reopen` | Maintains the active branch PR or explicit review threads. It cannot approve, reject, merge, complete, abandon, bypass policies, or manage reviewers. |
| Wiki context | `adomi ado wiki fetch ...` | Exports one page or a recursive subtree as Markdown and metadata. It does not search wikis or download linked attachments. |
| Pipeline status | `adomi ado pipeline list`, `get` | Returns one-shot compact JSON for in-progress or the most recent (`--last <N>`) Build runs, or one run's overall status. It does not poll, fetch execution detail, inspect classic Release deployments, or mutate pipelines. |

See the [Azure DevOps reference](docs/azure-devops.md) for the complete command forms, outputs, permissions, and limitations.

PR linking supports `--profile`, `--global`, and `--json`, treats reruns and already-linked work items as successful no-ops, and requires PAT permissions for code read and work item write. Output is buffered until every requested item succeeds. Because multiple updates are sequential rather than atomic, a later failure leaves earlier links persisted, reports the partial progress, and keeps stdout empty so the same command can be rerun safely.

## Install

Install Adomi directly from GitHub with Go 1.26.2 or newer:

```bash
go install github.com/Digni/adomi/cmd/adomi@latest
adomi --help
```

`go install` writes the binary to `GOBIN`, or to the `bin` directory below `go env GOPATH` when `GOBIN` is unset. Add that directory to your `PATH` if `adomi` is not found.

To build a repository-local binary instead, clone the repository:

```bash
git clone https://github.com/Digni/adomi.git
cd adomi
go build -o ./bin/adomi ./cmd/adomi
./bin/adomi --help
```

There is not yet a Homebrew formula, package-manager package, installer script, or published binary-release workflow. The [getting-started guide](docs/getting-started.md) has the complete installation and setup walkthrough.

## Quick start

Run these commands inside the Git repository that should receive the Azure DevOps context:

```bash
adomi config init
```

Adomi creates `.adomi/config.yaml` as a commented template. Uncomment and edit it with your Azure DevOps organization, project, and profile details, then store the PAT without putting it in YAML or shell history:

```bash
adomi ado login --profile company-cloud
```

Optionally install the generated Adomi instructions for coding agents in this repository:

```bash
adomi agent skill --provider codex --project
```

[Codex](https://developers.openai.com/codex/skills/), [OpenCode](https://opencode.ai/docs/skills/), [Pi](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/README.md#skills), [GitHub Copilot](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills), and [Cursor](https://cursor.com/docs/skills) use the same `.agents/skills/adomi/` installation. Select them with `--provider codex`, `opencode`, `pi`, `github-copilot`, or `cursor`; installing once serves all five. Claude uses `.claude/skills/adomi/` through `--provider claude`, and the existing `--claude` flag remains a compatible alias.

Global installation is the default and can be made explicit with `--global`; `--project` installs below the current repository. If the target exists, Adomi asks before replacing it. Use `--force` or `--yes` only when replacement is intentional; failed replacement attempts preserve the previous skill where possible.

Fetch a work item and inspect the path printed by Adomi:

```bash
CONTEXT=$(adomi ado fetch 12345)
ls "$CONTEXT"
```

The export is local working context. Add `.adomi/` to the target repository's `.gitignore` and do not commit fetched artifacts or credentials.

For global configuration, multiple profiles, pull request context, common setup errors, and the alternative first workflow, follow [Getting started](docs/getting-started.md).

## Safety model

Adomi is designed around context-first, user-authorized automation:

- Network-backed Azure DevOps commands run from a Git repository, including when `--global` selects user-level configuration.
- Repository configuration takes precedence over global configuration; the two files are not merged.
- Help, prompts, diagnostics, and errors go to stderr. Successful stdout stays limited to paths, IDs, profile names, or compact JSON.
- Redirects from Azure DevOps API requests are not followed, which avoids turning an expired credential into an interactive sign-in page fetch.
- Pipeline inspection uses HTTPS, except for direct loopback HTTP development endpoints, and remains read-only.
- Adomi never prints or stores PAT values in its YAML configuration.

Read the [Azure DevOps reference](docs/azure-devops.md) before using maintenance commands in an automated workflow.

## Command help

The CLI has side-effect-free help for every public action:

```bash
adomi --help
adomi ado --help
adomi ado pr ensure --help
adomi ado wiki fetch --help
adomi ado pipeline --help
```

## Development

From a repository clone:

```bash
gofmt -w cmd internal
go test ./...
go build ./...
```

Build a runnable development binary with `go build -o ./bin/adomi ./cmd/adomi`.

## Documentation

- [Getting started](docs/getting-started.md) — installation, configuration, credentials, agent setup, and the first context fetch.
- [Azure DevOps reference](docs/azure-devops.md) — commands, output bundles, profile resolution, permissions, and conservative write boundaries.
