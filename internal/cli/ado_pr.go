package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func (r Runner) runADOPullRequest(args []string, stdout io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "fetch":
			return r.runADOPullRequestFetch(args[1:], stdout)
		case "ensure":
			return r.runADOPullRequestEnsure(args[1:], stdout)
		case "comment":
			return r.runADOPullRequestComment(args[1:], stdout)
		case "reply":
			return r.runADOPullRequestReply(args[1:], stdout)
		case "resolve":
			return r.runADOPullRequestThreadStatus(args[1:], stdout, "resolve", "fixed", "resolved")
		case "reopen":
			return r.runADOPullRequestThreadStatus(args[1:], stdout, "reopen", "active", "reopened")
		}
	}
	return r.runADOPullRequestFetch(args, stdout)
}

func (r Runner) runADOPullRequestFetch(args []string, stdout io.Writer) error {
	prArgs, err := parsePRArgs(args)
	if err != nil {
		return err
	}

	deps := r.dependencies()
	repoRoot, homeDir, err := resolveLocations(deps)
	if err != nil {
		return err
	}
	scope := config.DefaultScope
	if prArgs.global {
		scope = config.GlobalScope
	}
	loaded, err := deps.LoadConfig(repoRoot, homeDir, prArgs.profile, scope)
	if err != nil {
		return err
	}
	profileConfig := loaded.Profile

	pat, err := deps.PATStore.Get(profileConfig.CredentialRef())
	if err != nil {
		return err
	}
	httpClient, err := deps.NewHTTPClient(profileConfig.Proxy)
	if err != nil {
		return err
	}
	client, err := deps.NewADOClient(httpClient, ado.ClientConfig{
		BaseURL:    profileConfig.BaseURL,
		Project:    profileConfig.Project,
		APIVersion: profileConfig.APIVersion,
		PAT:        pat,
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	bundle, err := deps.FetchPullRequest(ctx, client, prArgs.pullRequestID)
	if err != nil {
		return err
	}
	outputDir, err := deps.ExportPullRequest(ado.PullRequestExportOptions{
		RepoRoot:  repoRoot,
		Profile:   profileConfig.Name,
		Project:   profileConfig.Project,
		CreatedAt: deps.Now(),
	}, bundle)
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, outputDir)
	return nil
}

type prArgs struct {
	pullRequestID int
	profile       string
	global        bool
}

func parsePRArgs(args []string) (prArgs, error) {
	if len(args) == 0 {
		return prArgs{}, fmt.Errorf("usage: adomi ado pr fetch <pull-request-id> [--profile <profile-name>] [--global]")
	}
	pullRequestID, err := strconv.Atoi(args[0])
	if err != nil || pullRequestID <= 0 {
		return prArgs{}, fmt.Errorf("pull request ID must be a positive integer")
	}
	var profile string
	var global bool
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prArgs{}, fmt.Errorf("--profile requires a value")
			}
			profile = args[i+1]
			i++
		case "--global":
			global = true
		default:
			return prArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return prArgs{pullRequestID: pullRequestID, profile: profile, global: global}, nil
}
