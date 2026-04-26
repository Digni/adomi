package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
	"github.com/Digni/adomi/internal/securestore"
	"github.com/Digni/adomi/internal/workspace"
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
}

type Dependencies struct {
	PATStore      PATStore
	ReadSecret    func(prompt string, stdin io.Reader, stderr io.Writer) (string, error)
	Getwd         func() (string, error)
	UserHomeDir   func() (string, error)
	FindRepoRoot  func(start string) (string, error)
	LoadConfig    func(repoRoot, homeDir, requestedProfile string) (*config.Loaded, error)
	LoadAllConfig func(repoRoot, homeDir string) (*config.Loaded, error)
	NewHTTPClient func(proxyURL string) (*http.Client, error)
	NewADOClient  func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error)
	FetchTree     func(ctx context.Context, fetcher ado.WorkItemFetcher, rootID int) (*ado.WorkItemTree, error)
	ExportContext func(ctx context.Context, downloader ado.AttachmentDownloader, opts ado.ExportOptions, tree *ado.WorkItemTree) (string, error)
	Now           func() time.Time
}

func NewRunner() Runner {
	return Runner{deps: defaultDependencies()}
}

func (r Runner) Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: adomi ado <command>")
	}

	switch args[0] {
	case "ado":
		return r.runADO(args[1:], stdin, stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
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
		LoadConfig:    config.Load,
		LoadAllConfig: config.LoadAll,
		NewHTTPClient: ado.NewHTTPClient,
		NewADOClient: func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			return ado.NewClient(httpClient, cfg)
		},
		FetchTree:     ado.FetchTree,
		ExportContext: ado.ExportContext,
		Now:           time.Now,
	}
}
