package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
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

func TestADOWorkItemCommentCreatesCommentFromInlineMessage(t *testing.T) {
	var requestCount int
	var seenMethod, seenPath, seenAPIVersion, seenAuth string
	var seenBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"commentId":55,"workItemId":12345,"url":"https://dev.azure.com/my-org/MyProject/_workitems/edit/12345#comment-55"}`)
	}))
	t.Cleanup(server.Close)
	runner := workItemCommentHTTPRunner(t, server.Client(), server.URL)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "comment", "12345", "--message", "- Done", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "55\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	if seenMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", seenMethod)
	}
	if seenPath != "/MyProject/_apis/wit/workitems/12345/comments" {
		t.Fatalf("path = %q, want work item comments endpoint", seenPath)
	}
	if seenAPIVersion != "7.0-preview.3" {
		t.Fatalf("api-version = %q, want preview comments API", seenAPIVersion)
	}
	if seenAuth == "" {
		t.Fatal("Authorization header is empty")
	}
	if seenBody["text"] != "- Done" {
		t.Fatalf("request text = %q, want message", seenBody["text"])
	}
}

func TestADOWorkItemCommentCreatesCommentFromMessageFile(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "comment.md")
	if err := os.WriteFile(messagePath, []byte("Done from file\n"), 0o644); err != nil {
		t.Fatalf("writing comment: %v", err)
	}
	var seenBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"id":56,"workItemId":12345}`)
	}))
	t.Cleanup(server.Close)
	runner := workItemCommentHTTPRunner(t, server.Client(), server.URL)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "comment", "12345", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "56\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
	if seenBody["text"] != "Done from file\n" {
		t.Fatalf("request text = %q, want file content", seenBody["text"])
	}
}

func TestADOWorkItemCommentAliasCreatesComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":57,"workItemId":12345}`)
	}))
	t.Cleanup(server.Close)
	runner := workItemCommentHTTPRunner(t, server.Client(), server.URL)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "work-item", "comment", "12345", "--message", "Done"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "57\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
}

func TestADOWorkItemCommentJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":58,"workItemId":12345,"url":"https://dev.azure.com/my-org/MyProject/_workitems/edit/12345#comment-58"}`)
	}))
	t.Cleanup(server.Close)
	runner := workItemCommentHTTPRunner(t, server.Client(), server.URL)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "comment", "12345", "--message", "Done", "--json"}, strings.NewReader(""), &stdout, &stderr)
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
	if payload["workItemId"] != float64(12345) || payload["commentId"] != float64(58) || payload["action"] != "commented" || payload["url"] == "" {
		t.Fatalf("payload = %+v, want work item comment summary", payload)
	}
}

func TestADOWorkItemCommentGlobalUsesGlobalConfigScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":60,"workItemId":12345}`)
	}))
	t.Cleanup(server.Close)
	runner := workItemCommentHTTPRunnerWithScope(t, server.Client(), server.URL, config.GlobalScope)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "comment", "12345", "--message", "Done", "--global", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "60\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
}

