package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestADOWikiConfigurationFailureLeavesStdoutEmpty(t *testing.T) {
	runner := Runner{deps: Dependencies{
		Getwd:        func() (string, error) { return "/repo/nested", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" || requestedProfile != "company" || scope != config.GlobalScope {
				t.Fatalf("LoadConfig args = %q %q %q %v", repoRoot, homeDir, requestedProfile, scope)
			}
			return nil, errors.New("config failed")
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide", "--profile", "company", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "config failed") {
		t.Fatalf("error = %v, want config failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOWikiCredentialFailureLeavesStdoutEmpty(t *testing.T) {
	runner := Runner{deps: Dependencies{
		PATStore:     &fakePATStore{},
		Getwd:        func() (string, error) { return "/repo", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "company", PATRef: "shared-ado"}}, nil
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v, want credential failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOWikiHTTPClientFailureLeavesStdoutEmpty(t *testing.T) {
	runner := Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:        func() (string, error) { return "/repo", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "company", PATRef: "shared-ado", Proxy: "http://proxy.example:8080"}}, nil
		},
		NewHTTPClient: func(proxyURL string) (*http.Client, error) {
			if proxyURL != "http://proxy.example:8080" {
				t.Fatalf("proxy = %q, want configured proxy", proxyURL)
			}
			return nil, errors.New("client failed")
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "client failed") {
		t.Fatalf("error = %v, want HTTP client failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOWikiADOClientFailureLeavesStdoutEmpty(t *testing.T) {
	httpClient := &http.Client{}
	runner := Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:        func() (string, error) { return "/repo", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{
				Name:       "company",
				PATRef:     "shared-ado",
				BaseURL:    "https://dev.azure.com/org",
				Project:    "Project",
				APIVersion: "7.0",
			}}, nil
		},
		NewHTTPClient: func(string) (*http.Client, error) { return httpClient, nil },
		NewADOClient: func(gotHTTPClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			if gotHTTPClient != httpClient || cfg.BaseURL != "https://dev.azure.com/org" || cfg.Project != "Project" || cfg.APIVersion != "7.0" || cfg.PAT != "secret-pat" {
				t.Fatalf("NewADOClient args = %p %+v", gotHTTPClient, cfg)
			}
			return nil, errors.New("ADO client failed")
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "ADO client failed") {
		t.Fatalf("error = %v, want ADO client failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOWikiFetchFailureLeavesStdoutEmpty(t *testing.T) {
	runner := wikiFakeRunner(t, fakeADOClient{})
	runner.deps.FetchWikiContext = func(ctx context.Context, fetcher ado.WikiFetcher, wikiIdentifier, pagePath string, recursive bool, _ ado.ProgressFunc) (*ado.WikiContext, error) {
		if wikiIdentifier != "Engineering/Docs" || pagePath != "/Guide & Setup" || !recursive {
			t.Fatalf("fetch selection = %q %q %t", wikiIdentifier, pagePath, recursive)
		}
		return nil, errors.New("fetch failed")
	}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "wiki", "fetch", "Engineering/Docs", "--page", "/Guide & Setup", "--recursive"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("error = %v, want fetch failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOWikiExportFailureLeavesStdoutEmpty(t *testing.T) {
	wikiContext := &ado.WikiContext{
		Wiki:          &ado.Wiki{ID: "canonical-wiki"},
		RequestedPath: "/Guide",
		Pages:         []ado.WikiPage{{Path: "/Guide"}},
	}
	runner := wikiFakeRunner(t, fakeADOClient{})
	runner.deps.FetchWikiContext = func(_ context.Context, _ ado.WikiFetcher, _ string, _ string, _ bool, _ ado.ProgressFunc) (*ado.WikiContext, error) {
		return wikiContext, nil
	}
	runner.deps.ExportWikiContext = func(opts ado.WikiExportOptions, gotContext *ado.WikiContext) (string, error) {
		if gotContext != wikiContext {
			t.Fatal("ExportWikiContext received unexpected context")
		}
		return "", errors.New("export failed")
	}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "export failed") {
		t.Fatalf("error = %v, want export failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOWikiRealWiringPageFailureLeavesNoSuccessOutput(t *testing.T) {
	repoRoot, subdir, homeDir := newWikiTestRepo(t)
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret-pat"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != wantAuth {
			t.Fatalf("Authorization = %q, want Basic PAT", r.Header.Get("Authorization"))
		}
		switch r.URL.EscapedPath() {
		case "/MyProject/_apis/wiki/wikis/Engineering%2FDocs":
			fmt.Fprint(w, `{"id":"wiki-id","name":"Engineering/Docs"}`)
		case "/MyProject/_apis/wiki/wikis/wiki-id/pages":
			pagePath := r.URL.Query().Get("path")
			if r.URL.Query().Get("recursionLevel") == "full" {
				fmt.Fprint(w, `{"path":"/Guide","subPages":[{"path":"/Guide/Child"}]}`)
				return
			}
			if pagePath == "/Guide/Child" {
				http.Error(w, "page service exploded", http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, `{"path":%q,"content":"# Guide"}`, pagePath)
		default:
			t.Fatalf("unexpected path %q", r.URL.EscapedPath())
		}
	}))
	t.Cleanup(server.Close)
	writeWikiTestConfig(t, repoRoot, server.URL)
	runner := Runner{deps: Dependencies{
		PATStore:    &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:       func() (string, error) { return subdir, nil },
		UserHomeDir: func() (string, error) { return homeDir, nil },
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "wiki", "fetch", "Engineering/Docs", "--page", "/Guide", "--recursive"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %v, want page status failure", err)
	}
	if strings.Contains(err.Error(), "secret-pat") {
		t.Fatalf("error = %q, want no PAT leakage", err.Error())
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	indexPath := filepath.Join(ado.WikiOutputPath(repoRoot, "wiki-id"), "index.json")
	if _, statErr := os.Stat(indexPath); !os.IsNotExist(statErr) {
		t.Fatalf("index stat error = %v, want no success index", statErr)
	}
}

func TestADOWikiRedirectLeavesNoSuccessOutputOrSecretLeakage(t *testing.T) {
	repoRoot, subdir, homeDir := newWikiTestRepo(t)
	var signInRequests atomic.Int32
	signIn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signInRequests.Add(1)
		fmt.Fprint(w, "<html>interactive sign-in page</html>")
	}))
	t.Cleanup(signIn.Close)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/MyProject/_apis/wiki/wikis/Engineering%2FDocs":
			fmt.Fprint(w, `{"id":"wiki-id","name":"Engineering/Docs"}`)
		case "/MyProject/_apis/wiki/wikis/wiki-id/pages":
			http.Redirect(w, r, signIn.URL+"/signin?token=redirect-secret", http.StatusFound)
		default:
			t.Fatalf("unexpected path %q", r.URL.EscapedPath())
		}
	}))
	t.Cleanup(server.Close)
	writeWikiTestConfig(t, repoRoot, server.URL)
	runner := Runner{deps: Dependencies{
		PATStore:    &fakePATStore{values: map[string]string{"shared-ado": "invalid-pat"}},
		Getwd:       func() (string, error) { return subdir, nil },
		UserHomeDir: func() (string, error) { return homeDir, nil },
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "wiki", "fetch", "Engineering/Docs", "--page", "/Guide"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
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
	for _, leaked := range []string{"invalid-pat", "decoding", "interactive sign-in page", signIn.URL, "/signin", "redirect-secret"} {
		if strings.Contains(errorText, leaked) {
			t.Errorf("error = %q, want no %q", errorText, leaked)
		}
	}
	indexPath := filepath.Join(ado.WikiOutputPath(repoRoot, "wiki-id"), "index.json")
	if _, statErr := os.Stat(indexPath); !os.IsNotExist(statErr) {
		t.Fatalf("index stat error = %v, want no success index", statErr)
	}
}
