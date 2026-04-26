package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestADOLoginStoresPATForProfile(t *testing.T) {
	store := &fakePATStore{}
	runner := Runner{deps: Dependencies{PATStore: store}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "login", "--profile", "company-cloud"}, strings.NewReader("secret-pat\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := store.values["company-cloud"]; got != "secret-pat" {
		t.Fatalf("stored PAT = %q, want secret-pat", got)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Azure DevOps PAT for company-cloud:") {
		t.Fatalf("stderr = %q, want prompt", stderr.String())
	}
}

func TestADOLogoutDeletesPATForProfile(t *testing.T) {
	store := &fakePATStore{values: map[string]string{"company-cloud": "secret-pat"}}
	runner := Runner{deps: Dependencies{PATStore: store}}

	err := runner.Run([]string{"ado", "logout", "--profile", "company-cloud"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, ok := store.values["company-cloud"]; ok {
		t.Fatal("PAT still exists after logout")
	}
}

func TestADOLoginRequiresProfile(t *testing.T) {
	runner := Runner{deps: Dependencies{PATStore: &fakePATStore{}}}

	err := runner.Run([]string{"ado", "login"}, strings.NewReader("secret\n"), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want error")
	}
	if !strings.Contains(err.Error(), "--profile") {
		t.Fatalf("error = %q, want --profile", err.Error())
	}
}

func TestADOLoginUsesSecretReader(t *testing.T) {
	store := &fakePATStore{}
	var called bool
	runner := Runner{deps: Dependencies{
		PATStore: store,
		ReadSecret: func(prompt string, stdin io.Reader, stderr io.Writer) (string, error) {
			called = true
			if prompt != "Azure DevOps PAT for company-cloud: " {
				t.Fatalf("prompt = %q, want profile prompt", prompt)
			}
			return "secret-pat", nil
		},
	}}

	err := runner.Run([]string{"ado", "login", "--profile", "company-cloud"}, failReader{}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called {
		t.Fatal("ReadSecret was not called")
	}
	if got := store.values["company-cloud"]; got != "secret-pat" {
		t.Fatalf("stored PAT = %q, want secret-pat", got)
	}
}

func TestADOLoginRejectsPartialPATFromNonEOFReadError(t *testing.T) {
	store := &fakePATStore{}
	runner := Runner{deps: Dependencies{PATStore: store}}

	err := runner.Run([]string{"ado", "login", "--profile", "company-cloud"}, partialErrorReader{}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want read error")
	}
	if !strings.Contains(err.Error(), "reading PAT") {
		t.Fatalf("error = %q, want reading PAT context", err.Error())
	}
	if _, ok := store.values["company-cloud"]; ok {
		t.Fatal("PAT was stored after partial read error")
	}
}

func TestADOProfilesListPrintsConfiguredProfiles(t *testing.T) {
	runner := Runner{deps: Dependencies{
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadAllConfig: func(repoRoot, homeDir string) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" {
				t.Fatalf("LoadAllConfig args = %q %q", repoRoot, homeDir)
			}
			return &config.Loaded{Config: config.File{AzureDevOps: config.AzureDevOpsConfig{Profiles: map[string]config.Profile{
				"second": {},
				"first":  {},
			}}}}, nil
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "profiles", "list"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	got := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("profiles = %v, want %v", got, want)
	}
}

func TestADOFetchPrintsOnlyExportedPath(t *testing.T) {
	store := &fakePATStore{values: map[string]string{"company-cloud": "secret-pat"}}
	fakeClient := fakeADOClient{}
	createdAt := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	tree := &ado.WorkItemTree{RootID: 12345, WorkItems: []ado.WorkItem{{ID: 12345}}}
	runner := Runner{deps: Dependencies{
		PATStore:     store,
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(start string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string) (*config.Loaded, error) {
			if requestedProfile != "company-cloud" {
				t.Fatalf("requested profile = %q, want company-cloud", requestedProfile)
			}
			return &config.Loaded{Profile: config.Profile{
				Name:       "company-cloud",
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
			return "/repo/.adomi/azure-devops/company-cloud/MyProject/work-items/12345", nil
		},
		Now: func() time.Time { return createdAt },
	}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "fetch", "12345", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := "/repo/.adomi/azure-devops/company-cloud/MyProject/work-items/12345\n"
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
		LoadConfig: func(repoRoot, homeDir, requestedProfile string) (*config.Loaded, error) {
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

func TestADOLogoutRequiresProfile(t *testing.T) {
	runner := Runner{deps: Dependencies{PATStore: &fakePATStore{}}}

	err := runner.Run([]string{"ado", "logout"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want error")
	}
	if !strings.Contains(err.Error(), "--profile") {
		t.Fatalf("error = %q, want --profile", err.Error())
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
			fmt.Fprintf(w, `{"id":12345,"fields":{"System.WorkItemType":"Task","System.Title":"Task title"},"relations":[{"rel":"AttachedFile","url":%q,"attributes":{"name":"note.txt"}}]}`, baseURL+"/_apis/wit/attachments/note")
		case "/_apis/wit/attachments/note":
			fmt.Fprint(w, "attachment")
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
      baseUrl: `+server.URL+`
      project: MyProject
`), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	runner := Runner{deps: Dependencies{
		PATStore:    &fakePATStore{values: map[string]string{"company-cloud": "secret-pat"}},
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
	wantOutputDir := filepath.Join(repoRoot, ".adomi", "azure-devops", "company-cloud", "MyProject", "work-items", "12345")
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
	assertLocalFile(t, filepath.Join(outputDir, "work-items", "12345.json"))
	assertLocalFile(t, filepath.Join(outputDir, "html", "12345.html"))
	assertLocalFile(t, filepath.Join(outputDir, "attachments", "12345", "note.txt"))
}

type fakeADOClient struct{}

func (fakeADOClient) FetchWorkItem(ctx context.Context, id int) (*ado.WorkItem, error) {
	return &ado.WorkItem{ID: id}, nil
}

func (fakeADOClient) Download(ctx context.Context, rawURL string) ([]byte, error) {
	return nil, nil
}

type fakePATStore struct {
	values map[string]string
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) {
	return 0, errors.New("stdin should not be read")
}

type partialErrorReader struct {
	done bool
}

func (r partialErrorReader) Read(p []byte) (int, error) {
	copy(p, "partial")
	return len("partial"), errors.New("disk read failed")
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