func TestADOWorkItemCommentRejectsInvalidArgsBeforeCredentials(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing ID", args: []string{"ado", "comment"}, want: "usage"},
		{name: "non integer ID", args: []string{"ado", "comment", "abc", "--message", "x"}, want: "work item ID"},
		{name: "zero ID", args: []string{"ado", "comment", "0", "--message", "x"}, want: "positive"},
		{name: "negative ID", args: []string{"ado", "comment", "-1", "--message", "x"}, want: "positive"},
		{name: "missing message source", args: []string{"ado", "comment", "12345"}, want: "--message"},
		{name: "conflicting message sources", args: []string{"ado", "comment", "12345", "--message", "x", "--message-file", "comment.md"}, want: "exactly one"},
		{name: "blank inline message", args: []string{"ado", "comment", "12345", "--message", " \t"}, want: "--message cannot be empty"},
		{name: "missing message value", args: []string{"ado", "comment", "12345", "--message"}, want: "--message requires a value"},
		{name: "missing message-file value", args: []string{"ado", "comment", "12345", "--message-file"}, want: "--message-file requires a value"},
		{name: "missing profile value", args: []string{"ado", "comment", "12345", "--message", "x", "--profile"}, want: "--profile requires a value"},
		{name: "unknown flag", args: []string{"ado", "comment", "12345", "--message", "x", "--unknown"}, want: "unknown"},
		{name: "unsupported work item command", args: []string{"ado", "work-item", "update", "12345"}, want: "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := workItemCommentFailFastRunner(t)
			var stdout bytes.Buffer
			err := runner.Run(tt.args, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOWorkItemCommentRejectsEmptyMessageFileBeforeNetwork(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
	}{
		{name: "empty", content: nil},
		{name: "blank", content: []byte(" \n\t")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messagePath := filepath.Join(t.TempDir(), "comment.md")
			if err := os.WriteFile(messagePath, tt.content, 0o644); err != nil {
				t.Fatalf("writing comment: %v", err)
			}
			runner := workItemCommentFailFastRunner(t)
			var stdout bytes.Buffer
			err := runner.Run([]string{"ado", "comment", "12345", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), "--message-file cannot be empty") {
				t.Fatalf("error = %v, want empty message-file", err)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOWorkItemCommentAzureDevOpsFailuresLeaveStdoutEmpty(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "write failure", status: http.StatusUnauthorized, body: "nope", want: "401"},
		{name: "malformed response", status: http.StatusOK, body: `{"id":`, want: "decoding Azure DevOps work item comment"},
		{name: "empty response", status: http.StatusOK, body: ``, want: "decoding Azure DevOps work item comment"},
		{name: "missing comment ID", status: http.StatusOK, body: `{"workItemId":12345}`, want: "missing comment ID"},
		{name: "mismatched work item ID", status: http.StatusOK, body: `{"id":59,"workItemId":999}`, want: "does not match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			}))
			t.Cleanup(server.Close)
			runner := workItemCommentHTTPRunner(t, server.Client(), server.URL)
			var stdout bytes.Buffer

			err := runner.Run([]string{"ado", "comment", "12345", "--message", "Done"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOWorkItemCommentNetworkErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakeWorkItemCommentClient{createErr: errors.New("network down")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "comment", "12345", "--message", "Done"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("error = %v, want network error", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.createCalled != 1 || client.createOpts.WorkItemID != 12345 || client.createOpts.Text != "Done" {
		t.Fatalf("create called/opts = %d/%+v, want work item comment request", client.createCalled, client.createOpts)
	}
}

func workItemCommentHTTPRunner(t *testing.T, httpClient *http.Client, baseURL string) Runner {
	t.Helper()
	return workItemCommentHTTPRunnerWithScope(t, httpClient, baseURL, config.DefaultScope)
}

func workItemCommentHTTPRunnerWithScope(t *testing.T, httpClient *http.Client, baseURL string, expectedScope config.Scope) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" {
				t.Fatalf("LoadConfig args = %q %q", repoRoot, homeDir)
			}
			if requestedProfile != "" && requestedProfile != "company-cloud" {
				t.Fatalf("requested profile = %q, want empty or company-cloud", requestedProfile)
			}
			if scope != expectedScope {
				t.Fatalf("scope = %v, want %v", scope, expectedScope)
			}
			return &config.Loaded{Profile: config.Profile{Name: "company-cloud", PATRef: "shared-ado", BaseURL: baseURL, Project: "MyProject", APIVersion: "7.1"}}, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) { return httpClient, nil },
	}}
}

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
	for _, want := range []string{"fetch", "comment", "work-item", "pr", "login", "logout", "profiles"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func TestADOWorkItemNamespaceHelpListsSupportedOperations(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "work-item", "--help"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{"comment", "Supported operations"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	for _, notWant := range []string{"update", "assign", "state", "relation", "attachment", "delete", "reaction"} {
		if strings.Contains(stderr.String(), notWant) {
			t.Fatalf("stderr = %q, want no unsupported operation %q", stderr.String(), notWant)
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

func TestADOPullRequestCommentCreatesThreadFromInlineMessage(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdThread: &ado.PullRequestThread{ID: 14, Comments: []ado.PullRequestComment{{ID: 1}}}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "14\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if client.fetchPRCalled != 1 || client.fetchedPRID != 42 {
		t.Fatalf("fetch PR called/id = %d/%d, want 1/42", client.fetchPRCalled, client.fetchedPRID)
	}
	if client.threadCreateCalled != 1 || client.threadCreateOpts.RepositoryID != "repo-uuid" || client.threadCreateOpts.PullRequestID != 42 || client.threadCreateOpts.Content != "Looks good" {
		t.Fatalf("thread create called/opts = %d/%+v, want repo/pr/content", client.threadCreateCalled, client.threadCreateOpts)
	}
	if client.commentCalled != 0 || client.threadUpdateCalled != 0 {
		t.Fatalf("calls comment/threadUpdate = %d/%d, want none", client.commentCalled, client.threadUpdateCalled)
	}
}

func TestADOPullRequestCommentCreatesThreadFromMessageFile(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "comment.md")
	if err := os.WriteFile(messagePath, []byte("Looks good\n"), 0o644); err != nil {
		t.Fatalf("writing comment: %v", err)
	}
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdThread: &ado.PullRequestThread{ID: 15, Comments: []ado.PullRequestComment{{ID: 2}}}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "15\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	if client.threadCreateOpts.Content != "Looks good\n" {
		t.Fatalf("thread content = %q, want file content", client.threadCreateOpts.Content)
	}
}

func TestADOPullRequestCommentCreatesInlineThreadFromInlineMessage(t *testing.T) {
	client := &fakePRMaintenanceClient{
		fetched:    &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}},
		iterations: []ado.PullRequestIteration{{ID: 1}, {ID: 3}},
		iterationChanges: []ado.PullRequestIterationChange{{
			ChangeTrackingID: 77,
			ChangeType:       "edit",
			Item:             ado.PullRequestChangedItem{Path: "/src/app.go"},
		}},
		createdThread: &ado.PullRequestThread{ID: 16, Comments: []ado.PullRequestComment{{ID: 4}}},
	}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--file", "src/app.go", "--line", "42", "--message", "Nit"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "16\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if client.iterationsCalled != 1 || client.iterationChangesCalled != 1 {
		t.Fatalf("iterations/changes calls = %d/%d, want 1/1", client.iterationsCalled, client.iterationChangesCalled)
	}
	if client.iterationChangesOpts.RepositoryID != "repo-uuid" || client.iterationChangesOpts.PullRequestID != 42 || client.iterationChangesOpts.IterationID != 3 || client.iterationChangesOpts.CompareTo != 0 {
		t.Fatalf("iteration changes opts = %+v, want latest iteration common-base lookup", client.iterationChangesOpts)
	}
	assertInlineThreadCreateOpts(t, client.threadCreateOpts, "repo-uuid", 42, "Nit", "/src/app.go", 42, 77, 3, 3)
}

func TestADOPullRequestCommentCreatesInlineThreadFromMessageFile(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "comment.md")
	if err := os.WriteFile(messagePath, []byte("Inline note\n"), 0o644); err != nil {
		t.Fatalf("writing comment: %v", err)
	}
	client := &fakePRMaintenanceClient{
		fetched:          &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}},
		iterations:       []ado.PullRequestIteration{{ID: 2}},
		iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 8, ChangeType: "add", Item: ado.PullRequestChangedItem{Path: "/README.md"}}},
		createdThread:    &ado.PullRequestThread{ID: 17, Comments: []ado.PullRequestComment{{ID: 5}}},
	}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--file", "/README.md", "--line", "1", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "17\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	assertInlineThreadCreateOpts(t, client.threadCreateOpts, "repo-uuid", 42, "Inline note\n", "/README.md", 1, 8, 2, 2)
}

