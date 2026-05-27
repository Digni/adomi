## Context

`adomi` already requires repository-local execution for Azure DevOps context commands: `runADOPullRequest` resolves the repo root and home directory before loading config (`internal/cli/ado.go:83-96`), and `resolveLocations` fails when the current directory is not inside a Git repository (`internal/cli/ado.go:303-316`). The Cobra command tree currently exposes a single `adomi ado pr <pull-request-id>` command for fetching PR metadata and threads (`internal/cli/root.go:118-147`), with argument parsing limited to a positive PR ID plus `--profile` and `--global` (`internal/cli/ado.go:442-466`).

The Azure DevOps client already supports read-only PR operations: `FetchPullRequest` gets PR metadata (`internal/ado/client.go:101-127`), `FetchPullRequestThreads` gets all threads for the PR repository (`internal/ado/client.go:129-157`), and URL construction is centralized for PR and thread endpoints (`internal/ado/client.go:212-228`). The PR model already carries the key fields needed for write workflows, including ID, title, description, status, source/target refs, repository, reviewers, thread status, and comments (`internal/ado/pullrequest.go:14-82`).

Repository inference has a starting point but not the full shape needed for PR creation. `gitRemoteURLs` already shells out to `git -C <repoRoot> remote -v` and de-duplicates remote URLs (`internal/cli/root.go:271-284`), while config initialization can parse Azure DevOps remote URLs for organization/project (`internal/config/config.go:250-299`). It does not yet infer repository name/ID or current branch. The loaded profile config currently has base URL, organization, project, API version, proxy, and PAT reference fields only (`internal/config/config.go:34-42`).

The Azure DevOps REST documentation confirms the write API shape this change needs: listing PRs supports filtering by repository, source ref, target ref, and status; creating a PR is `POST .../_apis/git/repositories/{repositoryId}/pullrequests`; updating title/description/status is `PATCH .../_apis/git/repositories/{repositoryId}/pullrequests/{pullRequestId}`; replying to a thread is `POST .../_apis/git/repositories/{repositoryId}/pullRequests/{pullRequestId}/threads/{threadId}/comments`; updating thread status is `PATCH .../_apis/git/repositories/{repositoryId}/pullRequests/{pullRequestId}/threads/{threadId}`. These APIs require code-write or thread-write PAT scopes, which is a stricter credential need than read-only context fetching.

The generated `adomi` skill content is also part of the agent-facing surface. `generateSkillContent` currently documents work item fetch, the legacy PR fetch form, configuration discovery, and PAT safety (`internal/cli/agent.go:114-184`), with tests asserting that the generated skill mentions Azure DevOps, `adomi ado fetch <work-item-id>`, `adomi ado pr <pull-request-id>`, and credential commands (`internal/cli/agent_test.go:409-428`). This change must update that generated guidance so agents discover PR maintenance without overstepping the conservative write boundary.

## Goals / Non-Goals

**Goals:**

- Make PR creation/update safe for repeated agent runs through an idempotent `adomi ado pr ensure` command.
- Infer repository name/ID and source branch from the current Git repository by default, with explicit flags only as overrides or fallback when inference is ambiguous.
- Add explicit commands for replying to an existing PR thread and setting a thread status to `fixed` or `active`.
- Preserve the existing read-only PR context export behavior and keep `adomi ado pr <id>` as a compatibility alias.
- Keep stdout minimal and scriptable: default commands print only the created/updated resource ID followed by a newline; `--json` prints one compact JSON object.
- Keep tests isolated from real Azure DevOps and real Git wherever possible by extending the existing dependency-injection seams.
- Update generated `adomi` skill guidance so agents know the PR maintenance commands, required context-first workflow, PAT-scope implication, and explicit non-goals.

**Non-Goals:**

- Do not approve/reject PRs, complete/merge PRs, abandon PRs, set auto-complete, bypass policies, or manage reviewers.
- Do not add broad PR editing beyond title and description updates for existing active PRs.
- Do not auto-resolve all comments or infer that a thread is fixed without an explicit thread ID and command.
- Do not require new third-party Go dependencies.
- Do not change work item fetching, PR context export layout, PAT storage semantics, or the existing configuration precedence.
- Do not add draft PR creation or work-item linking in this first PR maintenance slice.

## Decisions

