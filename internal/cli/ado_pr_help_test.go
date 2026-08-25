package cli

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestADOPullRequestNamespaceHelpListsSupportedOperations(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer
	err := runner.Run([]string{"ado", "pr", "--help"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{"fetch", "ensure", "link", "comment", "reply", "resolve", "reopen", "complete", "auto-complete", "cancel-auto-complete", "abandon", "approve", "approve-with-suggestions", "reject", "explicit governance writes", "--file <path> --line <line>", "latest-version right-side inline threads"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func TestRunADOPullRequestGovernanceHelp(t *testing.T) {
	tests := []struct {
		verb string
		want []string
	}{
		{
			verb: "complete",
			want: []string{"Usage:", "adomi ado pr complete <pull-request-id>", "--merge-strategy", "no-fast-forward", "squash", "rebase", "rebase-merge", "--delete-source-branch <true|false>", "--transition-work-items <true|false>", "--merge-commit-message <text>", "--profile", "--global", "--json", "active", "non-draft", "source commit", "branch policies", "without polling", "completed", "abandoned", "explicit Azure DevOps write", "stdout", "pull request ID"},
		},
		{
			verb: "auto-complete",
			want: []string{"Usage:", "adomi ado pr auto-complete <pull-request-id>", "--merge-strategy", "--delete-source-branch <true|false>", "--transition-work-items <true|false>", "--merge-commit-message <text>", "--profile", "--global", "--json", "active", "non-draft", "branch policies", "authenticated user", "immediately", "without polling", "completed", "abandoned", "unless stored policy overrides must be cleared", "explicit Azure DevOps write", "stdout", "pull request ID"},
		},
		{
			verb: "cancel-auto-complete",
			want: []string{"Usage:", "adomi ado pr cancel-auto-complete <pull-request-id>", "--profile", "--global", "--json", "active", "clears", "completed", "abandoned", "explicit Azure DevOps write", "stdout", "pull request ID"},
		},
		{
			verb: "abandon",
			want: []string{"Usage:", "adomi ado pr abandon <pull-request-id>", "--profile", "--global", "--json", "active", "without merging", "completed", "explicit Azure DevOps write", "stdout", "pull request ID"},
		},
		{
			verb: "approve",
			want: []string{"Usage:", "adomi ado pr approve <pull-request-id>", "--profile", "--global", "--json", "active", "authenticated user", "vote 10", "no reviewer", "explicit Azure DevOps write", "stdout", "pull request ID"},
		},
		{
			verb: "approve-with-suggestions",
			want: []string{"Usage:", "adomi ado pr approve-with-suggestions <pull-request-id>", "--profile", "--global", "--json", "active", "authenticated user", "vote 5", "no reviewer", "explicit Azure DevOps write", "stdout", "pull request ID"},
		},
		{
			verb: "reject",
			want: []string{"Usage:", "adomi ado pr reject <pull-request-id>", "--profile", "--global", "--json", "active", "authenticated user", "vote -10", "no reviewer", "explicit Azure DevOps write", "stdout", "pull request ID"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.verb, func(t *testing.T) {
			stderr := runHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "pr", tt.verb, "not-an-id", "--help"})
			for _, want := range tt.want {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr = %q, want %q", stderr, want)
				}
			}
		})
	}
}

func TestADOPullRequestEnsureCommandHelp(t *testing.T) {
	stderr := runHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "pr", "ensure", "--help"})
	for _, want := range []string{
		"Usage:",
		"adomi ado pr ensure",
		"create",
		"update",
		"--title",
		"required when creating",
		"--description-file",
		"--source",
		"--target",
		"--repository",
		"--profile",
		"--global",
		"--json",
		"stdout",
		"pull request ID",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	for _, notWant := range []string{"approve", "approval", "merge", "reviewer"} {
		if strings.Contains(stderr, notWant) {
			t.Fatalf("stderr = %q, want no governance term %q", stderr, notWant)
		}
	}
}

func TestADOPullRequestShortHelpAlias(t *testing.T) {
	stderr := runHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "pr", "ensure", "-h"})
	for _, want := range []string{"Usage:", "adomi ado pr ensure", "--title", "pull request ID"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
}

func TestADOPullRequestOperationHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "link",
			args: []string{"ado", "pr", "link", "--help"},
			want: []string{"Usage:", "adomi ado pr link", "--work-item", "repeat", "--profile", "--global", "--json", "input order", "Already linked", "idempotent", "code read", "work item write", "not atomic", "stdout", "empty"},
		},
		{
			name: "comment",
			args: []string{"ado", "pr", "comment", "--help"},
			want: []string{"Usage:", "adomi ado pr comment", "PR-level", "inline", "--message", "--message-file", "exactly one", "--file", "--line", "--profile", "--global", "--json", "stdout", "thread ID"},
		},
		{
			name: "reply",
			args: []string{"ado", "pr", "reply", "--help"},
			want: []string{"Usage:", "adomi ado pr reply", "--thread", "thread ID", "--message", "--message-file", "exactly one", "--profile", "--global", "--json", "stdout", "comment ID"},
		},
		{
			name: "resolve",
			args: []string{"ado", "pr", "resolve", "--help"},
			want: []string{"Usage:", "adomi ado pr resolve", "--thread", "thread ID", "fixed", "--profile", "--global", "--json", "stdout", "thread ID"},
		},
		{
			name: "reopen",
			args: []string{"ado", "pr", "reopen", "--help"},
			want: []string{"Usage:", "adomi ado pr reopen", "--thread", "thread ID", "active", "--profile", "--global", "--json", "stdout", "thread ID"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := runHelp(t, Runner{deps: Dependencies{}}, tt.args)
			for _, want := range tt.want {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr = %q, want %q", stderr, want)
				}
			}
		})
	}
}

func TestADOPullRequestHelpIsSideEffectFree(t *testing.T) {
	runner := Runner{deps: Dependencies{
		PATStore: failOnGetPATStore{t: t},
		ReadSecret: func(prompt string, stdin io.Reader, stderr io.Writer) (string, error) {
			t.Fatalf("ReadSecret called for help request with prompt %q", prompt)
			return "", nil
		},
		Getwd: func() (string, error) {
			t.Fatal("Getwd called for help request")
			return "", nil
		},
		UserHomeDir: func() (string, error) {
			t.Fatal("UserHomeDir called for help request")
			return "", nil
		},
		FindRepoRoot: func(start string) (string, error) {
			t.Fatalf("FindRepoRoot called for help request with start %q", start)
			return "", nil
		},
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			t.Fatal("LoadConfig called for help request")
			return nil, nil
		},
		LoadAllConfig: func(repoRoot, homeDir string, scope config.Scope) (*config.Loaded, error) {
			t.Fatal("LoadAllConfig called for help request")
			return nil, nil
		},
		RemoteURLs: func(repoRoot string) ([]string, error) {
			t.Fatal("RemoteURLs called for help request")
			return nil, nil
		},
		GitRemotes: func(repoRoot string) ([]gitRemote, error) {
			t.Fatal("GitRemotes called for help request")
			return nil, nil
		},
		CurrentBranch: func(repoRoot string) (string, error) {
			t.Fatal("CurrentBranch called for help request")
			return "", nil
		},
		RemoteDefaultBranch: func(repoRoot, remoteName string) (string, error) {
			t.Fatal("RemoteDefaultBranch called for help request")
			return "", nil
		},
		NewHTTPClient: func(proxyURL string) (*http.Client, error) {
			t.Fatal("NewHTTPClient called for help request")
			return nil, nil
		},
		NewADOClient: func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			t.Fatal("NewADOClient called for help request")
			return nil, nil
		},
	}}

	stderr := runHelp(t, runner, []string{"ado", "pr", "ensure", "--description-file", "/does/not/exist", "--help"})
	if !strings.Contains(stderr, "adomi ado pr ensure") {
		t.Fatalf("stderr = %q, want ensure help", stderr)
	}

	stderr = runHelp(t, runner, []string{"ado", "pr", "link", "42", "--work-item", "101", "--help"})
	if !strings.Contains(stderr, "adomi ado pr link") {
		t.Fatalf("stderr = %q, want link help", stderr)
	}

	for _, verb := range []string{"complete", "auto-complete", "cancel-auto-complete", "abandon", "approve", "approve-with-suggestions", "reject"} {
		stderr = runHelp(t, runner, []string{"ado", "pr", verb, "not-an-id", "--unknown", "--help"})
		if !strings.Contains(stderr, "adomi ado pr "+verb) {
			t.Fatalf("stderr for %s = %q, want governance help", verb, stderr)
		}
	}
}