func assertInlineThreadCreateOpts(t *testing.T, opts ado.PullRequestThreadCreateOptions, repositoryID string, pullRequestID int, content string, filePath string, line int, changeTrackingID int, firstIteration int, secondIteration int) {
	t.Helper()
	if opts.RepositoryID != repositoryID || opts.PullRequestID != pullRequestID || opts.Content != content {
		t.Fatalf("thread create opts = %+v, want repo/pr/content", opts)
	}
	if opts.ThreadContext == nil {
		t.Fatal("ThreadContext = nil, want inline context")
	}
	if opts.ThreadContext.FilePath != filePath {
		t.Fatalf("FilePath = %q, want %q", opts.ThreadContext.FilePath, filePath)
	}
	if opts.ThreadContext.LeftFileStart != nil || opts.ThreadContext.LeftFileEnd != nil {
		t.Fatalf("left positions = %+v/%+v, want nil", opts.ThreadContext.LeftFileStart, opts.ThreadContext.LeftFileEnd)
	}
	if opts.ThreadContext.RightFileStart == nil || opts.ThreadContext.RightFileEnd == nil {
		t.Fatalf("right positions = %+v/%+v, want populated", opts.ThreadContext.RightFileStart, opts.ThreadContext.RightFileEnd)
	}
	if *opts.ThreadContext.RightFileStart != (ado.FilePosition{Line: line, Offset: 1}) || *opts.ThreadContext.RightFileEnd != (ado.FilePosition{Line: line, Offset: 1}) {
		t.Fatalf("right positions = %+v/%+v, want line %d offset 1", opts.ThreadContext.RightFileStart, opts.ThreadContext.RightFileEnd, line)
	}
	if opts.PullRequestThreadContext == nil || opts.PullRequestThreadContext.ChangeTrackingID != changeTrackingID || opts.PullRequestThreadContext.IterationContext == nil {
		t.Fatalf("PullRequestThreadContext = %+v, want change tracking", opts.PullRequestThreadContext)
	}
	if opts.PullRequestThreadContext.IterationContext.FirstComparingIteration != firstIteration || opts.PullRequestThreadContext.IterationContext.SecondComparingIteration != secondIteration {
		t.Fatalf("IterationContext = %+v, want %d/%d", opts.PullRequestThreadContext.IterationContext, firstIteration, secondIteration)
	}
}

