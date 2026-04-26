# Azure DevOps PAT References

Date: 2026-04-26

## Goal

Add an explicit Azure DevOps PAT reference to `adomi` profiles so multiple local/global config profiles can intentionally share one `adomi` keyring entry without coupling to Flive's storage.

User-approved constraints:

- Do not fall back to or read Flive secure storage.
- Add profile config support for a PAT reference, named `patRef`.
- Add direct login by PAT reference.
- Add profile-aware login for global/local config profiles.
- Preserve existing profile-name-based credential lookup when `patRef` is omitted.
- It is acceptable for `ado login --profile <name>` to become config-backed; backwards compatibility for direct profile-name login is not a requirement for this change.

## Verified Current Code

- `internal/config/config.go:33-40` defines `Profile` with `Name`, `BaseURL`, `Organization`, `Project`, `APIVersion`, and `Proxy`; there is no `patRef` field.
- `internal/config/config.go:52-80` loads the selected profile, assigns `profile.Name`, defaults `APIVersion`, validates required config fields, and stores it on `Loaded.Profile`.
- `internal/cli/ado.go:57-67` routes `fetch --global` to `config.GlobalScope`, then currently reads the PAT with `deps.PATStore.Get(profileConfig.Name)`.
- `internal/cli/ado.go:104-122` implements `ado login` by parsing only `--profile`, reading a secret, and storing it with `deps.PATStore.Set(profile, pat)`.
- `internal/cli/ado.go:278-296` has `parseProfileFlag`, which rejects any flag except `--profile`.
- `internal/securestore/keyring.go:9-15` defines the `adomi.azure-devops` keyring service and a `Store` interface keyed by the profile string.
- `internal/securestore/keyring.go:45-64` uses the fixed `adomi.azure-devops` service with the passed string as the keyring user/account for get/set/delete.
- `internal/cli/root.go:28-32` exposes `PATStore` as `Get`, `Set`, and `Delete` by string key; this interface can remain unchanged if the caller passes the resolved credential reference.
- `internal/config/config.go:160-183` renders the commented init template and currently has no `patRef` suggestion.
- `rg -n "patRef|PATRef|PatRef|adoPatRef|ado_pat_ref|--pat-ref" .` found no existing `adomi` implementation.
- Current branch is `main`, clean, at merge commit `f24b8fb` including PR #3.

## Design

### Config Shape

Add `PATRef string `yaml:"patRef"` to `config.Profile`.

Credential reference resolution:

- Add a small method/helper on `Profile`, likely `CredentialRef() string`.
- If `PATRef` is non-empty after trimming whitespace, return the trimmed `PATRef`.
- Otherwise return `Name`.

This keeps existing config valid and lets local/global profiles share an `adomi` credential by naming the same `patRef`.

`patRef` validation:

- Apply the same validation to YAML-loaded `PATRef` values and direct `--pat-ref` values.
- Normalize by `strings.TrimSpace`.
- Reject empty-after-trim when the user explicitly provided `patRef`/`--pat-ref`.
- Reject any Unicode control character, including newlines, carriage returns, and tabs.
- Reject values that start with `-` in CLI parsing so flags are not accepted as refs.
- Cap refs at 256 bytes after trimming. This is long enough for generated workspace refs but avoids backend-dependent pathological keyring names.
- Do not restrict to a narrow slug character set; Flive-style refs such as `ws-<id>` work, and future refs can use ordinary punctuation without migration.

Example:

```yaml
azureDevOps:
  profiles:
    flive-workspace-a:
      patRef: ws-123
      baseUrl: https://dev.azure.com/org
      project: ProjectA
