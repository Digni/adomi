package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func (r Runner) runADOWikiFetch(args []string, stdout io.Writer) error {
	wikiArgs, err := parseWikiFetchArgs(args)
	if err != nil {
		return err
	}
	deps := r.dependencies()
	repoRoot, homeDir, err := resolveLocations(deps)
	if err != nil {
		return err
	}
	scope := config.DefaultScope
	if wikiArgs.global {
		scope = config.GlobalScope
	}
	loaded, err := deps.LoadConfig(repoRoot, homeDir, wikiArgs.profile, scope)
	if err != nil {
		return err
	}
	pat, err := deps.PATStore.Get(loaded.Profile.CredentialRef())
	if err != nil {
		return err
	}
	httpClient, err := deps.NewHTTPClient(loaded.Profile.Proxy)
	if err != nil {
		return err
	}
	client, err := deps.NewADOClient(httpClient, ado.ClientConfig{
		BaseURL:    loaded.Profile.BaseURL,
		Project:    loaded.Profile.Project,
		APIVersion: loaded.Profile.APIVersion,
		PAT:        pat,
	})
	if err != nil {
		return err
	}
	wikiContext, err := deps.FetchWikiContext(context.Background(), client, wikiArgs.wikiIdentifier, wikiArgs.pagePath, wikiArgs.recursive)
	if err != nil {
		return err
	}
	outputDir, err := deps.ExportWikiContext(ado.WikiExportOptions{
		RepoRoot:  repoRoot,
		Profile:   loaded.Profile.Name,
		Project:   loaded.Profile.Project,
		CreatedAt: deps.Now(),
	}, wikiContext)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, outputDir)
	return nil
}

type wikiFetchArgs struct {
	wikiIdentifier string
	pagePath       string
	profile        string
	recursive      bool
	global         bool
}

func parseWikiFetchArgs(args []string) (wikiFetchArgs, error) {
	if len(args) == 0 {
		return wikiFetchArgs{}, fmt.Errorf("usage: adomi ado wiki fetch <wiki-id-or-name> --page <absolute-wiki-page-path> [--recursive] [--profile <profile-name>] [--global]")
	}
	if strings.TrimSpace(args[0]) == "" || strings.HasPrefix(args[0], "-") {
		return wikiFetchArgs{}, fmt.Errorf("wiki identifier is required")
	}

	parsed := wikiFetchArgs{wikiIdentifier: args[0]}
	seen := make(map[string]bool, 4)
	for i := 1; i < len(args); i++ {
		flag := args[i]
		switch flag {
		case "--page", "--profile", "--recursive", "--global":
			if seen[flag] {
				return wikiFetchArgs{}, fmt.Errorf("%s cannot be repeated", flag)
			}
			seen[flag] = true
		}

		switch flag {
		case "--page":
			value, err := parseFlagValue("--page", args, i)
			if err != nil || strings.TrimSpace(value) == "" {
				return wikiFetchArgs{}, fmt.Errorf("--page requires a value")
			}
			parsed.pagePath = value
			i++
		case "--profile":
			value, err := parseFlagValue("--profile", args, i)
			if err != nil || strings.TrimSpace(value) == "" {
				return wikiFetchArgs{}, fmt.Errorf("--profile requires a value")
			}
			parsed.profile = value
			i++
		case "--recursive":
			parsed.recursive = true
		case "--global":
			parsed.global = true
		default:
			return wikiFetchArgs{}, fmt.Errorf("unknown argument %q", flag)
		}
	}

	if !seen["--page"] {
		return wikiFetchArgs{}, fmt.Errorf("--page is required")
	}
	if !strings.HasPrefix(parsed.pagePath, "/") {
		return wikiFetchArgs{}, fmt.Errorf("--page must be an absolute wiki page path beginning with /")
	}
	return parsed, nil
}
