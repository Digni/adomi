package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentSkillCreatesGlobalDefaultSkillForRepository(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
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
	content := readTestFile(t, filepath.Join(targetDir, "SKILL.md"))
	if !strings.HasPrefix(content, "---\nname: adomi\ndescription: ") {
		t.Fatalf("SKILL.md = %q, want required front matter", content)
	}
	for _, want := range []string{"# adomi", "## Purpose", "## When to Use", "## Process"} {
		if !strings.Contains(content, want) {
			t.Fatalf("SKILL.md = %q, want %q", content, want)
		}
	}
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

func TestAgentSkillCreatesGlobalClaudeSkillForRepository(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--claude"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	targetDir := filepath.Join(homeDir, ".claude", "skills", "adomi")
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "SKILL.md")); err != nil {
		t.Fatalf("stat SKILL.md: %v", err)
	}
}

func TestAgentSkillCreatesProjectDefaultSkillForRepository(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
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
	if _, err := os.Stat(filepath.Join(targetDir, "SKILL.md")); err != nil {
		t.Fatalf("stat SKILL.md: %v", err)
	}
}

func TestAgentSkillCreatesProjectClaudeSkillForRepository(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
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
	if _, err := os.Stat(filepath.Join(targetDir, "SKILL.md")); err != nil {
		t.Fatalf("stat SKILL.md: %v", err)
	}
}

func TestAgentSkillRejectsGlobalAndProjectTogether(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--global", "--project"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want conflicting scope error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestAgentSkillRejectsPositionalArgument(t *testing.T) {
	runner := agentSkillRunner(t, t.TempDir(), t.TempDir())
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "."}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want positional argument error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestAgentSkillRejectsUnknownFlag(t *testing.T) {
	runner := agentSkillRunner(t, t.TempDir(), t.TempDir())
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--unknown"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want unknown flag error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestAgentSkillRejectsExistingTarget(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(homeDir, ".agents", "skills", "adomi")
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(targetDir, "SKILL.md")
	if err := os.WriteFile(existingPath, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want existing target error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := readTestFile(t, existingPath); got != "existing" {
		t.Fatalf("existing SKILL.md = %q, want unchanged", got)
	}
}

func TestAgentSkillDerivesKebabCaseNameFromRepository(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "My Agent Skill")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	targetDir := filepath.Join(homeDir, ".agents", "skills", "my-agent-skill")
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
}

func TestAgentSkillHelpDescribesScopesAndClaudeTarget(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--help"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{"--global", "--project", "~/.agents/skills", "--claude", ".claude/skills"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
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
