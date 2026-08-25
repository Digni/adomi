package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func workItemCommentFailFastRunner(t *testing.T) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		PATStore: failOnGetPATStore{t: t},
		Getwd:    func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir: func() (string, error) {
			t.Fatalf("UserHomeDir called before validation failure")
			return "", nil
		},
		FindRepoRoot: func(string) (string, error) {
			t.Fatalf("FindRepoRoot called before validation failure")
			return "", nil
		},
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			t.Fatalf("LoadConfig called before validation failure")
			return nil, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) {
			t.Fatalf("NewHTTPClient called before validation failure")
			return nil, nil
		},
		NewADOClient: func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			t.Fatalf("NewADOClient called before validation failure")
			return nil, nil
		},
	}}
}

func TestADOHelpOmitsHiddenConfigCommand(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "--help"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if strings.Contains(stderr.String(), "config") {
		t.Fatalf("stderr = %q, want hidden config alias omitted", stderr.String())
	}
	for _, want := range []string{"fetch", "comment", "work-item", "pr", "login", "logout", "profiles"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func TestADOActionCommandHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "fetch work item",
			args: []string{"ado", "fetch", "--help"},
			want: []string{"Usage:", "adomi ado fetch", "work-item-id", "--profile", "--global", "--json", "stdout", "context directory", "Progress and summary", "stderr"},
		},
		{
			name: "fetch pull request",
			args: []string{"ado", "pr", "fetch", "--help"},
			want: []string{"Usage:", "adomi ado pr fetch", "pull-request-id", "--profile", "--global", "--json", "stdout", "context directory", "Progress and summary", "stderr"},
		},
		{
			name: "work item comment shorthand",
			args: []string{"ado", "comment", "--help"},
			want: []string{"Usage:", "adomi ado comment", "work-item-id", "--message", "--message-file", "exactly one", "--profile", "--global", "--json", "stdout", "comment ID"},
		},
		{
			name: "work item comment namespace",
			args: []string{"ado", "work-item", "comment", "--help"},
			want: []string{"Usage:", "adomi ado work-item comment", "work-item-id", "--message", "--message-file", "exactly one", "--profile", "--global", "--json", "stdout", "comment ID"},
		},
		{
			name: "login",
			args: []string{"ado", "login", "--help"},
			want: []string{"Usage:", "adomi ado login", "--profile", "--pat-ref", "--global", "--json", "PAT", "Confirmation", "stderr", "stdout"},
		},
		{
			name: "logout",
			args: []string{"ado", "logout", "--help"},
			want: []string{"Usage:", "adomi ado logout", "--profile", "--pat-ref", "--global", "--json", "credential", "Confirmation", "stderr", "stdout"},
		},
		{
			name: "profiles list",
			args: []string{"ado", "profiles", "list", "--help"},
			want: []string{"Usage:", "adomi ado profiles list", "--global", "stdout", "profile"},
		},
		{
			name: "config init",
			args: []string{"config", "init", "--help"},
			want: []string{"Usage:", "adomi config init", "--global", "stdout", "configuration"},
		},
		{
			name: "hidden ado config init alias",
			args: []string{"ado", "config", "init", "--help"},
			want: []string{"Usage:", "adomi config init", "--global", "stdout", "configuration"},
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

func runHelp(t *testing.T, runner Runner, args []string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := runner.Run(args, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(%v) returned error: %v", args, err)
	}
	if stdout.String() != "" {
		t.Fatalf("Run(%v) stdout = %q, want empty", args, stdout.String())
	}
	if stderr.String() == "" {
		t.Fatalf("Run(%v) stderr empty, want help", args)
	}
	return stderr.String()
}

type fakeADOClient struct{}

func (fakeADOClient) ListInProgressPipelineRuns(context.Context) ([]ado.PipelineRun, error) {
	return []ado.PipelineRun{}, nil
}

func (fakeADOClient) ListRecentPipelineRuns(context.Context, int) ([]ado.PipelineRun, error) {
	return []ado.PipelineRun{}, nil
}

func (fakeADOClient) GetPipelineRun(context.Context, int) (*ado.PipelineRun, error) {
	return &ado.PipelineRun{}, nil
}

func (fakeADOClient) ResolveWiki(ctx context.Context, identifier string) (*ado.Wiki, error) {
	return &ado.Wiki{ID: identifier, Name: identifier}, nil
}

func (fakeADOClient) FetchWikiPage(ctx context.Context, wikiIdentifier string, opts ado.WikiPageFetchOptions) (*ado.WikiPage, error) {
	return &ado.WikiPage{Path: opts.Path}, nil
}

func (fakeADOClient) FetchWorkItem(ctx context.Context, id int) (*ado.WorkItem, error) {
	return &ado.WorkItem{ID: id}, nil
}

func (fakeADOClient) Download(ctx context.Context, rawURL string) ([]byte, error) {
	return nil, nil
}

func (fakeADOClient) CreateWorkItemComment(ctx context.Context, opts ado.WorkItemCommentCreateOptions) (*ado.WorkItemComment, error) {
	return &ado.WorkItemComment{ID: 1, WorkItemID: opts.WorkItemID}, nil
}

func (fakeADOClient) LinkWorkItemToPullRequest(ctx context.Context, workItemID, expectedRevision int, artifactURL string) (*ado.WorkItem, error) {
	return &ado.WorkItem{ID: workItemID, Rev: expectedRevision + 1, Relations: []ado.Relation{{Rel: "ArtifactLink", URL: artifactURL}}}, nil
}

func (fakeADOClient) FetchPullRequest(ctx context.Context, id int) (*ado.PullRequest, error) {
	return &ado.PullRequest{ID: id, Repository: ado.PullRequestRepo{ID: "repo"}}, nil
}

func (fakeADOClient) FetchPullRequestThreads(ctx context.Context, repositoryID string, pullRequestID int) ([]ado.PullRequestThread, error) {
	return nil, nil
}

func (fakeADOClient) ListPullRequests(ctx context.Context, opts ado.PullRequestListOptions) ([]ado.PullRequest, error) {
	return nil, nil
}

func (fakeADOClient) CreatePullRequest(ctx context.Context, opts ado.PullRequestCreateOptions) (*ado.PullRequest, error) {
	return &ado.PullRequest{}, nil
}

func (fakeADOClient) UpdatePullRequest(ctx context.Context, opts ado.PullRequestUpdateOptions) (*ado.PullRequest, error) {
	return &ado.PullRequest{}, nil
}

func (fakeADOClient) FetchAuthenticatedIdentity(context.Context) (*ado.IdentityRef, error) {
	return &ado.IdentityRef{ID: "caller-id"}, nil
}

func (fakeADOClient) SetPullRequestReviewerVote(_ context.Context, opts ado.PullRequestReviewerVoteOptions) (*ado.PullRequestReviewer, error) {
	return &ado.PullRequestReviewer{ID: opts.ReviewerID, Vote: opts.Vote, IsRequired: opts.IsRequired}, nil
}

func (fakeADOClient) ListPullRequestIterations(ctx context.Context, repositoryID string, pullRequestID int) ([]ado.PullRequestIteration, error) {
	return nil, nil
}

func (fakeADOClient) ListPullRequestIterationChanges(ctx context.Context, opts ado.PullRequestIterationChangesOptions) ([]ado.PullRequestIterationChange, error) {
	return nil, nil
}

func (fakeADOClient) CreatePullRequestThread(ctx context.Context, opts ado.PullRequestThreadCreateOptions) (*ado.PullRequestThread, error) {
	return &ado.PullRequestThread{}, nil
}

func (fakeADOClient) CreatePullRequestThreadComment(ctx context.Context, opts ado.PullRequestThreadCommentCreateOptions) (*ado.PullRequestComment, error) {
	return &ado.PullRequestComment{}, nil
}

func (fakeADOClient) UpdatePullRequestThread(ctx context.Context, opts ado.PullRequestThreadUpdateOptions) (*ado.PullRequestThread, error) {
	return &ado.PullRequestThread{}, nil
}

type fakePATStore struct {
	values map[string]string
}

type failOnGetPATStore struct {
	t *testing.T
}

func (s failOnGetPATStore) Get(profile string) (string, error) {
	s.t.Fatalf("PATStore.Get called before inference failure")
	return "", nil
}

func (s failOnGetPATStore) Set(profile, pat string) error {
	return nil
}

func (s failOnGetPATStore) Delete(profile string) error {
	return nil
}

func (f *fakePATStore) Get(profile string) (string, error) {
	if f.values == nil {
		return "", errors.New("not found")
	}
	value, ok := f.values[profile]
	if !ok {
		return "", errors.New("not found")
	}
	return value, nil
}

func (f *fakePATStore) Set(profile, pat string) error {
	if f.values == nil {
		f.values = map[string]string{}
	}
	f.values[profile] = pat
	return nil
}

func (f *fakePATStore) Delete(profile string) error {
	delete(f.values, profile)
	return nil
}

func assertLocalFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.Contains(string(content), want) {
		t.Fatalf("%s does not contain %q", path, want)
	}
}