func TestADOPullRequestCommentJSONOutput(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdThread: &ado.PullRequestThread{ID: 14, Comments: []ado.PullRequestComment{{ID: 3}}}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good", "--json"}, strings.NewReader(""), &stdout, &stderr)
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
	if payload["pullRequestId"] != float64(42) || payload["threadId"] != float64(14) || payload["commentId"] != float64(3) || payload["action"] != "commented" {
		t.Fatalf("payload = %+v, want comment summary", payload)
	}
}

func TestADOPullRequestCommentInlineJSONOutput(t *testing.T) {
	client := &fakePRMaintenanceClient{
		fetched:          &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}},
		iterations:       []ado.PullRequestIteration{{ID: 2}},
		iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 9, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
		createdThread:    &ado.PullRequestThread{ID: 18, Comments: []ado.PullRequestComment{{ID: 6}}},
	}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--file", "/src/app.go", "--line", "42", "--message", "Nit", "--json"}, strings.NewReader(""), &stdout, &stderr)
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
	if payload["pullRequestId"] != float64(42) || payload["threadId"] != float64(18) || payload["commentId"] != float64(6) || payload["filePath"] != "/src/app.go" || payload["line"] != float64(42) || payload["action"] != "commented" {
		t.Fatalf("payload = %+v, want inline comment summary", payload)
	}
}

func TestADOPullRequestCommentJSONOmitsMissingInitialCommentID(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdThread: &ado.PullRequestThread{ID: 14}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good", "--json"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if _, ok := payload["commentId"]; ok {
		t.Fatalf("payload = %+v, want omitted commentId", payload)
	}
}

