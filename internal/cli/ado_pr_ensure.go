package cli

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func (r Runner) runADOPullRequestEnsure(args []string, stdout io.Writer) error {
	ensureArgs, err := parsePREnsureArgs(args)
	if err != nil {
		return err
	}

	description, descriptionProvided, err := readOptionalNonEmptyFile("--description-file", ensureArgs.description)
	if err != nil {
		return err
	}

	deps := r.dependencies()
	repoRoot, homeDir, err := resolveLocations(deps)
	if err != nil {
		return err
	}
	scope := config.DefaultScope
	if ensureArgs.global {
		scope = config.GlobalScope
	}
	loaded, err := deps.LoadConfig(repoRoot, homeDir, ensureArgs.profile, scope)
	if err != nil {
		return err
	}
	profileConfig := loaded.Profile

	remotes, err := deps.GitRemotes(repoRoot)
	if err != nil {
		return fmt.Errorf("getting git remotes: %w", err)
	}
	selectedRepo, err := selectPRRepository(remotes, profileConfig, ensureArgs.repository)
	if err != nil {
		return err
	}
	refs, err := resolvePREnsureRefs(deps, repoRoot, selectedRepo.RemoteName, ensureArgs)
	if err != nil {
		return err
	}

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
	matches, err := client.ListPullRequests(ctx, ado.PullRequestListOptions{
		RepositoryID:  selectedRepo.Repository,
		SourceRefName: refs.SourceRef,
		TargetRefName: refs.TargetRef,
		Status:        "active",
	})
	if err != nil {
		return err
	}

	result := prEnsureResult{
		Action:        "unchanged",
		Repository:    selectedRepo.Repository,
		SourceRefName: refs.SourceRef,
		TargetRefName: refs.TargetRef,
	}
	switch len(matches) {
	case 0:
		if strings.TrimSpace(ensureArgs.title) == "" {
			return fmt.Errorf("--title is required when creating a pull request")
		}
		created, err := client.CreatePullRequest(ctx, ado.PullRequestCreateOptions{
			RepositoryID:  selectedRepo.Repository,
			SourceRefName: refs.SourceRef,
			TargetRefName: refs.TargetRef,
			Title:         ensureArgs.title,
			Description:   description,
		})
		if err != nil {
			return err
		}
		result.Action = "created"
		result.PullRequestID = created.ID
		result.URL = created.URL
	case 1:
		pr := matches[0]
		result.PullRequestID = pr.ID
		result.URL = pr.URL
		var update ado.PullRequestUpdateOptions
		update.RepositoryID = selectedRepo.Repository
		update.PullRequestID = pr.ID
		if ensureArgs.title != "" {
			update.Title = &ensureArgs.title
		}
		if descriptionProvided {
			update.Description = &description
		}
		if update.Title != nil || update.Description != nil {
			updated, err := client.UpdatePullRequest(ctx, update)
			if err != nil {
				return err
			}
			result.Action = "updated"
			result.PullRequestID = updated.ID
			result.URL = updated.URL
		}
	default:
		return fmt.Errorf("multiple active pull requests match source %s and target %s", refs.SourceRef, refs.TargetRef)
	}

	if ensureArgs.json {
		return writeJSONLine(stdout, result)
	}
	fmt.Fprintln(stdout, result.PullRequestID)
	return nil
}

type prEnsureArgs struct {
	source      string
	target      string
	repository  string
	title       string
	description string
	profile     string
	global      bool
	json        bool
}

type prRepositorySelection struct {
	RemoteName string
	Repository string
}

type prRefs struct {
	SourceRef string
	TargetRef string
}

type prEnsureResult struct {
	PullRequestID int    `json:"pullRequestId"`
	Action        string `json:"action"`
	Repository    string `json:"repository"`
	SourceRefName string `json:"sourceRefName"`
	TargetRefName string `json:"targetRefName"`
	URL           string `json:"url,omitempty"`
}

