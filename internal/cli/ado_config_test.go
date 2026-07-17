package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
