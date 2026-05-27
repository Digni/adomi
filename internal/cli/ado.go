package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
	"golang.org/x/term"
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

func (r Runner) runADOPullRequest(args []string, stdout io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "fetch":
			return r.runADOPullRequestFetch(args[1:], stdout)
		case "ensure":
			return r.runADOPullRequestEnsure(args[1:], stdout)
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

func (r Runner) runADOPullRequestReply(args []string, stdout io.Writer) error {
	replyArgs, err := parsePRThreadArgs("reply", args, true)
	if err != nil {
		return err
	}

	content := replyArgs.message
	if replyArgs.messageFile != "" {
		var provided bool
		content, provided, err = readOptionalNonBlankFile("--message-file", replyArgs.messageFile)
		if err != nil {
			return err
		}
		if !provided {
			return fmt.Errorf("--message-file is required")
		}
	}

	client, err := r.newADOPRMaintenanceClient(replyArgs.profile, replyArgs.global)
	if err != nil {
		return err
	}
	ctx := context.Background()
	repositoryID, err := fetchPullRequestRepositoryID(ctx, client, replyArgs.pullRequestID)
	if err != nil {
		return err
	}
	comment, err := client.CreatePullRequestThreadComment(ctx, ado.PullRequestThreadCommentCreateOptions{
		RepositoryID:  repositoryID,
		PullRequestID: replyArgs.pullRequestID,
		ThreadID:      replyArgs.threadID,
		Content:       content,
	})
	if err != nil {
		return err
	}
	result := prThreadResult{PullRequestID: replyArgs.pullRequestID, ThreadID: replyArgs.threadID, CommentID: comment.ID, Action: "replied"}
	if replyArgs.json {
		return writeJSONLine(stdout, result)
	}
	fmt.Fprintln(stdout, comment.ID)
	return nil
}

func (r Runner) runADOPullRequestThreadStatus(args []string, stdout io.Writer, command, status, action string) error {
	statusArgs, err := parsePRThreadArgs(command, args, false)
	if err != nil {
		return err
	}

	client, err := r.newADOPRMaintenanceClient(statusArgs.profile, statusArgs.global)
	if err != nil {
		return err
	}
	ctx := context.Background()
	repositoryID, err := fetchPullRequestRepositoryID(ctx, client, statusArgs.pullRequestID)
	if err != nil {
		return err
	}
	if _, err := client.UpdatePullRequestThread(ctx, ado.PullRequestThreadUpdateOptions{
		RepositoryID:  repositoryID,
		PullRequestID: statusArgs.pullRequestID,
		ThreadID:      statusArgs.threadID,
		Status:        status,
	}); err != nil {
		return err
	}
	result := prThreadResult{PullRequestID: statusArgs.pullRequestID, ThreadID: statusArgs.threadID, Status: status, Action: action}
	if statusArgs.json {
		return writeJSONLine(stdout, result)
	}
	fmt.Fprintln(stdout, statusArgs.threadID)
	return nil
}

func (r Runner) newADOPRMaintenanceClient(requestedProfile string, global bool) (ADOClient, error) {
	deps := r.dependencies()
	repoRoot, homeDir, err := resolveLocations(deps)
	if err != nil {
		return nil, err
	}
	scope := config.DefaultScope
	if global {
		scope = config.GlobalScope
	}
	loaded, err := deps.LoadConfig(repoRoot, homeDir, requestedProfile, scope)
	if err != nil {
		return nil, err
	}
	profileConfig := loaded.Profile

	pat, err := deps.PATStore.Get(profileConfig.CredentialRef())
	if err != nil {
		return nil, err
	}
	httpClient, err := deps.NewHTTPClient(profileConfig.Proxy)
	if err != nil {
		return nil, err
	}
	client, err := deps.NewADOClient(httpClient, ado.ClientConfig{
		BaseURL:    profileConfig.BaseURL,
		Project:    profileConfig.Project,
		APIVersion: profileConfig.APIVersion,
		PAT:        pat,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func fetchPullRequestRepositoryID(ctx context.Context, fetcher ado.PullRequestFetcher, pullRequestID int) (string, error) {
	pr, err := fetcher.FetchPullRequest(ctx, pullRequestID)
	if err != nil {
		return "", err
	}
	if pr.Repository.ID == "" {
		return "", fmt.Errorf("pull request %d response missing repository ID", pullRequestID)
	}
	return pr.Repository.ID, nil
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

func (r Runner) runADOLogin(args []string, stdin io.Reader, stderr io.Writer) error {
	credentialArgs, err := parseCredentialArgs(args)
	if err != nil {
		return err
	}
	deps := r.dependencies()
	credentialRef, err := resolveCredentialRef(deps, credentialArgs)
	if err != nil {
		return err
	}

	pat, err := deps.ReadSecret(fmt.Sprintf("Azure DevOps PAT for %s: ", credentialRef), stdin, stderr)
	if err != nil {
		return err
	}
	pat = strings.TrimRight(pat, "\r\n")
	if pat == "" {
		return fmt.Errorf("PAT cannot be empty")
	}

	if err := deps.PATStore.Set(credentialRef, pat); err != nil {
		return err
	}
	return nil
}

func readSecret(prompt string, stdin io.Reader, stderr io.Writer) (string, error) {
	fmt.Fprint(stderr, prompt)
	if file, ok := stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		data, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", fmt.Errorf("reading PAT: %w", err)
		}
		return string(data), nil
	}

	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil {
		if err == io.EOF && len(line) > 0 {
			return line, nil
		}
		return "", fmt.Errorf("reading PAT: %w", err)
	}
	return line, nil
}

func (r Runner) runADOLogout(args []string) error {
	credentialArgs, err := parseCredentialArgs(args)
	if err != nil {
		return err
	}
	deps := r.dependencies()
	credentialRef, err := resolveCredentialRef(deps, credentialArgs)
	if err != nil {
		return err
	}
	return deps.PATStore.Delete(credentialRef)
}

func (r Runner) runADOProfilesList(args []string, stdout io.Writer) error {
	global, err := parseGlobalFlag(args)
	if err != nil {
		return err
	}
	deps := r.dependencies()
	repoRoot := ""
	scope := config.DefaultScope
	homeDir, err := deps.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	if global {
		scope = config.GlobalScope
	} else {
		cwd, err := deps.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		repoRoot, err = deps.FindRepoRoot(cwd)
		if err != nil {
			return err
		}
	}
	loaded, err := deps.LoadAllConfig(repoRoot, homeDir, scope)
	if err != nil {
		return err
	}
	for _, profile := range loaded.ProfileNames() {
		fmt.Fprintln(stdout, profile)
	}
	return nil
}

func (r Runner) runADOConfigInit(args []string, stdout io.Writer) error {
	global, err := parseGlobalFlag(args)
	if err != nil {
		return err
	}
	deps := r.dependencies()

	var repoRoot string
	var homeDir string
	if global {
		homeDir, err = deps.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		if cwd, err := deps.Getwd(); err == nil {
			if resolved, err := deps.FindRepoRoot(cwd); err == nil {
				repoRoot = resolved
			}
		}
	} else {
		cwd, err := deps.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		repoRoot, err = deps.FindRepoRoot(cwd)
		if err != nil {
			return err
		}
	}

	targetPath := config.RepoConfigPath(repoRoot)
	if global {
		targetPath = config.GlobalConfigPath(homeDir)
	}

	values := config.DefaultInitTemplateValues()
	if repoRoot != "" {
		if remotes, err := deps.RemoteURLs(repoRoot); err == nil {
			if detected, ok := config.InitTemplateValuesFromRemotes(remotes); ok {
				values = detected
			}
		}
	}

	if err := writeNewFile(targetPath, []byte(config.RenderInitTemplate(values)), 0o600); err != nil {
		return err
	}
	fmt.Fprintln(stdout, targetPath)
	return nil
}

func writeNewFile(path string, data []byte, perm os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config %s already exists", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking config %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("config %s already exists", path)
		}
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

func resolveLocations(deps Dependencies) (string, string, error) {
	cwd, err := deps.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("getting current directory: %w", err)
	}
	repoRoot, err := deps.FindRepoRoot(cwd)
	if err != nil {
		return "", "", err
	}
	homeDir, err := deps.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("getting home directory: %w", err)
	}
	return repoRoot, homeDir, nil
}

func resolveCredentialRef(deps Dependencies, args credentialArgs) (string, error) {
	if args.patRef != "" {
		return args.patRef, nil
	}

	repoRoot := ""
	scope := config.DefaultScope
	var homeDir string
	var err error
	if args.global {
		scope = config.GlobalScope
		homeDir, err = deps.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
	} else {
		repoRoot, homeDir, err = resolveLocations(deps)
		if err != nil {
			return "", err
		}
	}

	loaded, err := deps.LoadConfig(repoRoot, homeDir, args.profile, scope)
	if err != nil {
		return "", err
	}
	return loaded.Profile.CredentialRef(), nil
}

type credentialArgs struct {
	profile string
	patRef  string
	global  bool
}

func parseCredentialArgs(args []string) (credentialArgs, error) {
	var parsed credentialArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return credentialArgs{}, fmt.Errorf("--profile requires a value")
			}
			parsed.profile = args[i+1]
			i++
		case "--pat-ref":
			if i+1 >= len(args) {
				return credentialArgs{}, fmt.Errorf("--pat-ref requires a value")
			}
			ref, err := parsePATRefValue(args[i+1])
			if err != nil {
				return credentialArgs{}, err
			}
			parsed.patRef = ref
			i++
		case "--global":
			parsed.global = true
		default:
			return credentialArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if parsed.global && parsed.profile == "" {
		return credentialArgs{}, fmt.Errorf("--global requires --profile")
	}
	if parsed.profile != "" && parsed.patRef != "" {
		return credentialArgs{}, fmt.Errorf("cannot use --profile with --pat-ref")
	}
	if parsed.profile == "" && parsed.patRef == "" {
		return credentialArgs{}, fmt.Errorf("--profile or --pat-ref is required")
	}
	return parsed, nil
}

