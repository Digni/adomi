# Global Azure DevOps Config And Init

Date: 2026-04-26

## Goal

Add explicit global config support and a config initialization workflow for Azure DevOps profiles.

The central behavior is:

- `--global` on config-consuming commands means "read Azure DevOps profiles from `~/.config/adomi/config.yaml` and ignore repo-local `.adomi/config.yaml`."
- `adomi ado config init` creates a repo-local config template at `<repo-root>/.adomi/config.yaml`.
- `adomi ado config init --global` creates a global config template at `~/.config/adomi/config.yaml`.
- Config init writes a fully commented-out YAML template. It may prefill suggested values, but every line remains commented so the user must intentionally enable/edit it.
- PAT login/logout remain profile-based and shared; they do not get separate local/global credential storage.

## User-Approved Behavior

- `adomi ado fetch <id> --global [--profile <name>]`
  - Uses global config only.
  - Still requires a Git repo for output because fetched context is written under `<repo-root>/.adomi/...`.
  - Uses the same keyring profile name as non-global fetch.
- `adomi ado profiles list --global`
  - Lists profiles from global config only.
  - Works outside a Git repo.
- `adomi ado config init`
  - Requires a Git repo.
  - Creates `<repo-root>/.adomi/config.yaml`.
  - If an Azure DevOps remote is detectable from the repo, pre-fills commented `baseUrl`, `organization`, `project`, and profile name suggestions.
- `adomi ado config init --global`
  - Creates `~/.config/adomi/config.yaml`.
  - If run from inside a Git repo and an Azure DevOps remote is detectable, uses the same commented prefill behavior.
  - If not in a Git repo or no ADO remote is detectable, still creates a generic commented template.
- `adomi ado login --profile <name>` and `adomi ado logout --profile <name>`
  - Keep current behavior.
  - No `--global` flag, because credentials are not repo-local today.

## Verified Current Code

- `internal/cli/ado.go:16-35` dispatches only `fetch`, `login`, `logout`, and `profiles`; there is no `config` subcommand yet.
- `internal/cli/ado.go:38-50` parses fetch args, resolves repo/home locations, then calls `deps.LoadConfig(repoRoot, homeDir, profile)`.
- `internal/cli/ado.go:142-149` resolves repo/home locations for `profiles list`, then calls `deps.LoadAllConfig(repoRoot, homeDir)`.
- `internal/cli/ado.go:92-110` login reads a PAT and stores it via `PATStore.Set(profile, pat)` without consulting repo/home config.
- `internal/cli/ado.go:134-139` logout deletes by profile name only.
- `internal/cli/ado.go:158-171` `resolveLocations` always requires a repo root and home directory together.
- `internal/config/config.go:38-67` `Load` currently delegates to `LoadAll` then selects/defaults a profile.
- `internal/config/config.go:69-91` `LoadAll` currently finds either repo-local or home config through `findConfigPath`.
- `internal/config/config.go:103-119` `findConfigPath` prefers `<repoRoot>/.adomi/config.yaml`, then falls back to `<homeDir>/.config/adomi/config.yaml`.
- Existing config tests in `internal/config/config_test.go:10-40` assert repo config overrides home config, and `internal/config/config_test.go:42-65` asserts home fallback.
- Existing CLI tests in `internal/cli/ado_test.go:109-135` and `internal/cli/ado_test.go:137-201` assert current profiles/fetch wiring passes both repo and home locations.
- The handover config example at `docs/adomi_azure_devops_handover.md:107-126` defines the YAML shape to use for the commented template.
- Current checkout remote is GitHub (`git remote -v` returned `git@github.com:Digni/adomi.git`), so Azure DevOps remote prefill must be covered with synthetic test remotes.

## Design

### Config Scope

Introduce a config scope in `internal/config`:

- Default scope: existing repo-then-home lookup.
- Global scope: direct home config lookup only.

Keep the loader API explicit rather than overloading magic `repoRoot` values:

- Add `config.LoadWithScope(repoRoot, homeDir, requestedProfile, scope)`.
- Add `config.LoadAllWithScope(repoRoot, homeDir, scope)`.
- Keep `Load` and `LoadAll` as wrappers using the default scope.

In CLI parsing:

- Extend fetch parsing to return `global bool` alongside work item ID and profile.
- Extend `profiles list` parsing to accept optional `--global`.
- When global scope is requested, still call `resolveLocations` for fetch because export needs a repo root.
- For `profiles list --global`, avoid repo-root lookup and only require `UserHomeDir`.
- Reject unknown arguments consistently.

### Config Init

Add `adomi ado config init [--global]`.

Path behavior:

- Local: `<repo-root>/.adomi/config.yaml`
- Global: `<homeDir>/.config/adomi/config.yaml`

Safety behavior:

- Create parent directories as needed.
- Refuse to overwrite an existing config file.
- Print the created config path to stdout and no extra text, matching the fetch command's machine-readable style.

Template behavior:

- Render the standard config shape as comments only.
- Use a default profile name such as `company-cloud` when no remote-derived values exist.
- Include a second commented on-prem example only if it remains concise; the cloud example is the primary path.
- If ADO remote detection succeeds, substitute suggested values into the commented cloud example.

Example template shape:

```yaml
# azureDevOps:
#   defaultProfile: my-project
#
#   profiles:
#     my-project:
#       baseUrl: https://dev.azure.com/my-org
#       organization: my-org
#       project: MyProject
#       apiVersion: "7.1"
#       proxy: ""
```

