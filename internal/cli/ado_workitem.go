package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Digni/adomi/internal/ado"
)

func (r Runner) runADOWorkItem(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: adomi ado work-item <command>")
	}
	switch args[0] {
	case "comment":
		return r.runADOWorkItemComment(args[1:], stdout)
	default:
		return fmt.Errorf("unsupported work item command %q", args[0])
	}
}

func (r Runner) runADOWorkItemComment(args []string, stdout io.Writer) error {
	commentArgs, err := parseWorkItemCommentArgs(args)
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

	client, err := r.newADOMaintenanceClient(commentArgs.profile, commentArgs.global)
	if err != nil {
		return err
	}
	ctx := context.Background()
	comment, err := client.CreateWorkItemComment(ctx, ado.WorkItemCommentCreateOptions{WorkItemID: commentArgs.workItemID, Text: content})
	if err != nil {
		return err
	}
	commentID := comment.CreatedID()
	if commentID <= 0 {
		return fmt.Errorf("Azure DevOps work item comment response missing comment ID")
	}
	if comment.WorkItemID > 0 && comment.WorkItemID != commentArgs.workItemID {
		return fmt.Errorf("Azure DevOps work item comment response work item ID %d does not match requested ID %d", comment.WorkItemID, commentArgs.workItemID)
	}
	result := workItemCommentResult{WorkItemID: commentArgs.workItemID, CommentID: commentID, Action: "commented", URL: comment.URL}
	if commentArgs.json {
		return writeJSONLine(stdout, result)
	}
	fmt.Fprintln(stdout, commentID)
	return nil
}

type workItemCommentArgs struct {
	workItemID  int
	message     string
	messageFile string
	profile     string
	global      bool
	json        bool
}

type workItemCommentResult struct {
	WorkItemID int    `json:"workItemId"`
	CommentID  int    `json:"commentId"`
	Action     string `json:"action"`
	URL        string `json:"url,omitempty"`
}

func parseWorkItemCommentArgs(args []string) (workItemCommentArgs, error) {
	if len(args) == 0 {
		return workItemCommentArgs{}, fmt.Errorf("usage: adomi ado comment <work-item-id> (--message <text> | --message-file <path>) [--profile <profile-name>] [--global] [--json]")
	}
	workItemID, err := strconv.Atoi(args[0])
	if err != nil || workItemID <= 0 {
		return workItemCommentArgs{}, fmt.Errorf("work item ID must be a positive integer")
	}
	parsed := workItemCommentArgs{workItemID: workItemID}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--message":
			value, err := parseWorkItemInlineMessageFlag(args, i)
			if err != nil {
				return workItemCommentArgs{}, err
			}
			parsed.message = value
			i++
		case "--message-file":
			value, err := parseFlagValue("--message-file", args, i)
			if err != nil {
				return workItemCommentArgs{}, err
			}
			parsed.messageFile = value
			i++
		case "--profile":
			value, err := parseFlagValue("--profile", args, i)
			if err != nil {
				return workItemCommentArgs{}, err
			}
			parsed.profile = value
			i++
		case "--global":
			parsed.global = true
		case "--json":
			parsed.json = true
		default:
			return workItemCommentArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := validateSingleMessageSource(parsed.message, parsed.messageFile); err != nil {
		return workItemCommentArgs{}, err
	}
	return parsed, nil
}

func parseWorkItemInlineMessageFlag(args []string, index int) (string, error) {
	if index+1 >= len(args) || args[index+1] == "" {
		return "", fmt.Errorf("--message requires a value")
	}
	value := args[index+1]
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("--message cannot be empty")
	}
	return value, nil
}
