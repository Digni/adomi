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
	"sync/atomic"
	"testing"
	"time"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestADOFetchPrintsOnlyExportedPath(t *testing.T) {
	store := &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}}
	fakeClient := fakeADOClient{}
	createdAt := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	tree := &ado.WorkItemTree{RootID: 12345, WorkItems: []ado.WorkItem{{ID: 12345}}}
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
		FetchTree: func(ctx context.Context, fetcher ado.WorkItemFetcher, rootID int) (*ado.WorkItemTree, error) {
			if rootID != 12345 {
				t.Fatalf("root ID = %d, want 12345", rootID)
			}
			return tree, nil
		},
		ExportContext: func(ctx context.Context, downloader ado.AttachmentDownloader, opts ado.ExportOptions, gotTree *ado.WorkItemTree) (string, error) {
			if opts.RepoRoot != "/repo" || opts.Profile != "company-cloud" || opts.Project != "MyProject" || !opts.CreatedAt.Equal(createdAt) {
				t.Fatalf("export options = %+v", opts)
			}
			if gotTree != tree {
				t.Fatal("ExportContext received unexpected tree")
			}
			return "/repo/.adomi/context/work-items/12345", nil
		},
		Now: func() time.Time { return createdAt },
	}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "fetch", "12345", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := "/repo/.adomi/context/work-items/12345\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestADOFetchLeavesStdoutEmptyOnError(t *testing.T) {
	runner := Runner{deps: Dependencies{
		PATStore:     &fakePATStore{},
		Getwd:        func() (string, error) { return "/repo", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "company-cloud", BaseURL: "https://dev.azure.com/org", Project: "MyProject", APIVersion: "7.1"}}, nil
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "fetch", "12345", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOFetchInvalidTokenRedirectReturnsStatusWithoutFollowingSignIn(t *testing.T) {
	var signInRequests atomic.Int32
	signIn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signInRequests.Add(1)
		fmt.Fprint(w, "<html>interactive sign-in page</html>")
	}))
	t.Cleanup(signIn.Close)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("Authorization header is empty")
		}
		http.Redirect(w, r, signIn.URL+"/signin", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	runner := Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"shared-ado": "invalid-pat"}},
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{
				Name:       "company-cloud",
				PATRef:     "shared-ado",
				BaseURL:    server.URL,
				Project:    "MyProject",
				APIVersion: "7.1",
			}}, nil
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "fetch", "12345", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want redirect status error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if signInRequests.Load() != 0 {
		t.Fatalf("sign-in target requests = %d, want 0", signInRequests.Load())
	}
	errorText := err.Error()
	for _, want := range []string{"302", "PAT", "base URL"} {
		if !strings.Contains(errorText, want) {
			t.Errorf("error = %q, want %q", errorText, want)
		}
	}
	for _, leaked := range []string{"decoding", "interactive sign-in page", signIn.URL, "/signin"} {
		if strings.Contains(errorText, leaked) {
			t.Errorf("error = %q, want no %q", errorText, leaked)
		}
	}
}

func TestADOFetchRejectsInvalidArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing ID", args: []string{"ado", "fetch"}, want: "usage"},
		{name: "non integer ID", args: []string{"ado", "fetch", "abc"}, want: "work item ID"},
		{name: "zero ID", args: []string{"ado", "fetch", "0"}, want: "positive"},
		{name: "negative ID", args: []string{"ado", "fetch", "-1"}, want: "positive"},
		{name: "missing profile value", args: []string{"ado", "fetch", "12345", "--profile"}, want: "--profile"},
		{name: "flag as profile value", args: []string{"ado", "fetch", "12345", "--profile", "--global"}, want: "--profile requires a value"},
		{name: "unknown flag", args: []string{"ado", "fetch", "12345", "--unknown"}, want: "unknown"},
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

