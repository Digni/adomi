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

func TestADOLoginStoresPATForDirectRef(t *testing.T) {
	store := &fakePATStore{}
	runner := Runner{deps: Dependencies{PATStore: store}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "login", "--pat-ref", "shared-ado"}, strings.NewReader("secret-pat\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := store.values["shared-ado"]; got != "secret-pat" {
		t.Fatalf("stored PAT = %q, want secret-pat", got)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Azure DevOps PAT for shared-ado:") {
		t.Fatalf("stderr = %q, want prompt", stderr.String())
	}
}

func TestADOLoginTrimsDirectPATRef(t *testing.T) {
	store := &fakePATStore{}
	runner := Runner{deps: Dependencies{PATStore: store}}

	err := runner.Run([]string{"ado", "login", "--pat-ref", " shared-ado "}, strings.NewReader("secret-pat\n"), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := store.values["shared-ado"]; got != "secret-pat" {
		t.Fatalf("stored PAT = %q, want secret-pat", got)
	}
	if _, ok := store.values[" shared-ado "]; ok {
		t.Fatal("PAT was stored under untrimmed ref")
	}
}

func TestADOLoginStoresPATForProfilePATRef(t *testing.T) {
	store := &fakePATStore{}
	runner := Runner{deps: Dependencies{
		PATStore:     store,
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" || requestedProfile != "company-cloud" || scope != config.DefaultScope {
				t.Fatalf("LoadConfig args = %q %q %q %v", repoRoot, homeDir, requestedProfile, scope)
			}
			return &config.Loaded{Profile: config.Profile{Name: "company-cloud", PATRef: "shared-ado", BaseURL: "https://dev.azure.com/org", Project: "MyProject"}}, nil
		},
	}}

	err := runner.Run([]string{"ado", "login", "--profile", "company-cloud"}, strings.NewReader("secret-pat\n"), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := store.values["shared-ado"]; got != "secret-pat" {
		t.Fatalf("stored PAT = %q, want secret-pat", got)
	}
}

func TestADOLoginGlobalProfileStoresPATForProfilePATRefWithoutRepo(t *testing.T) {
	findRepoRootCalled := false
	store := &fakePATStore{}
	runner := Runner{deps: Dependencies{
		PATStore:    store,
		Getwd:       func() (string, error) { return "/outside", nil },
		UserHomeDir: func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) {
			findRepoRootCalled = true
			return "", errors.New("not a repo")
		},
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "" || homeDir != "/home/me" || requestedProfile != "global-profile" || scope != config.GlobalScope {
				t.Fatalf("LoadConfig args = %q %q %q %v", repoRoot, homeDir, requestedProfile, scope)
			}
			return &config.Loaded{Profile: config.Profile{Name: "global-profile", PATRef: "shared-global", BaseURL: "https://dev.azure.com/org", Project: "MyProject"}}, nil
		},
	}}

	err := runner.Run([]string{"ado", "login", "--global", "--profile", "global-profile"}, strings.NewReader("secret-pat\n"), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if findRepoRootCalled {
		t.Fatal("FindRepoRoot was called for global profile login")
	}
	if got := store.values["shared-global"]; got != "secret-pat" {
		t.Fatalf("stored PAT = %q, want secret-pat", got)
	}
}

func TestADOLoginGlobalProfileFallsBackToProfileName(t *testing.T) {
	store := &fakePATStore{}
	runner := Runner{deps: Dependencies{
		PATStore:    store,
		UserHomeDir: func() (string, error) { return "/home/me", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "global-profile", BaseURL: "https://dev.azure.com/org", Project: "MyProject"}}, nil
		},
	}}

	err := runner.Run([]string{"ado", "login", "--global", "--profile", "global-profile"}, strings.NewReader("secret-pat\n"), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := store.values["global-profile"]; got != "secret-pat" {
		t.Fatalf("stored PAT = %q, want secret-pat", got)
	}
}

