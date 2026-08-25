package cli

const configInitHelp = `Create an adomi configuration template.

Usage:
  adomi config init [--global]

Flags:
  --global   create the user-level configuration instead of the repository configuration

stdout:
  Prints only the created configuration file path on success.

Compatibility:
  adomi ado config init [--global] is a hidden compatibility alias for this command.
`

const adoFetchHelp = `Fetch Azure DevOps work item context.

Usage:
  adomi ado fetch <work-item-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <work-item-id>   positive Azure DevOps work item ID

Flags:
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object with path, work item count, and attachment count

stdout:
  Prints only the exported work item context directory path on success, or one JSON object with --json.
  Progress and summary lines are written to stderr.
`

const adoPipelineHelp = `Inspect read-only Azure DevOps pipeline run status.

Usage:
  adomi ado pipeline list [--last <N>] [--profile <profile-name>] [--global]
  adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]

Scope:
  list without --last requests the exact inProgress runs across YAML and classic Build pipelines.
  list --last <N> requests the N most recently queued runs in the project across any status, with N in the range 1..200.
  Pagination is a best-effort one-shot view, not a transactional snapshot.
  get accepts a decimal Build run ID in the range 1..2147483647.
  Both commands require a Git repository, including with --global; --profile selects a configured profile.
  The PAT needs vso.build read scope. Pipeline endpoints must use HTTPS or loopback HTTP.
  Loopback HTTP requests bypass configured proxies so credentials remain on-machine.

stdout:
  Success is one compact JSON value. list returns an ordered runs array; get returns one run object.
  Every documented key is present, and unavailable result, source, timestamp, or web-link values are null.

Exclusions:
  These commands perform no polling, stage/job/environment detail lookup, mutation, or classic Release inspection.
`

const adoWikiNamespaceHelp = `Fetch Azure DevOps wiki context into the current Git repository.

Usage:
  adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive] [--profile <profile-name>] [--global] [--json]

The wiki identifier and --page are required. The page path must be an absolute Azure DevOps wiki path beginning with /.
Use --recursive to include all descendant pages; otherwise only the selected page is fetched.
--profile selects a configured Azure DevOps profile, --global uses user-level configuration, and --json writes one compact result object to stdout.
A Git repository is always required because output is written below .adomi/context/wikis in that repository.
Markdown links are preserved, but attachments and other linked resources are not downloaded. This command does not perform indexed wiki search.

stdout:
  Prints only the exported wiki context directory path on success, or one JSON object with --json.
  Progress and summary lines are written to stderr.
`

const adoWikiFetchHelp = `Fetch Azure DevOps wiki context into the current Git repository.

Usage:
  adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive] [--profile <profile-name>] [--global] [--json]

Arguments:
  <wiki-id-or-name>   required Azure DevOps wiki ID or name

Flags:
  --page <absolute-wiki-page-path>   required absolute wiki page path beginning with /
  --recursive                       include all descendant pages
  --profile <profile-name>          select a configured Azure DevOps profile
  --global                          use user-level configuration
  --json                            print one compact JSON object with path and page count

Rules:
  A Git repository is always required because output is written below .adomi/context/wikis in that repository.
  Markdown links are preserved, but attachments and other linked resources are not downloaded.
  This command does not perform indexed wiki search.

stdout:
  Prints only the exported wiki context directory path on success, or one JSON object with --json.
  Progress and summary lines are written to stderr.
`

const adoWorkItemCommentHelp = `Add a comment to an Azure DevOps work item.

Usage:
  adomi ado comment <work-item-id> (--message <text> | --message-file <path>) [--profile <profile-name>] [--global] [--json]

Arguments:
  <work-item-id>   positive Azure DevOps work item ID

Flags:
  --message <text>        inline comment text
  --message-file <path>   file containing comment text
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

Rules:
  Provide exactly one message source: --message or --message-file.
  No broader work item writes are exposed.

stdout:
  Prints only the created comment ID, or one JSON object when --json is used.
`

const adoWorkItemNamespaceHelp = `Manage Azure DevOps work item maintenance. Supported operations: comment.

Usage:
  adomi ado work-item comment <work-item-id> (--message <text> | --message-file <path>) [--profile <profile-name>] [--global] [--json]

Arguments:
  <work-item-id>   positive Azure DevOps work item ID

Flags:
  --message <text>        inline comment text
  --message-file <path>   file containing comment text
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

Rules:
  Provide exactly one message source: --message or --message-file.
  No broader work item writes are exposed.

stdout:
  Prints only the created comment ID, or one JSON object when --json is used.
`

const adoPRFetchHelp = `Fetch Azure DevOps pull request context.

Usage:
  adomi ado pr fetch <pull-request-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object with path, threadCount, and commentCount

stdout:
  Prints only the exported pull request context directory path on success, or one JSON object with --json.
  Progress and summary lines are written to stderr.
`

