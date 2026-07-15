package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
	"github.com/Digni/adomi/internal/securestore"
	"github.com/Digni/adomi/internal/workspace"
	"github.com/spf13/cobra"
)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	runner := NewRunner()
	return runner.Run(args, stdin, stdout, stderr)
}

type Runner struct {
	deps Dependencies
}

type PATStore interface {
	Get(profile string) (string, error)
	Set(profile, pat string) error
	Delete(profile string) error
}

type ADOClient interface {
	ado.WorkItemFetcher
	ado.WorkItemMaintainer
	ado.AttachmentDownloader
	ado.PullRequestFetcher
	ado.PullRequestMaintainer
	ado.WikiFetcher
	ado.PipelineRunReader
}

type Dependencies struct {
	PATStore            PATStore
	ReadSecret          func(prompt string, stdin io.Reader, stderr io.Writer) (string, error)
	Getwd               func() (string, error)
	UserHomeDir         func() (string, error)
	FindRepoRoot        func(start string) (string, error)
	LoadConfig          func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error)
	LoadAllConfig       func(repoRoot, homeDir string, scope config.Scope) (*config.Loaded, error)
	RemoteURLs          func(repoRoot string) ([]string, error)
	GitRemotes          func(repoRoot string) ([]gitRemote, error)
	CurrentBranch       func(repoRoot string) (string, error)
	RemoteDefaultBranch func(repoRoot, remoteName string) (string, error)
	NewHTTPClient       func(proxyURL string) (*http.Client, error)
	NewADOClient        func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error)
	FetchTree           func(ctx context.Context, fetcher ado.WorkItemFetcher, rootID int) (*ado.WorkItemTree, error)
	ExportContext       func(ctx context.Context, downloader ado.AttachmentDownloader, opts ado.ExportOptions, tree *ado.WorkItemTree) (string, error)
	FetchPullRequest    func(ctx context.Context, fetcher ado.PullRequestFetcher, id int) (*ado.PullRequestBundle, error)
	ExportPullRequest   func(opts ado.PullRequestExportOptions, bundle *ado.PullRequestBundle) (string, error)
	FetchWikiContext    func(ctx context.Context, fetcher ado.WikiFetcher, wikiIdentifier, pagePath string, recursive bool) (*ado.WikiContext, error)
	ExportWikiContext   func(opts ado.WikiExportOptions, wikiContext *ado.WikiContext) (string, error)
	Now                 func() time.Time
}

func NewRunner() Runner {
	return Runner{deps: defaultDependencies()}
}

func (r Runner) Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := r.newRootCommand(stdin, stdout, stderr)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func (r Runner) newRootCommand(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "adomi",
		Short:         "Fetch and manage agent context",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return fmt.Errorf("usage: adomi <command>")
		},
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetIn(stdin)
	// Cobra help and usage belong on stderr; command result data is written via explicit stdout writers.
	rootCmd.SetOut(stderr)
	rootCmd.SetErr(stderr)
	rootCmd.AddCommand(r.newAgentCommand(stdin, stdout, stderr), r.newConfigCommand(stdout), r.newADOCommand(stdin, stdout, stderr))
	return rootCmd
}

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
		r.newADOFetchCommand(stdout),
		r.newADOWorkItemCommentCommand(stdout),
		r.newADOWorkItemCommand(stdout),
		r.newADOPullRequestCommand(stdout),
		r.newADOWikiCommand(stdout),
		r.newADOPipelineCommand(stdout),
		r.newADOLoginCommand(stdin, stderr),
		r.newADOLogoutCommand(),
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

func (r Runner) newADOWikiCommand(stdout io.Writer) *cobra.Command {
	wikiCmd := &cobra.Command{
		Use:   "wiki",
		Short: "Fetch Azure DevOps wiki context",
		Long:  adoWikiNamespaceHelp,
	}
	wikiCmd.AddCommand(r.newADOWikiFetchCommand(stdout))
	return wikiCmd
}

func (r Runner) newADOWikiFetchCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive] [--profile <profile-name>] [--global]",
		Short:              "Fetch Azure DevOps wiki context",
		Long:               adoWikiFetchHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoWikiFetchHelp)
			}
			return r.runADOWikiFetch(args, stdout)
		},
	}
}

func (r Runner) newADOFetchCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "fetch <work-item-id> [--profile <profile-name>] [--global]",
		Short:              "Fetch Azure DevOps work item context",
		Long:               adoFetchHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoFetchHelp)
			}
			return r.runADOFetch(args, stdout)
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

func (r Runner) newADOPullRequestCommand(stdout io.Writer) *cobra.Command {
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
			return r.runADOPullRequest(args, stdout)
		},
	}
}

