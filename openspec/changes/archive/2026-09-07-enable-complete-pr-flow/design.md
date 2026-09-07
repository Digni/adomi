## Context

See [proposal.md](proposal.md) for motivation and the four affected capabilities under [specs/](specs/).

The current `adomi ado pr` path is a manual dispatcher in `internal/cli/ado_pr.go`. Write commands validate their own arguments, resolve repository/global configuration and PAT-backed credentials through `newADOMaintenanceClient` in `internal/cli/ado.go`, then call the shared `internal/ado.Client`. The client already reads a pull request by global ID and updates it through the repository-scoped Azure DevOps Git endpoint, but `PullRequestUpdateOptions` carries only title and description. `PullRequest` also models reviewers, identities, and commits as generic or missing data, so it cannot yet make or prove governance writes safely.

Azure DevOps exposes the required lifecycle fields through the [Pull Requests - Update API](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/update?view=azure-devops-rest-7.1), authenticated-user identity through organization connection data, and votes through the [Pull Request Reviewers - Create API](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-request-reviewers/create-pull-request-reviewer?view=azure-devops-rest-7.1). These calls use the existing PAT and HTTP client and require code write permission. Completion can be asynchronous, auto-complete can finish immediately, and repository policies remain authoritative.

This is a high-risk behavior change because completion is irreversible through Adomi and voting or abandonment changes shared Azure DevOps state. The implementation therefore needs request-body contract tests, state-machine tests, exact response validation, a conservative live verification gate, and multi-perspective review.

## Goals / Non-Goals

**Goals:**

- Add lifecycle and authenticated-user vote operations through the existing repository/profile/PAT client path without a new dependency or configuration format.
- Make every mutation state-aware, idempotent when the exact requested state already exists, and fail closed when the preflight or returned state cannot prove the requested effect.
- Pin immediate completion to the fetched source commit, retain supported user-visible completion preferences, and suppress any previously configured policy-bypass behavior.
- Preserve the established plain stdout, compact JSON, stderr, and side-effect-free help contracts.
- Keep agent use safe by documenting that the exact governance outcome needs explicit user authorization.

**Non-Goals:**

- Interactive confirmation prompts or a `--yes` flag. The explicit verb is the CLI authorization surface; generated agent guidance adds the separate user-authorization gate. A prompt would make the command unreliable for automation without adding protection against a wrongly inferred agent instruction.
- Polling Azure DevOps until a merge reaches a terminal merge status.
- Adding or removing arbitrary reviewers, selecting another reviewer identity, raw vote values, vote reset, or wait-for-author voting.
- Policy bypass, optional-policy suppression, abandoned-PR reactivation, completed-PR reversion, or merge reversion.
- Changing how profiles, PATs, proxies, repository roots, or Azure DevOps projects are resolved.

## Decisions

### 1. Add explicit verbs to the existing PR dispatcher

`internal/cli/ado_pr.go` will dispatch `complete`, `auto-complete`, `cancel-auto-complete`, `abandon`, `approve`, `approve-with-suggestions`, and `reject` before falling back to the compatibility fetch form. A focused `internal/cli/ado_pr_governance.go` will own their parsers, state machine, result type, and handler so existing fetch, ensure, link, comment, and thread handlers stay unchanged.

Completion flags will be parsed into presence-aware values rather than ordinary booleans. That preserves the difference between an omitted preference and an explicit `false`. All syntax and value validation will finish before `newADOMaintenanceClient` is called.

Alternative considered: model every verb as a nested Cobra command. The current PR namespace deliberately uses a small manual dispatch layer and command-specific help routing, so changing command construction would enlarge the compatibility surface without helping this feature.

### 2. Use one preflight read and explicit action-specific state rules

Every new command first fetches the pull request by the supplied global ID, validates that the returned ID matches, and resolves its repository ID. The action rules are:

| Action | Writeable preflight state | Idempotent state | Rejected state |
| --- | --- | --- | --- |
| `complete` | active, non-draft, source commit present | completed | abandoned or draft |
| `auto-complete` | active, non-draft | already enabled with no changed preference and no stored policy override | completed, abandoned, or draft |
| `cancel-auto-complete` | active with a setter | active without a setter | completed or abandoned |
| `abandon` | active | abandoned | completed |
| supported vote | active with authenticated identity | same caller vote | completed or abandoned |