func TestADOPullRequestCommentRejectsInvalidArgsBeforeCredentials(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing PR ID", args: []string{"ado", "pr", "comment"}, want: "usage"},
		{name: "non-positive PR ID", args: []string{"ado", "pr", "comment", "0", "--message", "x"}, want: "pull request ID"},
		{name: "missing message source", args: []string{"ado", "pr", "comment", "42"}, want: "--message"},
		{name: "conflicting message sources", args: []string{"ado", "pr", "comment", "42", "--message", "x", "--message-file", "comment.md"}, want: "exactly one"},
		{name: "empty message", args: []string{"ado", "pr", "comment", "42", "--message", "   "}, want: "--message cannot be empty"},
		{name: "thread is rejected", args: []string{"ado", "pr", "comment", "42", "--thread", "7", "--message", "x"}, want: "unknown"},
		{name: "file requires line", args: []string{"ado", "pr", "comment", "42", "--file", "/main.go", "--message", "x"}, want: "--file and --line"},
		{name: "line requires file", args: []string{"ado", "pr", "comment", "42", "--line", "12", "--message", "x"}, want: "--file and --line"},
		{name: "line must be positive", args: []string{"ado", "pr", "comment", "42", "--file", "/main.go", "--line", "0", "--message", "x"}, want: "--line must be a positive integer"},
		{name: "negative line must be positive", args: []string{"ado", "pr", "comment", "42", "--file", "/main.go", "--line", "-1", "--message", "x"}, want: "--line must be a positive integer"},
		{name: "line must be integer", args: []string{"ado", "pr", "comment", "42", "--file", "/main.go", "--line", "abc", "--message", "x"}, want: "--line must be a positive integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := Runner{deps: Dependencies{PATStore: &fakePATStore{}}}
			var stdout bytes.Buffer
			err := runner.Run(tt.args, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPullRequestCommentInlineTargetErrorsLeaveStdoutEmpty(t *testing.T) {
	tests := []struct {
		name   string
		client *fakePRMaintenanceClient
		want   string
	}{
		{name: "iterations error", client: &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, iterationsErr: errors.New("iterations failed")}, want: "iterations failed"},
		{name: "changes error", client: &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, iterations: []ado.PullRequestIteration{{ID: 1}}, iterationChangesErr: errors.New("changes failed")}, want: "changes failed"},
		{name: "no match", client: &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, iterations: []ado.PullRequestIteration{{ID: 1}}, iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/other.go"}}}}, want: "was not found"},
		{name: "unsupported", client: &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, iterations: []ado.PullRequestIteration{{ID: 1}}, iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "delete", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}}}, want: "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := prThreadTestRunner(t, tt.client)
			var stdout bytes.Buffer
			err := runner.Run([]string{"ado", "pr", "comment", "42", "--file", "/src/app.go", "--line", "42", "--message", "Nit"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if tt.client.threadCreateCalled != 0 {
				t.Fatalf("threadCreateCalled = %d, want 0", tt.client.threadCreateCalled)
			}
		})
	}
}

func TestResolvePullRequestInlineCommentTarget(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		client  *fakePRMaintenanceClient
		want    prInlineCommentTarget
		wantErr string
	}{
		{
			name: "matches normalized latest iteration change",
			file: "src/app.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}, {ID: 4}, {ID: 2}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			want: prInlineCommentTarget{FilePath: "/src/app.go", ChangeTrackingID: 77, FirstComparingIteration: 4, SecondComparingIteration: 4},
		},
		{
			name:    "no iterations",
			file:    "/src/app.go",
			client:  &fakePRMaintenanceClient{},
			wantErr: "no iterations",
		},
		{
			name: "missing file",
			file: "/missing.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "was not found",
		},
		{
			name: "duplicate match",
			file: "/src/app.go",
			client: &fakePRMaintenanceClient{
				iterations: []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{
					{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}},
					{ChangeTrackingID: 78, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}},
				},
			},
			wantErr: "multiple",
		},
		{
			name: "delete unsupported",
			file: "/src/app.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "delete", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "unsupported",
		},
		{
			name: "rename unsupported",
			file: "/src/app.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "rename", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "unsupported",
		},
		{
			name: "missing tracking unsupported",
			file: "/src/app.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "unsupported",
		},
		{
			name: "dot segment path rejected",
			file: "../src/app.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "was not found",
		},
		{
			name: "backslash path rejected",
			file: `src\app.go`,
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "was not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolvePullRequestInlineCommentTarget(context.Background(), tt.client, "repo-uuid", 42, tt.file)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("target = %+v, want %+v", got, tt.want)
			}
			if tt.client.iterationChangesOpts.IterationID != tt.want.SecondComparingIteration || tt.client.iterationChangesOpts.CompareTo != 0 {
				t.Fatalf("iteration changes opts = %+v, want latest/common-base", tt.client.iterationChangesOpts)
			}
		})
	}
}

func TestADOPullRequestCommentRejectsEmptyMessageFileBeforeNetwork(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "empty.md")
	if err := os.WriteFile(messagePath, nil, 0o644); err != nil {
		t.Fatalf("writing comment: %v", err)
	}
	client := &fakePRMaintenanceClient{}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--message-file cannot be empty") {
		t.Fatalf("error = %v, want empty message-file", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 0 || client.threadCreateCalled != 0 {
		t.Fatalf("calls fetch/threadCreate = %d/%d, want none", client.fetchPRCalled, client.threadCreateCalled)
	}
}

