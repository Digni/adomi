package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentSkillCreatesGlobalDefaultAdomiSkill(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "my-app")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	targetDir := filepath.Join(homeDir, ".agents", "skills", "adomi")
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
	assertAdomiSkillContent(t, filepath.Join(targetDir, "SKILL.md"))
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestAgentSkillAcceptsExplicitGlobalScope(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	targetDir := filepath.Join(homeDir, ".agents", "skills", "adomi")
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
}

func TestAgentSkillCreatesNamedProviderSkill(t *testing.T) {
	providers := []struct {
		name       string
		targetRoot string
	}{
		{name: "codex", targetRoot: ".agents"},
		{name: "opencode", targetRoot: ".agents"},
		{name: "pi", targetRoot: ".agents"},
		{name: "github-copilot", targetRoot: ".agents"},
		{name: "cursor", targetRoot: ".agents"},
		{name: "claude", targetRoot: ".claude"},
	}
	for _, provider := range providers {
		for _, project := range []bool{false, true} {
			scope := "global"
			if project {
				scope = "project"
			}
			t.Run(provider.name+"/"+scope, func(t *testing.T) {
				homeDir := t.TempDir()
				repoRoot := filepath.Join(t.TempDir(), "adomi")
				if err := os.Mkdir(repoRoot, 0o700); err != nil {
					t.Fatal(err)
				}
				runner := agentSkillRunner(t, homeDir, repoRoot)
				args := []string{"agent", "skill", "--provider", provider.name}
				baseDir := homeDir
				if project {
					args = append(args, "--project")
					baseDir = repoRoot
				}
				var stdout, stderr bytes.Buffer

				err := runner.Run(args, strings.NewReader(""), &stdout, &stderr)
				if err != nil {
					t.Fatalf("Run returned error: %v", err)
				}
				targetDir := filepath.Join(baseDir, provider.targetRoot, "skills", "adomi")
				if got := stdout.String(); got != targetDir+"\n" {
					t.Fatalf("stdout = %q, want target path", got)
				}
				if stderr.String() != "" {
					t.Fatalf("stderr = %q, want empty", stderr.String())
				}
				assertAdomiSkillContent(t, filepath.Join(targetDir, "SKILL.md"))
				assertOnlySkillTargetRoot(t, baseDir, provider.targetRoot)
			})
		}
	}
}

func TestAgentSkillCreatesGlobalClaudeSkill(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--claude"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	targetDir := filepath.Join(homeDir, ".claude", "skills", "adomi")
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want quiet compatibility alias", stderr.String())
	}
	assertAdomiSkillContent(t, filepath.Join(targetDir, "SKILL.md"))
}

func TestAgentSkillCreatesProjectDefaultAdomiSkill(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "my-app")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--project"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	targetDir := filepath.Join(repoRoot, ".agents", "skills", "adomi")
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
	assertAdomiSkillContent(t, filepath.Join(targetDir, "SKILL.md"))
}

func TestAgentSkillCreatesProjectClaudeSkill(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "my-app")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--claude", "--project"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	targetDir := filepath.Join(repoRoot, ".claude", "skills", "adomi")
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
	assertAdomiSkillContent(t, filepath.Join(targetDir, "SKILL.md"))
}

