# Getting started with Adomi

This guide takes you from a source checkout to the first Azure DevOps context bundle that you or a coding agent can inspect locally.

- [Back to the README](../README.md)
- [Azure DevOps command reference](azure-devops.md)

## Prerequisites

You need:

- Git
- Go 1.26.2 or newer, matching the minimum version in `go.mod`
- An Azure DevOps PAT with read access to the work items, pull requests, or wiki pages you plan to fetch
- Access to your operating system's credential store, where Adomi saves PAT values

Pipeline inspection additionally requires the PAT's `vso.build` read scope. Work item comments and pull request maintenance require the corresponding write permissions. Exact permission names for those workflows depend on the Azure DevOps environment; Adomi will return an authorization error if the selected credential lacks them.

Never put a PAT value in `.adomi/config.yaml`, a command argument, a committed file, or copied troubleshooting output.

## Install from a clone

The currently verified installation path uses the repository's Go module:

```bash
git clone https://github.com/Digni/adomi.git
cd adomi
go install ./cmd/adomi
adomi --help
```

`go install` downloads the declared module dependencies and installs `adomi` to:

- `GOBIN`, when `go env GOBIN` is non-empty; or
- the `bin` directory below `go env GOPATH`, normally `~/go/bin` on Unix-like systems.

If your shell cannot find `adomi`, inspect the Go locations:

```bash
go env GOBIN
go env GOPATH
```

Add the applicable binary directory to your `PATH`, restart the shell if needed, and rerun `adomi --help`.

There is currently no Homebrew formula, package-manager package, installer script, or published binary-release workflow for Adomi.

## Build a repository-local binary

From the clone, you can build without installing to your user-level Go binary directory:

```bash
go build -o ./bin/adomi ./cmd/adomi
./bin/adomi --help
```

The repository ignores `bin/`.

## Choose repository or global configuration

Adomi reads Azure DevOps profiles from YAML:

- Repository configuration: `<repo>/.adomi/config.yaml`
- Global configuration: `~/.config/adomi/config.yaml`

Use repository configuration when a checkout has a dedicated Azure DevOps project or profile:

```bash
cd /path/to/your/git-repository
adomi config init
```

Use global configuration when the same profiles should be available across repositories:

```bash
adomi config init --global
```

Both commands print only the created file path. They refuse to overwrite an existing file.

Important configuration rules:

- Without `--global`, Adomi uses repository config when present and otherwise falls back to global config.
- Repository and global files are not merged. A repository file shadows all profiles in the global file.
- `--global` forces user-level configuration, but network-backed Azure DevOps commands still run from a Git repository because their context and repository inference remain local.
- Profile names are exact and case-sensitive.

## Activate the generated profile

`adomi config init` creates a fully commented template. Uncomment and edit it before running Azure DevOps commands. A minimal cloud profile looks like this:

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
```

Fields:

- `defaultProfile` is used when a command does not provide `--profile`.
- `baseUrl` and `project` are required for an operational profile.
- `apiVersion` defaults to `"7.1"` when omitted.
- `proxy` is optional. When empty, Adomi uses the standard environment proxy settings.
- `patRef` is optional. When empty or omitted, the profile name is also the OS-keyring credential reference. Set the same non-empty `patRef` on multiple profiles only when they should deliberately share one stored credential.
- `organization` is included in generated cloud profiles and helps describe the target organization, but requests are built from `baseUrl` and `project`.

Azure DevOps Server/on-premises profiles can use a collection URL such as `https://tfs.company.local/tfs/DefaultCollection` as `baseUrl`.

## Store the PAT securely

For repository configuration:

```bash
adomi ado login --profile company-cloud
```

For a profile in global configuration:

```bash
adomi ado login --profile company-cloud --global
```

Adomi prompts for the secret through stderr, hides terminal input where supported, stores the PAT in the OS keyring, and writes no success value to stdout. It never adds the PAT to YAML.

You can inspect configured profile names without exposing credentials:

```bash
adomi ado profiles list
adomi ado profiles list --global
```

Remove a stored credential with the matching scope:

