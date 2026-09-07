package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

type prListArgs struct {
	source     string
	target     string
	status     string
	repository string
	profile    string
	global     bool
}

func parsePRListArgs(args []string) (prListArgs, error) {
	parsed := prListArgs{status: "active"}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		flag := args[i]
		switch flag {
		case "--source", "--target", "--status", "--repository", "--profile":
			if seen[flag] {
				return prListArgs{}, fmt.Errorf("%s may be specified only once", flag)
			}
			seen[flag] = true
			value, err := parseFlagValue(flag, args, i)
			if err != nil {
				return prListArgs{}, err
			}
			i++
			switch flag {
			case "--source", "--target":
				branch, err := normalizePRListBranch(value, flag)
				if err != nil {
					return prListArgs{}, err
				}
				if flag == "--source" {
					parsed.source = branch
				} else {
					parsed.target = branch
				}
			case "--status":
				parsed.status = strings.TrimSpace(value)
				if !validPRListStatus(parsed.status) {
					return prListArgs{}, fmt.Errorf("--status must be active, completed, abandoned, or all")
				}
			case "--repository":
				parsed.repository = strings.TrimSpace(value)
				if parsed.repository == "" {
					return prListArgs{}, fmt.Errorf("--repository requires a non-empty value")
				}
			case "--profile":
				parsed.profile = strings.TrimSpace(value)
				if parsed.profile == "" {
					return prListArgs{}, fmt.Errorf("--profile requires a non-empty value")
				}
			}
		case "--global":
			if seen[flag] {
				return prListArgs{}, fmt.Errorf("--global may be specified only once")
			}
			seen[flag] = true
			parsed.global = true
		default:
			return prListArgs{}, fmt.Errorf("unknown argument %q", flag)
		}
	}
	return parsed, nil
}

func normalizePRListBranch(value, flag string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s requires a non-empty value", flag)
	}
	return normalizeBranchRef(value), nil
}

func validPRListStatus(status string) bool {
	switch status {
	case "active", "completed", "abandoned", "all":
		return true
	default:
		return false
	}
}

func (r Runner) runADOPullRequestList(args []string, stdout io.Writer) error {
	listArgs, err := parsePRListArgs(args)
	if err != nil {
		return err
	}

	deps := r.dependencies()
	repoRoot, homeDir, err := resolveLocations(deps)
	if err != nil {
		return err
	}
	scope := config.DefaultScope
	if listArgs.global {
		scope = config.GlobalScope
	}
	loaded, err := deps.LoadConfig(repoRoot, homeDir, listArgs.profile, scope)
	if err != nil {
		return err
	}
	profileConfig := loaded.Profile

	var remotes []gitRemote
	if listArgs.repository == "" {
		remotes, err = deps.GitRemotes(repoRoot)
		if err != nil {
			return fmt.Errorf("getting git remotes: %w", err)
		}
	}
	selectedRepo, err := selectPRRepository(remotes, profileConfig, listArgs.repository)
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

	pullRequests, err := ado.DiscoverPullRequests(context.Background(), client, ado.PullRequestListOptions{
		RepositoryID:  selectedRepo.Repository,
		SourceRefName: listArgs.source,
		TargetRefName: listArgs.target,
		Status:        listArgs.status,
	})
	if err != nil {
		return err
	}
	result := prListJSONResult{PullRequests: make([]prListJSON, 0, len(pullRequests))}
	for i, pullRequest := range pullRequests {
		projected, err := projectPRListEntry(pullRequest)
		if err != nil {
			return fmt.Errorf("projecting pull request %d: %w", i, err)
		}
		result.PullRequests = append(result.PullRequests, projected)
	}
	return writeJSONLine(stdout, result)
}

type prListJSONResult struct {
	PullRequests []prListJSON `json:"pullRequests"`
}

type prListJSON struct {
	PullRequestID  int     `json:"pullRequestId"`
	Title          string  `json:"title"`
	Status         string  `json:"status"`
	IsDraft        bool    `json:"isDraft"`
	RepositoryID   string  `json:"repositoryId"`
	RepositoryName string  `json:"repositoryName"`
	SourceRefName  string  `json:"sourceRefName"`
	TargetRefName  string  `json:"targetRefName"`
	CreationDate   *string `json:"creationDate"`
	ClosedDate     *string `json:"closedDate"`
	URL            *string `json:"url"`
}

func projectPRListEntry(pullRequest ado.PullRequest) (prListJSON, error) {
	creationDate, err := normalizedPRListDate(pullRequest.CreationDate, "creationDate")
	if err != nil {
		return prListJSON{}, err
	}
	closedDate, err := normalizedPRListDate(pullRequest.ClosedDate, "closedDate")
	if err != nil {
		return prListJSON{}, err
	}
	url := optionalPRListString(pullRequest.URL)
	return prListJSON{
		PullRequestID:  pullRequest.ID,
		Title:          pullRequest.Title,
		Status:         pullRequest.Status,
		IsDraft:        pullRequest.IsDraft,
		RepositoryID:   pullRequest.Repository.ID,
		RepositoryName: pullRequest.Repository.Name,
		SourceRefName:  pullRequest.SourceRefName,
		TargetRefName:  pullRequest.TargetRefName,
		CreationDate:   creationDate,
		ClosedDate:     closedDate,
		URL:            url,
	}, nil
}

func normalizedPRListDate(value, field string) (*string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q: %w", field, value, err)
	}
	formatted := parsed.UTC().Format(time.RFC3339Nano)
	return &formatted, nil
}

func optionalPRListString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