An idempotent result performs no write and reports action `unchanged`. Other successes use the invoked command name as `action`. The handler will not retry a state mutation after a source-commit or state race; the caller must fetch and deliberately retry from current state.

Alternative considered: send the requested PATCH or vote unconditionally and rely on Azure DevOps errors. Preflight is required to avoid writes for known no-op or incompatible states and to obtain the repository, commit, completion options, current setter, and reviewer state needed to construct a safe request.

### 3. Introduce typed governance models and a narrow client interface

`internal/ado/pullrequest.go` will add typed identity, commit, completion-option, authenticated-user response, and reviewer-vote models. `PullRequest` will expose `autoCompleteSetBy`, `lastMergeSourceCommit`, `completionOptions`, and typed reviewers. Optional completion booleans use pointers so an absent value remains distinguishable from explicit `false`. This makes exported fetch JSON additively richer when Azure DevOps returns those fields.

`PullRequestUpdateOptions` will gain optional status, completion options, last-merge-source commit, and an explicit auto-complete operation. The operation is tri-state—omit, set to an identity, or clear—so JSON construction cannot confuse “do not touch” with cancellation. A small `PullRequestGovernor` interface will expose authenticated-user lookup and reviewer-vote update; `ADOClient` will compose it alongside the existing PR interfaces.

Alternative considered: continue using `map[string]any` throughout. Maps are still appropriate at the final PATCH-body assembly boundary, but typed inputs and responses are needed for compile-time request construction, reliable comparison, test fakes, and exact success validation.

### 4. Extend the existing repository-scoped PATCH, preserving only safe completion options

`Client.UpdatePullRequest` will remain the single PR PATCH seam. Immediate completion sends status `completed`, `lastMergeSourceCommit.commitId` from the preflight, and the merged safe completion options. Microsoft's REST 7.1 page does not list `lastMergeSourceCommit` among mutable fields, but Microsoft's Azure DevOps CLI sends the fetched value for completion; Adomi follows that compatibility behavior and requires the stale-source live gate below before claiming that Azure DevOps enforces it. Auto-completion sends `autoCompleteSetBy.id` for the authenticated user and the same safe option overlay. Cancellation sends the empty identity UUID used by Microsoft's Azure DevOps CLI to clear `autoCompleteSetBy`; the REST contract does not document cancellation and Microsoft's current Azure DevOps MCP instead sends `null`, so Adomi does not add a mutating fallback and requires live proof of the chosen form. Cancellation does not require current-user lookup. Abandonment sends only status `abandoned`.

Safe options are `mergeStrategy`, `deleteSourceBranch`, `transitionWorkItems`, and `mergeCommitMessage`. Explicit flags overlay fetched values; omitted values remain fetched when present and otherwise stay absent so Azure DevOps and repository policy choose them. If a fetched legacy completion option has no `mergeStrategy` but has deprecated `squashMerge: true`, Adomi normalizes that intent to explicit `mergeStrategy: squash`; `squashMerge: false` remains unspecified so the server's documented no-fast-forward default applies. Fetched `bypassPolicy`, `bypassReason`, optional-policy-ignore fields, and the deprecated `squashMerge` field are never copied as active settings. Every complete or auto-complete write instead sends a freshly constructed completion-options object with `bypassPolicy: false` and `autoCompleteIgnoreConfigIds: []`, so a previously stored bypass or optional-policy exclusion cannot survive the Adomi request. An already scheduled pull request with either unsafe setting is not an idempotent no-op: Adomi performs a preference-only sanitizing update even when the user supplied no flags. Accepted CLI strategies map exactly to Azure DevOps values:

| CLI value | Azure DevOps value |
| --- | --- |
| `no-fast-forward` | `noFastForward` |
| `squash` | `squash` |
| `rebase` | `rebase` |
| `rebase-merge` | `rebaseMerge` |

Alternative considered: define an Adomi default merge strategy. That could silently conflict with repository policy and change an existing auto-complete preference, so the server remains the default authority.

### 5. Resolve the caller once and use the reviewer endpoint only for that identity

For new auto-complete requests and all vote commands, the client will GET the organization's `/_apis/connectionData` endpoint with the existing authenticated HTTP path and require a non-empty `authenticatedUser.id`. Vote commands then PUT the reviewer resource at `repositories/{repositoryId}/pullRequests/{pullRequestId}/reviewers/{authenticatedUserId}` with the caller ID and exactly one supported vote: `10`, `5`, or `-10`.

