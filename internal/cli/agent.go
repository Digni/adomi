package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func (r Runner) newAgentCommand(stdout io.Writer) *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agent tooling",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return fmt.Errorf("usage: adomi agent <command>")
		},
	}
	agentCmd.AddCommand(r.newAgentSkillCommand(stdout))
	return agentCmd
}

func (r Runner) newAgentSkillCommand(stdout io.Writer) *cobra.Command {
	var claude bool
	var global bool
	var project bool
	skillCmd := &cobra.Command{
		Use:   "skill [--claude] [--global | --project]",
		Short: "Create an agent skill for this repository",
		Long: "Create an agent skill for this repository. By default, skills are installed globally under ~/.agents/skills. " +
			"Use --project to install under this repository's .agents/skills directory. " +
			"Use --claude to target Claude's .claude/skills directory instead.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if global && project {
				return fmt.Errorf("cannot use --global with --project")
			}
			return r.runAgentSkill(agentSkillArgs{claude: claude, project: project}, stdout)
		},
	}
	skillCmd.Flags().BoolVar(&claude, "claude", false, "install a Claude skill instead of a default shared-agent skill")
	skillCmd.Flags().BoolVar(&global, "global", false, "install under the user-level skills directory (default)")
	skillCmd.Flags().BoolVar(&project, "project", false, "install under this repository's project-level skills directory")
	return skillCmd
}

type agentSkillArgs struct {
	claude  bool
	project bool
}

func (r Runner) runAgentSkill(args agentSkillArgs, stdout io.Writer) error {
	deps := r.dependencies()
	cwd, err := deps.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}
	repoRoot, err := deps.FindRepoRoot(cwd)
	if err != nil {
		return err
	}

	name := kebabCase(filepath.Base(repoRoot))
	if !isValidSkillName(name) {
		return fmt.Errorf("skill name %q must be kebab-case", name)
	}

	skillsRoot, err := agentSkillRoot(deps, repoRoot, args)
	if err != nil {
		return err
	}
	targetDir := filepath.Join(skillsRoot, name)
	if err := writeNewSkill(targetDir, generateSkillContent(name)); err != nil {
		return err
	}
	fmt.Fprintln(stdout, targetDir)
	return nil
}

func agentSkillRoot(deps Dependencies, repoRoot string, args agentSkillArgs) (string, error) {
	baseDir := repoRoot
	if !args.project {
		homeDir, err := deps.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		baseDir = homeDir
	}

	if args.claude {
		return filepath.Join(baseDir, ".claude", "skills"), nil
	}
	return filepath.Join(baseDir, ".agents", "skills"), nil
}

func kebabCase(value string) string {
	var builder strings.Builder
	lastWasSeparator := true
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
			lastWasSeparator = false
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastWasSeparator = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastWasSeparator = false
		default:
			if !lastWasSeparator && builder.Len() > 0 {
				builder.WriteRune('-')
				lastWasSeparator = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func isValidSkillName(name string) bool {
	if name == "" || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func generateSkillContent(name string) string {
	return fmt.Sprintf(`---
name: %s
description: "Use when working with %s."
---

# %s

## Purpose

Describe what this skill helps the agent do.

## When to Use

Use this skill when the task involves %s-specific context or workflows.

## Process

1. Review the current task and relevant project files.
2. Apply the guidance in this skill.
3. Verify the result before reporting completion.
`, name, name, name, name)
}

func writeNewSkill(targetDir, content string) error {
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("skill %s already exists", targetDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking skill %s: %w", targetDir, err)
	}
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return fmt.Errorf("creating skill directory %s: %w", targetDir, err)
	}
	skillPath := filepath.Join(targetDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing skill %s: %w", skillPath, err)
	}
	return nil
}
