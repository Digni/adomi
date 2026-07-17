package config

import (
	"strings"
	"testing"
)

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