```bash
adomi ado logout --profile company-cloud
adomi ado logout --profile company-cloud --global
```

Advanced setups can use `adomi ado login --pat-ref <ref>` and the same `patRef` in one or more profiles.

## Install the optional agent skill

Adomi can generate a `SKILL.md` that teaches compatible coding agents when to fetch Azure DevOps context and where the safety boundaries are.

Choose one target:

```bash
# ~/.agents/skills/adomi/SKILL.md (default shared-agent target)
adomi agent skill

# <repo>/.agents/skills/adomi/SKILL.md
adomi agent skill --project

# ~/.claude/skills/adomi/SKILL.md
adomi agent skill --claude

# <repo>/.claude/skills/adomi/SKILL.md
adomi agent skill --claude --project
```

Global skill creation works from any directory. Project skill creation requires a Git repository. If the target already exists, Adomi asks before replacing it; use `--force` or `--yes` only when replacement is intentional.

The generated skill guides agents to inspect configuration and fetch context before acting. It does not automatically choose a profile, run commands, or grant Azure DevOps permissions.

## Fetch your first context bundle

Run network-backed commands from the Git repository that should receive the export.

### Option A: work item context

```bash
cd /path/to/your/git-repository
WORK_ITEM_CONTEXT=$(adomi ado fetch 12345)
ls "$WORK_ITEM_CONTEXT"
```

The printed directory is `.adomi/context/work-items/12345/` below the repository root. It can contain:

```text
index.json
tree.json
items/<work-item-id>.json
html/<work-item-id>.html
attachments/<work-item-id>/...
```

The tree contains the requested item, its parent chain up to an Epic or the last available parent, and the requested item's direct children. It does not recursively fetch every descendant.

Point your agent at `index.json` and `tree.json` first, then the item or attachment files relevant to the task.

### Option B: pull request context

```bash
cd /path/to/your/git-repository
PR_CONTEXT=$(adomi ado pr fetch 42)
ls "$PR_CONTEXT"
```

The printed directory is `.adomi/context/pull-requests/42/` and contains PR metadata, thread JSON, per-thread files, and `comments.md`. Read that context before changing code or maintaining review threads.

When no repository config exists, you can select a global profile explicitly while keeping the output repository-local:

```bash
adomi ado fetch 12345 --profile company-cloud --global
```

## Keep local context out of Git

Add this entry to the target repository's `.gitignore`:

```gitignore
.adomi/
```

The directory can contain downloaded work item attachments and Azure DevOps content. Treat it as local, refreshable working context rather than source to commit.

## Common setup problems

### `not inside a git repository`

Run the command from the checkout that should receive or provide context. `--global` changes configuration scope; it does not remove the Git-repository requirement from network-backed Azure DevOps operations.

### Config exists but profiles are missing

The generated template is commented out. Remove the leading `#` markers and provide `azureDevOps.defaultProfile` plus at least one profile with `baseUrl` and `project`.

### A global profile is not found

A repository `.adomi/config.yaml` shadows the global file. Use `--global` to force the global config, or add the profile to the repository file. Confirm exact spelling with `adomi ado profiles list --global`.

### Credential lookup fails

Run `adomi ado login` with the same profile scope or `patRef` used by the command. Also confirm that the current shell or agent process can access the OS keyring.

### Azure DevOps returns a redirect or authorization error

Adomi does not follow API redirects to browser sign-in pages. Verify `baseUrl`, project, PAT validity, and the permissions required for the requested workflow. Only pipeline status has a repository-confirmed exact scope name: `vso.build` read.

### A proxy is required

Set `proxy` on the profile to an absolute proxy URL. When it is empty, standard environment proxy variables apply. Pipeline requests to direct loopback HTTP endpoints intentionally bypass proxies so credentials remain on the local machine.

## Next steps

- Use `adomi <command> --help` for exact flags without contacting Azure DevOps.
- Read the [Azure DevOps reference](azure-devops.md) for work item comments, PR maintenance, wiki fetch, pipeline JSON, output contracts, and explicit exclusions.
- Return to the [README](../README.md) for the product overview and contributor commands.
