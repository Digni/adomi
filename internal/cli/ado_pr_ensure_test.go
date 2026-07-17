package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestSelectPRRepositoryFromAzureDevOpsRemotes(t *testing.T) {
	remotes := []gitRemote{
		{Name: "origin", URL: "https://dev.azure.com/my-org/MyProject/_git/adomi"},
	}
	selected, err := selectPRRepository(remotes, config.Profile{BaseURL: "https://dev.azure.com/my-org", Project: "myproject"}, "")
	if err != nil {
		t.Fatalf("selectPRRepository returned error: %v", err)
	}
	if selected.Repository != "adomi" || selected.RemoteName != "origin" {
		t.Fatalf("selected = %+v, want adomi origin", selected)
	}
}

func TestSelectPRRepositoryUsesOriginTieBreak(t *testing.T) {
	remotes := []gitRemote{
		{Name: "upstream", URL: "https://dev.azure.com/my-org/MyProject/_git/adomi"},
		{Name: "origin", URL: "git@ssh.dev.azure.com:v3/my-org/MyProject/adomi"},
	}
	selected, err := selectPRRepository(remotes, config.Profile{BaseURL: "https://dev.azure.com/my-org", Project: "MyProject"}, "")
	if err != nil {
		t.Fatalf("selectPRRepository returned error: %v", err)
	}
	if selected.RemoteName != "origin" || selected.Repository != "adomi" {
		t.Fatalf("selected = %+v, want origin adomi", selected)
	}
}

func TestSelectPRRepositoryRejectsAmbiguousNonOriginRemotes(t *testing.T) {
	remotes := []gitRemote{
		{Name: "one", URL: "https://dev.azure.com/my-org/MyProject/_git/adomi"},
		{Name: "two", URL: "https://dev.azure.com/my-org/MyProject/_git/other"},
	}
	_, err := selectPRRepository(remotes, config.Profile{BaseURL: "https://dev.azure.com/my-org", Project: "MyProject"}, "")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error = %v, want ambiguous", err)
	}
}

func TestSelectPRRepositoryAllowsExplicitRepositoryOverride(t *testing.T) {
	selected, err := selectPRRepository(nil, config.Profile{BaseURL: "https://dev.azure.com/my-org", Project: "MyProject"}, "manual-repo")
	if err != nil {
		t.Fatalf("selectPRRepository returned error: %v", err)
	}
	if selected.Repository != "manual-repo" {
		t.Fatalf("repository = %q, want manual-repo", selected.Repository)
	}
}

func TestSelectPRRepositoryRejectsUnsupportedRemotes(t *testing.T) {
	_, err := selectPRRepository([]gitRemote{{Name: "origin", URL: "git@github.com:Digni/adomi.git"}}, config.Profile{BaseURL: "https://dev.azure.com/my-org", Project: "MyProject"}, "")
	if err == nil || !strings.Contains(err.Error(), "repository") {
		t.Fatalf("error = %v, want repository inference error", err)
	}
}

func TestResolvePREnsureRefsInfersBranches(t *testing.T) {
	refs, err := resolvePREnsureRefs(Dependencies{
		CurrentBranch: func(repoRoot string) (string, error) {
			if repoRoot != "/repo" {
				t.Fatalf("repoRoot = %q, want /repo", repoRoot)
			}
			return "feature/x", nil
		},
		RemoteDefaultBranch: func(repoRoot, remoteName string) (string, error) {
			if remoteName != "origin" {
				t.Fatalf("remoteName = %q, want origin", remoteName)
			}
			return "main", nil
		},
	}, "/repo", "origin", prEnsureArgs{})
	if err != nil {
		t.Fatalf("resolvePREnsureRefs returned error: %v", err)
	}
	if refs.SourceRef != "refs/heads/feature/x" || refs.TargetRef != "refs/heads/main" {
		t.Fatalf("refs = %+v, want normalized source/target", refs)
	}
}

