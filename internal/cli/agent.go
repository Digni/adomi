package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	adomiSkillName               = "adomi"
	supportedAgentSkillProviders = "codex, opencode, pi, github-copilot, cursor, claude"
)

func (r Runner) newAgentCommand(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agent tooling",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return fmt.Errorf("usage: adomi agent <command>")
		},
	}
	agentCmd.AddCommand(r.newAgentSkillCommand(stdin, stdout, stderr))
	return agentCmd
}

func (r Runner) newAgentSkillCommand(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var claude bool
	var provider string
	var global bool
	var project bool
	var force bool
	var yes bool
	skillCmd := &cobra.Command{
		Use:   "skill [--provider <name> | --claude] [--global | --project] [--force | --yes]",
		Short: "Create the adomi agent skill",
		Long: "Create the adomi agent skill for Azure DevOps work. By default, skills are installed globally under ~/.agents/skills. " +
			"Use --project to install under this repository's .agents/skills directory. " +
			"Use --provider with one of: " + supportedAgentSkillProviders + ". " +
			"Codex, OpenCode, Pi, GitHub Copilot, and Cursor share the .agents/skills target; Claude uses .claude/skills. " +
			"Use --claude as a compatibility alias for --provider claude. " +
			"Use --force or --yes to replace an existing skill without prompting.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if global && project {
				return fmt.Errorf("cannot use --global with --project")
			}
			return r.runAgentSkill(agentSkillArgs{
				claude:      claude,
				provider:    provider,
				providerSet: cmd.Flags().Changed("provider"),
				global:      global,
				project:     project,
				force:       force || yes,
			}, stdin, stdout, stderr)
		},
	}
	skillCmd.Flags().BoolVar(&claude, "claude", false, "compatibility alias for --provider claude")
	skillCmd.Flags().StringVar(&provider, "provider", "", "coding-agent provider ("+supportedAgentSkillProviders+")")
	skillCmd.Flags().BoolVar(&global, "global", false, "install under the user-level skills directory (default)")
	skillCmd.Flags().BoolVar(&project, "project", false, "install under this repository's project-level skills directory")
	skillCmd.Flags().BoolVar(&force, "force", false, "replace an existing skill without prompting")
	skillCmd.Flags().BoolVar(&yes, "yes", false, "replace an existing skill without prompting")
	return skillCmd
}

type agentSkillArgs struct {
	claude      bool
	provider    string
	providerSet bool
	global      bool
	project     bool
	force       bool
}

func (r Runner) runAgentSkill(args agentSkillArgs, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := validateAgentSkillProvider(args.provider, args.providerSet, args.claude); err != nil {
		return err
	}
	deps := r.dependencies()
	skillsRoot, err := agentSkillRoot(deps, args)
	if err != nil {
		return err
	}
	targetDir := filepath.Join(skillsRoot, adomiSkillName)
	if err := writeSkill(targetDir, generateSkillContent(), stdin, stderr, args.force); err != nil {
		return err
	}
	fmt.Fprintln(stdout, targetDir)
	return nil
}

func validateAgentSkillProvider(provider string, providerSet, claude bool) error {
	if providerSet && claude {
		return fmt.Errorf("cannot use --provider with --claude")
	}
	if !providerSet {
		return nil
	}
	switch provider {
	case "codex", "opencode", "pi", "github-copilot", "cursor", "claude":
		return nil
	default:
		return fmt.Errorf("unsupported agent skill provider %q (supported: %s)", provider, supportedAgentSkillProviders)
	}
}

func agentSkillRoot(deps Dependencies, args agentSkillArgs) (string, error) {
	baseDir, err := agentSkillBaseDir(deps, args)
	if err != nil {
		return "", err
	}
	if args.claude || args.provider == "claude" {
		return filepath.Join(baseDir, ".claude", "skills"), nil
	}
	return filepath.Join(baseDir, ".agents", "skills"), nil
}

func agentSkillBaseDir(deps Dependencies, args agentSkillArgs) (string, error) {
	if args.global || !args.project {
		homeDir, err := deps.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		return homeDir, nil
	}
	cwd, err := deps.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}
	repoRoot, err := deps.FindRepoRoot(cwd)
	if err != nil {
		return "", err
	}
	return repoRoot, nil
}

