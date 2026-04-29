package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentSkillCreatesDefaultSkillFromDirectory(t *testing.T) {
	homeDir := t.TempDir()
	sourceDir := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(sourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := Runner{deps: Dependencies{UserHomeDir: func() (string, error) { return homeDir, nil }}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill", sourceDir}, strings.NewReader(""), &stdout, &stderr)
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

func TestAgentSkillCreatesClaudeSkillFromDirectory(t *testing.T) {
	homeDir := t.TempDir()
	sourceDir := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(sourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := Runner{deps: Dependencies{UserHomeDir: func() (string, error) { return homeDir, nil }}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--claude", sourceDir}, strings.NewReader(""), &stdout, &bytes.Buffer{})
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

func TestAgentSkillRejectsDefaultFlag(t *testing.T) {
	homeDir := t.TempDir()
	sourceDir := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(sourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := Runner{deps: Dependencies{UserHomeDir: func() (string, error) { return homeDir, nil }}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--default", sourceDir}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want unsupported flag error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if _, statErr := os.Stat(filepath.Join(homeDir, ".agents", "skills", "adomi")); !os.IsNotExist(statErr) {
		t.Fatalf("target stat error = %v, want not exist", statErr)
	}
}

func TestAgentSkillRequiresSourcePath(t *testing.T) {
	runner := Runner{deps: Dependencies{UserHomeDir: func() (string, error) { return t.TempDir(), nil }}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--claude"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want missing source path error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestAgentSkillRejectsUnknownFlag(t *testing.T) {
	runner := Runner{deps: Dependencies{UserHomeDir: func() (string, error) { return t.TempDir(), nil }}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--unknown", "."}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want unknown flag error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestAgentSkillRejectsMissingSourceDirectory(t *testing.T) {
	homeDir := t.TempDir()
	runner := Runner{deps: Dependencies{UserHomeDir: func() (string, error) { return homeDir, nil }}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", filepath.Join(t.TempDir(), "missing")}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want missing directory error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestAgentSkillRejectsFileSource(t *testing.T) {
	homeDir := t.TempDir()
	sourceFile := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(sourceFile, []byte("readme"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := Runner{deps: Dependencies{UserHomeDir: func() (string, error) { return homeDir, nil }}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", sourceFile}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want file source error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestAgentSkillRejectsExistingTarget(t *testing.T) {
	homeDir := t.TempDir()
	sourceDir := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(sourceDir, 0o700); err != nil {
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
	runner := Runner{deps: Dependencies{UserHomeDir: func() (string, error) { return homeDir, nil }}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", sourceDir}, strings.NewReader(""), &stdout, &bytes.Buffer{})
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

func TestAgentSkillDerivesKebabCaseNameFromDirectory(t *testing.T) {
	homeDir := t.TempDir()
	sourceDir := filepath.Join(t.TempDir(), "My Agent Skill")
	if err := os.Mkdir(sourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := Runner{deps: Dependencies{UserHomeDir: func() (string, error) { return homeDir, nil }}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", sourceDir}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	targetDir := filepath.Join(homeDir, ".agents", "skills", "my-agent-skill")
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
}

func TestAgentSkillUsesExistingSkillMetadataAndContent(t *testing.T) {
	homeDir := t.TempDir()
	sourceDir := filepath.Join(t.TempDir(), "source")
	if err := os.Mkdir(sourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sourceSkill := "---\nname: custom-skill\ndescription: Custom skill description\n---\n\n# Custom\n\nUse this skill.\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte(sourceSkill), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := Runner{deps: Dependencies{UserHomeDir: func() (string, error) { return homeDir, nil }}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", sourceDir}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	targetDir := filepath.Join(homeDir, ".agents", "skills", "custom-skill")
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
	if got := readTestFile(t, filepath.Join(targetDir, "SKILL.md")); got != sourceSkill {
		t.Fatalf("installed SKILL.md = %q, want source content", got)
	}
}

func TestAgentSkillHelpDescribesDefaultAndClaudeTargets(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--help"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{"~/.agents/skills", "--claude", "<path>"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