func selectPRRepository(remotes []gitRemote, profile config.Profile, override string) (prRepositorySelection, error) {
	override = strings.TrimSpace(override)
	if override != "" {
		return prRepositorySelection{Repository: override}, nil
	}
	var matches []prRepositorySelection
	for _, remote := range remotes {
		info, ok := config.AzureDevOpsRemoteInfoFromRemote(remote.URL)
		if !ok || !remoteMatchesProfile(info, profile) {
			continue
		}
		matches = append(matches, prRepositorySelection{RemoteName: remote.Name, Repository: info.Repository})
	}
	if len(matches) == 0 {
		return prRepositorySelection{}, fmt.Errorf("could not infer Azure DevOps repository from git remotes")
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	var originMatches []prRepositorySelection
	for _, match := range matches {
		if match.RemoteName == "origin" {
			originMatches = append(originMatches, match)
		}
	}
	if len(originMatches) == 1 {
		return originMatches[0], nil
	}
	return prRepositorySelection{}, fmt.Errorf("ambiguous Azure DevOps repository; use --repository")
}

func remoteMatchesProfile(info config.AzureDevOpsRemoteInfo, profile config.Profile) bool {
	profileProject, err := url.PathUnescape(strings.TrimSpace(profile.Project))
	if err != nil {
		profileProject = strings.TrimSpace(profile.Project)
	}
	if !strings.EqualFold(info.Project, profileProject) {
		return false
	}
	profileBase, err := url.Parse(strings.TrimRight(profile.BaseURL, "/"))
	if err != nil || profileBase.Host == "" {
		return false
	}
	remoteBase, err := url.Parse(info.BaseURL)
	if err != nil || remoteBase.Host == "" {
		return false
	}
	return strings.EqualFold(profileBase.Scheme, remoteBase.Scheme) &&
		strings.EqualFold(profileBase.Host, remoteBase.Host) &&
		strings.EqualFold(strings.Trim(profileBase.Path, "/"), strings.Trim(remoteBase.Path, "/"))
}

func resolvePREnsureRefs(deps Dependencies, repoRoot, remoteName string, args prEnsureArgs) (prRefs, error) {
	source := args.source
	if strings.TrimSpace(source) == "" {
		if deps.CurrentBranch == nil {
			return prRefs{}, fmt.Errorf("current branch discovery is not configured")
		}
		current, err := deps.CurrentBranch(repoRoot)
		if err != nil {
			return prRefs{}, fmt.Errorf("getting current branch: %w", err)
		}
		source = current
	}
	if strings.TrimSpace(source) == "" {
		return prRefs{}, fmt.Errorf("current branch is empty; use --source")
	}

	target := args.target
	if strings.TrimSpace(target) == "" {
		if deps.RemoteDefaultBranch == nil || remoteName == "" {
			return prRefs{}, fmt.Errorf("--target is required")
		}
		defaultBranch, err := deps.RemoteDefaultBranch(repoRoot, remoteName)
		if err != nil || strings.TrimSpace(defaultBranch) == "" {
			return prRefs{}, fmt.Errorf("--target is required")
		}
		target = defaultBranch
	}

	refs := prRefs{SourceRef: normalizeBranchRef(source), TargetRef: normalizeBranchRef(target)}
	if refs.SourceRef == refs.TargetRef {
		return prRefs{}, fmt.Errorf("source and target branches must be different")
	}
	return refs, nil
}

func parsePREnsureArgs(args []string) (prEnsureArgs, error) {
	var parsed prEnsureArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prEnsureArgs{}, fmt.Errorf("--source requires a value")
			}
			parsed.source = args[i+1]
			i++
		case "--target":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prEnsureArgs{}, fmt.Errorf("--target requires a value")
			}
			parsed.target = args[i+1]
			i++
		case "--repository":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prEnsureArgs{}, fmt.Errorf("--repository requires a value")
			}
			parsed.repository = args[i+1]
			i++
		case "--title":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prEnsureArgs{}, fmt.Errorf("--title requires a value")
			}
			parsed.title = args[i+1]
			i++
		case "--description-file":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prEnsureArgs{}, fmt.Errorf("--description-file requires a value")
			}
			parsed.description = args[i+1]
			i++
		case "--profile":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prEnsureArgs{}, fmt.Errorf("--profile requires a value")
			}
			parsed.profile = args[i+1]
			i++
		case "--global":
			parsed.global = true
		case "--json":
			parsed.json = true
		default:
			return prEnsureArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return parsed, nil
}

func normalizeBranchRef(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" || strings.HasPrefix(branch, "refs/") {
		return branch
	}
	return "refs/heads/" + branch
}
