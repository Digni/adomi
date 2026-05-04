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
	ado.AttachmentDownloader
	ado.PullRequestFetcher
}

type Dependencies struct {
	PATStore          PATStore
	ReadSecret        func(prompt string, stdin io.Reader, stderr io.Writer) (string, error)
	Getwd             func() (string, error)
	UserHomeDir       func() (string, error)
	FindRepoRoot      func(start string) (string, error)
	LoadConfig        func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error)
	LoadAllConfig     func(repoRoot, homeDir string, scope config.Scope) (*config.Loaded, error)
	RemoteURLs        func(repoRoot string) ([]string, error)
	NewHTTPClient     func(proxyURL string) (*http.Client, error)
	NewADOClient      func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error)
	FetchTree         func(ctx context.Context, fetcher ado.WorkItemFetcher, rootID int) (*ado.WorkItemTree, error)
	ExportContext     func(ctx context.Context, downloader ado.AttachmentDownloader, opts ado.ExportOptions, tree *ado.WorkItemTree) (string, error)
	FetchPullRequest  func(ctx context.Context, fetcher ado.PullRequestFetcher, id int) (*ado.PullRequestBundle, error)
	ExportPullRequest func(opts ado.PullRequestExportOptions, bundle *ado.PullRequestBundle) (string, error)
	Now               func() time.Time
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
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
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
		r.newADOPullRequestCommand(stdout),
		r.newADOLoginCommand(stdin, stderr),
		r.newADOLogoutCommand(),
		r.newADOProfilesCommand(stdout),
		r.newADOConfigAliasCommand(stdout),
	)
	return adoCmd
}

func (r Runner) newADOFetchCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "fetch <work-item-id> [--profile <profile-name>] [--global]",
		Short:              "Fetch Azure DevOps work item context",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return r.runADOFetch(args, stdout)
		},
	}
}

func (r Runner) newADOPullRequestCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "pr <pull-request-id> [--profile <profile-name>] [--global]",
		Short:              "Fetch Azure DevOps pull request comments",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return r.runADOPullRequest(args, stdout)
		},
	}
}

func (r Runner) newADOLoginCommand(stdin io.Reader, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "login (--profile <profile-name> | --pat-ref <ref>) [--global]",
		Short:              "Store an Azure DevOps PAT",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return r.runADOLogin(args, stdin, stderr)
		},
	}
}

func (r Runner) newADOLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:                "logout (--profile <profile-name> | --pat-ref <ref>) [--global]",
		Short:              "Delete an Azure DevOps PAT",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
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
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
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
	if deps.Now == nil {
		deps.Now = defaults.Now
	}
	return deps
}

func defaultDependencies() Dependencies {
	return Dependencies{
		PATStore:      securestore.NewKeyringStore(),
		ReadSecret:    readSecret,
		Getwd:         os.Getwd,
		UserHomeDir:   os.UserHomeDir,
		FindRepoRoot:  workspace.FindRepoRoot,
		LoadConfig:    config.LoadWithScope,
		LoadAllConfig: config.LoadAllWithScope,
		RemoteURLs:    gitRemoteURLs,
		NewHTTPClient: ado.NewHTTPClient,
		NewADOClient: func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			return ado.NewClient(httpClient, cfg)
		},
		FetchTree:         ado.FetchTree,
		ExportContext:     ado.ExportContext,
		FetchPullRequest:  ado.FetchPullRequestBundle,
		ExportPullRequest: ado.ExportPullRequest,
		Now:               time.Now,
	}
}

func gitRemoteURLs(repoRoot string) ([]string, error) {
	output, err := exec.Command("git", "-C", repoRoot, "remote", "-v").Output()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var urls []string
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || seen[fields[1]] {
			continue
		}
		seen[fields[1]] = true
		urls = append(urls, fields[1])
	}
	return urls, nil
}
