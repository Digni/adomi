package cli

import (
	"context"
	"net/http"
	"testing"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

type fakePRMaintenanceClient struct {
	fakeADOClient
	listed                 []ado.PullRequest
	listErr                error
	created                *ado.PullRequest
	createErr              error
	updated                *ado.PullRequest
	updateErr              error
	fetched                *ado.PullRequest
	fetchPRErr             error
	createdThread          *ado.PullRequestThread
	threadCreateErr        error
	createdComment         *ado.PullRequestComment
	commentErr             error
	updatedThread          *ado.PullRequestThread
	threadUpdateErr        error
	iterations             []ado.PullRequestIteration
	iterationsErr          error
	iterationChanges       []ado.PullRequestIterationChange
	iterationChangesErr    error
	listCalled             int
	createCalled           int
	updateCalled           int
	fetchPRCalled          int
	threadCreateCalled     int
	commentCalled          int
	threadUpdateCalled     int
	iterationsCalled       int
	iterationChangesCalled int
	fetchedPRID            int
	listOpts               ado.PullRequestListOptions
	createOpts             ado.PullRequestCreateOptions
	updateOpts             ado.PullRequestUpdateOptions
	threadCreateOpts       ado.PullRequestThreadCreateOptions
	commentOpts            ado.PullRequestThreadCommentCreateOptions
	threadUpdateOpts       ado.PullRequestThreadUpdateOptions
	iterationChangesOpts   ado.PullRequestIterationChangesOptions
}

func (f *fakePRMaintenanceClient) FetchPullRequest(ctx context.Context, id int) (*ado.PullRequest, error) {
	f.fetchPRCalled++
	f.fetchedPRID = id
	if f.fetchPRErr != nil {
		return nil, f.fetchPRErr
	}
	if f.fetched != nil {
		return f.fetched, nil
	}
	return &ado.PullRequest{ID: id, Repository: ado.PullRequestRepo{ID: "repo"}}, nil
}

func (f *fakePRMaintenanceClient) ListPullRequests(ctx context.Context, opts ado.PullRequestListOptions) ([]ado.PullRequest, error) {
	f.listCalled++
	f.listOpts = opts
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listed, nil
}

func (f *fakePRMaintenanceClient) CreatePullRequest(ctx context.Context, opts ado.PullRequestCreateOptions) (*ado.PullRequest, error) {
	f.createCalled++
	f.createOpts = opts
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.created != nil {
		return f.created, nil
	}
	return &ado.PullRequest{ID: 101}, nil
}

func (f *fakePRMaintenanceClient) UpdatePullRequest(ctx context.Context, opts ado.PullRequestUpdateOptions) (*ado.PullRequest, error) {
	f.updateCalled++
	f.updateOpts = opts
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updated != nil {
		return f.updated, nil
	}
	return &ado.PullRequest{ID: opts.PullRequestID}, nil
}

func (f *fakePRMaintenanceClient) ListPullRequestIterations(ctx context.Context, repositoryID string, pullRequestID int) ([]ado.PullRequestIteration, error) {
	f.iterationsCalled++
	if f.iterationsErr != nil {
		return nil, f.iterationsErr
	}
	return f.iterations, nil
}

func (f *fakePRMaintenanceClient) ListPullRequestIterationChanges(ctx context.Context, opts ado.PullRequestIterationChangesOptions) ([]ado.PullRequestIterationChange, error) {
	f.iterationChangesCalled++
	f.iterationChangesOpts = opts
	if f.iterationChangesErr != nil {
		return nil, f.iterationChangesErr
	}
	return f.iterationChanges, nil
}

func (f *fakePRMaintenanceClient) CreatePullRequestThread(ctx context.Context, opts ado.PullRequestThreadCreateOptions) (*ado.PullRequestThread, error) {
	f.threadCreateCalled++
	f.threadCreateOpts = opts
	if f.threadCreateErr != nil {
		return nil, f.threadCreateErr
	}
	if f.createdThread != nil {
		return f.createdThread, nil
	}
	return &ado.PullRequestThread{ID: 14, Status: "active", Comments: []ado.PullRequestComment{{ID: 1}}}, nil
}

func (f *fakePRMaintenanceClient) CreatePullRequestThreadComment(ctx context.Context, opts ado.PullRequestThreadCommentCreateOptions) (*ado.PullRequestComment, error) {
	f.commentCalled++
	f.commentOpts = opts
	if f.commentErr != nil {
		return nil, f.commentErr
	}
	if f.createdComment != nil {
		return f.createdComment, nil
	}
	return &ado.PullRequestComment{ID: 8}, nil
}

func (f *fakePRMaintenanceClient) UpdatePullRequestThread(ctx context.Context, opts ado.PullRequestThreadUpdateOptions) (*ado.PullRequestThread, error) {
	f.threadUpdateCalled++
	f.threadUpdateOpts = opts
	if f.threadUpdateErr != nil {
		return nil, f.threadUpdateErr
	}
	if f.updatedThread != nil {
		return f.updatedThread, nil
	}
	return &ado.PullRequestThread{ID: opts.ThreadID, Status: opts.Status}, nil
}

func prThreadTestRunner(t *testing.T, client ADOClient) Runner {
	t.Helper()
	store := &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}}
	return Runner{deps: Dependencies{
		PATStore:     store,
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" || scope != config.DefaultScope {
				t.Fatalf("LoadConfig args = %q %q %q %v", repoRoot, homeDir, requestedProfile, scope)
			}
			if requestedProfile != "" && requestedProfile != "company-cloud" {
				t.Fatalf("requested profile = %q, want empty or company-cloud", requestedProfile)
			}
			return &config.Loaded{Profile: config.Profile{Name: "company-cloud", PATRef: "shared-ado", BaseURL: "https://dev.azure.com/my-org", Project: "MyProject", APIVersion: "7.1"}}, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient: func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			if cfg.PAT != "secret-pat" || cfg.Project != "MyProject" || cfg.BaseURL != "https://dev.azure.com/my-org" {
				t.Fatalf("client config = %+v", cfg)
			}
			return client, nil
		},
	}}
}