If the caller is already in the fetched typed reviewer list, the request preserves `isRequired`; if absent, it adds only that caller as a non-required reviewer. Success requires the response identity and vote to match exactly. The lifecycle state remains the preflight state because a vote command does not request a PR transition.

Alternative considered: accept a reviewer ID or generic numeric vote. That would turn a bounded self-vote feature into reviewer administration and would bypass the explicit command semantics requested here.

### 6. Validate the returned external state before emitting stdout

Lifecycle PATCH responses must repeat the requested PR ID and prove the action-specific result: completed; active with the expected setter or completed for newly enabled auto-complete; active with a setter or completed for an existing schedule's preference-only update; active with no setter object for cancellation; or abandoned. A successful HTTP status without that evidence is an error. Vote responses must repeat the authenticated reviewer ID, exact numeric vote, and exact required-reviewer designation sent by Adomi.

Plain success emits only `<pull-request-id>\n`. JSON success emits one compact object with `pullRequestId`, `action`, and `status`; it conditionally adds `mergeStatus`, `autoCompleteEnabled`, `reviewerId`, and `vote`. No success data is written until all response checks pass. Complete and auto-complete do not poll: returned `status`, `mergeStatus`, and `autoCompleteEnabled` describe only the accepted response.

Alternative considered: treat any 2xx as success. These operations are too consequential to report success when Azure DevOps returns a different resource or a state that does not match the request.

### 7. Treat live API details as a controlled verification boundary

HTTP contract tests will lock down URLs, methods, bodies, credentials, and malformed-response failures. A live gate will then use disposable Azure DevOps pull requests, because three service behaviors cannot be proven by a fake server: clearing `autoCompleteSetBy` with the empty identity UUID, whether an eligible auto-complete request returns active/scheduled or immediately completed, and whether completion rejects a source branch that advances after preflight instead of silently ignoring `lastMergeSourceCommit`.

The live gate requires explicit user authorization and designated disposable PR IDs. It will use separate PRs where completion, cancellation, abandonment, and terminal states would interfere. If Azure DevOps behavior contradicts the design, implementation stops and the durable proposal, specs, design, and tasks are updated before proceeding.

## Risks / Trade-offs

- [A source branch advances between preflight and immediate completion, and Azure DevOps ignores the submitted `lastMergeSourceCommit`] → Send the exact fetched commit, do not retry a stale-source failure, and exercise an actual preflight-to-PATCH source advance in the live gate; do not claim or ship source-pinned completion if the service still completes it.
- [Auto-complete cancellation encoding changes or is rejected by the service] → Lock the intended empty-identity request in tests, require the response to omit the setter, and verify the operation against a disposable policy-blocked PR before claiming live support.
- [A vote accidentally changes reviewer governance] → Resolve only the authenticated identity, reject caller-supplied identities and raw votes, preserve an existing `isRequired` value, and validate the returned reviewer and vote.
- [A previously stored bypass setting leaks into completion] → Build a fresh completion-options object, force `bypassPolicy` false and optional-policy-ignore IDs empty, and sanitize an existing schedule instead of treating it as unchanged.
- [Azure DevOps returns `completed` while merge processing is still queued or later fails] → Do not poll or claim merge success; expose the returned merge status in JSON and document the asynchronous boundary.
- [Idempotent preflight data becomes stale before a skipped write] → Report only the fetched state. This is an accepted race for a no-op command; callers needing stronger freshness can rerun the command.
- [Typed PR fields change existing fetch JSON] → Governance fields are additive, typed reviewer parsing preserves unknown and explicitly present reviewer properties during re-export, and current export paths remain unchanged.

## Migration Plan

1. Add typed models, lifecycle PATCH construction, authenticated identity lookup, and reviewer-vote transport behind tests.
2. Add the command parser/state machine/output layer and side-effect-free help behind tests.
3. Update generated skill content and public documentation, then run the controlled live Azure DevOps gate when authorized and disposable PRs are available.
4. Run high-risk review, address findings, and finish with repository-wide formatting, tests, vet, build, OpenSpec validation, and diff checks.

There is no configuration or data migration. Rolling back the binary removes the new command surface but cannot undo an already completed pull request. Abandonment and votes can be changed later in Azure DevOps, but reactivation and vote reset remain outside Adomi. Auto-complete can be reversed with the new cancellation command while the pull request is still active.
