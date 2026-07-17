package cli

import (
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
