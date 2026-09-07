package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestParsePRListArgsNormalizesBranchesAndDefaultsStatus(t *testing.T) {
	parsed, err := parsePRListArgs([]string{"--source", " feature/example ", "--target", "refs/heads/main", "--repository", " repo ", "--global"})
	if err != nil {
		t.Fatalf("parsePRListArgs returned error: %v", err)
	}
	if parsed.source != "refs/heads/feature/example" || parsed.target != "refs/heads/main" {
		t.Fatalf("branches = %+v, want normalized refs", parsed)
	}
	if parsed.repository != "repo" || parsed.status != "active" || !parsed.global {
		t.Fatalf("parsed = %+v, want repository/status/global", parsed)
	}
}

func TestParsePRListArgsRejectsInvalidAndRepeatedFlags(t *testing.T) {
	tests := [][]string{
		{"--source", " "},
		{"--status", "unknown"},
		{"--profile", " "},
		{"--source", "one", "--source", "two"},
		{"--global", "--global"},
		{"--repository", "repo", "unexpected"},
		{"--json"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if _, err := parsePRListArgs(args); err == nil {
				t.Fatalf("parsePRListArgs(%v) succeeded, want error", args)
			}
		})
	}
}

func TestADOPullRequestListUsesExplicitRepositoryAndProjectsDates(t *testing.T) {
	client := &prListSequenceClient{pages: [][]ado.PullRequest{
		{{
			ID: 7, Title: "Title", Status: "completed", IsDraft: true,
			CreationDate: "2026-09-07T12:11:12.123+02:00", SourceRefName: "refs/heads/feature/example", TargetRefName: "refs/heads/main",
			URL:        "https://dev.azure.com/org/project/_apis/git/repositories/repo/pullRequests/7",
			Repository: ado.PullRequestRepo{ID: "repo-id", Name: "repo"},
		}},
		{},
	}}
	runner := prListTestRunner(t, client)
	runner.deps.GitRemotes = func(string) ([]gitRemote, error) {
		t.Fatal("GitRemotes called despite explicit repository")
		return nil, nil
	}
	runner.deps.CurrentBranch = func(string) (string, error) {
		t.Fatal("CurrentBranch called for pr list")
		return "", nil
	}

	var stdout bytes.Buffer
	err := runner.Run([]string{"ado", "pr", "list", "--source", " feature/example ", "--target", "main", "--status", "all", "--repository", "repo-id"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := `{"pullRequests":[{"pullRequestId":7,"title":"Title","status":"completed","isDraft":true,"repositoryId":"repo-id","repositoryName":"repo","sourceRefName":"refs/heads/feature/example","targetRefName":"refs/heads/main","creationDate":"2026-09-07T10:11:12.123Z","closedDate":null,"url":"https://dev.azure.com/org/project/_apis/git/repositories/repo/pullRequests/7"}]}` + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if len(client.opts) != 2 {
		t.Fatalf("list calls = %d, want first page and terminal empty page", len(client.opts))
	}
	if got := client.opts[0]; got.RepositoryID != "repo-id" || got.SourceRefName != "refs/heads/feature/example" || got.TargetRefName != "refs/heads/main" || got.Status != "all" {
		t.Fatalf("first list opts = %+v, want explicit filters", got)
	}
}

func TestADOPullRequestListFailureLeavesStdoutEmpty(t *testing.T) {
	client := &prListSequenceClient{listErr: errors.New("request failed")}
	runner := prListTestRunner(t, client)
	var stdout bytes.Buffer
	err := runner.Run([]string{"ado", "pr", "list", "--repository", "repo-id"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("error = %v, want request failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPullRequestListSupportsDetachedHeadAndInferredRepository(t *testing.T) {
	client := &prListSequenceClient{pages: [][]ado.PullRequest{{}}}
	runner := prListTestRunner(t, client)
	runner.deps.CurrentBranch = nil
	var stdout bytes.Buffer
	if err := runner.Run([]string{"ado", "pr", "list"}, strings.NewReader(""), &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got, want := stdout.String(), "{\"pullRequests\":[]}\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if len(client.opts) != 1 || client.opts[0].RepositoryID != "repo" || client.opts[0].SourceRefName != "" || client.opts[0].TargetRefName != "" || client.opts[0].Status != "active" {
		t.Fatalf("list opts = %+v, want inferred repo with no branch filters and active status", client.opts)
	}
}

func TestADOPullRequestListLaterPageFailureLeavesStdoutEmpty(t *testing.T) {
	client := &prListSequenceClient{
		pages:     [][]ado.PullRequest{{{ID: 1, Repository: ado.PullRequestRepo{ID: "repo"}}}},
		listErrAt: 2,
		listErr:   errors.New("later page failed"),
	}
	runner := prListTestRunner(t, client)
	var stdout bytes.Buffer
	err := runner.Run([]string{"ado", "pr", "list", "--repository", "repo"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "later page failed") {
		t.Fatalf("error = %v, want later-page failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPullRequestListHelpDoesNotLoadCredentials(t *testing.T) {
	runner := Runner{deps: Dependencies{
		PATStore: failOnGetPATStore{t: t},
		Getwd: func() (string, error) {
			t.Fatal("Getwd called for help")
			return "", nil
		},
	}}
	var stdout, stderr bytes.Buffer
	if err := runner.Run([]string{"ado", "pr", "list", "--help"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "adomi ado pr list") || !strings.Contains(stderr.String(), "pullRequests") {
		t.Fatalf("stdout/stderr = %q/%q, want side-effect-free list help", stdout.String(), stderr.String())
	}
}

type prListSequenceClient struct {
	fakeADOClient
	pages     [][]ado.PullRequest
	listErr   error
	listErrAt int
	calls     int
	opts      []ado.PullRequestListOptions
}

func (c *prListSequenceClient) ListPullRequests(_ context.Context, opts ado.PullRequestListOptions) ([]ado.PullRequest, error) {
	c.calls++
	c.opts = append(c.opts, opts)
	if c.listErr != nil && (c.listErrAt == 0 || c.calls == c.listErrAt) {
		return nil, c.listErr
	}
	if len(c.pages) == 0 {
		return []ado.PullRequest{}, nil
	}
	page := c.pages[0]
	c.pages = c.pages[1:]
	return page, nil
}

func prListTestRunner(t *testing.T, client ADOClient) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"ado": "pat"}},
		Getwd:        func() (string, error) { return "/repo", nil },
		UserHomeDir:  func() (string, error) { return "/home", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(string, string, string, config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "profile", PATRef: "ado", BaseURL: "https://dev.azure.com/org", Project: "project", APIVersion: "7.1"}}, nil
		},
		GitRemotes: func(string) ([]gitRemote, error) {
			return []gitRemote{{Name: "origin", URL: "https://dev.azure.com/org/project/_git/repo"}}, nil
		},
		NewHTTPClient: func(string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient:  func(*http.Client, ado.ClientConfig) (ADOClient, error) { return client, nil },
	}}
}
