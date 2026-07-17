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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestADOWikiNamespaceAppearsInHelp(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "--help"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "wiki") {
		t.Fatalf("stderr = %q, want wiki namespace", stderr.String())
	}
}

func TestADOWikiNamespaceHelpDescribesFetchContract(t *testing.T) {
	stderr := runHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "wiki", "--help"})
	for _, want := range []string{
		"adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path>",
		"required",
		"--recursive",
		"--profile",
		"--global",
		"Git repository",
		".adomi/context/wikis",
		"attachments",
		"search",
		"stdout",
		"directory path",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
}

func TestADOWikiFetchHelpDescribesSelectionAndOutput(t *testing.T) {
	stderr := runHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "wiki", "fetch", "--help"})
	for _, want := range []string{
		"adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path>",
		"--recursive",
		"--profile",
		"--global",
		"Git repository",
		".adomi/context/wikis",
		"attachments",
		"search",
		"stdout",
		"directory path",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
}

func TestADOWikiRejectsInvalidArgsBeforeDependencies(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing wiki identifier", args: []string{"ado", "wiki", "fetch"}, want: "usage"},
		{name: "blank wiki identifier", args: []string{"ado", "wiki", "fetch", "", "--page", "/Guide"}, want: "wiki identifier"},
		{name: "whitespace wiki identifier", args: []string{"ado", "wiki", "fetch", " \t", "--page", "/Guide"}, want: "wiki identifier"},
		{name: "page flag instead of wiki identifier", args: []string{"ado", "wiki", "fetch", "--page", "/Guide"}, want: "wiki identifier"},
		{name: "missing page", args: []string{"ado", "wiki", "fetch", "Engineering"}, want: "--page is required"},
		{name: "blank page", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", ""}, want: "--page requires a value"},
		{name: "whitespace page", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", " \t"}, want: "--page requires a value"},
		{name: "non absolute page", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", "Guide"}, want: "absolute"},
		{name: "repeated page", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide", "--page", "/Other"}, want: "--page cannot be repeated"},
		{name: "repeated profile", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide", "--profile", "one", "--profile", "two"}, want: "--profile cannot be repeated"},
		{name: "repeated recursive", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide", "--recursive", "--recursive"}, want: "--recursive cannot be repeated"},
		{name: "repeated global", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide", "--global", "--global"}, want: "--global cannot be repeated"},
		{name: "missing page value", args: []string{"ado", "wiki", "fetch", "Engineering", "--page"}, want: "--page requires a value"},
		{name: "flag as page value", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", "--recursive"}, want: "--page requires a value"},
		{name: "missing profile value", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide", "--profile"}, want: "--profile requires a value"},
		{name: "flag as profile value", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide", "--profile", "--global"}, want: "--profile requires a value"},
		{name: "extra positional", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide", "extra"}, want: "unknown argument"},
		{name: "unknown flag", args: []string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide", "--unknown"}, want: "unknown argument"},
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

func TestADOWikiRequiresRepositoryWithGlobalScope(t *testing.T) {
	loadConfigCalled := false
	runner := Runner{deps: Dependencies{
		Getwd: func() (string, error) { return "/outside", nil },
		FindRepoRoot: func(string) (string, error) {
			return "", errors.New("not a repo")
		},
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			loadConfigCalled = true
			return nil, errors.New("should not load config")
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "wiki", "fetch", "Engineering", "--page", "/Guide", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not a repo") {
		t.Fatalf("error = %v, want repository error", err)
	}
	if loadConfigCalled {
		t.Fatal("LoadConfig was called after repository resolution failed")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOWikiParsesSelectionAndPrintsOnlyExportedPath(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		recursive bool
	}{
		{
			name: "single page",
			args: []string{"ado", "wiki", "fetch", "Engineering/Docs", "--page", "/Guide & Setup", "--profile", "company"},
		},
		{
			name:      "recursive subtree",
			args:      []string{"ado", "wiki", "fetch", "Engineering/Docs", "--page", "/Guide & Setup", "--recursive", "--profile", "company"},
			recursive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdAt := time.Date(2026, 7, 13, 10, 30, 0, 0, time.UTC)
			fakeClient := fakeADOClient{}
			wikiContext := &ado.WikiContext{
				Wiki:          &ado.Wiki{ID: "canonical-wiki", Name: "Engineering/Docs"},
				RequestedPath: "/Guide & Setup",
				Recursive:     tt.recursive,
				Pages:         []ado.WikiPage{{Path: "/Guide & Setup", Content: "# Guide\n"}},
			}
			runner := Runner{deps: Dependencies{
				PATStore:     &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
				Getwd:        func() (string, error) { return "/repo/nested", nil },
				FindRepoRoot: func(string) (string, error) { return "/repo", nil },
				UserHomeDir:  func() (string, error) { return "/home/me", nil },
				LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
					if repoRoot != "/repo" || homeDir != "/home/me" || requestedProfile != "company" || scope != config.DefaultScope {
						t.Fatalf("LoadConfig args = %q %q %q %v", repoRoot, homeDir, requestedProfile, scope)
					}
					return &config.Loaded{Profile: config.Profile{
						Name:       "company",
						PATRef:     "shared-ado",
						BaseURL:    "https://dev.azure.com/org",
						Project:    "Project",
						APIVersion: "7.0",
						Proxy:      "http://proxy.example:8080",
					}}, nil
				},
				NewHTTPClient: func(proxyURL string) (*http.Client, error) {
					if proxyURL != "http://proxy.example:8080" {
						t.Fatalf("proxy = %q, want configured proxy", proxyURL)
					}
					return http.DefaultClient, nil
				},
				NewADOClient: func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
					if cfg.BaseURL != "https://dev.azure.com/org" || cfg.Project != "Project" || cfg.APIVersion != "7.0" || cfg.PAT != "secret-pat" {
						t.Fatalf("client config = %+v", cfg)
					}
					return fakeClient, nil
				},
				FetchWikiContext: func(ctx context.Context, fetcher ado.WikiFetcher, wikiIdentifier, pagePath string, recursive bool) (*ado.WikiContext, error) {
					if _, ok := fetcher.(fakeADOClient); !ok {
						t.Fatalf("fetcher = %T, want fakeADOClient", fetcher)
					}
					if wikiIdentifier != "Engineering/Docs" || pagePath != "/Guide & Setup" || recursive != tt.recursive {
						t.Fatalf("fetch selection = %q %q %t", wikiIdentifier, pagePath, recursive)
					}
					return wikiContext, nil
				},
				ExportWikiContext: func(opts ado.WikiExportOptions, gotContext *ado.WikiContext) (string, error) {
					if opts.RepoRoot != "/repo" || opts.Profile != "company" || opts.Project != "Project" || !opts.CreatedAt.Equal(createdAt) {
						t.Fatalf("export options = %+v", opts)
					}
					if gotContext != wikiContext {
						t.Fatal("ExportWikiContext received unexpected context")
					}
					return "/repo/.adomi/context/wikis/canonical-wiki", nil
				},
				Now: func() time.Time { return createdAt },
			}}
			var stdout, stderr bytes.Buffer

			err := runner.Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
			if stdout.String() != "/repo/.adomi/context/wikis/canonical-wiki\n" {
				t.Fatalf("stdout = %q, want exported path only", stdout.String())
			}
			if stderr.String() != "" {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestADOWikiRealWiringExportsRecursiveContext(t *testing.T) {
	repoRoot := t.TempDir()
	subdir := filepath.Join(repoRoot, "nested")
	homeDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("creating nested directory: %v", err)
	}

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret-pat"))
	var requests []string
	pageContent := map[string]string{
		"/Guide & Setup":            "# Guide\n",
		"/Guide & Setup/Alpha":      "# Alpha\n",
		"/Guide & Setup/Alpha/Deep": "# Deep\n",
		"/Guide & Setup/Zeta":       "# Zeta\n",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != wantAuth {
			t.Fatalf("Authorization = %q, want Basic PAT", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("api-version") != "7.0" {
			t.Fatalf("api-version = %q, want configured version", r.URL.Query().Get("api-version"))
		}
		switch r.URL.EscapedPath() {
		case "/MyProject/_apis/wiki/wikis/Engineering%2FDocs":
			requests = append(requests, "resolve")
			fmt.Fprint(w, `{"id":"wiki-id","name":"Engineering/Docs"}`)
		case "/MyProject/_apis/wiki/wikis/wiki-id/pages":
			pagePath := r.URL.Query().Get("path")
			if !strings.Contains(r.URL.RawQuery, "path=%2FGuide+%26+Setup") {
				t.Fatalf("raw query = %q, want encoded page path", r.URL.RawQuery)
			}
			if r.URL.Query().Get("recursionLevel") == "full" {
				if r.URL.Query().Get("includeContent") != "false" {
					t.Fatalf("metadata includeContent = %q, want false", r.URL.Query().Get("includeContent"))
				}
				requests = append(requests, "tree:"+pagePath)
				fmt.Fprint(w, `{"path":"/Guide & Setup","subPages":[{"path":"/Guide & Setup/Zeta"},{"path":"/Guide & Setup/Alpha","subPages":[{"path":"/Guide & Setup/Alpha/Deep"}]}]}`)
				return
			}
			if r.URL.Query().Get("includeContent") != "true" {
				t.Fatalf("page includeContent = %q, want true", r.URL.Query().Get("includeContent"))
			}
			content, ok := pageContent[pagePath]
			if !ok {
				t.Fatalf("unexpected page path %q", pagePath)
			}
			requests = append(requests, "page:"+pagePath)
			fmt.Fprintf(w, `{"path":%q,"content":%q}`, pagePath, content)
		default:
			t.Fatalf("unexpected path %q", r.URL.EscapedPath())
		}
	}))
	t.Cleanup(server.Close)

	writeWikiTestConfig(t, repoRoot, server.URL)
	createdAt := time.Date(2026, 7, 13, 11, 0, 0, 0, time.UTC)
	runner := Runner{deps: Dependencies{
		PATStore:    &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:       func() (string, error) { return subdir, nil },
		UserHomeDir: func() (string, error) { return homeDir, nil },
		Now:         func() time.Time { return createdAt },
	}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "wiki", "fetch", "Engineering/Docs", "--page", "/Guide & Setup", "--recursive"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	wantOutputDir := ado.WikiOutputPath(repoRoot, "wiki-id")
	if stdout.String() != wantOutputDir+"\n" {
		t.Fatalf("stdout = %q, want path plus newline only", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	wantRequests := []string{
		"resolve",
		"tree:/Guide & Setup",
		"page:/Guide & Setup",
		"page:/Guide & Setup/Alpha",
		"page:/Guide & Setup/Alpha/Deep",
		"page:/Guide & Setup/Zeta",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}
	for _, name := range []string{"wiki.json", "pages.json", "index.json"} {
		assertLocalFile(t, filepath.Join(wantOutputDir, name))
	}
	assertFileContains(t, filepath.Join(wantOutputDir, "index.json"), "Engineering/Docs")
	assertFileContains(t, filepath.Join(wantOutputDir, "index.json"), createdAt.Format(time.RFC3339))
	assertFileContains(t, filepath.Join(wantOutputDir, "pages.json"), "Alpha/Deep")
	pageFiles, err := os.ReadDir(filepath.Join(wantOutputDir, "pages"))
	if err != nil {
		t.Fatalf("reading exported Markdown pages: %v", err)
	}
	if len(pageFiles) != len(pageContent) {
		t.Fatalf("Markdown files = %d, want %d", len(pageFiles), len(pageContent))
	}
}

func wikiFakeRunner(t *testing.T, client ADOClient) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:        func() (string, error) { return "/repo/nested", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{
				Name:       "company",
				PATRef:     "shared-ado",
				BaseURL:    "https://dev.azure.com/org",
				Project:    "Project",
				APIVersion: "7.1",
			}}, nil
		},
		NewHTTPClient: func(string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient:  func(*http.Client, ado.ClientConfig) (ADOClient, error) { return client, nil },
	}}
}

func writeWikiTestConfig(t *testing.T, repoRoot, baseURL string) {
	t.Helper()
	configPath := filepath.Join(repoRoot, ".adomi", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("creating config directory: %v", err)
	}
	content := fmt.Sprintf(`
azureDevOps:
  defaultProfile: company
  profiles:
    company:
      patRef: shared-ado
      baseUrl: %s
      project: MyProject
      apiVersion: "7.0"
`, baseURL)
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}

func newWikiTestRepo(t *testing.T) (repoRoot, subdir, homeDir string) {
	t.Helper()
	repoRoot = t.TempDir()
	subdir = filepath.Join(repoRoot, "nested")
	homeDir = t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("creating nested directory: %v", err)
	}
	return repoRoot, subdir, homeDir
}
