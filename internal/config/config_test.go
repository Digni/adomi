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

func TestRenderInitTemplateCommentsEveryLineAndUsesPrefill(t *testing.T) {
	template := RenderInitTemplate(InitTemplateValues{
		ProfileName:  "my-project",
		BaseURL:      "https://dev.azure.com/my-org",
		Organization: "my-org",
		Project:      "MyProject",
	})

	for _, line := range strings.Split(strings.TrimSpace(template), "\n") {
		if !strings.HasPrefix(line, "#") {
			t.Fatalf("template line %q is not commented", line)
		}
	}
	for _, want := range []string{
		"defaultProfile: my-project",
		"baseUrl: https://dev.azure.com/my-org",
		"organization: my-org",
		"project: MyProject",
	} {
		if !strings.Contains(template, want) {
			t.Fatalf("template = %q, want %q", template, want)
		}
	}
}

func TestRenderInitTemplateSanitizesMultilineRemoteValues(t *testing.T) {
	template := RenderInitTemplate(InitTemplateValues{
		ProfileName:  "bad\nprofile",
		BaseURL:      "https://dev.azure.com/my-org\nuncommented: true",
		Organization: "my-org\r\nuncommented: true",
		Project:      "MyProject\nuncommented: true",
	})

	for _, line := range strings.Split(strings.TrimSpace(template), "\n") {
		if !strings.HasPrefix(line, "#") {
			t.Fatalf("template line %q is not commented", line)
		}
	}
	if strings.Contains(template, "\nuncommented") {
		t.Fatalf("template contains uncommented injected line: %q", template)
	}
}

func TestInitTemplateValuesFromAzureDevOpsHTTPSRemote(t *testing.T) {
	values, ok := InitTemplateValuesFromRemote("https://dev.azure.com/my-org/MyProject/_git/adomi")
	if !ok {
		t.Fatal("InitTemplateValuesFromRemote ok = false, want true")
	}
	if values.Organization != "my-org" || values.Project != "MyProject" || values.BaseURL != "https://dev.azure.com/my-org" {
		t.Fatalf("values = %+v, want org/project/base URL", values)
	}
	if values.ProfileName != "MyProject" {
		t.Fatalf("profile = %q, want project name", values.ProfileName)
	}
}

func TestInitTemplateValuesFromAzureDevOpsHTTPSRemoteWithUserInfo(t *testing.T) {
	values, ok := InitTemplateValuesFromRemote("https://my-org@dev.azure.com/my-org/MyProject/_git/adomi")
	if !ok {
		t.Fatal("InitTemplateValuesFromRemote ok = false, want true")
	}
	if values.Organization != "my-org" || values.Project != "MyProject" || values.BaseURL != "https://dev.azure.com/my-org" {
		t.Fatalf("values = %+v, want org/project/base URL", values)
	}
}

func TestInitTemplateValuesFromMixedCaseAzureDevOpsOrgRemote(t *testing.T) {
	values, ok := InitTemplateValuesFromRemote("https://dev.azure.com/My-Org/MyProject/_git/adomi")
	if !ok {
		t.Fatal("InitTemplateValuesFromRemote ok = false, want true")
	}
	if values.Organization != "my-org" || values.BaseURL != "https://dev.azure.com/my-org" {
		t.Fatalf("values = %+v, want normalized org/base URL", values)
	}
}

func TestInitTemplateValuesFromAzureDevOpsSSHRemote(t *testing.T) {
	values, ok := InitTemplateValuesFromRemote("git@ssh.dev.azure.com:v3/my-org/MyProject/adomi")
	if !ok {
		t.Fatal("InitTemplateValuesFromRemote ok = false, want true")
	}
	if values.Organization != "my-org" || values.Project != "MyProject" || values.BaseURL != "https://dev.azure.com/my-org" {
		t.Fatalf("values = %+v, want org/project/base URL", values)
	}
}

func TestInitTemplateValuesFromMixedCaseAzureDevOpsSSHRemote(t *testing.T) {
	values, ok := InitTemplateValuesFromRemote("git@ssh.dev.azure.com:v3/My-Org/MyProject/adomi")
	if !ok {
		t.Fatal("InitTemplateValuesFromRemote ok = false, want true")
	}
	if values.Organization != "my-org" || values.BaseURL != "https://dev.azure.com/my-org" {
		t.Fatalf("values = %+v, want normalized org/base URL", values)
	}
}