func TestResolvePREnsureRefsUsesExplicitOverrides(t *testing.T) {
	refs, err := resolvePREnsureRefs(Dependencies{}, "/repo", "origin", prEnsureArgs{source: "refs/heads/custom", target: "release"})
	if err != nil {
		t.Fatalf("resolvePREnsureRefs returned error: %v", err)
	}
	if refs.SourceRef != "refs/heads/custom" || refs.TargetRef != "refs/heads/release" {
		t.Fatalf("refs = %+v, want explicit normalized refs", refs)
	}
}

func TestResolvePREnsureRefsRejectsDetachedHead(t *testing.T) {
	_, err := resolvePREnsureRefs(Dependencies{CurrentBranch: func(string) (string, error) { return "", nil }}, "/repo", "origin", prEnsureArgs{target: "main"})
	if err == nil || !strings.Contains(err.Error(), "current branch") {
		t.Fatalf("error = %v, want current branch", err)
	}
}

func TestResolvePREnsureRefsRequiresTargetWhenDefaultUnavailable(t *testing.T) {
	_, err := resolvePREnsureRefs(Dependencies{
		CurrentBranch:       func(string) (string, error) { return "feature/x", nil },
		RemoteDefaultBranch: func(string, string) (string, error) { return "", errors.New("no remote head") },
	}, "/repo", "origin", prEnsureArgs{})
	if err == nil || !strings.Contains(err.Error(), "--target") {
		t.Fatalf("error = %v, want --target", err)
	}
}

func TestResolvePREnsureRefsRejectsSameSourceAndTarget(t *testing.T) {
	_, err := resolvePREnsureRefs(Dependencies{}, "/repo", "origin", prEnsureArgs{source: "main", target: "refs/heads/main"})
	if err == nil || !strings.Contains(err.Error(), "source") || !strings.Contains(err.Error(), "target") {
		t.Fatalf("error = %v, want source target mismatch", err)
	}
}

func TestADOPullRequestEnsureCreatesMissingPR(t *testing.T) {
	client := &fakePRMaintenanceClient{created: &ado.PullRequest{ID: 101, URL: "https://dev.azure.com/my-org/MyProject/_git/adomi/pullrequest/101"}}
	runner := prEnsureTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--title", "Add feature", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "101\n" {
		t.Fatalf("stdout = %q, want PR ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if client.listOpts.RepositoryID != "adomi" || client.listOpts.SourceRefName != "refs/heads/feature/x" || client.listOpts.TargetRefName != "refs/heads/main" || client.listOpts.Status != "active" {
		t.Fatalf("list opts = %+v, want active feature/main adomi", client.listOpts)
	}
	if client.createCalled != 1 || client.createOpts.RepositoryID != "adomi" || client.createOpts.SourceRefName != "refs/heads/feature/x" || client.createOpts.TargetRefName != "refs/heads/main" || client.createOpts.Title != "Add feature" {
		t.Fatalf("create called/opts = %d/%+v", client.createCalled, client.createOpts)
	}
	if client.updateCalled != 0 {
		t.Fatalf("updateCalled = %d, want 0", client.updateCalled)
	}
}

func TestADOPullRequestEnsureRejectsMissingTitleOnCreate(t *testing.T) {
	client := &fakePRMaintenanceClient{}
	runner := prEnsureTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--source", "feature/x", "--target", "main", "--repository", "adomi"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--title") {
		t.Fatalf("error = %v, want --title", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.createCalled != 0 || client.updateCalled != 0 {
		t.Fatalf("mutations create/update = %d/%d, want none", client.createCalled, client.updateCalled)
	}
}

func TestADOPullRequestEnsureUpdatesExistingTitle(t *testing.T) {
	client := &fakePRMaintenanceClient{listed: []ado.PullRequest{{ID: 42, Repository: ado.PullRequestRepo{ID: "adomi"}, SourceRefName: "refs/heads/feature/x", TargetRefName: "refs/heads/main"}}}
	runner := prEnsureTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--source", "feature/x", "--target", "main", "--repository", "adomi", "--title", "New title"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "42\n" {
		t.Fatalf("stdout = %q, want PR ID", stdout.String())
	}
	if client.updateCalled != 1 || client.updateOpts.PullRequestID != 42 || client.updateOpts.Title == nil || *client.updateOpts.Title != "New title" {
		t.Fatalf("update called/opts = %d/%+v", client.updateCalled, client.updateOpts)
	}
	if client.createCalled != 0 {
		t.Fatalf("createCalled = %d, want 0", client.createCalled)
	}
}

