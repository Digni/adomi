package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func (r Runner) newConfigCommand(stdout io.Writer) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage adomi configuration",
	}
	configCmd.AddCommand(r.newConfigInitCommand(stdout))
	return configCmd
}

func (r Runner) newConfigInitCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "init [--global]",
		Short:              "Create an adomi configuration template",
		Long:               configInitHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, configInitHelp)
			}
			return r.runADOConfigInit(args, stdout)
		},
	}
}

func (r Runner) newADOCommand(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	adoCmd := &cobra.Command{
		Use:   "ado",
		Short: "Manage Azure DevOps context and credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return fmt.Errorf("usage: adomi ado <command>")
		},
	}
	adoCmd.AddCommand(
		r.newADOFetchCommand(stdout, stderr),
		r.newADOWorkItemCommentCommand(stdout),
		r.newADOWorkItemCommand(stdout),
		r.newADOPullRequestCommand(stdout, stderr),
		r.newADOWikiCommand(stdout, stderr),
		r.newADOPipelineCommand(stdout),
		r.newADOLoginCommand(stdin, stdout, stderr),
		r.newADOLogoutCommand(stdout, stderr),
		r.newADOProfilesCommand(stdout),
		r.newADOConfigAliasCommand(stdout),
	)
	return adoCmd
}

func (r Runner) newADOPipelineCommand(stdout io.Writer) *cobra.Command {
	pipelineCmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Inspect read-only Azure DevOps pipeline run status",
		Long:  adoPipelineHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return fmt.Errorf("usage: adomi ado pipeline <command>")
		},
	}
	pipelineCmd.AddCommand(r.newADOPipelineListCommand(stdout), r.newADOPipelineGetCommand(stdout))
	return pipelineCmd
}

func (r Runner) newADOPipelineListCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "list [--profile <profile-name>] [--global]",
		Short:              "List in-progress Azure DevOps pipeline runs",
		Long:               adoPipelineHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoPipelineHelp)
			}
			return r.runADOPipelineList(args, stdout)
		},
	}
}

func (r Runner) newADOPipelineGetCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "get <run-id> [--profile <profile-name>] [--global]",
		Short:              "Get one Azure DevOps pipeline run",
		Long:               adoPipelineHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoPipelineHelp)
			}
			return r.runADOPipelineGet(args, stdout)
		},
	}
}

func (r Runner) newADOWikiCommand(stdout, stderr io.Writer) *cobra.Command {
	wikiCmd := &cobra.Command{
		Use:   "wiki",
		Short: "Fetch Azure DevOps wiki context",
		Long:  adoWikiNamespaceHelp,
	}
	wikiCmd.AddCommand(r.newADOWikiFetchCommand(stdout, stderr))
	return wikiCmd
}

func (r Runner) newADOWikiFetchCommand(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive] [--profile <profile-name>] [--global] [--json]",
		Short:              "Fetch Azure DevOps wiki context",
		Long:               adoWikiFetchHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoWikiFetchHelp)
			}
			return r.runADOWikiFetch(args, stdout, stderr)
		},
	}
}

func (r Runner) newADOFetchCommand(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "fetch <work-item-id> [--profile <profile-name>] [--global] [--json]",
		Short:              "Fetch Azure DevOps work item context",
		Long:               adoFetchHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoFetchHelp)
			}
			return r.runADOFetch(args, stdout, stderr)
		},
	}
}

func (r Runner) newADOWorkItemCommentCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "comment <work-item-id> (--message <text> | --message-file <path>) [--profile <profile-name>] [--global] [--json]",
		Short:              "Add a comment to an Azure DevOps work item",
		Long:               adoWorkItemCommentHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoWorkItemCommentHelp)
			}
			return r.runADOWorkItemComment(args, stdout)
		},
	}
}

func (r Runner) newADOWorkItemCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "work-item comment",
		Short:              "Manage Azure DevOps work item maintenance",
		Long:               adoWorkItemNamespaceHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoWorkItemNamespaceHelp)
			}
			return r.runADOWorkItem(args, stdout)
		},
	}
}

func (r Runner) newADOPullRequestCommand(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "pr <pull-request-id>|fetch|ensure|comment|reply|resolve|reopen",
		Short: "Manage Azure DevOps pull request context and maintenance",
		Long: "Manage Azure DevOps pull request context and maintenance. Supported operations: fetch, ensure, comment, reply, resolve, and reopen. " +
			"Use `comment` without file flags for PR-level threads, or with paired `--file <path> --line <line>` for latest-version right-side inline threads. " +
			"The compatibility form `adomi ado pr <pull-request-id>` behaves like `adomi ado pr fetch <pull-request-id>`.",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && isHelpToken(args[0]) {
				_ = cmd.Help()
				return nil
			}
			if helpText, ok := prOperationHelp(args); ok {
				return writeCommandHelp(cmd, helpText)
			}
			return r.runADOPullRequest(args, stdout, stderr)
		},
	}
}

func (r Runner) newADOLoginCommand(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "login (--profile <profile-name> | --pat-ref <ref>) [--global] [--json]",
		Short:              "Store an Azure DevOps PAT",
		Long:               adoLoginHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoLoginHelp)
			}
			return r.runADOLogin(args, stdin, stdout, stderr)
		},
	}
}

func (r Runner) newADOLogoutCommand(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "logout (--profile <profile-name> | --pat-ref <ref>) [--global] [--json]",
		Short:              "Delete an Azure DevOps PAT",
		Long:               adoLogoutHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoLogoutHelp)
			}
			return r.runADOLogout(args, stdout, stderr)
		},
	}
}

func (r Runner) newADOProfilesCommand(stdout io.Writer) *cobra.Command {
	profilesCmd := &cobra.Command{
		Use:   "profiles",
		Short: "Manage Azure DevOps profiles",
	}
	profilesCmd.AddCommand(&cobra.Command{
		Use:                "list [--global]",
		Short:              "List configured Azure DevOps profiles",
		Long:               adoProfilesListHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoProfilesListHelp)
			}
			return r.runADOProfilesList(args, stdout)
		},
	})
	return profilesCmd
}

func (r Runner) newADOConfigAliasCommand(stdout io.Writer) *cobra.Command {
	configCmd := &cobra.Command{
		Use:    "config",
		Hidden: true,
	}
	configCmd.AddCommand(r.newConfigInitCommand(stdout))
	return configCmd
}

func prOperationHelp(args []string) (string, bool) {
	if len(args) == 0 || !isHelpRequest(args[1:]) {
		return "", false
	}
	switch args[0] {
	case "fetch":
		return adoPRFetchHelp, true
	case "ensure":
		return adoPREnsureHelp, true
	case "comment":
		return adoPRCommentHelp, true
	case "reply":
		return adoPRReplyHelp, true
	case "resolve":
		return adoPRResolveHelp, true
	case "reopen":
		return adoPRReopenHelp, true
	default:
		return "", false
	}
}
