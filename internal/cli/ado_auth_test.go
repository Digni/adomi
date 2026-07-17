package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