func TestADOPullRequestCommentRejectsWhitespaceOnlyMessageFileBeforeNetwork(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "blank.md")
	if err := os.WriteFile(messagePath, []byte(" \n\t"), 0o644); err != nil {
		t.Fatalf("writing comment: %v", err)
	}
	client := &fakePRMaintenanceClient{}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--message-file cannot be empty") {
		t.Fatalf("error = %v, want empty message-file", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 0 || client.threadCreateCalled != 0 {
		t.Fatalf("calls fetch/threadCreate = %d/%d, want none", client.fetchPRCalled, client.threadCreateCalled)
	}
}

func TestADOPullRequestCommentRejectsMissingRepositoryIDBeforeThreadCreate(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "repository ID") {
		t.Fatalf("error = %v, want repository ID", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.threadCreateCalled != 0 {
		t.Fatalf("calls fetch/threadCreate = %d/%d, want fetch only", client.fetchPRCalled, client.threadCreateCalled)
	}
}

func TestADOPullRequestCommentMutationErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, threadCreateErr: errors.New("thread failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "thread failed") {
		t.Fatalf("error = %v, want thread failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPullRequestCommentFetchErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetchPRErr: errors.New("fetch failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("error = %v, want fetch failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.fetchedPRID != 42 || client.threadCreateCalled != 0 || client.commentCalled != 0 || client.threadUpdateCalled != 0 {
		t.Fatalf("calls fetch/id/threadCreate/comment/threadUpdate = %d/%d/%d/%d/%d, want 1/42/0/0/0", client.fetchPRCalled, client.fetchedPRID, client.threadCreateCalled, client.commentCalled, client.threadUpdateCalled)
	}
}

func TestADOPullRequestCommentRejectsMissingThreadIDResponse(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdThread: &ado.PullRequestThread{}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "thread ID") {
		t.Fatalf("error = %v, want thread ID", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPullRequestReplyCreatesCommentFromInlineMessage(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdComment: &ado.PullRequestComment{ID: 8}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "Fixed this", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "8\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if client.fetchPRCalled != 1 || client.fetchedPRID != 42 {
		t.Fatalf("fetch PR called/id = %d/%d, want 1/42", client.fetchPRCalled, client.fetchedPRID)
	}
	if client.commentCalled != 1 || client.commentOpts.RepositoryID != "repo-uuid" || client.commentOpts.PullRequestID != 42 || client.commentOpts.ThreadID != 7 || client.commentOpts.Content != "Fixed this" {
		t.Fatalf("comment called/opts = %d/%+v, want repo/pr/thread/content", client.commentCalled, client.commentOpts)
	}
	if client.threadUpdateCalled != 0 {
		t.Fatalf("threadUpdateCalled = %d, want 0", client.threadUpdateCalled)
	}
}

func TestADOPullRequestReplyCreatesCommentFromMessageFile(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "reply.md")
	if err := os.WriteFile(messagePath, []byte("Done\n"), 0o644); err != nil {
		t.Fatalf("writing reply: %v", err)
	}
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdComment: &ado.PullRequestComment{ID: 9}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "9\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
	if client.commentOpts.Content != "Done\n" {
		t.Fatalf("comment content = %q, want file content", client.commentOpts.Content)
	}
}

func TestADOPullRequestReplyJSONOutput(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdComment: &ado.PullRequestComment{ID: 8}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "Fixed this", "--json"}, strings.NewReader(""), &stdout, &stderr)
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
	if payload["pullRequestId"] != float64(42) || payload["threadId"] != float64(7) || payload["commentId"] != float64(8) || payload["action"] != "replied" {
		t.Fatalf("payload = %+v, want reply summary", payload)
	}
}

func TestADOPullRequestResolveUpdatesThreadFixed(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, updatedThread: &ado.PullRequestThread{ID: 7, Status: "fixed"}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "resolve", "42", "--thread", "7"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "7\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if client.threadUpdateCalled != 1 || client.threadUpdateOpts.RepositoryID != "repo-uuid" || client.threadUpdateOpts.PullRequestID != 42 || client.threadUpdateOpts.ThreadID != 7 || client.threadUpdateOpts.Status != "fixed" {
		t.Fatalf("thread update called/opts = %d/%+v, want fixed", client.threadUpdateCalled, client.threadUpdateOpts)
	}
	if client.commentCalled != 0 {
		t.Fatalf("commentCalled = %d, want 0", client.commentCalled)
	}
}