```

### Fetch Lookup

Change `ado fetch` to call:

```go
deps.PATStore.Get(profileConfig.CredentialRef())
```

Output paths and exported context should continue to use `profileConfig.Name`; `patRef` is only credential identity, not profile identity.

### Login Parsing

Replace the login-only use of `parseProfileFlag` with a dedicated parser accepting:

- `--profile <profile-name>`
- `--pat-ref <ref>`
- `--global`

Supported forms:

1. `adomi ado login --pat-ref <ref>`
   - Directly stores the entered PAT under `<ref>` in `adomi.azure-devops`.
   - Does not require a repo, config file, or home config lookup.

2. `adomi ado login --profile <profile-name>`
   - For local/default scope, resolve repo/home config like fetch does.
   - Load the selected profile with `config.DefaultScope`.
   - Store under `profile.PATRef` if set, else `profile.Name`.

3. `adomi ado login --global --profile <profile-name>`
   - Requires home dir.
   - Does not require a repo.
   - Loads global config with `config.GlobalScope`.
   - Stores under `profile.PATRef` if set, else `profile.Name`.

Invalid combinations:

- `--global` without `--profile`: reject, because direct `--pat-ref` already works outside config and `--global` only makes sense for config-backed profile resolution.
- `--profile` and `--pat-ref` together: reject as ambiguous.
- No `--profile` and no `--pat-ref`: reject.
- Missing values or flag-looking values for either option: reject.

Prompt text should use the resolved credential ref, e.g. `Azure DevOps PAT for <ref>:`, so users can see where the PAT is being stored.

### Logout

Update logout in this step so credential lifecycle matches login/fetch.

Supported forms:

1. `adomi ado logout --pat-ref <ref>`
   - Directly deletes `<ref>` from the `adomi.azure-devops` keyring service.
   - Does not require repo/config/home lookup.

2. `adomi ado logout --profile <profile-name>`
   - Resolve default scoped config like config-backed login.
   - Delete `profile.PATRef` if set, else `profile.Name`.

3. `adomi ado logout --global --profile <profile-name>`
   - Resolve global config without requiring a repo.
   - Delete `profile.PATRef` if set, else `profile.Name`.

Invalid combinations mirror login: reject `--profile` with `--pat-ref`, reject `--global --pat-ref`, reject `--global` without `--profile`, reject missing values and flag-looking values.

### Init Template

Add commented `patRef` to the init template so the new feature is discoverable:

```yaml
#       patRef: my-shared-ado-pat
```

Default can be empty or derived from the profile name. Prefer empty/commented placeholder in the template values so users opt in intentionally. Since all template lines remain commented, this does not activate anything by default.

## Test Plan

Write failing tests first.

1. `internal/config`
   - Profile YAML with `patRef` loads into `Profile.PATRef`.
   - `Profile.CredentialRef()` returns `patRef` when set.
   - `Profile.CredentialRef()` falls back to `Name` when `patRef` is empty.
   - `Profile.CredentialRef()` trims surrounding whitespace before returning `patRef`.
   - Config validation rejects `patRef` values containing control characters.
   - Config validation rejects explicit whitespace-only `patRef`.
   - Config validation rejects `patRef` values longer than 256 bytes.
   - Rendered init template includes a commented `patRef` line and every line remains commented.

2. `internal/cli`
   - `ado fetch` uses `patRef` for `PATStore.Get` while export still uses the profile name.
   - `ado login --pat-ref shared` stores under `shared` without calling repo/config lookup.
   - `ado login --pat-ref " shared "` stores under `shared`.
   - `ado login --pat-ref` rejects missing, flag-looking, control-character, whitespace-only, and too-long refs.
   - `ado login --profile repo-profile` resolves default config and stores under the profile's `patRef`.
   - `ado login --global --profile global-profile` resolves global config without requiring a repo and stores under the profile's `patRef`.
   - `ado login --global --profile global-profile` falls back to profile name when `patRef` is empty.
   - `ado logout --pat-ref shared` deletes `shared` without calling repo/config lookup.
   - `ado logout --profile repo-profile` resolves default config and deletes the profile's `patRef`.
   - `ado logout --global --profile global-profile` resolves global config without requiring a repo and deletes the profile's `patRef`.
   - Reject `ado login --profile x --pat-ref y`.
   - Reject `ado login --global --pat-ref y`.
   - Reject the equivalent invalid logout combinations.
   - Reject missing values and flag-looking values for `--profile` and `--pat-ref`.
   - Real wiring test: create `.adomi/config.yaml` with `patRef`, store the fake PAT under the ref, run `ado fetch`, assert client creation uses the PAT and exported output path/context still use the profile name.

Verification checkpoints:

- After red tests: `go test -count=1 ./internal/config ./internal/cli` should fail on missing `patRef` behavior.
- After config implementation: `go test -count=1 ./internal/config` should pass.
- After CLI implementation: `go test -count=1 ./internal/cli` should pass.
- Final: `gofmt -l cmd internal`, `go test -count=1 ./...`, `go build -o /tmp/adomi-smoke ./cmd/adomi`, and `git diff --check`.

## Edge Cases

- Existing configs without `patRef`: continue using profile name as the keyring ref.
- Empty `patRef`: treat as omitted and fall back to profile name.
- Whitespace-only `patRef`: reject during config validation because it is an explicit but invalid credential ref.
- Explicit whitespace-only `--pat-ref`: reject because the user provided a direct credential ref.
- `patRef` with control characters: reject during config validation before any keyring lookup or prompt rendering.
- `--pat-ref` with control characters: reject during argument parsing before prompting for a PAT.
- `patRef` longer than 256 bytes: reject during config validation; direct `--pat-ref` also rejects before prompting.
- `patRef` points to a missing keyring entry: fetch returns the existing keyring read error, but the error should mention the resolved ref rather than the profile name.
- Global login outside a repo: works only for `--global --profile`, because it needs only home config.
- Local/default profile login outside a repo: fails repo resolution, matching default scoped config behavior.
- Direct `--pat-ref` login outside a repo: works and does not inspect config.
- `--profile` and `--pat-ref` together: rejected to avoid storing under the wrong key.
- `--global --pat-ref`: rejected because no config profile is being resolved.
- Direct `--pat-ref` logout outside a repo: works and does not inspect config.
- Profile-backed logout outside a repo: local/default scope fails repo resolution; `--global --profile` uses global config only.
- Profile identity and credential identity diverge: export paths and context metadata continue to use profile name; only keyring lookup/storage uses credential ref.
- No Flive fallback: all reads/writes stay inside `securestore.KeyringStore`'s `adomi.azure-devops` service.

## Blast Radius

- `internal/config/config.go`: add `patRef` YAML field, credential reference helper, and init template line.
- `internal/config/config_test.go`: add config loading/helper/template tests.
- `internal/cli/ado.go`: update fetch credential lookup and login/logout argument parsing/storage flow.
- `internal/cli/ado_test.go`: add login, logout, and fetch coverage for `patRef`.
- `internal/securestore/keyring.go`: no storage service change expected; only error wording may be changed from "profile" to "reference" if needed.
- `internal/cli/root.go`: dependency signatures likely unchanged.
- `docs/adomi_azure_devops_handover.md`: update the canonical config example to include commented/documented `patRef`.

## Pre-Mortem

1. The feature fails because login stores under `patRef` but fetch still reads by `profileConfig.Name` at `internal/cli/ado.go:67`. The plan explicitly changes fetch to resolve `Profile.CredentialRef()`.
2. The feature breaks direct login because `--pat-ref` accidentally goes through config resolution and requires a repo. The plan separates direct `--pat-ref` from config-backed `--profile` login.
3. The feature causes confusing output paths if `patRef` replaces the profile name everywhere. The plan keeps `profileConfig.Name` for export identity and uses `patRef` only for keyring identity.
4. The feature leaks active PATs because logout still deletes by profile name after login/fetch switch to `patRef`. The plan now updates logout with direct and config-backed credential-ref deletion.
5. Invalid refs create hard-to-delete keyring entries or broken prompts. The plan now requires normalization and validation for YAML and CLI refs.

## Planning Verification

- [x] Every file/line reference was read directly by me (not from subagent summary alone)
- [x] I ran diagnostic commands myself for facts in the plan
- [x] Each step has a verification checkpoint with concrete command and expected outcome
- [x] I searched for existing patterns before proposing new ones
- [x] I checked current filesystem state for counts, paths, and names
- [x] Blast radius listed if shared code is touched (all usages traced)
- [x] Edge cases documented for every integration point and data transformation
