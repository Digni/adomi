package cli

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
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

func (r Runner) runADOPullRequestComment(args []string, stdout io.Writer) error {
	commentArgs, err := parsePRCommentArgs(args)
	if err != nil {
		return err
	}

	content := commentArgs.message
	if commentArgs.messageFile != "" {
		var provided bool
		content, provided, err = readOptionalNonBlankFile("--message-file", commentArgs.messageFile)
		if err != nil {
			return err
		}
		if !provided {
			return fmt.Errorf("--message-file is required")
		}
	}

	client, err := r.newADOPRMaintenanceClient(commentArgs.profile, commentArgs.global)
	if err != nil {
		return err
	}
	ctx := context.Background()
	repositoryID, err := fetchPullRequestRepositoryID(ctx, client, commentArgs.pullRequestID)
	if err != nil {
		return err
	}
	createOpts := ado.PullRequestThreadCreateOptions{
		RepositoryID:  repositoryID,
		PullRequestID: commentArgs.pullRequestID,
		Content:       content,
	}
	var inlineTarget prInlineCommentTarget
	if commentArgs.inlineTarget() {
		inlineTarget, err = resolvePullRequestInlineCommentTarget(ctx, client, repositoryID, commentArgs.pullRequestID, commentArgs.file)
		if err != nil {
			return err
		}
		position := &ado.FilePosition{Line: commentArgs.line, Offset: 1}
		createOpts.ThreadContext = &ado.ThreadContext{
			FilePath:       inlineTarget.FilePath,
			RightFileStart: position,
			RightFileEnd:   position,
		}
		createOpts.PullRequestThreadContext = &ado.PullRequestThreadContext{
			ChangeTrackingID: inlineTarget.ChangeTrackingID,
			IterationContext: &ado.CommentIterationContext{
				FirstComparingIteration:  inlineTarget.FirstComparingIteration,
				SecondComparingIteration: inlineTarget.SecondComparingIteration,
			},
		}
	}
	thread, err := client.CreatePullRequestThread(ctx, createOpts)
	if err != nil {
		return err
	}
	if thread.ID <= 0 {
		return fmt.Errorf("Azure DevOps pull request thread response missing thread ID")
	}
	result := prThreadResult{PullRequestID: commentArgs.pullRequestID, ThreadID: thread.ID, Action: "commented"}
	if commentArgs.inlineTarget() {
		result.FilePath = inlineTarget.FilePath
		result.Line = commentArgs.line
	}
	if len(thread.Comments) > 0 && thread.Comments[0].ID > 0 {
		result.CommentID = thread.Comments[0].ID
	}
	if commentArgs.json {
		return writeJSONLine(stdout, result)
	}
	fmt.Fprintln(stdout, thread.ID)
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
	return r.newADOMaintenanceClient(requestedProfile, global)
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

type prInlineCommentTarget struct {
	FilePath                 string
	ChangeTrackingID         int
	FirstComparingIteration  int
	SecondComparingIteration int
}

func resolvePullRequestInlineCommentTarget(ctx context.Context, client ado.PullRequestMaintainer, repositoryID string, pullRequestID int, requestedPath string) (prInlineCommentTarget, error) {
	iterations, err := client.ListPullRequestIterations(ctx, repositoryID, pullRequestID)
	if err != nil {
		return prInlineCommentTarget{}, err
	}
	latestIterationID := 0
	for _, iteration := range iterations {
		if iteration.ID > latestIterationID {
			latestIterationID = iteration.ID
		}
	}
	if latestIterationID <= 0 {
		return prInlineCommentTarget{}, fmt.Errorf("pull request %d has no iterations for inline comment targeting", pullRequestID)
	}
	changes, err := client.ListPullRequestIterationChanges(ctx, ado.PullRequestIterationChangesOptions{
		RepositoryID:  repositoryID,
		PullRequestID: pullRequestID,
		IterationID:   latestIterationID,
		CompareTo:     0,
	})
	if err != nil {
		return prInlineCommentTarget{}, err
	}
	requested := normalizePullRequestFilePath(requestedPath)
	var matched *ado.PullRequestIterationChange
	for i := range changes {
		change := changes[i]
		if normalizePullRequestFilePath(change.Item.Path) != requested {
			continue
		}
		if matched != nil {
			return prInlineCommentTarget{}, fmt.Errorf("multiple pull request changes match %q; cannot choose inline comment target", requestedPath)
		}
		matched = &change
	}
	if matched == nil {
		return prInlineCommentTarget{}, fmt.Errorf("inline comments can only target supported changed files in the latest pull request version; %q was not found", requestedPath)
	}
	changeType := strings.ToLower(matched.ChangeType)
	if strings.Contains(changeType, "delete") || strings.Contains(changeType, "rename") || matched.Item.Path == "" || matched.ChangeTrackingID <= 0 {
		return prInlineCommentTarget{}, fmt.Errorf("inline comments can only target supported changed files in the latest pull request version; %q is unsupported", requestedPath)
	}
	return prInlineCommentTarget{
		FilePath:                 matched.Item.Path,
		ChangeTrackingID:         matched.ChangeTrackingID,
		FirstComparingIteration:  latestIterationID,
		SecondComparingIteration: latestIterationID,
	}, nil
}

func normalizePullRequestFilePath(filePath string) string {
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" || strings.Contains(trimmed, "\\") {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		if part == "." || part == ".." {
			return ""
		}
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	if cleaned == "." {
		return ""
	}
	return cleaned
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

type prCommentArgs struct {
	pullRequestID int
	message       string
	messageFile   string
	file          string
	line          int
	profile       string
	global        bool
	json          bool
}

func (a prCommentArgs) inlineTarget() bool {
	return a.file != "" && a.line > 0
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
	FilePath      string `json:"filePath,omitempty"`
	Line          int    `json:"line,omitempty"`
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

func parseInlineMessageFlag(args []string, index int) (string, error) {
	value, err := parseFlagValue("--message", args, index)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("--message cannot be empty")
	}
	return value, nil
}

func parsePRCommentArgs(args []string) (prCommentArgs, error) {
	if len(args) == 0 {
		return prCommentArgs{}, fmt.Errorf("usage: adomi ado pr comment <pull-request-id> (--message <text> | --message-file <path>)")
	}
	pullRequestID, err := strconv.Atoi(args[0])
	if err != nil || pullRequestID <= 0 {
		return prCommentArgs{}, fmt.Errorf("pull request ID must be a positive integer")
	}
	parsed := prCommentArgs{pullRequestID: pullRequestID}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--message":
			value, err := parseInlineMessageFlag(args, i)
			if err != nil {
				return prCommentArgs{}, err
			}
			parsed.message = value
			i++
		case "--message-file":
			value, err := parseFlagValue("--message-file", args, i)
			if err != nil {
				return prCommentArgs{}, err
			}
			parsed.messageFile = value
			i++
		case "--file":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" || strings.HasPrefix(args[i+1], "-") {
				return prCommentArgs{}, fmt.Errorf("--file requires a value")
			}
			parsed.file = args[i+1]
			i++
		case "--line":
			if i+1 >= len(args) || args[i+1] == "" {
				return prCommentArgs{}, fmt.Errorf("--line requires a value")
			}
			line, err := strconv.Atoi(args[i+1])
			if err != nil || line <= 0 {
				return prCommentArgs{}, fmt.Errorf("--line must be a positive integer")
			}
			parsed.line = line
			i++
		case "--profile":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return prCommentArgs{}, fmt.Errorf("--profile requires a value")
			}
			parsed.profile = args[i+1]
			i++
		case "--global":
			parsed.global = true
		case "--json":
			parsed.json = true
		default:
			return prCommentArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := validateSingleMessageSource(parsed.message, parsed.messageFile); err != nil {
		return prCommentArgs{}, err
	}
	if (parsed.file == "") != (parsed.line == 0) {
		return prCommentArgs{}, fmt.Errorf("--file and --line must be provided together")
	}
	return parsed, nil
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
			value, err := parseInlineMessageFlag(args, i)
			if err != nil {
				return prThreadArgs{}, err
			}
			parsed.message = value
			i++
		case "--message-file":
			if !requireMessage {
				return prThreadArgs{}, fmt.Errorf("unknown argument %q", args[i])
			}
			value, err := parseFlagValue("--message-file", args, i)
			if err != nil {
				return prThreadArgs{}, err
			}
			parsed.messageFile = value
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
		if err := validateSingleMessageSource(parsed.message, parsed.messageFile); err != nil {
			return prThreadArgs{}, err
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