func generateSkillContent() string {
	return strings.Join([]string{
		"---",
		"name: adomi",
		"description: \"Use when working with Azure DevOps, ADO work items, pull requests, or project context via the adomi CLI.\"",
		"---",
		"",
		"# adomi",
		"",
		"## Purpose",
		"",
		"Use this skill when the user asks you to work with Azure DevOps, ADO, work items, pull requests, backlog items, or project context that can be fetched with the `adomi` CLI.",
		"",
		"`adomi` fetches Azure DevOps context into local files so you can inspect requirements, comments, attachments, wiki pages, and pull request discussions before planning or coding. It can also post narrow work item comments when the user asks you to report a result back to Azure DevOps.",
		"",
		"## When to Use",
		"",
		"Use this skill when:",
		"- The user mentions Azure DevOps, ADO, work items, backlog items, tasks, bugs, features, epics, wikis, or pull requests.",
		"- The user provides an Azure DevOps work item ID or pull request ID.",
		"- You need project context that may already be configured for the current repository.",
		"",
		"## Configuration Discovery",
		"",
		"Before asking the user for Azure DevOps project or profile details:",
		"1. Inspect repository-local configuration such as `.adomi/config.yaml` when present.",
		"2. If the current checkout is a git worktree, also check the main/root repository's `.adomi/config.yaml` to determine the intended profile before asking the user.",
		"3. Consider global configuration created with `adomi config init --global`.",
		"4. Use repository names, remote URLs, and folder path structure as hints for the Azure DevOps organization, project, or profile.",
		"5. Always include `--profile <profile-name>` when there is no `.adomi/` folder present, even if global configuration defines a default profile.",
		"6. If multiple profiles or projects remain plausible, ask the user to choose.",
		"",
		"Useful configuration and credential commands:",
		"- `adomi config init` creates repository-local configuration.",
		"- `adomi config init --global` creates user-level configuration.",
		"- `adomi ado profiles list` lists configured Azure DevOps profile names.",
		"- `adomi ado login --profile <profile-name> [--json]` stores an Azure DevOps PAT for a profile.",
		"- `adomi ado login --pat-ref <ref> [--json]` stores an Azure DevOps PAT under a direct credential reference.",
		"- `adomi ado logout --profile <profile-name> [--json]` or `adomi ado logout --pat-ref <ref> [--json]` removes stored credentials.",
		"",
		"Never print, log, echo, commit, or otherwise expose PAT values or other secrets. Work item comments require Azure DevOps credentials with work item write permission. PR maintenance commands require appropriate PR/thread write permissions.",
		"",
		"## Output Contract",
		"",
		"Without `--json`, fetch commands write only the exported directory path to stdout. `--json` replaces the default path stdout with one compact JSON object describing the result.",
		"Fetch progress and summaries are written to stderr, so shell path capture from stdout remains safe.",
		"Authentication confirmations are written to stderr and default auth stdout is empty. With `--json`, login and logout instead write one stdout object containing `action` and `credentialRef`.",
		"",
		"## Azure DevOps Context Workflow",
		"",
		"### Pipeline runs",
		"",
		"Inspect overall pipeline-run status with:",
		"```bash",
		"adomi ado pipeline list [--profile <profile-name>] [--global]",
		"adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]",
		"```",
		"",
		"`pipeline list` returns the exact `inProgress` runs observed across YAML and classic Build pipelines. Continuation pages form a best-effort one-shot view, not a transactional snapshot, so runs that change during pagination may be omitted.",
		"`pipeline get` accepts a decimal Build run ID from 1 through 2147483647 and returns that run's current overall status and terminal result when available. Both commands emit compact JSON with run and pipeline identity, source revision, timestamps, and web link; unavailable result, source, timestamp, and link values are JSON null.",
		"Run both commands inside a Git repository, including when using `--global`; use `--profile <profile-name>` to select a configured profile. The PAT needs the `vso.build` read scope, and the configured endpoint must use HTTPS or HTTP on exact localhost or a direct IPv4/IPv6 loopback address. Loopback HTTP requests bypass configured proxies so credentials remain on-machine.",
		"These commands do not poll or wait, fetch stage, job, task, timeline, log, artifact, approval, environment, or deployment details, inspect classic Release deployments, or queue, cancel, retry, approve, or otherwise mutate a pipeline.",
		"",
		"### Work items",
		"",
		"Run:",
		"```bash",
		"adomi ado fetch <work-item-id> [--json]",
		"```",
		"",
		"Optional flags:",
		"- `--profile <profile-name>` selects a configured profile.",
		"- `--global` uses global configuration.",
		"- `--json` writes `path`, `workItems`, and `attachments` instead of the plain path.",
		"",
		"Without `--json`, the command prints only the exported context directory path to stdout. Read the files in that directory before making implementation decisions. The export can include work item JSON, HTML, tree/index files, attachments, parent chain, and direct children.",
		"",
		"Comment on a work item only when the user explicitly asks you to report back to Azure DevOps:",
		"```bash",
		"adomi ado comment <work-item-id> --message-file <path>",
		"adomi ado work-item comment <work-item-id> --message-file <path>",
		"```",
		"",
		"Use exactly one message source: `--message <text>` for short explicit comments or `--message-file <path>` for prepared content. Plain stdout returns the created work item comment ID; `--json` returns one compact JSON object.",
		"No work item field updates, state transitions, relation edits, attachment uploads, comment updates/deletions, or reaction management are supported.",
		"",
		"### Wikis",
		"",
		"Fetch repository-local wiki context with:",
		"```bash",
		"adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive] [--json]",
		"```",
		"",
		"The command exports wiki Markdown and metadata below `.adomi/context/wikis/`. Without `--json`, stdout is only the exported path; `--json` writes `path` and `pages`.",
		"Only the requested page is fetched by default. Add `--recursive` to include all descendant pages below the requested path.",
		"Wiki fetch does not perform wiki search or download wiki attachments; remote links in Markdown remain unchanged.",
		"",
		"### Pull requests",
		"",
		"Fetch pull request context with the canonical command:",
		"```bash",
		"adomi ado pr fetch <pull-request-id> [--json]",
		"```",
		"",
		"The compatibility alias `adomi ado pr <pull-request-id>` also fetches pull request context.",
		"",
		"Optional flags:",
		"- `--profile <profile-name>` selects a configured profile.",
		"- `--global` uses global configuration.",
		"- `--json` writes `path`, `threadCount`, and `commentCount` instead of the plain path.",
		"",
		"Without `--json`, the fetch command prints only the exported pull request context directory path to stdout. Read the exported pull request and comment thread context before responding to review feedback or making changes.",
		"",
		"### Pull request maintenance",
		"",
		"Use conservative PR maintenance commands only when the user asks you to create/update your branch PR or maintain explicit review threads:",
		"- `adomi ado pr ensure --title <title>` creates the active pull request for the current repository branch when none exists, or updates title/description fields you explicitly provide on the existing active PR.",
		"- `adomi ado pr ensure --description-file <path>` updates an existing PR description from a non-empty file; include `--target <branch>` or `--repository <name-or-id>` when inference is ambiguous.",
		"- `adomi ado pr comment <pull-request-id> --message-file <path>` creates a new PR-level comment thread; `--message <text>` is available for short explicit comments. Plain stdout returns the created thread ID.",
		"- `adomi ado pr comment <pull-request-id> --file <path> --line <line> --message-file <path>` creates a right-side single-line inline comment on the latest changed PR file version. Use `--message <text>` for short explicit inline comments. Deleted/left-side comments, ranges, offsets, suggestions, and manual iteration overrides are not supported.",
		"- `adomi ado pr reply <pull-request-id> --thread <thread-id> --message-file <path>` replies to an existing review thread; `--message <text>` is available for short explicit replies.",
		"- `adomi ado pr resolve <pull-request-id> --thread <thread-id>` marks a thread fixed.",
		"- `adomi ado pr reopen <pull-request-id> --thread <thread-id>` marks a thread active again.",
		"",
		"Before creating, replying to, or resolving/reopening review threads, fetch and inspect PR context with `adomi ado pr fetch <pull-request-id>` unless the user has already provided the relevant thread details.",
		"PR maintenance runs from the current Git repository, infers the Azure DevOps repository/source branch/target branch from git remotes and branch metadata where possible, and uses explicit flags such as `--repository`, `--source`, and `--target` to resolve ambiguity.",
		"Successful PR maintenance commands print only data to stdout: PR ID for `ensure`, thread ID for `comment`, comment ID for `reply`, thread ID for `resolve`/`reopen`, or one compact JSON object when `--json` is used. Validation, prompts, diagnostics, and Azure DevOps errors belong on stderr and leave stdout empty.",
		"Safety boundary: do not use `adomi` to approve, reject, merge, complete, abandon, set auto-complete, bypass policies, or manage reviewers; those PR governance actions are out of scope.",
		"",
		"## Process",
		"",
		"1. Identify whether the user is asking about a pipeline run, work item, wiki, pull request, PR maintenance, or general Azure DevOps context.",
		"2. Resolve the most likely profile/project from repo config, global config, remotes, and folder path hints.",
		"3. Run the appropriate `adomi ado pipeline list`, `adomi ado pipeline get`, `adomi ado fetch`, `adomi ado wiki fetch`, `adomi ado pr fetch`, `adomi ado comment`, or conservative PR maintenance command, adding `--profile <profile-name>` when no `.adomi/` folder is present.",
		"4. Read exported context before planning, editing code, replying to review threads, or resolving/reopening threads unless the user already provided the relevant thread details.",
		"5. Ask the user only when required IDs, profile, project, repository/branch inference, or credential setup remain ambiguous.",
		"",
	}, "\n")
}

