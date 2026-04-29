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
	skillCmd := &cobra.Command{
		Use:   "skill [--claude] <path>",
		Short: "Create an agent skill from a local directory",
		Long: "Create an agent skill from a local directory. By default, skills are installed under ~/.agents/skills. " +
			"Use --claude to install under ~/.claude/skills instead.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return r.runAgentSkill(agentSkillArgs{sourcePath: args[0], claude: claude}, stdout)
		},
	}
	skillCmd.Flags().BoolVar(&claude, "claude", false, "install under ~/.claude/skills instead of the default ~/.agents/skills")
	return skillCmd
}

type agentSkillArgs struct {
	sourcePath string
	claude     bool
}

type skillMetadata struct {
	Name        string
	Description string
}

func (r Runner) runAgentSkill(args agentSkillArgs, stdout io.Writer) error {
	deps := r.dependencies()
	homeDir, err := deps.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	sourceDir, err := resolveSkillSourceDir(args.sourcePath)
	if err != nil {
		return err
	}

	content, name, err := skillContentForSource(sourceDir)
	if err != nil {
		return err
	}
	if !isValidSkillName(name) {
		return fmt.Errorf("skill name %q must be kebab-case", name)
	}

	skillsRoot := filepath.Join(homeDir, ".agents", "skills")
	if args.claude {
		skillsRoot = filepath.Join(homeDir, ".claude", "skills")
	}
	targetDir := filepath.Join(skillsRoot, name)
	if err := writeNewSkill(targetDir, content); err != nil {
		return err
	}
	fmt.Fprintln(stdout, targetDir)
	return nil
}

func resolveSkillSourceDir(sourcePath string) (string, error) {
	if sourcePath == "" {
		return "", fmt.Errorf("source path is required")
	}
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", fmt.Errorf("resolving source path %q: %w", sourcePath, err)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("checking source path %s: %w", absPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("source path %s is not a directory", absPath)
	}
	return absPath, nil
}

func skillContentForSource(sourceDir string) (string, string, error) {
	sourceSkillPath := filepath.Join(sourceDir, "SKILL.md")
	if data, err := os.ReadFile(sourceSkillPath); err == nil {
		content := string(data)
		metadata, ok := parseSkillMetadata(content)
		if ok && metadata.Name != "" {
			if !isValidSkillName(metadata.Name) {
				return "", "", fmt.Errorf("skill name %q must be kebab-case", metadata.Name)
			}
			return content, metadata.Name, nil
		}
	} else if !os.IsNotExist(err) {
		return "", "", fmt.Errorf("reading source skill %s: %w", sourceSkillPath, err)
	}

	name := kebabCase(filepath.Base(sourceDir))
	if !isValidSkillName(name) {
		return "", "", fmt.Errorf("skill name %q must be kebab-case", name)
	}
	return generateSkillContent(name), name, nil
}

func parseSkillMetadata(content string) (skillMetadata, bool) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return skillMetadata{}, false
	}
	remaining := strings.TrimPrefix(content, "---\n")
	end := strings.Index(remaining, "\n---")
	if end < 0 {
		return skillMetadata{}, false
	}
	frontMatter := remaining[:end]
	var metadata skillMetadata
	for _, line := range strings.Split(frontMatter, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, "\"'")
		switch strings.TrimSpace(key) {
		case "name":
			metadata.Name = value
		case "description":
			metadata.Description = value
		}
	}
	return metadata, true
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