func TestADOLogoutDeletesPATForDirectRef(t *testing.T) {
	store := &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}}
	runner := Runner{deps: Dependencies{PATStore: store}}

	err := runner.Run([]string{"ado", "logout", "--pat-ref", "shared-ado"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, ok := store.values["shared-ado"]; ok {
		t.Fatal("PAT still exists after logout")
	}
}

func TestADOLogoutProfileDeletesPATRef(t *testing.T) {
	store := &fakePATStore{values: map[string]string{"shared-ado": "secret-pat", "company-cloud": "old-pat"}}
	runner := Runner{deps: Dependencies{
		PATStore:     store,
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "company-cloud", PATRef: "shared-ado", BaseURL: "https://dev.azure.com/org", Project: "MyProject"}}, nil
		},
	}}

	err := runner.Run([]string{"ado", "logout", "--profile", "company-cloud"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, ok := store.values["shared-ado"]; ok {
		t.Fatal("PAT ref still exists after logout")
	}
	if got := store.values["company-cloud"]; got != "old-pat" {
		t.Fatalf("profile-name PAT = %q, want untouched old-pat", got)
	}
}

func TestADOLogoutGlobalProfileDeletesPATRefWithoutRepo(t *testing.T) {
	findRepoRootCalled := false
	store := &fakePATStore{values: map[string]string{"shared-global": "secret-pat"}}
	runner := Runner{deps: Dependencies{
		PATStore:    store,
		UserHomeDir: func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) {
			findRepoRootCalled = true
			return "", errors.New("not a repo")
		},
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "" || homeDir != "/home/me" || requestedProfile != "global-profile" || scope != config.GlobalScope {
				t.Fatalf("LoadConfig args = %q %q %q %v", repoRoot, homeDir, requestedProfile, scope)
			}
			return &config.Loaded{Profile: config.Profile{Name: "global-profile", PATRef: "shared-global", BaseURL: "https://dev.azure.com/org", Project: "MyProject"}}, nil
		},
	}}

	err := runner.Run([]string{"ado", "logout", "--global", "--profile", "global-profile"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if findRepoRootCalled {
		t.Fatal("FindRepoRoot was called for global profile logout")
	}
	if _, ok := store.values["shared-global"]; ok {
		t.Fatal("PAT ref still exists after logout")
	}
}

func TestADOLoginRequiresProfileOrPATRef(t *testing.T) {
	runner := Runner{deps: Dependencies{PATStore: &fakePATStore{}}}

	err := runner.Run([]string{"ado", "login"}, strings.NewReader("secret\n"), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want error")
	}
	if !strings.Contains(err.Error(), "--profile") || !strings.Contains(err.Error(), "--pat-ref") {
		t.Fatalf("error = %q, want --profile and --pat-ref", err.Error())
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

	err := runner.Run([]string{"ado", "login", "--pat-ref", "company-cloud"}, failReader{}, &bytes.Buffer{}, &bytes.Buffer{})
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

	err := runner.Run([]string{"ado", "login", "--pat-ref", "company-cloud"}, partialErrorReader{}, &bytes.Buffer{}, &bytes.Buffer{})
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
		LoadAllConfig: func(repoRoot, homeDir string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" {
				t.Fatalf("LoadAllConfig args = %q %q", repoRoot, homeDir)
			}
			if scope != config.DefaultScope {
				t.Fatalf("scope = %v, want default", scope)
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

func TestADOProfilesListGlobalDoesNotRequireRepo(t *testing.T) {
	findRepoRootCalled := false
	runner := Runner{deps: Dependencies{
		Getwd:       func() (string, error) { return "/outside", nil },
		UserHomeDir: func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) {
			findRepoRootCalled = true
			return "", errors.New("not a repo")
		},
		LoadAllConfig: func(repoRoot, homeDir string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "" || homeDir != "/home/me" || scope != config.GlobalScope {
				t.Fatalf("LoadAllConfig args = %q %q %v", repoRoot, homeDir, scope)
			}
			return &config.Loaded{Config: config.File{AzureDevOps: config.AzureDevOpsConfig{Profiles: map[string]config.Profile{"home": {}}}}}, nil
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "profiles", "list", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if findRepoRootCalled {
		t.Fatal("FindRepoRoot was called for global profile listing")
	}
	if strings.TrimSpace(stdout.String()) != "home" {
		t.Fatalf("stdout = %q, want home", stdout.String())
	}
}

func TestADOProfilesListRejectsUnknownArgs(t *testing.T) {
	runner := Runner{deps: Dependencies{}}

	err := runner.Run([]string{"ado", "profiles", "list", "--unknown"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("error = %q, want unknown argument", err.Error())
	}
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
	for _, want := range []string{"fetch", "pr", "login", "logout", "profiles"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func TestADOLoginRejectsInvalidCredentialArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "profile and PAT ref", args: []string{"ado", "login", "--profile", "company-cloud", "--pat-ref", "shared"}, want: "cannot use"},
		{name: "global PAT ref", args: []string{"ado", "login", "--global", "--pat-ref", "shared"}, want: "--global requires --profile"},
		{name: "global without profile", args: []string{"ado", "login", "--global"}, want: "--global requires --profile"},
		{name: "missing PAT ref value", args: []string{"ado", "login", "--pat-ref"}, want: "--pat-ref requires a value"},
		{name: "flag as PAT ref value", args: []string{"ado", "login", "--pat-ref", "--profile"}, want: "--pat-ref requires a value"},
		{name: "blank PAT ref value", args: []string{"ado", "login", "--pat-ref", "   "}, want: "patRef"},
		{name: "control character PAT ref", args: []string{"ado", "login", "--pat-ref", "shared\nref"}, want: "patRef"},
		{name: "too long PAT ref", args: []string{"ado", "login", "--pat-ref", strings.Repeat("a", 257)}, want: "patRef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := Runner{deps: Dependencies{PATStore: &fakePATStore{}}}
			err := runner.Run(tt.args, strings.NewReader("secret\n"), &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil {
				t.Fatal("Run error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestADOLogoutRejectsInvalidCredentialArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "profile and PAT ref", args: []string{"ado", "logout", "--profile", "company-cloud", "--pat-ref", "shared"}, want: "cannot use"},
		{name: "global PAT ref", args: []string{"ado", "logout", "--global", "--pat-ref", "shared"}, want: "--global requires --profile"},
		{name: "global without profile", args: []string{"ado", "logout", "--global"}, want: "--global requires --profile"},
		{name: "missing PAT ref value", args: []string{"ado", "logout", "--pat-ref"}, want: "--pat-ref requires a value"},
		{name: "flag as PAT ref value", args: []string{"ado", "logout", "--pat-ref", "--profile"}, want: "--pat-ref requires a value"},
		{name: "blank PAT ref value", args: []string{"ado", "logout", "--pat-ref", "   "}, want: "patRef"},
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

func TestConfigInitCreatesRepoConfig(t *testing.T) {
	repoRoot := t.TempDir()
	runner := Runner{deps: Dependencies{
		Getwd:        func() (string, error) { return filepath.Join(repoRoot, "subdir"), nil },
		UserHomeDir:  func() (string, error) { return "", errors.New("home should not be required") },
		FindRepoRoot: func(string) (string, error) { return repoRoot, nil },
		RemoteURLs:   func(string) ([]string, error) { return nil, nil },
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"config", "init"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	configPath := filepath.Join(repoRoot, ".adomi", "config.yaml")
	if strings.TrimSpace(stdout.String()) != configPath {
		t.Fatalf("stdout = %q, want config path", stdout.String())
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	assertAllCommented(t, string(data))
	assertPathPerm(t, filepath.Dir(configPath), 0o700)
	assertPathPerm(t, configPath, 0o600)
}

func TestADOConfigInitCreatesRepoConfigCompatibilityAlias(t *testing.T) {
	repoRoot := t.TempDir()
	runner := Runner{deps: Dependencies{
		Getwd:        func() (string, error) { return filepath.Join(repoRoot, "subdir"), nil },
		UserHomeDir:  func() (string, error) { return "", errors.New("home should not be required") },
		FindRepoRoot: func(string) (string, error) { return repoRoot, nil },
		RemoteURLs:   func(string) ([]string, error) { return nil, nil },
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "config", "init"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	configPath := filepath.Join(repoRoot, ".adomi", "config.yaml")
	if strings.TrimSpace(stdout.String()) != configPath {
		t.Fatalf("stdout = %q, want config path", stdout.String())
	}
}

func TestADOConfigInitOutsideRepoReturnsError(t *testing.T) {
	homeDir := t.TempDir()
	runner := Runner{deps: Dependencies{
		Getwd:       func() (string, error) { return "/outside", nil },
		UserHomeDir: func() (string, error) { return homeDir, nil },
		FindRepoRoot: func(string) (string, error) {
			return "", errors.New("not a repo")
		},
	}}

	err := runner.Run([]string{"ado", "config", "init"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want repo error")
	}
	if _, statErr := os.Stat(filepath.Join(homeDir, ".config", "adomi", "config.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("global config stat = %v, want not exist", statErr)
	}
}

func TestADOConfigInitGlobalCreatesHomeConfigOutsideRepo(t *testing.T) {
	homeDir := t.TempDir()
	runner := Runner{deps: Dependencies{
		Getwd:       func() (string, error) { return "/outside", nil },
		UserHomeDir: func() (string, error) { return homeDir, nil },
		FindRepoRoot: func(string) (string, error) {
			return "", errors.New("not a repo")
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"config", "init", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	configPath := filepath.Join(homeDir, ".config", "adomi", "config.yaml")
	if strings.TrimSpace(stdout.String()) != configPath {
		t.Fatalf("stdout = %q, want global config path", stdout.String())
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	assertAllCommented(t, string(data))
	assertPathPerm(t, filepath.Dir(configPath), 0o700)
	assertPathPerm(t, configPath, 0o600)
}

func TestADOConfigInitRefusesOverwrite(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, ".adomi", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	runner := Runner{deps: Dependencies{
		Getwd:        func() (string, error) { return repoRoot, nil },
		UserHomeDir:  func() (string, error) { return t.TempDir(), nil },
		FindRepoRoot: func(string) (string, error) { return repoRoot, nil },
	}}

	err := runner.Run([]string{"ado", "config", "init"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want overwrite error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %q, want already exists", err.Error())
	}
}

func TestADOConfigInitRejectsExistingSymlink(t *testing.T) {
	repoRoot := t.TempDir()
	targetPath := filepath.Join(repoRoot, "target.yaml")
	if err := os.WriteFile(targetPath, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("writing target: %v", err)
	}
	configPath := filepath.Join(repoRoot, ".adomi", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.Symlink(targetPath, configPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}
	runner := Runner{deps: Dependencies{
		Getwd:        func() (string, error) { return repoRoot, nil },
		UserHomeDir:  func() (string, error) { return t.TempDir(), nil },
		FindRepoRoot: func(string) (string, error) { return repoRoot, nil },
	}}

	err := runner.Run([]string{"ado", "config", "init"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want existing symlink error")
	}
	data, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("reading target: %v", readErr)
	}
	if string(data) != "existing\n" {
		t.Fatalf("target was modified: %q", data)
	}
}

func TestADOConfigInitRejectsUnknownArgs(t *testing.T) {
	runner := Runner{deps: Dependencies{}}

	err := runner.Run([]string{"config", "init", "--unknown"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want unknown argument error")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("error = %q, want unknown argument", err.Error())
	}
}

func TestADOConfigInitPrefillsAzureDevOpsRemote(t *testing.T) {
	repoRoot := t.TempDir()
	runner := Runner{deps: Dependencies{
		Getwd:        func() (string, error) { return repoRoot, nil },
		UserHomeDir:  func() (string, error) { return t.TempDir(), nil },
		FindRepoRoot: func(string) (string, error) { return repoRoot, nil },
		RemoteURLs: func(string) ([]string, error) {
			return []string{"https://dev.azure.com/my-org/MyProject/_git/adomi"}, nil
		},
	}}

	err := runner.Run([]string{"ado", "config", "init"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, ".adomi", "config.yaml"))
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"#   defaultProfile: MyProject",
		"#       baseUrl: https://dev.azure.com/my-org",
		"#       organization: my-org",
		"#       project: MyProject",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config = %q, want %q", text, want)
		}
	}
}

func TestADOLogoutRequiresProfile(t *testing.T) {
	runner := Runner{deps: Dependencies{PATStore: &fakePATStore{}}}

	err := runner.Run([]string{"ado", "logout"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want error")
	}
	if !strings.Contains(err.Error(), "--profile") || !strings.Contains(err.Error(), "--pat-ref") {
		t.Fatalf("error = %q, want --profile and --pat-ref", err.Error())
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
		FetchPullRequest: func(ctx context.Context, fetcher ado.PullRequestFetcher, id int) (*ado.PullRequestBundle, error) {
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
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
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
		FetchPullRequest: func(ctx context.Context, fetcher ado.PullRequestFetcher, id int) (*ado.PullRequestBundle, error) {
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
		FetchPullRequest: func(ctx context.Context, fetcher ado.PullRequestFetcher, id int) (*ado.PullRequestBundle, error) {
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
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
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
			fmt.Fprintf(w, `{"id":12345,"fields":{"System.WorkItemType":"Task","System.Title":"Task title"},"relations":[{"rel":"System.LinkTypes.Hierarchy-Forward","url":%q},{"rel":"AttachedFile","url":%q,"attributes":{"name":"note.txt"}}]}`, baseURL+"/MyProject/_apis/wit/workItems/12346", baseURL+"/_apis/wit/attachments/note")
		case "/MyProject/_apis/wit/workitems/12346":
			fmt.Fprintf(w, `{"id":12346,"fields":{"System.WorkItemType":"Task","System.Title":"Child task"},"relations":[{"rel":"AttachedFile","url":%q,"attributes":{"name":"child-note.txt"}}]}`, baseURL+"/_apis/wit/attachments/child-note")
		case "/_apis/wit/attachments/note":
			fmt.Fprint(w, "attachment")
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
	assertLocalFile(t, filepath.Join(outputDir, "items", "12346.json"))
	assertLocalFile(t, filepath.Join(outputDir, "html", "12345.html"))
	assertLocalFile(t, filepath.Join(outputDir, "html", "12346.html"))
	assertLocalFile(t, filepath.Join(outputDir, "attachments", "12345", "note.txt"))
	assertLocalFile(t, filepath.Join(outputDir, "attachments", "12346", "child-note.txt"))
	assertFileContains(t, filepath.Join(outputDir, "index.json"), "12346")
	assertFileContains(t, filepath.Join(outputDir, "tree.json"), "Child task")
}

type fakeADOClient struct{}

func (fakeADOClient) FetchWorkItem(ctx context.Context, id int) (*ado.WorkItem, error) {
	return &ado.WorkItem{ID: id}, nil
}

func (fakeADOClient) Download(ctx context.Context, rawURL string) ([]byte, error) {
	return nil, nil
}

func (fakeADOClient) FetchPullRequest(ctx context.Context, id int) (*ado.PullRequest, error) {
	return &ado.PullRequest{ID: id, Repository: ado.PullRequestRepo{ID: "repo"}}, nil
}

func (fakeADOClient) FetchPullRequestThreads(ctx context.Context, repositoryID string, pullRequestID int) ([]ado.PullRequestThread, error) {
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

func assertAllCommented(t *testing.T, content string) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		if !strings.HasPrefix(line, "#") {
			t.Fatalf("line %q is not commented", line)
		}
	}
}

func assertPathPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}