1. **Make `adomi ado pr` a subcommand namespace while preserving the numeric alias.**
   - Canonical commands become `adomi ado pr fetch <id>`, `adomi ado pr ensure`, `adomi ado pr reply <id> --thread <thread-id> --message-file <path>`, `adomi ado pr resolve <id> --thread <thread-id>`, and `adomi ado pr reopen <id> --thread <thread-id>`.
   - `adomi ado pr <id>` remains a visible or hidden compatibility path that behaves like `adomi ado pr fetch <id>`.
   - Rationale: The current single `pr` command shape in `internal/cli/root.go:140-147` cannot grow cleanly without overloading the first positional argument. A namespace is clearer for agents while preserving existing scripts.
   - Alternative considered: Add sibling commands such as `adomi ado pr-create` and `adomi ado pr-resolve`. Rejected because all behavior is PR-specific and should be discoverable under one namespace.

2. **Use an idempotent ensure workflow instead of a create-only command.**
   - `ensure` resolves source ref, target ref, and repository, lists active PRs matching those refs, and creates a PR only when none exists.
   - If exactly one matching active PR exists, `ensure` patches only fields explicitly provided by flags (`--title`, `--description-file`).
   - If more than one matching active PR exists, `ensure` fails before mutation and reports the ambiguity on stderr through the existing error path.
   - Creating a missing PR requires `--title`; if no title is provided and no matching active PR exists, `ensure` fails before mutation instead of guessing from a branch name.
   - Rationale: Agents rerun commands; idempotency avoids duplicate PRs.
   - Alternative considered: `create` with a server-side duplicate error. Rejected because Azure DevOps permits multiple active PRs from similar refs in some workflows and because duplicate prevention belongs in the agent-facing tool.

3. **Infer repository context from Git, then allow explicit overrides.**
   - Source branch defaults to `git -C <repoRoot> branch --show-current`; detached HEAD is an error unless `--source` is provided.
   - Repository defaults to the Azure DevOps remote URL whose host/base path matches the loaded profile `baseUrl` and whose project path segment matches the loaded profile project, case-insensitively after URL decoding. `--repository <name-or-id>` overrides ambiguous or non-standard remotes.
   - When multiple remotes match, use the one named `origin` only if it is the sole `origin` match; otherwise fail as ambiguous.
   - Target branch defaults to `--target` when provided; otherwise infer the remote default branch from the selected remote's symbolic `refs/remotes/<remote>/HEAD` when available. If neither is available, fail and require `--target`.
   - Branch flags accept short names (`main`, `feature/x`) and normalize them to `refs/heads/<name>` before calling Azure DevOps. If source and target normalize to the same ref, fail before mutation.
   - Rationale: The user wants this to run inside a repo and be mostly inferred. Requiring a repository config field would duplicate information already present in Azure Repos remotes.
   - Alternative considered: Add required `repository` and `targetBranch` fields to `.adomi/config.yaml`. Rejected for the first version because it makes agents depend on extra setup and violates the inference-first direction.

4. **Keep PR write operations conservative and explicit.**
   - `ensure` creates a PR with source ref, target ref, required title, and optional description. For updates, it only patches title/description when the user provides those flags.
   - `reply` creates a text comment on an existing thread from a message file or explicit `--message` value, preferring `--message-file` in docs to keep shell quoting predictable.
   - `reply`, `resolve`, and `reopen` first fetch the PR by ID to obtain the repository ID required by Azure DevOps thread URLs; if the PR response has no repository ID, the command fails before thread mutation, matching the existing fetch-bundle guard in `internal/ado/pullrequest.go:94-96`.
   - `resolve` patches the thread status to `fixed`; `reopen` patches it to `active`.
   - Rationale: The user explicitly wants comments/thread resolution, but nothing more. Avoiding reviewer management, voting, and completion keeps PAT usage safer and behavior agent-safe.
   - Alternative considered: Implement `resolve --all-fixed` or infer thread status from fetched comments. Rejected because it is too magical for review state changes.

5. **Extend existing client/model seams rather than adding a second Azure DevOps client.**
   - Add interfaces for PR maintenance operations alongside the existing `PullRequestFetcher` (`internal/ado/pullrequest.go:79-82`) and extend the CLI `ADOClient` interface (`internal/cli/root.go:35-39`).
   - Add reusable URL helpers for repository-scoped PR list/create/update, thread-comment create, and thread update endpoints, matching the current URL helper pattern in `internal/ado/client.go:212-228`.
   - Rationale: The current dependency seams (`Dependencies.NewADOClient`, `FetchPullRequest`, `ExportPullRequest`) make CLI tests fast and deterministic (`internal/cli/root.go:41-56`). Write workflows should follow that pattern.
   - Alternative considered: Put all write orchestration in `internal/cli`. Rejected because Azure DevOps request/response logic belongs with the existing `internal/ado` client and models.

