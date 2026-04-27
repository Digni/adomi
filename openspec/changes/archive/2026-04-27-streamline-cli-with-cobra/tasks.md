## 1. Test Coverage First

- [x] 1.1 Add failing CLI tests for `adomi config init` and `adomi config init --global` using existing config-init fixtures. Verification: `go test -count=1 ./internal/cli -run 'TestConfigInit|TestADOConfigInit'` fails because the new top-level command is not implemented yet.
- [x] 1.2 Add failing CLI tests that `adomi ado config init` still works but is hidden from `adomi ado --help`. Verification: `go test -count=1 ./internal/cli -run 'TestADOConfigInit|TestADOHelp'` fails on help/hidden alias expectations until Cobra is wired.
- [x] 1.3 Add/update root and main tests for no-args usage, unknown command errors, and stderr/stdout separation. Verification: `go test -count=1 ./cmd/adomi ./internal/cli -run 'TestMain|TestRoot|TestUnknown'` fails only on the new Cobra command-surface expectations.

## 2. Cobra Setup

- [x] 2.1 Add Cobra to the Go module with `go get github.com/spf13/cobra`. Verification: `go list -m github.com/spf13/cobra` prints the selected module version and `go.mod` / `go.sum` contain the dependency.
- [x] 2.2 Replace manual root dispatch in `internal/cli/root.go` with a Cobra root command built per `Runner.Run` invocation, setting args, stdin, stdout, and stderr on the command. Verification: `go test -count=1 ./internal/cli -run 'TestRoot|TestUnknown|TestMain'` reports root behavior matching the updated tests.
- [x] 2.3 Configure Cobra error/usage behavior so errors are returned to `cmd/adomi/main.go` without duplicate stderr output. Verification: `go test -count=1 ./cmd/adomi ./internal/cli -run 'TestMain|TestUnknown|TestRoot'` passes and stdout remains empty on error cases that expect it.

## 3. Command Migration

- [x] 3.1 Extract config initialization into a top-level `config init` Cobra command that delegates to the existing config-init action. Verification: `go test -count=1 ./internal/cli -run 'TestConfigInit'` passes for repo, global, overwrite, symlink, remote prefill, and unknown-argument coverage.
- [x] 3.2 Convert `ado fetch`, `ado login`, `ado logout`, and `ado profiles list` to Cobra subcommands while preserving the existing `Runner` methods and dependency injection. Verification: `go test -count=1 ./internal/cli -run 'TestADOFetch|TestADOLogin|TestADOLogout|TestADOProfiles'` passes.
- [x] 3.3 Add a hidden `ado config init` compatibility alias that calls the same action as `config init`. Verification: `go test -count=1 ./internal/cli -run 'TestADOConfigInit|TestADOHelp'` passes and help output omits the alias.
- [x] 3.4 Remove or simplify obsolete manual argument parsing helpers that Cobra now owns, while keeping domain-specific validation such as positive work item IDs and `patRef` normalization. Verification: `go test -count=1 ./internal/cli ./internal/config` passes.

## 4. Documentation And Final Verification

- [x] 4.1 Update command examples and dependency notes in `docs/adomi_azure_devops_handover.md` or a dedicated follow-up CLI doc to show `adomi config init` and clarify that Cobra is now used after the MVP. Verification: `rg -n 'ado config|Avoid Cobra|adomi config init' docs` shows no stale primary guidance for `adomi ado config init`.
- [x] 4.2 Run formatting and whitespace checks. Verification: `gofmt -w cmd internal` makes no unintended changes after review, and `git diff --check` exits 0.
- [x] 4.3 Run the full test suite. Verification: `go test -count=1 ./...` passes.
- [x] 4.4 Build a smoke-test binary and inspect help for the streamlined surface. Verification: `go build -o /tmp/adomi-cobra-smoke ./cmd/adomi`, `/tmp/adomi-cobra-smoke --help`, `/tmp/adomi-cobra-smoke config --help`, and `/tmp/adomi-cobra-smoke ado --help` all exit 0 and show `config` at the top level while omitting the hidden `ado config` alias.