const adoPREnsureHelp = `Create or update the active pull request for the current repository branch.

Usage:
  adomi ado pr ensure [--title <title>] [--description-file <path>] [--source <branch>] [--target <branch>] [--repository <name-or-id>] [--profile <profile-name>] [--global] [--json]

Flags:
  --title <title>            set the pull request title; required when creating a new pull request
  --description-file <path>  read the pull request description from a non-empty file
  --source <branch>          override the inferred source branch
  --target <branch>          override the inferred target branch
  --repository <name-or-id>  override the inferred Azure DevOps repository
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

Rules:
  When no active pull request exists, create a new one; otherwise update only fields you provide.
  No PR governance commands are exposed here.

stdout:
  Prints only the pull request ID, or one JSON object when --json is used.
`

const adoPRLinkHelp = `Link existing Azure DevOps work items to an existing pull request.

Usage:
  adomi ado pr link <pull-request-id> --work-item <work-item-id> [--work-item <work-item-id>...] [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --work-item <work-item-id>   positive work item ID; repeat to link multiple work items
  --profile <profile-name>     select a configured Azure DevOps profile
  --global                     use user-level configuration
  --json                       print one compact JSON object

Rules:
  Every work item is preflighted before the first write. Missing links are then added sequentially in input order.
  Already linked work items are successful no-ops, so re-running the command is idempotent.
  Multi-item linking is not atomic: if a later update fails, earlier links remain and the error identifies them.
  The PAT needs code read and work item write permissions.
  This command adds only explicit pull request links; generic relation editing, unlinking, and work item field or state changes are not supported.

stdout:
  Prints every requested work item ID in input order, one per line, or one compact result object with --json.
  Output is buffered until all requested items succeed; stdout stays empty on failure.
`

const adoPRCommentHelp = `Create a new Azure DevOps pull request comment thread.

Usage:
  adomi ado pr comment <pull-request-id> (--message <text> | --message-file <path>) [--file <path> --line <line>] [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --message <text>        inline comment text
  --message-file <path>   file containing comment text
  --file <path>           changed file path for an inline thread
  --line <line>           positive right-side line number for an inline thread
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

Rules:
  Provide exactly one message source: --message or --message-file.
  Omit --file and --line for a PR-level thread; provide both for an inline thread.

stdout:
  Prints only the created thread ID, or one JSON object when --json is used.
`

const adoPRReplyHelp = `Reply to an existing Azure DevOps pull request thread.

Usage:
  adomi ado pr reply <pull-request-id> --thread <thread-id> (--message <text> | --message-file <path>) [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --thread <thread-id>    positive thread ID to reply to
  --message <text>        inline reply text
  --message-file <path>   file containing reply text
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

Rules:
  Provide exactly one message source: --message or --message-file.

stdout:
  Prints only the created comment ID, or one JSON object when --json is used.
`

const adoPRResolveHelp = `Resolve an Azure DevOps pull request thread as fixed.

Usage:
  adomi ado pr resolve <pull-request-id> --thread <thread-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --thread <thread-id>    positive thread ID to mark fixed
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

stdout:
  Prints only the thread ID, or one JSON object when --json is used.
`

const adoPRReopenHelp = `Reopen an Azure DevOps pull request thread as active.

Usage:
  adomi ado pr reopen <pull-request-id> --thread <thread-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --thread <thread-id>    positive thread ID to mark active
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

stdout:
  Prints only the thread ID, or one JSON object when --json is used.
`

const adoPRCompleteHelp = `Request immediate completion of an Azure DevOps pull request.

Usage:
  adomi ado pr complete <pull-request-id> [--merge-strategy <no-fast-forward|squash|rebase|rebase-merge>] [--delete-source-branch <true|false>] [--transition-work-items <true|false>] [--merge-commit-message <text>] [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --merge-strategy <no-fast-forward|squash|rebase|rebase-merge>   select an Azure DevOps merge strategy
  --delete-source-branch <true|false>                              explicitly enable or disable source branch deletion
  --transition-work-items <true|false>                             explicitly enable or disable linked work item transition
  --merge-commit-message <text>                                    set a non-empty merge commit message
  --profile <profile-name>                                         select a configured Azure DevOps profile
  --global                                                         use user-level configuration
  --json                                                           print one compact JSON result

Rules:
  This explicit Azure DevOps write accepts only an active, non-draft pull request and pins completion to its fetched source commit.
  An already completed pull request is an unchanged success; an abandoned or draft pull request is rejected.
  Supported fetched completion preferences are preserved unless overridden, and required branch policies remain enforced without bypass.
  Azure DevOps may report completed while merge processing is still queued; this command returns without polling for final merge success.

stdout:
  Prints only the pull request ID, or one JSON object with the returned lifecycle and merge state when --json is used.
`

