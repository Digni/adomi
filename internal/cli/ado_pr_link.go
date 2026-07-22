package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Digni/adomi/internal/ado"
)

func (r Runner) runADOPullRequestLink(args []string, stdout io.Writer) error {
	linkArgs, err := parsePRLinkArgs(args)
	if err != nil {
		return err
	}

	client, err := r.newADOPRMaintenanceClient(linkArgs.profile, linkArgs.global)
	if err != nil {
		return err
	}
	ctx := context.Background()
	pullRequest, err := client.FetchPullRequest(ctx, linkArgs.pullRequestID)
	if err != nil {
		return fmt.Errorf("fetching pull request %d for work-item linking: %w", linkArgs.pullRequestID, err)
	}
	if pullRequest == nil {
		return fmt.Errorf("pull request %d response is empty", linkArgs.pullRequestID)
	}
	if pullRequest.ID != linkArgs.pullRequestID {
		return fmt.Errorf("Azure DevOps pull request response ID %d does not match requested ID %d", pullRequest.ID, linkArgs.pullRequestID)
	}
	artifactURL, err := pullRequest.ArtifactURL()
	if err != nil {
		return fmt.Errorf("pull request %d identity is incomplete: %w", linkArgs.pullRequestID, err)
	}

	workItems := make([]*ado.WorkItem, 0, len(linkArgs.workItemIDs))
	alreadyLinkedIDs := make([]int, 0, len(linkArgs.workItemIDs))
	for _, workItemID := range linkArgs.workItemIDs {
		item, err := client.FetchWorkItem(ctx, workItemID)
		if err != nil {
			return fmt.Errorf("preflighting work item %d: %w", workItemID, err)
		}
		if item == nil {
			return fmt.Errorf("preflighting work item %d: Azure DevOps response is empty", workItemID)
		}
		if item.ID != workItemID {
			return fmt.Errorf("Azure DevOps work item response ID %d does not match requested ID %d", item.ID, workItemID)
		}
		if item.Rev <= 0 {
			return fmt.Errorf("Azure DevOps work item %d response missing positive revision", workItemID)
		}
		if item.HasArtifactLink(artifactURL) {
			alreadyLinkedIDs = append(alreadyLinkedIDs, workItemID)
		}
		workItems = append(workItems, item)
	}

	linkedIDs := make([]int, 0, len(linkArgs.workItemIDs))
	for _, item := range workItems {
		if item.HasArtifactLink(artifactURL) {
			continue
		}
		updated, err := client.LinkWorkItemToPullRequest(ctx, item.ID, item.Rev, artifactURL)
		if err == nil {
			err = validateLinkedWorkItem(updated, item.ID, artifactURL)
		}
		if err != nil {
			if len(linkedIDs) > 0 {
				return fmt.Errorf("linking work item %d to pull request %d failed; earlier linked work item IDs: %v: %w", item.ID, linkArgs.pullRequestID, linkedIDs, err)
			}
			return fmt.Errorf("linking work item %d to pull request %d: %w", item.ID, linkArgs.pullRequestID, err)
		}
		linkedIDs = append(linkedIDs, item.ID)
	}

	action := "unchanged"
	if len(linkedIDs) > 0 {
		action = "linked"
	}
	result := prLinkResult{
		PullRequestID:            linkArgs.pullRequestID,
		WorkItemIDs:              linkArgs.workItemIDs,
		LinkedWorkItemIDs:        linkedIDs,
		AlreadyLinkedWorkItemIDs: alreadyLinkedIDs,
		Action:                   action,
	}
	if linkArgs.json {
		return writeJSONLine(stdout, result)
	}
	var output strings.Builder
	for _, workItemID := range linkArgs.workItemIDs {
		fmt.Fprintln(&output, workItemID)
	}
	_, err = io.WriteString(stdout, output.String())
	return err
}

func validateLinkedWorkItem(item *ado.WorkItem, requestedID int, artifactURL string) error {
	if item == nil {
		return fmt.Errorf("Azure DevOps work item %d link response is empty", requestedID)
	}
	if item.ID != requestedID {
		return fmt.Errorf("Azure DevOps work item link response ID %d does not match requested ID %d", item.ID, requestedID)
	}
	if item.Rev <= 0 {
		return fmt.Errorf("Azure DevOps work item %d link response missing positive revision", requestedID)
	}
	if !item.HasArtifactLink(artifactURL) {
		return fmt.Errorf("Azure DevOps work item %d link response missing pull request ArtifactLink", requestedID)
	}
	return nil
}

type prLinkArgs struct {
	pullRequestID int
	workItemIDs   []int
	profile       string
	global        bool
	json          bool
}

type prLinkResult struct {
	PullRequestID            int    `json:"pullRequestId"`
	WorkItemIDs              []int  `json:"workItemIds"`
	LinkedWorkItemIDs        []int  `json:"linkedWorkItemIds"`
	AlreadyLinkedWorkItemIDs []int  `json:"alreadyLinkedWorkItemIds"`
	Action                   string `json:"action"`
}

func parsePRLinkArgs(args []string) (prLinkArgs, error) {
	if len(args) == 0 {
		return prLinkArgs{}, fmt.Errorf("usage: adomi ado pr link <pull-request-id> --work-item <work-item-id> [--work-item <work-item-id>...] [--profile <profile-name>] [--global] [--json]")
	}
	pullRequestID, err := strconv.Atoi(args[0])
	if err != nil || pullRequestID <= 0 {
		return prLinkArgs{}, fmt.Errorf("pull request ID must be a positive integer")
	}

	parsed := prLinkArgs{pullRequestID: pullRequestID}
	seenWorkItems := make(map[int]bool)
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--work-item":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
				return prLinkArgs{}, fmt.Errorf("--work-item requires a value")
			}
			workItemID, err := strconv.Atoi(args[i+1])
			if err != nil || workItemID <= 0 {
				return prLinkArgs{}, fmt.Errorf("work item ID must be a positive integer")
			}
			if seenWorkItems[workItemID] {
				return prLinkArgs{}, fmt.Errorf("duplicate work item ID %d", workItemID)
			}
			seenWorkItems[workItemID] = true
			parsed.workItemIDs = append(parsed.workItemIDs, workItemID)
			i++
		case "--profile":
			value, err := parseFlagValue("--profile", args, i)
			if err != nil {
				return prLinkArgs{}, err
			}
			parsed.profile = value
			i++
		case "--global":
			parsed.global = true
		case "--json":
			parsed.json = true
		default:
			return prLinkArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if len(parsed.workItemIDs) == 0 {
		return prLinkArgs{}, fmt.Errorf("at least one --work-item is required")
	}
	return parsed, nil
}
