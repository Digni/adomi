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
	LoadConfig    func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error)
	LoadAllConfig func(repoRoot, homeDir string, scope config.Scope) (*config.Loaded, error)
	RemoteURLs    func(repoRoot string) ([]string, error)
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
		FetchTree:     ado.FetchTree,
		ExportContext: ado.ExportContext,
		Now:           time.Now,
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
