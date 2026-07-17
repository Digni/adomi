package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrefersRepoConfigOverHomeConfig(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	writeConfig(t, filepath.Join(repoRoot, ".adomi", "config.yaml"), `
azureDevOps:
  defaultProfile: repo
  profiles:
    repo:
      baseUrl: https://dev.azure.com/repo
      project: RepoProject
`)
	writeConfig(t, filepath.Join(homeDir, ".config", "adomi", "config.yaml"), `
azureDevOps:
  defaultProfile: home
  profiles:
    home:
      baseUrl: https://dev.azure.com/home
      project: HomeProject
`)

	loaded, err := Load(repoRoot, homeDir, "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Profile.Name != "repo" {
		t.Fatalf("profile name = %q, want repo", loaded.Profile.Name)
	}
	if loaded.Profile.Project != "RepoProject" {
		t.Fatalf("project = %q, want RepoProject", loaded.Profile.Project)
	}
}

func TestLoadFallsBackToHomeConfig(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	writeConfig(t, filepath.Join(homeDir, ".config", "adomi", "config.yaml"), `
azureDevOps:
  defaultProfile: home
  profiles:
    home:
      baseUrl: https://dev.azure.com/home
      project: HomeProject
      apiVersion: "7.0"
`)

	loaded, err := Load(repoRoot, homeDir, "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Path != filepath.Join(homeDir, ".config", "adomi", "config.yaml") {
		t.Fatalf("path = %q, want home config path", loaded.Path)
	}
	if loaded.Profile.APIVersion != "7.0" {
		t.Fatalf("api version = %q, want 7.0", loaded.Profile.APIVersion)
	}
}

func TestLoadWithGlobalScopeIgnoresRepoConfig(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	writeConfig(t, filepath.Join(repoRoot, ".adomi", "config.yaml"), `
azureDevOps:
  defaultProfile: repo
  profiles:
    repo:
      baseUrl: https://dev.azure.com/repo
      project: RepoProject
`)
	writeConfig(t, filepath.Join(homeDir, ".config", "adomi", "config.yaml"), `
azureDevOps:
  defaultProfile: home
  profiles:
    home:
      baseUrl: https://dev.azure.com/home
      project: HomeProject
`)

	loaded, err := LoadWithScope(repoRoot, homeDir, "", GlobalScope)
	if err != nil {
		t.Fatalf("LoadWithScope returned error: %v", err)
	}
	if loaded.Profile.Name != "home" {
		t.Fatalf("profile name = %q, want home", loaded.Profile.Name)
	}
	if loaded.Path != filepath.Join(homeDir, ".config", "adomi", "config.yaml") {
		t.Fatalf("path = %q, want global config path", loaded.Path)
	}
}

func TestLoadAllWithGlobalScopeListsOnlyGlobalProfiles(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	writeConfig(t, filepath.Join(repoRoot, ".adomi", "config.yaml"), `
azureDevOps:
  profiles:
    repo:
      baseUrl: https://dev.azure.com/repo
      project: RepoProject
`)
	writeConfig(t, filepath.Join(homeDir, ".config", "adomi", "config.yaml"), `
azureDevOps:
  profiles:
    global:
      baseUrl: https://dev.azure.com/global
      project: GlobalProject
`)

	loaded, err := LoadAllWithScope(repoRoot, homeDir, GlobalScope)
	if err != nil {
		t.Fatalf("LoadAllWithScope returned error: %v", err)
	}
	got := loaded.ProfileNames()
	if strings.Join(got, ",") != "global" {
		t.Fatalf("profile names = %v, want global only", got)
	}
}

func TestLoadAllWithGlobalScopeMissingConfigMentionsOnlyGlobalPath(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()

	_, err := LoadAllWithScope(repoRoot, homeDir, GlobalScope)
	if err == nil {
		t.Fatal("LoadAllWithScope error = nil, want missing config error")
	}
	if strings.Contains(err.Error(), ".adomi/config.yaml") {
		t.Fatalf("error = %q, want no repo config path", err.Error())
	}
	if !strings.Contains(err.Error(), ".config/adomi/config.yaml") {
		t.Fatalf("error = %q, want global config path", err.Error())
	}
}

func TestLoadAllWithUnknownScopeReturnsError(t *testing.T) {
	_, err := LoadAllWithScope(t.TempDir(), t.TempDir(), Scope(99))
	if err == nil {
		t.Fatal("LoadAllWithScope error = nil, want unknown scope error")
	}
	if !strings.Contains(err.Error(), "unknown config scope") {
		t.Fatalf("error = %q, want unknown scope", err.Error())
	}
}

func TestLoadAllReturnsErrorWhenConfigMissing(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()

	_, err := LoadAll(repoRoot, homeDir)
	if err == nil {
		t.Fatal("LoadAll error = nil, want missing config error")
	}
	if !strings.Contains(err.Error(), ".adomi/config.yaml") || !strings.Contains(err.Error(), ".config/adomi/config.yaml") {
		t.Fatalf("error = %q, want both config paths", err.Error())
	}
}