func parsePATRefValue(value string) (string, error) {
	if value == "" || strings.HasPrefix(strings.TrimSpace(value), "-") {
		return "", fmt.Errorf("--pat-ref requires a value")
	}
	ref, err := config.NormalizePATRef(value)
	if err != nil {
		return "", fmt.Errorf("patRef %w", err)
	}
	return ref, nil
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

type prArgs struct {
	pullRequestID int
	profile       string
	global        bool
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

type prThreadArgs struct {
	pullRequestID int
	threadID      int
	message       string
	messageFile   string
	profile       string
	global        bool
	json          bool
}

type prThreadResult struct {
	PullRequestID int    `json:"pullRequestId"`
	ThreadID      int    `json:"threadId"`
	CommentID     int    `json:"commentId,omitempty"`
	Status        string `json:"status,omitempty"`
	Action        string `json:"action"`
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

func parsePRThreadArgs(command string, args []string, requireMessage bool) (prThreadArgs, error) {
	if len(args) == 0 {
		return prThreadArgs{}, fmt.Errorf("usage: adomi ado pr %s <pull-request-id> --thread <thread-id>", command)
	}
	pullRequestID, err := strconv.Atoi(args[0])
	if err != nil || pullRequestID <= 0 {
		return prThreadArgs{}, fmt.Errorf("pull request ID must be a positive integer")
	}
	parsed := prThreadArgs{pullRequestID: pullRequestID}
	threadSet := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--thread":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prThreadArgs{}, fmt.Errorf("--thread requires a value")
			}
			threadID, err := strconv.Atoi(args[i+1])
			if err != nil || threadID <= 0 {
				return prThreadArgs{}, fmt.Errorf("thread ID must be a positive integer")
			}
			parsed.threadID = threadID
			threadSet = true
			i++
		case "--message":
			if !requireMessage {
				return prThreadArgs{}, fmt.Errorf("unknown argument %q", args[i])
			}
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prThreadArgs{}, fmt.Errorf("--message requires a value")
			}
			if strings.TrimSpace(args[i+1]) == "" {
				return prThreadArgs{}, fmt.Errorf("--message cannot be empty")
			}
			parsed.message = args[i+1]
			i++
		case "--message-file":
			if !requireMessage {
				return prThreadArgs{}, fmt.Errorf("unknown argument %q", args[i])
			}
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prThreadArgs{}, fmt.Errorf("--message-file requires a value")
			}
			parsed.messageFile = args[i+1]
			i++
		case "--profile":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prThreadArgs{}, fmt.Errorf("--profile requires a value")
			}
			parsed.profile = args[i+1]
			i++
		case "--global":
			parsed.global = true
		case "--json":
			parsed.json = true
		default:
			return prThreadArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if !threadSet {
		return prThreadArgs{}, fmt.Errorf("--thread is required")
	}
	if requireMessage {
		hasMessage := parsed.message != ""
		hasFile := parsed.messageFile != ""
		if hasMessage == hasFile {
			return prThreadArgs{}, fmt.Errorf("exactly one of --message or --message-file is required")
		}
	}
	return parsed, nil
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

func readOptionalNonEmptyFile(flagName, path string) (string, bool, error) {
	if strings.TrimSpace(path) == "" {
		return "", false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("reading %s: %w", flagName, err)
	}
	if len(data) == 0 {
		return "", false, fmt.Errorf("%s cannot be empty", flagName)
	}
	return string(data), true, nil
}

func readOptionalNonBlankFile(flagName, path string) (string, bool, error) {
	content, provided, err := readOptionalNonEmptyFile(flagName, path)
	if err != nil || !provided {
		return content, provided, err
	}
	if strings.TrimSpace(content) == "" {
		return "", false, fmt.Errorf("%s cannot be empty", flagName)
	}
	return content, true, nil
}

func writeJSONLine(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
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

func parseGlobalFlag(args []string) (bool, error) {
	var global bool
	for _, arg := range args {
		switch arg {
		case "--global":
			global = true
		default:
			return false, fmt.Errorf("unknown argument %q", arg)
		}
	}
	return global, nil
}