func TestADOPullRequestEnsureUpdatesExistingDescriptionFromFile(t *testing.T) {
	descriptionPath := filepath.Join(t.TempDir(), "description.md")
	if err := os.WriteFile(descriptionPath, []byte("Updated body\n"), 0o644); err != nil {
		t.Fatalf("writing description: %v", err)
	}
	client := &fakePRMaintenanceClient{listed: []ado.PullRequest{{ID: 42, Repository: ado.PullRequestRepo{ID: "adomi"}}}}
	runner := prEnsureTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--source", "feature/x", "--target", "main", "--repository", "adomi", "--description-file", descriptionPath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "42\n" {
		t.Fatalf("stdout = %q, want PR ID", stdout.String())
	}
	if client.updateCalled != 1 || client.updateOpts.Description == nil || *client.updateOpts.Description != "Updated body\n" {
		t.Fatalf("update opts = %+v, want description", client.updateOpts)
	}
}

func TestADOPullRequestEnsureLeavesExistingPRUnchangedWithoutUpdateFields(t *testing.T) {
	client := &fakePRMaintenanceClient{listed: []ado.PullRequest{{ID: 42, Repository: ado.PullRequestRepo{ID: "adomi"}, SourceRefName: "refs/heads/feature/x", TargetRefName: "refs/heads/main"}}}
	runner := prEnsureTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--source", "feature/x", "--target", "main", "--repository", "adomi"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "42\n" {
		t.Fatalf("stdout = %q, want PR ID", stdout.String())
	}
	if client.createCalled != 0 || client.updateCalled != 0 {
		t.Fatalf("mutations create/update = %d/%d, want none", client.createCalled, client.updateCalled)
	}
}

func TestADOPullRequestEnsureRejectsMultipleMatchingPRs(t *testing.T) {
	client := &fakePRMaintenanceClient{listed: []ado.PullRequest{{ID: 42}, {ID: 43}}}
	runner := prEnsureTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--source", "feature/x", "--target", "main", "--repository", "adomi", "--title", "New title"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("error = %v, want multiple", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.createCalled != 0 || client.updateCalled != 0 {
		t.Fatalf("mutations create/update = %d/%d, want none", client.createCalled, client.updateCalled)
	}
}