func TestLoadDefaultsAPIVersion(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	writeConfig(t, filepath.Join(repoRoot, ".adomi", "config.yaml"), `
azureDevOps:
  profiles:
    cloud:
      baseUrl: https://dev.azure.com/org
      project: Project
`)

	loaded, err := Load(repoRoot, homeDir, "cloud")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Profile.APIVersion != "7.1" {
		t.Fatalf("api version = %q, want 7.1", loaded.Profile.APIVersion)
	}
}

func TestLoadProfilePATRef(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	writeConfig(t, filepath.Join(repoRoot, ".adomi", "config.yaml"), `
azureDevOps:
  profiles:
    cloud:
      patRef: shared-ado
      baseUrl: https://dev.azure.com/org
      project: Project
`)

	loaded, err := Load(repoRoot, homeDir, "cloud")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Profile.PATRef != "shared-ado" {
		t.Fatalf("PATRef = %q, want shared-ado", loaded.Profile.PATRef)
	}
	if got := loaded.Profile.CredentialRef(); got != "shared-ado" {
		t.Fatalf("CredentialRef = %q, want shared-ado", got)
	}
}

func TestProfileCredentialRefFallsBackToName(t *testing.T) {
	profile := Profile{Name: "company-cloud"}

	if got := profile.CredentialRef(); got != "company-cloud" {
		t.Fatalf("CredentialRef = %q, want profile name", got)
	}
}

func TestProfileCredentialRefTrimsPATRef(t *testing.T) {
	profile := Profile{Name: "company-cloud", PATRef: " shared-ado "}

	if got := profile.CredentialRef(); got != "shared-ado" {
		t.Fatalf("CredentialRef = %q, want trimmed PAT ref", got)
	}
}

func TestLoadSelectsExplicitProfile(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	writeConfig(t, filepath.Join(repoRoot, ".adomi", "config.yaml"), `
azureDevOps:
  defaultProfile: first
  profiles:
    first:
      baseUrl: https://dev.azure.com/first
      project: First
    second:
      baseUrl: https://dev.azure.com/second
      project: Second
      proxy: http://proxy.example:8080
`)

	loaded, err := Load(repoRoot, homeDir, "second")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Profile.Name != "second" {
		t.Fatalf("profile = %q, want second", loaded.Profile.Name)
	}
	if loaded.Profile.Proxy != "http://proxy.example:8080" {
		t.Fatalf("proxy = %q, want configured proxy", loaded.Profile.Proxy)
	}
}

func TestLoadReturnsUsefulValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		profile    string
		want       string
	}{
		{
			name: "missing profile",
			configYAML: `
azureDevOps:
  profiles:
    cloud:
      baseUrl: https://dev.azure.com/org
      project: Project
`,
			profile: "missing",
			want:    `profile "missing"`,
		},
		{
			name: "missing base URL",
			configYAML: `
azureDevOps:
  profiles:
    cloud:
      project: Project
`,
			profile: "cloud",
			want:    "baseUrl",
		},
		{
			name: "missing project",
			configYAML: `
azureDevOps:
  profiles:
    cloud:
      baseUrl: https://dev.azure.com/org
`,
			profile: "cloud",
			want:    "project",
		},
		{
			name: "whitespace PAT ref",
			configYAML: `
azureDevOps:
  profiles:
    cloud:
      patRef: "   "
      baseUrl: https://dev.azure.com/org
      project: Project
`,
			profile: "cloud",
			want:    "patRef",
		},
		{
			name: "control character PAT ref",
			configYAML: `
azureDevOps:
  profiles:
    cloud:
      patRef: "shared\tref"
      baseUrl: https://dev.azure.com/org
      project: Project
`,
			profile: "cloud",
			want:    "patRef",
		},
		{
			name: "too long PAT ref",
			configYAML: `
azureDevOps:
  profiles:
    cloud:
      patRef: ` + strings.Repeat("a", 257) + `
      baseUrl: https://dev.azure.com/org
      project: Project
`,
			profile: "cloud",
			want:    "patRef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			homeDir := t.TempDir()
			writeConfig(t, filepath.Join(repoRoot, ".adomi", "config.yaml"), tt.configYAML)

			_, err := Load(repoRoot, homeDir, tt.profile)
			if err == nil {
				t.Fatal("Load error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestLoadAllListsProfiles(t *testing.T) {
	repoRoot := t.TempDir()
	homeDir := t.TempDir()
	writeConfig(t, filepath.Join(repoRoot, ".adomi", "config.yaml"), `
azureDevOps:
  profiles:
    second:
      baseUrl: https://dev.azure.com/second
      project: Second
    first:
      baseUrl: https://dev.azure.com/first
      project: First
`)

	loaded, err := LoadAll(repoRoot, homeDir)
	if err != nil {
		t.Fatalf("LoadAll returned error: %v", err)
	}
	got := loaded.ProfileNames()
	want := []string{"first", "second"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("profile names = %v, want %v", got, want)
	}
}

func writeConfig(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}