Because every line is commented, `config init` does not create an active profile. Users must uncomment/edit intentionally before `fetch` or `profiles list` can load it.

### ADO Remote Prefill

Add a small internal helper for best-effort Azure DevOps remote detection.

Inputs:

- Remote URLs from the current Git repo, provided through an injectable dependency for tests.

Detection should be local-only and best-effort. No network calls.

Support common Azure DevOps remote URL forms:

- `https://dev.azure.com/<org>/<project>/_git/<repo>`
- `https://<org>@dev.azure.com/<org>/<project>/_git/<repo>`
- `git@ssh.dev.azure.com:v3/<org>/<project>/<repo>`
- `https://<org>.visualstudio.com/<project>/_git/<repo>` if straightforward to parse.

Prefill fields:

- `organization`: from the URL where available.
- `project`: from the URL path where available.
- `baseUrl`: `https://dev.azure.com/<org>` for Azure DevOps Services cloud URLs.
- `defaultProfile` and profile key: sanitized project name if available, otherwise `company-cloud`.

If parsing fails or the remote is not Azure DevOps, init still writes the generic commented template.

Implementation placement:

- Keep path lookup and YAML loading in `internal/config`.
- Put template rendering and remote prefill logic in `internal/config` or a small adjacent package if it keeps `internal/cli/ado.go` from growing too large.
- Add an injectable dependency in `internal/cli/root.go` for remote URL discovery. Default implementation can shell out to `git -C <repoRoot> remote get-url --all origin` or a broader remote listing, but test code should inject remotes directly.

## Test Plan

Write failing tests first.

1. `internal/config`
   - `LoadWithScope(..., GlobalScope)` ignores repo config and selects home config even when repo config exists.
   - `LoadAllWithScope(..., GlobalScope)` returns only home profiles.
   - Global missing-config errors mention only the global path.
   - `RenderInitTemplate` or equivalent emits commented-only YAML.
   - ADO cloud HTTPS remote prefill extracts org/project/base URL.
   - ADO SSH remote prefill extracts org/project/base URL.
   - Non-ADO remote produces generic commented template values.

2. `internal/cli`
   - `ado fetch <id> --global --profile home` passes global scope to the loader and still exports under repo root.
   - `ado profiles list --global` does not call `FindRepoRoot` and lists global profiles.
   - `ado profiles list --unknown` is rejected.
   - `ado login --global --profile name` is rejected as unknown, documenting that credentials are not scoped by config location.
   - `ado config init` creates repo-local config and prints only the path.
   - `ado config init --global` creates global config and prints only the path.
   - `ado config init --global` outside a repo still creates the global generic template.
   - `ado config init` refuses to overwrite an existing file.
   - `ado config init` uses injected ADO remote URLs to prefill commented values.

Verification checkpoints:

- After red tests: targeted tests should fail for missing APIs/parsing.
- After config implementation: `go test -count=1 ./internal/config`.
- After CLI implementation: `go test -count=1 ./internal/cli`.
- Final: `gofmt -l cmd internal`, `go test -count=1 ./...`, and `go build -o /tmp/adomi-smoke ./cmd/adomi`.

## Edge Cases

- Global config missing: `--global` should report only the global path as missing, not mention repo-local config.
- Repo config and global config both define the same profile name: default lookup keeps repo precedence; `--global` selects the global profile.
- `profiles list --global` outside a repo: should work, because listing global profiles does not need a repo output path.
- `fetch --global` outside a repo: should still fail repo resolution, because export output remains repo-local.
- `config init --global` outside a repo: should still work with a generic template.
- `config init` outside a repo: should fail because there is no local repo config target.
- Existing config file: init should fail rather than overwrite.
- ADO remote detection failure: should not fail init; it only removes the prefill.
- Multiple remotes: prefer the first parseable Azure DevOps remote. If none are parseable, use generic values.
- Login/logout: `--global` is not accepted, avoiding the false implication of separate credential scopes.

## Blast Radius

- `internal/config/config.go`: loader path-selection logic, init template rendering, and possibly remote prefill helpers.
- `internal/cli/ado.go`: argument parsing and config scope routing for fetch/profiles list/config init.
- `internal/cli/root.go`: dependency function signatures may need to accept a scope and expose remote URL discovery.
- Tests in `internal/config` and `internal/cli`.
- No changes expected in Azure DevOps HTTP client, export layout, keyring backend, or workspace repo discovery.

## Pre-Mortem

1. The feature fails because `profiles list --global` still calls `FindRepoRoot` through `resolveLocations`. The plan avoids that by splitting home-dir-only resolution for global profile listing.
2. The feature is confusing because `login --global` appears to work but stores the same key as normal login. The plan rejects `--global` on login/logout.
3. Init creates an active config accidentally, causing `profiles list` to show unverified placeholder profiles. The plan requires every template line to be commented out.
4. Remote prefill becomes brittle by requiring ADO detection to succeed. The plan treats remote parsing as best-effort and never fails init when parsing fails.
5. The loader API becomes ambiguous if callers pass empty repo roots to mean global. The plan uses explicit scope APIs and keeps existing wrappers for compatibility.

## Planning Verification

- [x] Every file/line reference was read directly by me (not from subagent summary alone)
- [x] I ran diagnostic commands myself for facts in the plan
- [x] Each step has a verification checkpoint with concrete command and expected outcome
- [x] I searched for existing patterns before proposing new ones
- [x] I checked current filesystem state for counts, paths, and names
- [x] Blast radius listed if shared code is touched (all usages traced)
- [x] Edge cases documented for every integration point and data transformation