func TestInitTemplateValuesFromAzureDevOpsSSHURLRemote(t *testing.T) {
	values, ok := InitTemplateValuesFromRemote("ssh://git@ssh.dev.azure.com/v3/my-org/MyProject/adomi")
	if !ok {
		t.Fatal("InitTemplateValuesFromRemote ok = false, want true")
	}
	if values.Organization != "my-org" || values.Project != "MyProject" || values.BaseURL != "https://dev.azure.com/my-org" {
		t.Fatalf("values = %+v, want org/project/base URL", values)
	}
}

func TestInitTemplateValuesFromVisualStudioRemote(t *testing.T) {
	values, ok := InitTemplateValuesFromRemote("https://my-org.visualstudio.com/MyProject/_git/adomi")
	if !ok {
		t.Fatal("InitTemplateValuesFromRemote ok = false, want true")
	}
	if values.Organization != "my-org" || values.Project != "MyProject" || values.BaseURL != "https://dev.azure.com/my-org" {
		t.Fatalf("values = %+v, want org/project/base URL", values)
	}
}

func TestInitTemplateValuesFromMixedCaseVisualStudioRemote(t *testing.T) {
	values, ok := InitTemplateValuesFromRemote("https://My-Org.VisualStudio.com/MyProject/_git/adomi")
	if !ok {
		t.Fatal("InitTemplateValuesFromRemote ok = false, want true")
	}
	if values.Organization != "my-org" {
		t.Fatalf("organization = %q, want normalized my-org", values.Organization)
	}
}

func TestAzureDevOpsRemoteInfoFromRemote(t *testing.T) {
	tests := []struct {
		name        string
		remote      string
		wantOrg     string
		wantProject string
		wantRepo    string
		wantBase    string
	}{
		{
			name:        "https dev.azure.com",
			remote:      "https://dev.azure.com/my-org/My%20Project/_git/adomi",
			wantOrg:     "my-org",
			wantProject: "My Project",
			wantRepo:    "adomi",
			wantBase:    "https://dev.azure.com/my-org",
		},
		{
			name:        "https dev.azure.com with user info",
			remote:      "https://my-org@dev.azure.com/my-org/MyProject/_git/adomi",
			wantOrg:     "my-org",
			wantProject: "MyProject",
			wantRepo:    "adomi",
			wantBase:    "https://dev.azure.com/my-org",
		},
		{
			name:        "scp ssh",
			remote:      "git@ssh.dev.azure.com:v3/my-org/MyProject/adomi",
			wantOrg:     "my-org",
			wantProject: "MyProject",
			wantRepo:    "adomi",
			wantBase:    "https://dev.azure.com/my-org",
		},
		{
			name:        "ssh url",
			remote:      "ssh://git@ssh.dev.azure.com/v3/my-org/MyProject/adomi",
			wantOrg:     "my-org",
			wantProject: "MyProject",
			wantRepo:    "adomi",
			wantBase:    "https://dev.azure.com/my-org",
		},
		{
			name:        "visualstudio",
			remote:      "https://My-Org.VisualStudio.com/MyProject/_git/adomi",
			wantOrg:     "my-org",
			wantProject: "MyProject",
			wantRepo:    "adomi",
			wantBase:    "https://dev.azure.com/my-org",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := AzureDevOpsRemoteInfoFromRemote(tt.remote)
			if !ok {
				t.Fatal("AzureDevOpsRemoteInfoFromRemote ok = false, want true")
			}
			if info.Organization != tt.wantOrg || info.Project != tt.wantProject || info.Repository != tt.wantRepo || info.BaseURL != tt.wantBase {
				t.Fatalf("info = %+v, want org=%q project=%q repo=%q base=%q", info, tt.wantOrg, tt.wantProject, tt.wantRepo, tt.wantBase)
			}
		})
	}
}

func TestAzureDevOpsRemoteInfoFromUnsupportedRemote(t *testing.T) {
	_, ok := AzureDevOpsRemoteInfoFromRemote("git@github.com:Digni/adomi.git")
	if ok {
		t.Fatal("AzureDevOpsRemoteInfoFromRemote ok = true, want false")
	}
}

func TestInitTemplateValuesFromNonAzureDevOpsRemote(t *testing.T) {
	_, ok := InitTemplateValuesFromRemote("git@github.com:Digni/adomi.git")
	if ok {
		t.Fatal("InitTemplateValuesFromRemote ok = true, want false")
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

func TestRenderInitTemplateIncludesPATRefComment(t *testing.T) {
	template := RenderInitTemplate(DefaultInitTemplateValues())

	if !strings.Contains(template, "#       patRef:") {
		t.Fatalf("template = %q, want commented patRef line", template)
	}
	for _, line := range strings.Split(strings.TrimSpace(template), "\n") {
		if !strings.HasPrefix(line, "#") {
			t.Fatalf("template line %q is not commented", line)
		}
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