const adoPRAutoCompleteHelp = `Schedule policy-gated completion of an Azure DevOps pull request.

Usage:
  adomi ado pr auto-complete <pull-request-id> [--merge-strategy <no-fast-forward|squash|rebase|rebase-merge>] [--delete-source-branch <true|false>] [--transition-work-items <true|false>] [--merge-commit-message <text>] [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --merge-strategy <no-fast-forward|squash|rebase|rebase-merge>   select an Azure DevOps merge strategy
  --delete-source-branch <true|false>                              explicitly enable or disable source branch deletion
  --transition-work-items <true|false>                             explicitly enable or disable linked work item transition
  --merge-commit-message <text>                                    set a non-empty merge commit message
  --profile <profile-name>                                         select a configured Azure DevOps profile
  --global                                                         use user-level configuration
  --json                                                           print one compact JSON result

Rules:
  This explicit Azure DevOps write accepts only an active, non-draft pull request and sets auto-complete as the authenticated user.
  A completed, abandoned, or draft pull request is rejected. An already scheduled request with no preference changes is an unchanged success unless stored policy overrides must be cleared.
  Supported completion preferences remain subject to required branch policies; policy bypass is unavailable.
  Azure DevOps may complete the pull request immediately or leave it active and scheduled; this command returns without polling.

stdout:
  Prints only the pull request ID, or one JSON object with the returned lifecycle, merge, and auto-complete state when --json is used.
`

const adoPRCancelAutoCompleteHelp = `Cancel scheduled completion of an Azure DevOps pull request.

Usage:
  adomi ado pr cancel-auto-complete <pull-request-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON result

Rules:
  This explicit Azure DevOps write clears auto-complete only for an active pull request.
  An active pull request without auto-complete is an unchanged success; completed and abandoned pull requests are rejected.

stdout:
  Prints only the pull request ID, or one JSON object proving auto-complete is disabled when --json is used.
`

const adoPRAbandonHelp = `Abandon an Azure DevOps pull request without merging it.

Usage:
  adomi ado pr abandon <pull-request-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON result

Rules:
  This explicit Azure DevOps write changes an active pull request to abandoned without merging it.
  An already abandoned pull request is an unchanged success; a completed pull request is rejected.
  Abandoning a scheduled pull request does not issue a separate auto-complete cancellation write.

stdout:
  Prints only the pull request ID, or one JSON object proving the abandoned state when --json is used.
`

const adoPRApproveHelp = `Approve an Azure DevOps pull request as the authenticated user.

Usage:
  adomi ado pr approve <pull-request-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON result

Rules:
  This explicit Azure DevOps write casts vote 10 only as the authenticated user on an active pull request.
  It accepts no reviewer selector or raw vote and preserves an existing required-reviewer designation.
  Completed and abandoned pull requests are rejected; an existing vote 10 is an unchanged success.

stdout:
  Prints only the pull request ID, or one JSON object with the authenticated reviewer ID and vote when --json is used.
`

const adoPRApproveWithSuggestionsHelp = `Approve an Azure DevOps pull request with suggestions as the authenticated user.

Usage:
  adomi ado pr approve-with-suggestions <pull-request-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON result

Rules:
  This explicit Azure DevOps write casts vote 5 only as the authenticated user on an active pull request.
  It accepts no reviewer selector or raw vote and preserves an existing required-reviewer designation.
  Completed and abandoned pull requests are rejected; an existing vote 5 is an unchanged success.

stdout:
  Prints only the pull request ID, or one JSON object with the authenticated reviewer ID and vote when --json is used.
`

const adoPRRejectHelp = `Reject an Azure DevOps pull request as the authenticated user.

Usage:
  adomi ado pr reject <pull-request-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON result

Rules:
  This explicit Azure DevOps write casts vote -10 only as the authenticated user on an active pull request.
  It accepts no reviewer selector or raw vote and preserves an existing required-reviewer designation.
  Completed and abandoned pull requests are rejected; an existing vote -10 is an unchanged success.

stdout:
  Prints only the pull request ID, or one JSON object with the authenticated reviewer ID and vote when --json is used.
`

const adoLoginHelp = `Store an Azure DevOps PAT.

Usage:
  adomi ado login (--profile <profile-name> | --pat-ref <ref>) [--global] [--json]

Flags:
  --profile <profile-name>   load the credential reference from a configured profile
  --pat-ref <ref>            store the PAT under an explicit credential reference
  --global                   use user-level configuration when resolving --profile
  --json                     print one compact JSON object with action and credentialRef

Streams:
  The secret prompt is written to stderr. Confirmation lines appear on stderr.
  With --json, the JSON result is written to stdout.
`

const adoLogoutHelp = `Delete an Azure DevOps PAT credential.

Usage:
  adomi ado logout (--profile <profile-name> | --pat-ref <ref>) [--global] [--json]

Flags:
  --profile <profile-name>   load the credential reference from a configured profile
  --pat-ref <ref>            delete an explicit credential reference
  --global                   use user-level configuration when resolving --profile
  --json                     print one compact JSON object with action and credentialRef

Streams:
  Confirmation lines appear on stderr and default stdout is empty.
  With --json, the JSON result is written to stdout.
`

const adoProfilesListHelp = `List configured Azure DevOps profiles.

Usage:
  adomi ado profiles list [--global]

Flags:
  --global   list user-level profiles instead of repository profiles

stdout:
  Prints one profile name per line in sorted order.
`