func (r Runner) newADOLoginCommand(stdin io.Reader, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "login (--profile <profile-name> | --pat-ref <ref>) [--global]",
		Short:              "Store an Azure DevOps PAT",
		Long:               adoLoginHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoLoginHelp)
			}
			return r.runADOLogin(args, stdin, stderr)
		},
	}
}

func (r Runner) newADOLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:                "logout (--profile <profile-name> | --pat-ref <ref>) [--global]",
		Short:              "Delete an Azure DevOps PAT",
		Long:               adoLogoutHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoLogoutHelp)
			}
			return r.runADOLogout(args)
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

func isHelpToken(arg string) bool {
	return arg == "--help" || arg == "-h"
}

func isHelpRequest(args []string) bool {
	for _, arg := range args {
		if isHelpToken(arg) {
			return true
		}
	}
	return false
}

func writeCommandHelp(cmd *cobra.Command, text string) error {
	_, err := fmt.Fprint(cmd.OutOrStderr(), text)
	return err
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

const configInitHelp = `Create an adomi configuration template.

Usage:
  adomi config init [--global]

Flags:
  --global   create the user-level configuration instead of the repository configuration

stdout:
  Prints only the created configuration file path on success.

Compatibility:
  adomi ado config init [--global] is a hidden compatibility alias for this command.
`

const adoFetchHelp = `Fetch Azure DevOps work item context.

Usage:
  adomi ado fetch <work-item-id> [--profile <profile-name>] [--global]

Arguments:
  <work-item-id>   positive Azure DevOps work item ID

Flags:
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration

stdout:
  Prints only the exported work item context directory path on success.
`

const adoPipelineHelp = `Inspect read-only Azure DevOps pipeline run status.

Usage:
  adomi ado pipeline list [--profile <profile-name>] [--global]
  adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]

Scope:
  list requests the exact inProgress runs across YAML and classic Build pipelines.
  Pagination is a best-effort one-shot view, not a transactional snapshot.
  get accepts a decimal Build run ID in the range 1..2147483647.
  Both commands require a Git repository, including with --global; --profile selects a configured profile.
  The PAT needs vso.build read scope. Pipeline endpoints must use HTTPS or loopback HTTP.
  Loopback HTTP requests bypass configured proxies so credentials remain on-machine.

stdout:
  Success is one compact JSON value. list returns an ordered runs array; get returns one run object.
  Every documented key is present, and unavailable result, source, timestamp, or web-link values are null.

Exclusions:
  These commands perform no polling, stage/job/environment detail lookup, mutation, or classic Release inspection.
`

const adoWikiNamespaceHelp = `Fetch Azure DevOps wiki context into the current Git repository.

Usage:
  adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive] [--profile <profile-name>] [--global]

The wiki identifier and --page are required. The page path must be an absolute Azure DevOps wiki path beginning with /.
Use --recursive to include all descendant pages; otherwise only the selected page is fetched.
--profile selects a configured Azure DevOps profile, and --global uses user-level configuration.
A Git repository is always required because output is written below .adomi/context/wikis in that repository.
Markdown links are preserved, but attachments and other linked resources are not downloaded. This command does not perform indexed wiki search.

stdout:
  Prints only the exported wiki context directory path on success.
`

const adoWikiFetchHelp = `Fetch Azure DevOps wiki context into the current Git repository.

Usage:
  adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive] [--profile <profile-name>] [--global]

Arguments:
  <wiki-id-or-name>   required Azure DevOps wiki ID or name

Flags:
  --page <absolute-wiki-page-path>   required absolute wiki page path beginning with /
  --recursive                       include all descendant pages
  --profile <profile-name>          select a configured Azure DevOps profile
  --global                          use user-level configuration

Rules:
  A Git repository is always required because output is written below .adomi/context/wikis in that repository.
  Markdown links are preserved, but attachments and other linked resources are not downloaded.
  This command does not perform indexed wiki search.

stdout:
  Prints only the exported wiki context directory path on success.
`

const adoWorkItemCommentHelp = `Add a comment to an Azure DevOps work item.

Usage:
  adomi ado comment <work-item-id> (--message <text> | --message-file <path>) [--profile <profile-name>] [--global] [--json]

Arguments:
  <work-item-id>   positive Azure DevOps work item ID

Flags:
  --message <text>        inline comment text
  --message-file <path>   file containing comment text
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

Rules:
  Provide exactly one message source: --message or --message-file.
  No broader work item writes are exposed.

stdout:
  Prints only the created comment ID, or one JSON object when --json is used.
`

const adoWorkItemNamespaceHelp = `Manage Azure DevOps work item maintenance. Supported operations: comment.

Usage:
  adomi ado work-item comment <work-item-id> (--message <text> | --message-file <path>) [--profile <profile-name>] [--global] [--json]

Arguments:
  <work-item-id>   positive Azure DevOps work item ID

Flags:
  --message <text>        inline comment text
  --message-file <path>   file containing comment text
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

Rules:
  Provide exactly one message source: --message or --message-file.
  No broader work item writes are exposed.

stdout:
  Prints only the created comment ID, or one JSON object when --json is used.
`

const adoPRFetchHelp = `Fetch Azure DevOps pull request context.

Usage:
  adomi ado pr fetch <pull-request-id> [--profile <profile-name>] [--global]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration

stdout:
  Prints only the exported pull request context directory path on success.
`

const adoPREnsureHelp = `Create or update the active pull request for the current repository branch.

Usage:
  adomi ado pr ensure [--title <title>] [--description-file <path>] [--source <branch>] [--target <branch>] [--repository <name-or-id>] [--profile <profile-name>] [--global] [--json]

Flags:
  --title <title>            set the pull request title; required when creating a new pull request
  --description-file <path>  read the pull request description from a non-empty file
  --source <branch>          override the inferred source branch
  --target <branch>          override the inferred target branch
  --repository <name-or-id>  override the inferred Azure DevOps repository
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

Rules:
  When no active pull request exists, create a new one; otherwise update only fields you provide.
  No PR governance commands are exposed here.

stdout:
  Prints only the pull request ID, or one JSON object when --json is used.
`

const adoPRCommentHelp = `Create a new Azure DevOps pull request comment thread.

Usage:
  adomi ado pr comment <pull-request-id> (--message <text> | --message-file <path>) [--file <path> --line <line>] [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --message <text>        inline comment text
  --message-file <path>   file containing comment text
  --file <path>           changed file path for an inline thread
  --line <line>           positive right-side line number for an inline thread
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

Rules:
  Provide exactly one message source: --message or --message-file.
  Omit --file and --line for a PR-level thread; provide both for an inline thread.

stdout:
  Prints only the created thread ID, or one JSON object when --json is used.
`

const adoPRReplyHelp = `Reply to an existing Azure DevOps pull request thread.

Usage:
  adomi ado pr reply <pull-request-id> --thread <thread-id> (--message <text> | --message-file <path>) [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --thread <thread-id>    positive thread ID to reply to
  --message <text>        inline reply text
  --message-file <path>   file containing reply text
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

Rules:
  Provide exactly one message source: --message or --message-file.

stdout:
  Prints only the created comment ID, or one JSON object when --json is used.
`

const adoPRResolveHelp = `Resolve an Azure DevOps pull request thread as fixed.

Usage:
  adomi ado pr resolve <pull-request-id> --thread <thread-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --thread <thread-id>    positive thread ID to mark fixed
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

stdout:
  Prints only the thread ID, or one JSON object when --json is used.
`

const adoPRReopenHelp = `Reopen an Azure DevOps pull request thread as active.

Usage:
  adomi ado pr reopen <pull-request-id> --thread <thread-id> [--profile <profile-name>] [--global] [--json]

Arguments:
  <pull-request-id>   positive Azure DevOps pull request ID

Flags:
  --thread <thread-id>    positive thread ID to mark active
  --profile <profile-name>   select a configured Azure DevOps profile
  --global                   use user-level configuration
  --json                     print one compact JSON object

stdout:
  Prints only the thread ID, or one JSON object when --json is used.
`

const adoLoginHelp = `Store an Azure DevOps PAT.

Usage:
  adomi ado login (--profile <profile-name> | --pat-ref <ref>) [--global]

Flags:
  --profile <profile-name>   load the credential reference from a configured profile
  --pat-ref <ref>            store the PAT under an explicit credential reference
  --global                   use user-level configuration when resolving --profile

Streams:
  The secret prompt is written to stderr. This command prints no success data to stdout.
`

const adoLogoutHelp = `Delete an Azure DevOps PAT credential.

Usage:
  adomi ado logout (--profile <profile-name> | --pat-ref <ref>) [--global]

Flags:
  --profile <profile-name>   load the credential reference from a configured profile
  --pat-ref <ref>            delete an explicit credential reference
  --global                   use user-level configuration when resolving --profile
`

const adoProfilesListHelp = `List configured Azure DevOps profiles.

Usage:
  adomi ado profiles list [--global]

Flags:
  --global   list user-level profiles instead of repository profiles

stdout:
  Prints one profile name per line in sorted order.
`

func (r Runner) dependencies() Dependencies {
	deps := r.deps
	defaults := defaultDependencies()
	if deps.PATStore == nil {
		deps.PATStore = defaults.PATStore
	}
	if deps.ReadSecret == nil {
		deps.ReadSecret = defaults.ReadSecret
	}
	if deps.Getwd == nil {
		deps.Getwd = defaults.Getwd
	}
	if deps.UserHomeDir == nil {
		deps.UserHomeDir = defaults.UserHomeDir
	}
	if deps.FindRepoRoot == nil {
		deps.FindRepoRoot = defaults.FindRepoRoot
	}
	if deps.LoadConfig == nil {
		deps.LoadConfig = defaults.LoadConfig
	}
	if deps.LoadAllConfig == nil {
		deps.LoadAllConfig = defaults.LoadAllConfig
	}
	if deps.RemoteURLs == nil {
		deps.RemoteURLs = defaults.RemoteURLs
	}
	if deps.GitRemotes == nil {
		deps.GitRemotes = defaults.GitRemotes
	}
	if deps.CurrentBranch == nil {
		deps.CurrentBranch = defaults.CurrentBranch
	}
	if deps.RemoteDefaultBranch == nil {
		deps.RemoteDefaultBranch = defaults.RemoteDefaultBranch
	}
	if deps.NewHTTPClient == nil {
		deps.NewHTTPClient = defaults.NewHTTPClient
	}
	if deps.NewADOClient == nil {
		deps.NewADOClient = defaults.NewADOClient
	}
	if deps.FetchTree == nil {
		deps.FetchTree = defaults.FetchTree
	}
	if deps.ExportContext == nil {
		deps.ExportContext = defaults.ExportContext
	}
	if deps.FetchPullRequest == nil {
		deps.FetchPullRequest = defaults.FetchPullRequest
	}
	if deps.ExportPullRequest == nil {
		deps.ExportPullRequest = defaults.ExportPullRequest
	}
	if deps.FetchWikiContext == nil {
		deps.FetchWikiContext = defaults.FetchWikiContext
	}
	if deps.ExportWikiContext == nil {
		deps.ExportWikiContext = defaults.ExportWikiContext
	}
	if deps.Now == nil {
		deps.Now = defaults.Now
	}
	return deps
}

func defaultDependencies() Dependencies {
	return Dependencies{
		PATStore:            securestore.NewKeyringStore(),
		ReadSecret:          readSecret,
		Getwd:               os.Getwd,
		UserHomeDir:         os.UserHomeDir,
		FindRepoRoot:        workspace.FindRepoRoot,
		LoadConfig:          config.LoadWithScope,
		LoadAllConfig:       config.LoadAllWithScope,
		RemoteURLs:          gitRemoteURLs,
		GitRemotes:          gitRemotes,
		CurrentBranch:       gitCurrentBranch,
		RemoteDefaultBranch: gitRemoteDefaultBranch,
		NewHTTPClient:       ado.NewHTTPClient,
		NewADOClient: func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			return ado.NewClient(httpClient, cfg)
		},
		FetchTree:         ado.FetchTree,
		ExportContext:     ado.ExportContext,
		FetchPullRequest:  ado.FetchPullRequestBundle,
		ExportPullRequest: ado.ExportPullRequest,
		FetchWikiContext:  ado.FetchWikiContext,
		ExportWikiContext: ado.ExportWikiContext,
		Now:               time.Now,
	}
}

type gitRemote struct {
	Name string
	URL  string
}

func gitRemoteURLs(repoRoot string) ([]string, error) {
	remotes, err := gitRemotes(repoRoot)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var urls []string
	for _, remote := range remotes {
		if seen[remote.URL] {
			continue
		}
		seen[remote.URL] = true
		urls = append(urls, remote.URL)
	}
	return urls, nil
}

func gitRemotes(repoRoot string) ([]gitRemote, error) {
	output, err := exec.Command("git", "-C", repoRoot, "remote", "-v").Output()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var remotes []gitRemote
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := fields[0] + "\x00" + fields[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		remotes = append(remotes, gitRemote{Name: fields[0], URL: fields[1]})
	}
	return remotes, nil
}

func gitCurrentBranch(repoRoot string) (string, error) {
	output, err := exec.Command("git", "-C", repoRoot, "branch", "--show-current").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func gitRemoteDefaultBranch(repoRoot, remoteName string) (string, error) {
	output, err := exec.Command("git", "-C", repoRoot, "symbolic-ref", "--quiet", "--short", "refs/remotes/"+remoteName+"/HEAD").Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(output))
	branch = strings.TrimPrefix(branch, remoteName+"/")
	return branch, nil
}