func prEnsureTestRunner(t *testing.T, client ADOClient) Runner {
	t.Helper()
	store := &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}}
	return Runner{deps: Dependencies{
		PATStore:     store,
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" || scope != config.DefaultScope {
				t.Fatalf("LoadConfig args = %q %q %q %v", repoRoot, homeDir, requestedProfile, scope)
			}
			if requestedProfile != "" && requestedProfile != "company-cloud" {
				t.Fatalf("requested profile = %q, want empty or company-cloud", requestedProfile)
			}
			return &config.Loaded{Profile: config.Profile{Name: "company-cloud", PATRef: "shared-ado", BaseURL: "https://dev.azure.com/my-org", Project: "MyProject", APIVersion: "7.1"}}, nil
		},
		GitRemotes: func(repoRoot string) ([]gitRemote, error) {
			return []gitRemote{{Name: "origin", URL: "https://dev.azure.com/my-org/MyProject/_git/adomi"}}, nil
		},
		CurrentBranch:       func(repoRoot string) (string, error) { return "feature/x", nil },
		RemoteDefaultBranch: func(repoRoot, remoteName string) (string, error) { return "main", nil },
		NewHTTPClient:       func(proxy string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient: func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			if cfg.PAT != "secret-pat" || cfg.Project != "MyProject" || cfg.BaseURL != "https://dev.azure.com/my-org" {
				t.Fatalf("client config = %+v", cfg)
			}
			return client, nil
		},
	}}
}

func prEnsureInferenceFailureTestRunner(t *testing.T, client ADOClient) Runner {
	t.Helper()
	runner := prEnsureTestRunner(t, client)
	runner.deps.PATStore = failOnGetPATStore{t: t}
	runner.deps.NewHTTPClient = func(proxy string) (*http.Client, error) {
		t.Fatalf("NewHTTPClient called before inference failure")
		return nil, nil
	}
	runner.deps.NewADOClient = func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
		t.Fatalf("NewADOClient called before inference failure")
		return nil, nil
	}
	return runner
}
