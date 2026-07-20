package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
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
	FetchTree           func(ctx context.Context, fetcher ado.WorkItemFetcher, rootID int, progress ado.ProgressFunc) (*ado.WorkItemTree, error)
	ExportContext       func(ctx context.Context, downloader ado.AttachmentDownloader, opts ado.ExportOptions, tree *ado.WorkItemTree, progress ado.ProgressFunc) (string, error)
	FetchPullRequest    func(ctx context.Context, fetcher ado.PullRequestFetcher, id int, progress ado.ProgressFunc) (*ado.PullRequestBundle, error)
	ExportPullRequest   func(opts ado.PullRequestExportOptions, bundle *ado.PullRequestBundle) (string, error)
	FetchWikiContext    func(ctx context.Context, fetcher ado.WikiFetcher, wikiIdentifier, pagePath string, recursive bool, progress ado.ProgressFunc) (*ado.WikiContext, error)
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