func TestADOPullRequestReopenUpdatesThreadActive(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, updatedThread: &ado.PullRequestThread{ID: 7, Status: "active"}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reopen", "42", "--thread", "7"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "7\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	if client.threadUpdateOpts.Status != "active" {
		t.Fatalf("thread update status = %q, want active", client.threadUpdateOpts.Status)
	}
}

func TestADOPullRequestThreadStatusJSONOutput(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, updatedThread: &ado.PullRequestThread{ID: 7, Status: "fixed"}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "resolve", "42", "--thread", "7", "--json"}, strings.NewReader(""), &stdout, &stderr)
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
	if payload["pullRequestId"] != float64(42) || payload["threadId"] != float64(7) || payload["status"] != "fixed" || payload["action"] != "resolved" {
		t.Fatalf("payload = %+v, want resolve summary", payload)
	}
}

func TestADOPullRequestReplyRejectsInvalidArgsBeforeCredentials(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing PR ID", args: []string{"ado", "pr", "reply"}, want: "usage"},
		{name: "non-positive PR ID", args: []string{"ado", "pr", "reply", "0", "--thread", "7", "--message", "x"}, want: "pull request ID"},
		{name: "missing thread", args: []string{"ado", "pr", "reply", "42", "--message", "x"}, want: "--thread"},
		{name: "non-positive thread", args: []string{"ado", "pr", "reply", "42", "--thread", "0", "--message", "x"}, want: "thread ID"},
		{name: "missing message source", args: []string{"ado", "pr", "reply", "42", "--thread", "7"}, want: "--message"},
		{name: "conflicting message sources", args: []string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "x", "--message-file", "reply.md"}, want: "exactly one"},
		{name: "empty message", args: []string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "   "}, want: "--message cannot be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := Runner{deps: Dependencies{PATStore: &fakePATStore{}}}
			var stdout bytes.Buffer
			err := runner.Run(tt.args, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPullRequestThreadStatusRejectsInvalidArgsBeforeCredentials(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "resolve missing PR ID", args: []string{"ado", "pr", "resolve"}, want: "usage: adomi ado pr resolve <pull-request-id> --thread <thread-id>"},
		{name: "reopen missing PR ID", args: []string{"ado", "pr", "reopen"}, want: "usage: adomi ado pr reopen <pull-request-id> --thread <thread-id>"},
		{name: "non-positive PR ID", args: []string{"ado", "pr", "resolve", "0", "--thread", "7"}, want: "pull request ID"},
		{name: "missing thread", args: []string{"ado", "pr", "resolve", "42"}, want: "--thread"},
		{name: "non-positive thread", args: []string{"ado", "pr", "resolve", "42", "--thread", "0"}, want: "thread ID"},
		{name: "message is rejected", args: []string{"ado", "pr", "resolve", "42", "--thread", "7", "--message", "x"}, want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := Runner{deps: Dependencies{PATStore: &fakePATStore{}}}
			var stdout bytes.Buffer
			err := runner.Run(tt.args, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPullRequestReplyRejectsEmptyMessageFileBeforeNetwork(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "empty.md")
	if err := os.WriteFile(messagePath, nil, 0o644); err != nil {
		t.Fatalf("writing reply: %v", err)
	}
	client := &fakePRMaintenanceClient{}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--message-file cannot be empty") {
		t.Fatalf("error = %v, want empty message-file", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 0 || client.commentCalled != 0 {
		t.Fatalf("calls fetch/comment = %d/%d, want none", client.fetchPRCalled, client.commentCalled)
	}
}

func TestADOPullRequestReplyRejectsWhitespaceOnlyMessageFileBeforeNetwork(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "blank.md")
	if err := os.WriteFile(messagePath, []byte(" \n\t"), 0o644); err != nil {
		t.Fatalf("writing reply: %v", err)
	}
	client := &fakePRMaintenanceClient{}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--message-file cannot be empty") {
		t.Fatalf("error = %v, want empty message-file", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 0 || client.commentCalled != 0 {
		t.Fatalf("calls fetch/comment = %d/%d, want none", client.fetchPRCalled, client.commentCalled)
	}
}

func TestADOPullRequestReplyRejectsMissingRepositoryIDBeforeComment(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "Fixed"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "repository ID") {
		t.Fatalf("error = %v, want repository ID", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.commentCalled != 0 {
		t.Fatalf("calls fetch/comment = %d/%d, want fetch only", client.fetchPRCalled, client.commentCalled)
	}
}

func TestADOPullRequestResolveRejectsMissingRepositoryIDBeforeThreadUpdate(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "resolve", "42", "--thread", "7"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "repository ID") {
		t.Fatalf("error = %v, want repository ID", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.threadUpdateCalled != 0 {
		t.Fatalf("calls fetch/threadUpdate = %d/%d, want fetch only", client.fetchPRCalled, client.threadUpdateCalled)
	}
}

func TestADOPullRequestReplyMutationErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, commentErr: errors.New("comment failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "Fixed"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "comment failed") {
		t.Fatalf("error = %v, want comment failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPullRequestReplyFetchErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetchPRErr: errors.New("fetch failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "Fixed"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("error = %v, want fetch failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.fetchedPRID != 42 || client.commentCalled != 0 || client.threadUpdateCalled != 0 {
		t.Fatalf("calls fetch/id/comment/threadUpdate = %d/%d/%d/%d, want 1/42/0/0", client.fetchPRCalled, client.fetchedPRID, client.commentCalled, client.threadUpdateCalled)
	}
}

func TestADOPullRequestResolveFetchErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetchPRErr: errors.New("fetch failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "resolve", "42", "--thread", "7"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("error = %v, want fetch failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.fetchedPRID != 42 || client.commentCalled != 0 || client.threadUpdateCalled != 0 {
		t.Fatalf("calls fetch/id/comment/threadUpdate = %d/%d/%d/%d, want 1/42/0/0", client.fetchPRCalled, client.fetchedPRID, client.commentCalled, client.threadUpdateCalled)
	}
}

func TestADOPullRequestResolveMutationErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, threadUpdateErr: errors.New("thread failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "resolve", "42", "--thread", "7"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "thread failed") {
		t.Fatalf("error = %v, want thread failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

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
		FetchPullRequest: func(ctx context.Context, fetcher ado.PullRequestFetcher, id int) (*ado.PullRequestBundle, error) {
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
	for _, want := range []string{"fetch", "ensure", "comment", "reply", "resolve", "reopen", "--file <path> --line <line>", "latest-version right-side inline threads"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
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

func TestADOActionCommandHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "fetch work item",
			args: []string{"ado", "fetch", "--help"},
			want: []string{"Usage:", "adomi ado fetch", "work-item-id", "--profile", "--global", "stdout", "context directory"},
		},
		{
			name: "fetch pull request",
			args: []string{"ado", "pr", "fetch", "--help"},
			want: []string{"Usage:", "adomi ado pr fetch", "pull-request-id", "--profile", "--global", "stdout", "context directory"},
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
			want: []string{"Usage:", "adomi ado login", "--profile", "--pat-ref", "--global", "PAT", "stderr"},
		},
		{
			name: "logout",
			args: []string{"ado", "logout", "--help"},
			want: []string{"Usage:", "adomi ado logout", "--profile", "--pat-ref", "--global", "credential"},
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

type fakeADOClient struct{}

func (fakeADOClient) ListInProgressPipelineRuns(context.Context) ([]ado.PipelineRun, error) {
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

type fakeWorkItemCommentClient struct {
	fakeADOClient
	created      *ado.WorkItemComment
	createErr    error
	createCalled int
	createOpts   ado.WorkItemCommentCreateOptions
}

func (f *fakeWorkItemCommentClient) CreateWorkItemComment(ctx context.Context, opts ado.WorkItemCommentCreateOptions) (*ado.WorkItemComment, error) {
	f.createCalled++
	f.createOpts = opts
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.created != nil {
		return f.created, nil
	}
	return &ado.WorkItemComment{ID: 1, WorkItemID: opts.WorkItemID}, nil
}

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
