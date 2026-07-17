package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentSkillDeclinesReplacementWhenUserDoesNotConfirm(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	existingPath := createExistingAdomiSkill(t, filepath.Join(homeDir, ".agents", "skills", "adomi"), "existing")
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill"}, strings.NewReader("n\n"), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run error = nil, want existing target error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Replace it?") {
		t.Fatalf("stderr = %q, want replacement prompt", stderr.String())
	}
	if got := readTestFile(t, existingPath); got != "existing" {
		t.Fatalf("existing SKILL.md = %q, want unchanged", got)
	}
}

func TestAgentSkillReplacesExistingTargetWhenUserConfirms(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(homeDir, ".agents", "skills", "adomi")
	createExistingAdomiSkill(t, targetDir, "existing")
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill"}, strings.NewReader("yes\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
	if !strings.Contains(stderr.String(), "Replace it?") {
		t.Fatalf("stderr = %q, want replacement prompt", stderr.String())
	}
	assertAdomiSkillContent(t, filepath.Join(targetDir, "SKILL.md"))
}

func TestAgentSkillReplacesExistingTargetWhenUserConfirmsShortY(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(homeDir, ".agents", "skills", "adomi")
	createExistingAdomiSkill(t, targetDir, "existing")
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill"}, strings.NewReader("y\n"), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
	assertAdomiSkillContent(t, filepath.Join(targetDir, "SKILL.md"))
}

func TestAgentSkillDeclinesReplacementOnEOF(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	existingPath := createExistingAdomiSkill(t, filepath.Join(homeDir, ".agents", "skills", "adomi"), "existing")
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run error = nil, want existing target error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Replace it?") {
		t.Fatalf("stderr = %q, want replacement prompt", stderr.String())
	}
	if got := readTestFile(t, existingPath); got != "existing" {
		t.Fatalf("existing SKILL.md = %q, want unchanged", got)
	}
}

func TestAgentSkillForceReplacesExistingTargetWithoutPrompt(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(homeDir, ".agents", "skills", "adomi")
	createExistingAdomiSkill(t, targetDir, "existing")
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--force"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertAdomiSkillContent(t, filepath.Join(targetDir, "SKILL.md"))
}

func TestAgentSkillYesReplacesExistingTargetWithoutPrompt(t *testing.T) {
	homeDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "adomi")
	if err := os.Mkdir(repoRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(homeDir, ".agents", "skills", "adomi")
	createExistingAdomiSkill(t, targetDir, "existing")
	runner := agentSkillRunner(t, homeDir, repoRoot)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--yes"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != targetDir+"\n" {
		t.Fatalf("stdout = %q, want target path", got)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertAdomiSkillContent(t, filepath.Join(targetDir, "SKILL.md"))
}
