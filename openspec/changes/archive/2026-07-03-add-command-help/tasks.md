## 1. Test Coverage

- [x] 1.1 Add focused failing tests for `adomi ado pr ensure --help`, asserting success, empty stdout, stderr usage content, create/update guidance, required `--title` create behavior, supported override flags, stdout contract, and no approval/merge/reviewer-governance wording.
- [x] 1.2 Add table-driven tests for `adomi ado pr comment|reply|resolve|reopen --help`, asserting success, empty stdout, operation-specific required arguments/flags, message-source rules where applicable, and plain stdout result descriptions.
- [x] 1.3 Add tests for non-PR action help: `adomi ado fetch --help`, `adomi ado pr fetch --help`, `adomi ado comment --help`, `adomi ado work-item comment --help`, `adomi ado login --help`, `adomi ado logout --help`, `adomi ado profiles list --help`, `adomi config init --help`, and hidden compatibility alias `adomi ado config init --help`.
- [x] 1.4 Add a side-effect guard test proving a representative routed PR help request does not call repo/config/credential/client dependencies, does not read message/description files, and writes help to stderr with stdout empty.
- [x] 1.5 Verification checkpoint: run `go test ./internal/cli` and confirm the new tests fail only because command-specific help is not implemented yet.

## 2. Help Implementation

- [x] 2.1 Add small shared helpers for detecting help tokens and writing hand-authored command-specific help to the configured stderr/help writer while keeping stdout untouched; do not rely on Cobra auto-rendering flags for commands with `DisableFlagParsing: true` and no registered flags.
- [x] 2.2 Wire help handling into custom-parsed leaf commands (`config init`, `ado fetch`, `ado comment`, `ado work-item comment`, `ado login`, `ado logout`, and `ado profiles list`) before validation or dependency access.
- [x] 2.3 Wire routed PR operation help into `newADOPullRequestCommand`'s `RunE` before it calls `r.runADOPullRequest(args, stdout)`, or explicitly thread a stderr/help writer through PR handlers; in either approach, `adomi ado pr fetch|ensure|comment|reply|resolve|reopen --help` must run before argument parsing, repository inference, config loading, credential access, file reads, HTTP client creation, or Azure DevOps calls.
- [x] 2.4 Include concise usage, required arguments, relevant flags, stdout behavior, and conservative write boundaries in hand-authored help text; avoid adding unsupported operations to namespace or operation help.
- [x] 2.5 Verification checkpoint: run `gofmt -w internal cmd` and `go test ./internal/cli`; expect all CLI tests to pass.

## 3. Full Verification

- [x] 3.1 Run `go test ./...`; expect the full Go test suite to pass.
- [x] 3.2 Run `go build -o bin/adomi ./cmd/adomi`; expect the CLI binary to build successfully.
- [x] 3.3 Run `openspec validate add-command-help --strict`; expect the change proposal/spec/tasks to validate.
- [x] 3.4 Run representative runtime checks such as `go run ./cmd/adomi ado pr ensure --help` and `go run ./cmd/adomi ado comment --help`; expect exit 0, empty stdout, and command-specific help on stderr.
- [x] 3.5 Run implementation review (`/agent-review` or equivalent) and address any correctness, safety, or plan-adherence findings before marking the change complete.