func writeSkill(targetDir, content string, stdin io.Reader, stderr io.Writer, force bool) error {
	if _, err := os.Stat(targetDir); err == nil {
		if !force {
			confirmed, err := confirmReplaceSkill(targetDir, stdin, stderr)
			if err != nil {
				return err
			}
			if !confirmed {
				return fmt.Errorf("skill %s already exists", targetDir)
			}
		}
		return replaceSkill(targetDir, content)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking skill %s: %w", targetDir, err)
	}
	return createSkill(targetDir, content)
}

func confirmReplaceSkill(targetDir string, stdin io.Reader, stderr io.Writer) (bool, error) {
	fmt.Fprintf(stderr, "Skill %s already exists. Replace it? [y/N]: ", targetDir)
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("reading confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func createSkill(targetDir, content string) error {
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return fmt.Errorf("creating skill directory %s: %w", targetDir, err)
	}
	skillPath := filepath.Join(targetDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing skill %s: %w", skillPath, err)
	}
	return nil
}

func replaceSkill(targetDir, content string) error {
	backupDir, err := backupSkillDir(targetDir)
	if err != nil {
		return err
	}
	if err := os.Rename(targetDir, backupDir); err != nil {
		return fmt.Errorf("backing up skill %s: %w", targetDir, err)
	}
	if err := createSkill(targetDir, content); err != nil {
		if removeErr := os.RemoveAll(targetDir); removeErr != nil {
			return fmt.Errorf("%w; removing partial replacement %s: %v; previous skill backup remains at %s", err, targetDir, removeErr, backupDir)
		}
		if restoreErr := os.Rename(backupDir, targetDir); restoreErr != nil {
			return fmt.Errorf("%w; restoring previous skill from %s: %v", err, backupDir, restoreErr)
		}
		return err
	}
	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("removing skill backup %s: %w", backupDir, err)
	}
	return nil
}

func backupSkillDir(targetDir string) (string, error) {
	parent := filepath.Dir(targetDir)
	base := filepath.Base(targetDir)
	backupDir, err := os.MkdirTemp(parent, "."+base+".backup.")
	if err != nil {
		return "", fmt.Errorf("creating skill backup path: %w", err)
	}
	if err := os.Remove(backupDir); err != nil {
		return "", fmt.Errorf("preparing skill backup path %s: %w", backupDir, err)
	}
	return backupDir, nil
}