6. **Update generated agent skill guidance as part of the feature.**
   - Add a PR maintenance section to generated `SKILL.md` covering `adomi ado pr fetch <id>`, `ensure`, `reply`, `resolve`, and `reopen`.
   - Instruct agents to fetch/read PR context before replying or resolving threads unless the user already supplied the relevant thread details.
   - State the safety boundary: no approval, rejection, merge/complete, abandon, auto-complete, policy bypass, or reviewer management.
   - Preserve PAT secrecy guidance and note that PR maintenance requires credentials with PR/thread write permissions.
   - Rationale: the generated skill is how other agents learn the CLI surface; if it remains read-only, agents will miss the new capability or invent unsafe workflows.

7. **Keep stdout data-only with resource IDs by default.**
   - `ensure` prints the PR ID; `reply` prints the created comment ID; `resolve` and `reopen` print the thread ID.
   - `--json` prints a compact object with at least resource ID, action (`created`, `updated`, `unchanged`, `replied`, `resolved`, `reopened`), repository, source/target refs where relevant, and URL fields when available.
   - Rationale: IDs are stable and immediately useful to subsequent agent commands. This matches the existing stream discipline where successful fetch prints only data to stdout and help/errors go to stderr (`openspec/specs/cli-command-surface/spec.md:104-110`).
   - Alternative considered: Print the web URL by default. Rejected because Azure DevOps API responses expose API URLs, while browser URL construction can vary for on-prem/server base paths.

## Risks / Trade-offs

- **Repository inference fails for non-Azure remote formats or multiple ADO remotes** → Mitigation: fail before mutation with a clear error and support `--repository` override.
- **Target branch inference is unavailable** → Mitigation: fail before mutation and require `--target`; do not guess `main`.
- **Source branch has not been pushed to Azure Repos** → Mitigation: do not add a local preflight in the first slice; surface the Azure DevOps create error with existing non-2xx response context and document that agents should push before `ensure`.
- **PAT has read scope but not write/thread scope** → Mitigation: return Azure DevOps non-2xx errors with existing response-body context and document required scopes.
- **`--description-file` overwrites human edits on an existing PR** → Mitigation: only update description when explicitly provided; leave it untouched otherwise. Future managed-block behavior can be proposed separately if this becomes painful.
- **More than one active PR matches source/target** → Mitigation: do not mutate; require explicit manual resolution or a future `--pull-request` update command.
- **Thread status enum differs across ADO Server versions** → Mitigation: only send statuses already observed in current read model/docs (`active`, `fixed`) and surface server errors without translating them away.

## Migration Plan

- Add the PR namespace while preserving `adomi ado pr <id>` fetch behavior.
- Extend repository inference and PR write client operations behind tests before wiring commands.
- Update generated skill guidance and docs so agents prefer `adomi ado pr fetch <id>` and use explicit thread IDs for replies/resolution.
- Rollback is a code revert; no persistent data migration is required because the change does not alter exported context formats or credential storage.

## Edge Cases And Failure Modes

- Current directory is outside a Git repo: fail through `FindRepoRoot` before loading config or mutating Azure DevOps.
- Git branch is detached or empty: fail unless `--source` is supplied.
- Remote URL parsing returns no Azure DevOps repository: fail unless `--repository` is supplied.
- Multiple Azure DevOps remotes match profile/project: fail unless `--repository` or remote-selection logic disambiguates.
- Target branch cannot be inferred: fail unless `--target` is supplied.
- Branch names already include `refs/heads/`: preserve them; otherwise prefix exactly once.
- PR list response is empty: create a new PR.
- PR list response contains one active PR: patch only explicitly supplied fields; return unchanged when no patch fields are supplied.
- PR list response contains multiple active PRs: fail before mutation.
- Create/update/comment/thread API returns non-2xx or malformed JSON: return an error, keep stdout empty, and do not attempt follow-up mutations.
- Message file is missing, empty, or unreadable: fail before making a network request.
- Thread ID or PR ID is non-positive: fail during argument parsing before loading credentials.
- `ensure` would create a PR but no `--title` is provided: fail before mutation.
- Source and target refs normalize to the same `refs/heads/...` value: fail before mutation.
- Reply/resolve/reopen PR fetch returns no repository ID: fail before creating a comment or updating thread status.