func TestADOPullRequestEnsureRejectsSameSourceAndTargetBeforeNetwork(t *testing.T) {
	client := &fakePRMaintenanceClient{}
	runner := prEnsureTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--source", "main", "--target", "refs/heads/main", "--repository", "adomi", "--title", "Title"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "source") || !strings.Contains(err.Error(), "target") {
		t.Fatalf("error = %v, want source/target", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.listCalled != 0 || client.createCalled != 0 || client.updateCalled != 0 {
		t.Fatalf("calls list/create/update = %d/%d/%d, want none", client.listCalled, client.createCalled, client.updateCalled)
	}
}

func TestADOPullRequestEnsureRepositoryInferenceFailuresLeaveStdoutEmpty(t *testing.T) {
	tests := []struct {
		name    string
		remotes []gitRemote
		want    string
	}{
		{name: "unsupported remote", remotes: []gitRemote{{Name: "origin", URL: "git@github.com:Digni/adomi.git"}}, want: "could not infer"},
		{name: "ambiguous remotes", remotes: []gitRemote{
			{Name: "upstream", URL: "https://dev.azure.com/my-org/MyProject/_git/adomi"},
			{Name: "fork", URL: "https://dev.azure.com/my-org/MyProject/_git/adomi-fork"},
		}, want: "ambiguous"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakePRMaintenanceClient{}
			runner := prEnsureInferenceFailureTestRunner(t, client)
			runner.deps.GitRemotes = func(repoRoot string) ([]gitRemote, error) { return tt.remotes, nil }
			var stdout bytes.Buffer

			err := runner.Run([]string{"ado", "pr", "ensure", "--title", "Title"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if client.listCalled != 0 || client.createCalled != 0 || client.updateCalled != 0 {
				t.Fatalf("calls list/create/update = %d/%d/%d, want none", client.listCalled, client.createCalled, client.updateCalled)
			}
		})
	}
}

func TestADOPullRequestEnsureBranchInferenceFailuresLeaveStdoutEmpty(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Runner)
		want   string
	}{
		{name: "current branch unavailable", want: "current branch", modify: func(r *Runner) {
			r.deps.CurrentBranch = func(repoRoot string) (string, error) { return "", nil }
		}},
		{name: "target fallback unavailable", want: "--target", modify: func(r *Runner) {
			r.deps.CurrentBranch = func(repoRoot string) (string, error) { return "feature/x", nil }
			r.deps.RemoteDefaultBranch = func(repoRoot, remoteName string) (string, error) { return "", errors.New("no remote head") }
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakePRMaintenanceClient{}
			runner := prEnsureInferenceFailureTestRunner(t, client)
			tt.modify(&runner)
			var stdout bytes.Buffer

			err := runner.Run([]string{"ado", "pr", "ensure", "--title", "Title"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if client.listCalled != 0 || client.createCalled != 0 || client.updateCalled != 0 {
				t.Fatalf("calls list/create/update = %d/%d/%d, want none", client.listCalled, client.createCalled, client.updateCalled)
			}
		})
	}
}

func TestADOPullRequestEnsureListErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{listErr: errors.New("list failed")}
	runner := prEnsureTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--source", "feature/x", "--target", "main", "--repository", "adomi", "--title", "Title"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "list failed") {
		t.Fatalf("error = %v, want list failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.listCalled != 1 || client.createCalled != 0 || client.updateCalled != 0 {
		t.Fatalf("calls list/create/update = %d/%d/%d, want list only", client.listCalled, client.createCalled, client.updateCalled)
	}
}

func TestADOPullRequestEnsureCreateErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{createErr: errors.New("create failed")}
	runner := prEnsureTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--source", "feature/x", "--target", "main", "--repository", "adomi", "--title", "Title"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "create failed") {
		t.Fatalf("error = %v, want create failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.listCalled != 1 || client.createCalled != 1 || client.updateCalled != 0 {
		t.Fatalf("calls list/create/update = %d/%d/%d, want list and create only", client.listCalled, client.createCalled, client.updateCalled)
	}
}

func TestADOPullRequestEnsureUpdateErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{
		listed:    []ado.PullRequest{{ID: 42, Repository: ado.PullRequestRepo{ID: "adomi"}}},
		updateErr: errors.New("update failed"),
	}
	runner := prEnsureTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--source", "feature/x", "--target", "main", "--repository", "adomi", "--title", "New title"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("error = %v, want update failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.listCalled != 1 || client.createCalled != 0 || client.updateCalled != 1 {
		t.Fatalf("calls list/create/update = %d/%d/%d, want list and update only", client.listCalled, client.createCalled, client.updateCalled)
	}
}

func TestADOPullRequestEnsureJSONOutput(t *testing.T) {
	client := &fakePRMaintenanceClient{listed: []ado.PullRequest{{ID: 42, URL: "https://dev.azure.com/my-org/MyProject/_git/adomi/pullrequest/42", Repository: ado.PullRequestRepo{ID: "adomi"}, SourceRefName: "refs/heads/feature/x", TargetRefName: "refs/heads/main"}}}
	runner := prEnsureTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "ensure", "--source", "feature/x", "--target", "main", "--repository", "adomi", "--json"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.HasSuffix(stdout.String(), "\n") || strings.HasSuffix(strings.TrimSuffix(stdout.String(), "\n"), "\n") {
		t.Fatalf("stdout = %q, want exactly one trailing newline", stdout.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if payload["pullRequestId"] != float64(42) || payload["action"] != "unchanged" || payload["repository"] != "adomi" || payload["sourceRefName"] != "refs/heads/feature/x" || payload["targetRefName"] != "refs/heads/main" {
		t.Fatalf("payload = %+v, want unchanged PR summary", payload)
	}
}
