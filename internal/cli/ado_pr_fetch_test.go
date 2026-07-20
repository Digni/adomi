package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestADOPullRequestFetchCommandPrintsOnlyExportedPath(t *testing.T) {
	store := &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}}
	bundle := &ado.PullRequestBundle{PullRequest: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}}
	runner := Runner{deps: Dependencies{
		PATStore:     store,
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "company-cloud", PATRef: "shared-ado", BaseURL: "https://dev.azure.com/org", Project: "MyProject", APIVersion: "7.1"}}, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient:  func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) { return fakeADOClient{}, nil },
		FetchPullRequest: func(ctx context.Context, fetcher ado.PullRequestFetcher, id int, _ ado.ProgressFunc) (*ado.PullRequestBundle, error) {
			if id != 42 {
				t.Fatalf("id = %d, want 42", id)
			}
			return bundle, nil
		},
		ExportPullRequest: func(opts ado.PullRequestExportOptions, gotBundle *ado.PullRequestBundle) (string, error) {
			return "/repo/.adomi/context/pull-requests/42", nil
		},
	}}
	var stdout bytes.Buffer
	err := runner.Run([]string{"ado", "pr", "fetch", "42", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "/repo/.adomi/context/pull-requests/42\n" {
		t.Fatalf("stdout = %q, want fetch path", stdout.String())
	}
}

func TestADOPullRequestFetchJSONWritesExactResultAndCountsDeletedContent(t *testing.T) {
	bundle := &ado.PullRequestBundle{
		PullRequest: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}},
		Threads: []ado.PullRequestThread{
			{ID: 1, Comments: []ado.PullRequestComment{{ID: 10}, {ID: 11, IsDeleted: true}}},
			{ID: 2, IsDeleted: true, Comments: []ado.PullRequestComment{{ID: 20}}},
			{ID: 3},
		},
	}
	runner := Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(string, string, string, config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "company", PATRef: "shared-ado", BaseURL: "https://dev.azure.com/org", Project: "Project"}}, nil
		},
		NewHTTPClient: func(string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient:  func(*http.Client, ado.ClientConfig) (ADOClient, error) { return fakeADOClient{}, nil },
		FetchPullRequest: func(_ context.Context, _ ado.PullRequestFetcher, _ int, progress ado.ProgressFunc) (*ado.PullRequestBundle, error) {
			progress("Fetched pull request 42")
			progress("Fetched 3 review threads")
			return bundle, nil
		},
		ExportPullRequest: func(ado.PullRequestExportOptions, *ado.PullRequestBundle) (string, error) {
			return "/repo/.adomi/context/pull-requests/42", nil
		},
	}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "fetch", "42", "--profile", "company", "--json"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got, want := stdout.String(), "{\"path\":\"/repo/.adomi/context/pull-requests/42\",\"threadCount\":3,\"commentCount\":2}\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "Fetched pull request 42\nFetched 3 review threads\nExported pull request 42 bundle to /repo/.adomi/context/pull-requests/42\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestADOPullRequestPrintsOnlyExportedPath(t *testing.T) {
	store := &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}}
	fakeClient := fakeADOClient{}
	createdAt := time.Date(2026, 4, 28, 9, 30, 0, 0, time.UTC)
	bundle := &ado.PullRequestBundle{
		PullRequest: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}},
	}
	runner := Runner{deps: Dependencies{
		PATStore:     store,
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(start string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if requestedProfile != "company-cloud" {
				t.Fatalf("requested profile = %q, want company-cloud", requestedProfile)
			}
			if scope != config.DefaultScope {
				t.Fatalf("scope = %v, want default", scope)
			}
			return &config.Loaded{Profile: config.Profile{
				Name:       "company-cloud",
				PATRef:     "shared-ado",
				BaseURL:    "https://dev.azure.com/org",
				Project:    "MyProject",
				APIVersion: "7.1",
				Proxy:      "http://proxy.example:8080",
			}}, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) {
			if proxy != "http://proxy.example:8080" {
				t.Fatalf("proxy = %q, want configured proxy", proxy)
			}
			return http.DefaultClient, nil
		},
		NewADOClient: func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			if cfg.PAT != "secret-pat" || cfg.Project != "MyProject" {
				t.Fatalf("client config = %+v, want PAT and project", cfg)
			}
			return fakeClient, nil
		},
		FetchPullRequest: func(ctx context.Context, fetcher ado.PullRequestFetcher, id int, _ ado.ProgressFunc) (*ado.PullRequestBundle, error) {
			if id != 42 {
				t.Fatalf("PR ID = %d, want 42", id)
			}
			return bundle, nil
		},
		ExportPullRequest: func(opts ado.PullRequestExportOptions, gotBundle *ado.PullRequestBundle) (string, error) {
			if opts.RepoRoot != "/repo" || opts.Profile != "company-cloud" || opts.Project != "MyProject" || !opts.CreatedAt.Equal(createdAt) {
				t.Fatalf("export options = %+v", opts)
			}
			if gotBundle != bundle {
				t.Fatal("ExportPullRequest received unexpected bundle")
			}
			return "/repo/.adomi/context/pull-requests/42", nil
		},
		Now: func() time.Time { return createdAt },
	}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "42", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := "/repo/.adomi/context/pull-requests/42\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if !strings.Contains(stderr.String(), "Exported") {
		if stderr.String() == "" {
			t.Fatalf("stderr empty, want feedback")
		}
		t.Fatalf("stderr = %q, want summary line", stderr.String())
	}
}

func TestADOPullRequestRejectsInvalidArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing ID", args: []string{"ado", "pr"}, want: "usage"},
		{name: "non integer ID", args: []string{"ado", "pr", "abc"}, want: "pull request ID"},
		{name: "zero ID", args: []string{"ado", "pr", "0"}, want: "positive"},
		{name: "negative ID", args: []string{"ado", "pr", "-1"}, want: "positive"},
		{name: "missing profile value", args: []string{"ado", "pr", "42", "--profile"}, want: "--profile"},
		{name: "flag as profile value", args: []string{"ado", "pr", "42", "--profile", "--global"}, want: "--profile requires a value"},
		{name: "unknown flag", args: []string{"ado", "pr", "42", "--unknown"}, want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := Runner{deps: Dependencies{PATStore: &fakePATStore{}}}
			err := runner.Run(tt.args, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil {
				t.Fatal("Run error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestADOPullRequestGlobalUsesGlobalConfigScope(t *testing.T) {
	store := &fakePATStore{values: map[string]string{"home": "secret-pat"}}
	fakeClient := fakeADOClient{}
	bundle := &ado.PullRequestBundle{PullRequest: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "r"}}}
	runner := Runner{deps: Dependencies{
		PATStore:     store,
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(start string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if scope != config.GlobalScope {
				t.Fatalf("scope = %v, want global", scope)
			}
			return &config.Loaded{Profile: config.Profile{Name: "home", BaseURL: "https://dev.azure.com/home", Project: "MyProject", APIVersion: "7.1"}}, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient:  func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) { return fakeClient, nil },
		FetchPullRequest: func(ctx context.Context, fetcher ado.PullRequestFetcher, id int, _ ado.ProgressFunc) (*ado.PullRequestBundle, error) {
			return bundle, nil
		},
		ExportPullRequest: func(opts ado.PullRequestExportOptions, gotBundle *ado.PullRequestBundle) (string, error) {
			return "/repo/.adomi/context/pull-requests/42", nil
		},
	}}

	err := runner.Run([]string{"ado", "pr", "42", "--global", "--profile", "home"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestADOPullRequestLeavesStdoutEmptyOnFetchError(t *testing.T) {
	runner := Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"company-cloud": "secret-pat"}},
		Getwd:        func() (string, error) { return "/repo", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "company-cloud", BaseURL: "https://dev.azure.com/org", Project: "MyProject", APIVersion: "7.1"}}, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient:  func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) { return fakeADOClient{}, nil },
		FetchPullRequest: func(ctx context.Context, fetcher ado.PullRequestFetcher, id int, _ ado.ProgressFunc) (*ado.PullRequestBundle, error) {
			return nil, errors.New("boom")
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "42", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPullRequestWithRealWiringWritesContextAndPrintsOnlyPath(t *testing.T) {
	repoRoot := t.TempDir()
	subdir := filepath.Join(repoRoot, "nested")
	homeDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("creating nested dir: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		switch r.URL.Path {
		case "/MyProject/_apis/git/pullrequests/42":
			fmt.Fprint(w, `{"pullRequestId":42,"title":"Improve checkout","status":"active","sourceRefName":"refs/heads/feature/x","targetRefName":"refs/heads/main","repository":{"id":"repo-uuid","name":"adomi"}}`)
		case "/MyProject/_apis/git/repositories/repo-uuid/pullrequests/42/threads":
			fmt.Fprint(w, `{"count":2,"value":[{"id":1,"status":"fixed","comments":[{"id":100,"author":{"displayName":"Bob"},"content":"LGTM","publishedDate":"2026-04-28T08:00:00Z"}]},{"id":2,"status":"active","threadContext":{"filePath":"/cmd/adomi/main.go"},"comments":[{"id":200,"author":{"displayName":"Alice"},"content":"Nit","publishedDate":"2026-04-28T09:00:00Z"}]}]}`)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	configPath := filepath.Join(repoRoot, ".adomi", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`
azureDevOps:
  defaultProfile: company-cloud
  profiles:
    company-cloud:
      patRef: shared-ado
      baseUrl: `+server.URL+`
      project: MyProject
`), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	runner := Runner{deps: Dependencies{
		PATStore:    &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:       func() (string, error) { return subdir, nil },
		UserHomeDir: func() (string, error) { return homeDir, nil },
		Now:         func() time.Time { return time.Date(2026, 4, 28, 9, 30, 0, 0, time.UTC) },
	}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "42"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	outputDir := strings.TrimSpace(stdout.String())
	wantOutputDir := filepath.Join(repoRoot, ".adomi", "context", "pull-requests", "42")
	if outputDir != wantOutputDir {
		t.Fatalf("stdout path = %q, want %q", outputDir, wantOutputDir)
	}
	if stdout.String() != wantOutputDir+"\n" {
		t.Fatalf("stdout = %q, want path plus newline only", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Exported") {
		if stderr.String() == "" {
			t.Fatalf("stderr empty, want feedback")
		}
		t.Fatalf("stderr = %q, want summary line", stderr.String())
	}
	assertLocalFile(t, filepath.Join(outputDir, "index.json"))
	assertLocalFile(t, filepath.Join(outputDir, "pull-request.json"))
	assertLocalFile(t, filepath.Join(outputDir, "threads.json"))
	assertLocalFile(t, filepath.Join(outputDir, "threads", "1.json"))
	assertLocalFile(t, filepath.Join(outputDir, "threads", "2.json"))
	assertLocalFile(t, filepath.Join(outputDir, "comments.md"))
	assertFileContains(t, filepath.Join(outputDir, "index.json"), "repo-uuid")
	assertFileContains(t, filepath.Join(outputDir, "index.json"), "Improve checkout")
	assertFileContains(t, filepath.Join(outputDir, "comments.md"), "LGTM")
	assertFileContains(t, filepath.Join(outputDir, "comments.md"), "Nit")
	assertFileContains(t, filepath.Join(outputDir, "comments.md"), "/cmd/adomi/main.go")
	assertFileContains(t, filepath.Join(outputDir, "threads", "2.json"), "Nit")
}