func TestADOFetchGlobalStillRequiresRepo(t *testing.T) {
	loadConfigCalled := false
	runner := Runner{deps: Dependencies{
		PATStore:    &fakePATStore{values: map[string]string{"home": "secret-pat"}},
		Getwd:       func() (string, error) { return "/outside", nil },
		UserHomeDir: func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) {
			return "", errors.New("not a repo")
		},
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			loadConfigCalled = true
			return nil, errors.New("should not load config")
		},
	}}

	err := runner.Run([]string{"ado", "fetch", "12345", "--global", "--profile", "home"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want repo error")
	}
	if loadConfigCalled {
		t.Fatal("LoadConfig was called after repo resolution failed")
	}
}

func TestADOFetchGlobalUsesGlobalConfigScope(t *testing.T) {
	store := &fakePATStore{values: map[string]string{"home": "secret-pat"}}
	fakeClient := fakeADOClient{}
	tree := &ado.WorkItemTree{RootID: 12345, WorkItems: []ado.WorkItem{{ID: 12345}}}
	runner := Runner{deps: Dependencies{
		PATStore:     store,
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(start string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" || requestedProfile != "home" {
				t.Fatalf("LoadConfig args = %q %q %q", repoRoot, homeDir, requestedProfile)
			}
			if scope != config.GlobalScope {
				t.Fatalf("scope = %v, want global", scope)
			}
			return &config.Loaded{Profile: config.Profile{Name: "home", BaseURL: "https://dev.azure.com/home", Project: "MyProject", APIVersion: "7.1"}}, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient:  func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) { return fakeClient, nil },
		FetchTree: func(ctx context.Context, fetcher ado.WorkItemFetcher, rootID int) (*ado.WorkItemTree, error) {
			return tree, nil
		},
		ExportContext: func(ctx context.Context, downloader ado.AttachmentDownloader, opts ado.ExportOptions, gotTree *ado.WorkItemTree) (string, error) {
			if opts.RepoRoot != "/repo" {
				t.Fatalf("repo root = %q, want /repo", opts.RepoRoot)
			}
			return "/repo/.adomi/context/work-items/12345", nil
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "fetch", "12345", "--global", "--profile", "home"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestADOFetchWithRealWiringWritesContextAndPrintsOnlyPath(t *testing.T) {
	repoRoot := t.TempDir()
	subdir := filepath.Join(repoRoot, "nested")
	homeDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("creating nested dir: %v", err)
	}

	var baseURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		switch r.URL.Path {
		case "/MyProject/_apis/wit/workitems/12345":
			fmt.Fprintf(w, `{"id":12345,"fields":{"System.WorkItemType":"Task","System.Title":"Task title"},"relations":[{"rel":"System.LinkTypes.Hierarchy-Reverse","url":%q},{"rel":"System.LinkTypes.Hierarchy-Forward","url":%q},{"rel":"AttachedFile","url":%q,"attributes":{"name":"note.txt"}}]}`, baseURL+"/MyProject/_apis/wit/workItems/12000", baseURL+"/MyProject/_apis/wit/workItems/12346", baseURL+"/_apis/wit/attachments/note")
		case "/MyProject/_apis/wit/workitems/12000":
			fmt.Fprintf(w, `{"id":12000,"fields":{"System.WorkItemType":"User Story","System.Title":"Parent story"},"relations":[{"rel":"System.LinkTypes.Hierarchy-Reverse","url":%q},{"rel":"AttachedFile","url":%q,"attributes":{"name":"parent-note.txt"}}]}`, baseURL+"/MyProject/_apis/wit/workItems/10000", baseURL+"/_apis/wit/attachments/parent-note")
		case "/MyProject/_apis/wit/workitems/10000":
			fmt.Fprintf(w, `{"id":10000,"fields":{"System.WorkItemType":"Epic","System.Title":"Grandparent epic"},"relations":[{"rel":"AttachedFile","url":%q,"attributes":{"name":"grandparent-note.txt"}}]}`, baseURL+"/_apis/wit/attachments/grandparent-note")
		case "/MyProject/_apis/wit/workitems/12346":
			fmt.Fprintf(w, `{"id":12346,"fields":{"System.WorkItemType":"Task","System.Title":"Child task"},"relations":[{"rel":"AttachedFile","url":%q,"attributes":{"name":"child-note.txt"}}]}`, baseURL+"/_apis/wit/attachments/child-note")
		case "/_apis/wit/attachments/note":
			fmt.Fprint(w, "attachment")
		case "/_apis/wit/attachments/parent-note":
			fmt.Fprint(w, "parent attachment")
		case "/_apis/wit/attachments/grandparent-note":
			fmt.Fprint(w, "grandparent attachment")
		case "/_apis/wit/attachments/child-note":
			fmt.Fprint(w, "child attachment")
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	baseURL = server.URL

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
		Now:         func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) },
	}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "fetch", "12345"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	outputDir := strings.TrimSpace(stdout.String())
	wantOutputDir := filepath.Join(repoRoot, ".adomi", "context", "work-items", "12345")
	if outputDir != wantOutputDir {
		t.Fatalf("stdout path = %q, want %q", outputDir, wantOutputDir)
	}
	if stdout.String() != wantOutputDir+"\n" {
		t.Fatalf("stdout = %q, want path plus newline only", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertLocalFile(t, filepath.Join(outputDir, "index.json"))
	assertLocalFile(t, filepath.Join(outputDir, "tree.json"))
	assertLocalFile(t, filepath.Join(outputDir, "items", "12345.json"))
	assertLocalFile(t, filepath.Join(outputDir, "items", "12000.json"))
	assertLocalFile(t, filepath.Join(outputDir, "items", "10000.json"))
	assertLocalFile(t, filepath.Join(outputDir, "items", "12346.json"))
	assertLocalFile(t, filepath.Join(outputDir, "html", "12345.html"))
	assertLocalFile(t, filepath.Join(outputDir, "html", "12000.html"))
	assertLocalFile(t, filepath.Join(outputDir, "html", "10000.html"))
	assertLocalFile(t, filepath.Join(outputDir, "html", "12346.html"))
	assertLocalFile(t, filepath.Join(outputDir, "attachments", "12345", "note.txt"))
	assertLocalFile(t, filepath.Join(outputDir, "attachments", "12000", "parent-note.txt"))
	assertLocalFile(t, filepath.Join(outputDir, "attachments", "10000", "grandparent-note.txt"))
	assertLocalFile(t, filepath.Join(outputDir, "attachments", "12346", "child-note.txt"))
	assertFileContains(t, filepath.Join(outputDir, "index.json"), "12000")
	assertFileContains(t, filepath.Join(outputDir, "index.json"), "10000")
	assertFileContains(t, filepath.Join(outputDir, "index.json"), "12346")
	assertFileContains(t, filepath.Join(outputDir, "tree.json"), "Parent story")
	assertFileContains(t, filepath.Join(outputDir, "tree.json"), "Grandparent epic")
	assertFileContains(t, filepath.Join(outputDir, "tree.json"), "Child task")
}

func TestADOFetchAttachmentDownloadFailureLeavesStdoutEmpty(t *testing.T) {
	repoRoot := t.TempDir()
	subdir := filepath.Join(repoRoot, "nested")
	homeDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("creating nested dir: %v", err)
	}

	var baseURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		switch r.URL.Path {
		case "/MyProject/_apis/wit/workitems/12345":
			fmt.Fprintf(w, `{"id":12345,"fields":{"System.WorkItemType":"Task","System.Title":"Task title"},"relations":[{"rel":"AttachedFile","url":%q,"attributes":{"name":"note.txt"}}]}`, baseURL+"/_apis/wit/attachments/note")
		case "/_apis/wit/attachments/note":
			http.Error(w, "attachment failed", http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	baseURL = server.URL

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
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "fetch", "12345"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want attachment download error")
	}
	if !strings.Contains(err.Error(), "downloading attachment") {
		t.Fatalf("error = %q, want attachment download context", err.Error())
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