func TestAgentSkillHelpDescribesScopesProvidersAndReplacement(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--help"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{
		"--global",
		"--project",
		"--provider",
		"codex",
		"opencode",
		"pi",
		"github-copilot",
		"cursor",
		"claude",
		"~/.agents/skills",
		".claude/skills",
		"--claude",
		"compatibility alias",
		"--force",
		"--yes",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func assertAdomiSkillContent(t *testing.T, path string) {
	t.Helper()
	content := readTestFile(t, path)
	if !strings.HasPrefix(content, "---\nname: adomi\ndescription: ") {
		t.Fatalf("SKILL.md = %q, want required front matter", content)
	}
	for _, want := range requiredAdomiSkillContent() {
		if !strings.Contains(content, want) {
			t.Fatalf("SKILL.md = %q, want %q", content, want)
		}
	}
}

func assertOnlySkillTargetRoot(t *testing.T, baseDir, targetRoot string) {
	t.Helper()
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != targetRoot {
		t.Fatalf("entries under %s = %v, want only %s", baseDir, entries, targetRoot)
	}
}

func requiredAdomiSkillContent() []string {
	return []string{
		"Azure DevOps",
		"ADO work items",
		"pull requests",
		"project context",
		"adomi ado fetch <work-item-id>",
		"adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive]",
		"repository-local wiki context",
		"Only the requested page is fetched by default",
		"does not perform wiki search or download wiki attachments",
		"adomi ado comment <work-item-id> --message-file <path>",
		"adomi ado work-item comment <work-item-id> --message-file <path>",
		"Plain stdout returns the created work item comment ID",
		"No work item field updates, state transitions, relation edits, attachment uploads, comment updates/deletions, or reaction management are supported",
		"work item write permission",
		"adomi ado pr fetch <pull-request-id>",
		"adomi ado pr <pull-request-id>",
		"adomi ado pr ensure --title <title>",
		"adomi ado pr ensure --description-file <path>",
		"adomi ado pr comment <pull-request-id> --message-file <path>",
		"adomi ado pr comment <pull-request-id> --file <path> --line <line> --message-file <path>",
		"latest changed PR file version",
		"adomi ado pr reply <pull-request-id> --thread <thread-id>",
		"adomi ado pr resolve <pull-request-id> --thread <thread-id>",
		"adomi ado pr reopen <pull-request-id> --thread <thread-id>",
		"fetch and inspect PR context",
		"PR/thread write permissions",
		"approve, reject, merge, complete, abandon, set auto-complete, bypass policies, or manage reviewers",
		"thread ID for `comment`",
		"Successful PR maintenance commands print only data to stdout",
		"leave stdout empty",
		"adomi config init",
		"adomi config init --global",
		"adomi ado profiles list",
		"adomi ado login --profile <profile-name>",
		"adomi ado login --pat-ref <ref>",
		"adomi ado logout --profile <profile-name>",
		"adomi ado logout --pat-ref <ref>",
		".adomi/config.yaml",
		"git worktree",
		"main/root repository's `.adomi/config.yaml`",
		"Always include `--profile <profile-name>` when there is no `.adomi/` folder present",
		"adding `--profile <profile-name>` when no `.adomi/` folder is present",
		"folder path structure",
		"Never print, log, echo, commit, or otherwise expose PAT values",
		"adomi ado pipeline list [--profile <profile-name>] [--global]",
		"adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]",
		"exact `inProgress` runs",
		"YAML and classic Build pipelines",
		"best-effort one-shot view, not a transactional snapshot",
		"compact JSON",
		"JSON null",
		"inside a Git repository, including when using `--global`",
		"`vso.build` read scope",
		"HTTPS or HTTP on exact localhost or a direct IPv4/IPv6 loopback address",
		"Loopback HTTP requests bypass configured proxies so credentials remain on-machine",
		"do not poll or wait",
		"stage, job, task, timeline, log, artifact, approval, environment, or deployment details",
		"classic Release deployments",
		"queue, cancel, retry, approve, or otherwise mutate a pipeline",
	}
}

func createExistingAdomiSkill(t *testing.T, targetDir, content string) string {
	t.Helper()
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(targetDir, "SKILL.md")
	if err := os.WriteFile(existingPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return existingPath
}

func agentSkillRunner(t *testing.T, homeDir, repoRoot string) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		Getwd:        func() (string, error) { return filepath.Join(repoRoot, "subdir"), nil },
		UserHomeDir:  func() (string, error) { return homeDir, nil },
		FindRepoRoot: func(string) (string, error) { return repoRoot, nil },
	}}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