## Verification Checkpoints

- After repository inference helpers: run `go test ./internal/config ./internal/cli -run 'Test.*Remote|Test.*Branch|Test.*Repository'`; expected: Azure DevOps remote parsing, branch detection, and ambiguity cases pass without network calls.
- After Azure DevOps write client methods: run `go test ./internal/ado -run 'TestClient.*PullRequest|Test.*PullRequestMaintenance|Test.*Thread'`; expected: request method, URL, auth, body, non-2xx, and decoding tests pass.
- After ensure workflow orchestration: run `go test ./internal/ado ./internal/cli -run 'Test.*Ensure'`; expected: create, update, unchanged, ambiguous, and inference-failure scenarios pass.
- After reply/resolve/reopen command wiring: run `go test ./internal/cli -run 'Test.*Reply|Test.*Resolve|Test.*Reopen'`; expected: stdout is ID-only, stderr has prompts/errors only, and invalid args make no network calls.
- After generated skill updates: run `go test ./internal/cli -run TestAgentSkill`; expected: generated skill tests pass and mention PR maintenance commands plus safety boundaries.
- After specs/docs alignment: run `openspec validate support-ado-pr-maintenance --strict`; expected: OpenSpec validation passes.
- Final verification: run `go test ./...`; expected: full suite passes.

## Blast Radius

- `internal/cli/root.go`: PR command tree changes from one numeric command to a namespace with a compatibility numeric path; `ADOClient` and `Dependencies` need write-operation seams.
- `internal/cli/ado.go`: current PR fetch runner and parser are split/extended for fetch, ensure, reply, resolve, and reopen.
- `internal/ado/client.go`: new write methods and URL helpers share auth, response-error, base URL, project, and API-version behavior with existing methods.
- `internal/ado/pullrequest.go`: existing PR/thread models may be extended for list responses, write request bodies, and result summaries.
- `internal/config/config.go`: remote parsing helpers may be extended to include repository names; existing config loading should remain compatible.
- `openspec/specs/cli-command-surface/spec.md`: public CLI command and stream behavior expands.
- `internal/cli/agent.go` and `internal/cli/agent_test.go`: generated skill content and tests need to mention conservative PR maintenance commands, context-before-maintenance guidance, PAT write-scope notes, and non-goals.
- Generated `adomi` skill/docs: agent instructions need to mention conservative PR maintenance commands and non-goals.

## Pre-Mortem

1. The change failed because repository inference parsed organization/project but not repository from SSH remotes, mirroring the current config-init parser that stops after org/project (`internal/config/config.go:264-299`). Mitigation: add explicit tests for HTTPS, SSH, visualstudio.com, and unsupported remote forms that include repository extraction.
2. The change failed because the new `adomi ado pr` namespace broke existing `adomi ado pr 42` workflows currently parsed as a numeric ID (`internal/cli/root.go:140-147`, `internal/cli/ado.go:442-466`). Mitigation: keep a compatibility path and test both `pr 42` and `pr fetch 42`.
3. The change failed because an agent rerun created duplicate PRs when target branch inference differed from Azure DevOps search refs. Mitigation: normalize source/target refs to `refs/heads/<name>` before both list and create calls, and test duplicate-prevention around the exact search query.
4. The change shipped but generated skills still documented only read-only PR context fetching, so agents did not discover `ensure`, `reply`, `resolve`, or `reopen`. Mitigation: add an `agent-skill-creation` spec delta and tests for generated skill content.

## Open Questions

- Should a future follow-up add managed description blocks to avoid replacing human-edited PR descriptions? This proposal intentionally keeps description replacement explicit and whole-field only when `--description-file` is supplied.

## Planning Verification

- [x] Every file/line reference was read directly by me.
- [x] I ran diagnostic commands myself for facts in the plan: `openspec list --json`, `openspec status --change support-ado-pr-maintenance --json`, `openspec instructions ...`, `find openspec/specs`, `rg` searches for PR/remote/branch patterns, `nl -ba` reads for referenced code/spec lines, and fetched Azure DevOps REST documentation pages for PR list/create/update/comment/thread APIs.
- [x] Each step has a verification checkpoint with concrete command and expected outcome.
- [x] I searched for existing patterns before proposing new ones.
- [x] I checked current filesystem state for spec names and change paths.
- [x] Blast radius listed if shared code is touched, with usages traced.
- [x] Edge cases documented for every integration point and data transformation.
