package cli

import (
	"context"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/Digni/adomi/internal/ado"
)

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
