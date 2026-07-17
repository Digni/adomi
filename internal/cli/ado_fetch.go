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

func (r Runner) runADOFetch(args []string, stdout io.Writer) error {
	fetchArgs, err := parseFetchArgs(args)
	if err != nil {
		return err
	}

	deps := r.dependencies()
	repoRoot, homeDir, err := resolveLocations(deps)
	if err != nil {
		return err
	}
	scope := config.DefaultScope
	if fetchArgs.global {
		scope = config.GlobalScope
	}
	loaded, err := deps.LoadConfig(repoRoot, homeDir, fetchArgs.profile, scope)
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
	tree, err := deps.FetchTree(ctx, client, fetchArgs.workItemID)
	if err != nil {
		return err
	}
	outputDir, err := deps.ExportContext(ctx, client, ado.ExportOptions{
		RepoRoot:  repoRoot,
		Profile:   profileConfig.Name,
		Project:   profileConfig.Project,
		CreatedAt: deps.Now(),
	}, tree)
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, outputDir)
	return nil
}

type fetchArgs struct {
	workItemID int
	profile    string
	global     bool
}

func parseFetchArgs(args []string) (fetchArgs, error) {
	if len(args) == 0 {
		return fetchArgs{}, fmt.Errorf("usage: adomi ado fetch <work-item-id> [--profile <profile-name>] [--global]")
	}
	workItemID, err := strconv.Atoi(args[0])
	if err != nil || workItemID <= 0 {
		return fetchArgs{}, fmt.Errorf("work item ID must be a positive integer")
	}
	var profile string
	var global bool
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return fetchArgs{}, fmt.Errorf("--profile requires a value")
			}
			profile = args[i+1]
			i++
		case "--global":
			global = true
		default:
			return fetchArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return fetchArgs{workItemID: workItemID, profile: profile, global: global}, nil
}
