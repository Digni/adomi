package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentSkillRejectsGlobalAndProjectTogether(t *testing.T) {
	runner := agentSkillRunner(t, t.TempDir(), t.TempDir())
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--global", "--project"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want conflicting scope error")
	}
	if !strings.Contains(err.Error(), "cannot use --global with --project") {
		t.Fatalf("error = %q, want conflicting scope message", err.Error())
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

func TestAgentSkillRejectsUnknownProviderBeforePathResolution(t *testing.T) {
	runner := Runner{deps: Dependencies{
		UserHomeDir: func() (string, error) { return "", errors.New("UserHomeDir should not be called") },
		Getwd:       func() (string, error) { return "", errors.New("Getwd should not be called") },
		FindRepoRoot: func(string) (string, error) {
			return "", errors.New("FindRepoRoot should not be called")
		},
	}}
	for _, provider := range []string{"unknown", "", "Codex"} {
		t.Run(provider, func(t *testing.T) {
			var stdout bytes.Buffer
			err := runner.Run([]string{"agent", "skill", "--provider", provider}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), "unsupported agent skill provider") {
				t.Fatalf("error = %v, want unsupported provider error", err)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestAgentSkillRejectsProviderAndClaudeSelectorsBeforePathResolution(t *testing.T) {
	runner := Runner{deps: Dependencies{
		UserHomeDir: func() (string, error) { return "", errors.New("UserHomeDir should not be called") },
		Getwd:       func() (string, error) { return "", errors.New("Getwd should not be called") },
		FindRepoRoot: func(string) (string, error) {
			return "", errors.New("FindRepoRoot should not be called")
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--provider", "codex", "--claude"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "cannot use --provider with --claude") {
		t.Fatalf("error = %v, want conflicting provider selector error", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestAgentBareCommandShowsError(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"agent"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run error = nil, want usage error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestAgentSkillGlobalDoesNotRequireRepository(t *testing.T) {
	homeDir := t.TempDir()
	runner := Runner{deps: Dependencies{
		UserHomeDir: func() (string, error) { return homeDir, nil },
		Getwd:       func() (string, error) { return "", errors.New("Getwd should not be called") },
		FindRepoRoot: func(string) (string, error) {
			return "", errors.New("FindRepoRoot should not be called")
		},
	}}
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

func TestAgentSkillProjectGetwdError(t *testing.T) {
	runner := Runner{deps: Dependencies{
		Getwd: func() (string, error) { return "", errors.New("cwd failed") },
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--project"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "getting current directory") {
		t.Fatalf("error = %v, want current directory error", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestAgentSkillProjectFindRepoRootError(t *testing.T) {
	runner := Runner{deps: Dependencies{
		Getwd: func() (string, error) { return "/repo/subdir", nil },
		FindRepoRoot: func(start string) (string, error) {
			if start != "/repo/subdir" {
				t.Fatalf("FindRepoRoot start = %q, want cwd", start)
			}
			return "", errors.New("not a repo")
		},
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill", "--project"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not a repo") {
		t.Fatalf("error = %v, want repo error", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestAgentSkillUserHomeDirError(t *testing.T) {
	runner := Runner{deps: Dependencies{
		UserHomeDir: func() (string, error) { return "", errors.New("home failed") },
	}}
	var stdout bytes.Buffer

	err := runner.Run([]string{"agent", "skill"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "getting home directory") {
		t.Fatalf("error = %v, want home directory error", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
