package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Digni/adomi/internal/ado"
)

const clearedAutoCompleteIdentityID = "00000000-0000-0000-0000-000000000000"

type prGovernanceArgs struct {
	pullRequestID int
	profile       string
	global        bool
	json          bool
	completion    completionPreferenceArgs
}

type completionPreferenceArgs struct {
	mergeStrategy       *string
	deleteSourceBranch  *bool
	transitionWorkItems *bool
	mergeCommitMessage  *string
}

func (p completionPreferenceArgs) supplied() bool {
	return p.mergeStrategy != nil || p.deleteSourceBranch != nil || p.transitionWorkItems != nil || p.mergeCommitMessage != nil
}

type prGovernanceResult struct {
	PullRequestID       int    `json:"pullRequestId"`
	Action              string `json:"action"`
	Status              string `json:"status"`
	MergeStatus         string `json:"mergeStatus,omitempty"`
	AutoCompleteEnabled *bool  `json:"autoCompleteEnabled,omitempty"`
	ReviewerID          string `json:"reviewerId,omitempty"`
	Vote                *int   `json:"vote,omitempty"`
}

func (r Runner) runADOPullRequestGovernance(action string, args []string, stdout io.Writer) error {
	parsed, err := parsePRGovernanceArgs(action, args)
	if err != nil {
		return err
	}
	client, err := r.newADOMaintenanceClient(parsed.profile, parsed.global)
	if err != nil {
		return err
	}
	result, err := executePRGovernance(context.Background(), client, action, parsed)
	if err != nil {
		return err
	}
	if parsed.json {
		return writeJSONLine(stdout, result)
	}
	_, err = fmt.Fprintln(stdout, result.PullRequestID)
	return err
}

func parsePRGovernanceArgs(action string, args []string) (prGovernanceArgs, error) {
	if len(args) == 0 {
		return prGovernanceArgs{}, fmt.Errorf("usage: adomi ado pr %s <pull-request-id> [flags]", action)
	}
	pullRequestID, err := strconv.Atoi(args[0])
	if err != nil || pullRequestID <= 0 {
		return prGovernanceArgs{}, fmt.Errorf("pull request ID must be a positive integer")
	}
	parsed := prGovernanceArgs{pullRequestID: pullRequestID}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			value, err := parseFlagValue("--profile", args, i)
			if err != nil {
				return prGovernanceArgs{}, err
			}
			if parsed.profile != "" && parsed.profile != value {
				return prGovernanceArgs{}, fmt.Errorf("--profile cannot select more than one profile")
			}
			parsed.profile = value
			i++
		case "--global":
			parsed.global = true
		case "--json":
			parsed.json = true
		case "--merge-strategy":
			if !acceptsCompletionPreferences(action) {
				return prGovernanceArgs{}, fmt.Errorf("%s does not accept completion preferences", action)
			}
			value, err := parseFlagValue("--merge-strategy", args, i)
			if err != nil {
				return prGovernanceArgs{}, err
			}
			strategy, ok := mapMergeStrategy(value)
			if !ok {
				return prGovernanceArgs{}, fmt.Errorf("unsupported merge strategy %q", value)
			}
			parsed.completion.mergeStrategy = &strategy
			i++
		case "--delete-source-branch":
			if !acceptsCompletionPreferences(action) {
				return prGovernanceArgs{}, fmt.Errorf("%s does not accept completion preferences", action)
			}
			value, err := parseFlagValue("--delete-source-branch", args, i)
			if err != nil {
				return prGovernanceArgs{}, err
			}
			parsed.completion.deleteSourceBranch, err = parseExplicitBool("--delete-source-branch", value)
			if err != nil {
				return prGovernanceArgs{}, err
			}
			i++
		case "--transition-work-items":
			if !acceptsCompletionPreferences(action) {
				return prGovernanceArgs{}, fmt.Errorf("%s does not accept completion preferences", action)
			}
			value, err := parseFlagValue("--transition-work-items", args, i)
			if err != nil {
				return prGovernanceArgs{}, err
			}
			parsed.completion.transitionWorkItems, err = parseExplicitBool("--transition-work-items", value)
			if err != nil {
				return prGovernanceArgs{}, err
			}
			i++
		case "--merge-commit-message":
			if !acceptsCompletionPreferences(action) {
				return prGovernanceArgs{}, fmt.Errorf("%s does not accept completion preferences", action)
			}
			value, err := parseFlagValue("--merge-commit-message", args, i)
			if err != nil {
				return prGovernanceArgs{}, err
			}
			if strings.TrimSpace(value) == "" {
				return prGovernanceArgs{}, fmt.Errorf("--merge-commit-message cannot be empty")
			}
			parsed.completion.mergeCommitMessage = &value
			i++
		default:
			return prGovernanceArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return parsed, nil
}

func acceptsCompletionPreferences(action string) bool {
	return action == "complete" || action == "auto-complete"
}

func mapMergeStrategy(value string) (string, bool) {
	switch value {
	case "no-fast-forward":
		return "noFastForward", true
	case "squash":
		return "squash", true
	case "rebase":
		return "rebase", true
	case "rebase-merge":
		return "rebaseMerge", true
	default:
		return "", false
	}
}

func parseExplicitBool(flag, value string) (*bool, error) {
	var parsed bool
	switch value {
	case "true":
		parsed = true
	case "false":
		parsed = false
	default:
		return nil, fmt.Errorf("%s requires true or false", flag)
	}
	return &parsed, nil
}

func executePRGovernance(ctx context.Context, client ADOClient, action string, args prGovernanceArgs) (prGovernanceResult, error) {
	pr, err := client.FetchPullRequest(ctx, args.pullRequestID)
	if err != nil {
		return prGovernanceResult{}, err
	}
	if pr == nil {
		return prGovernanceResult{}, fmt.Errorf("Azure DevOps pull request %d preflight returned no pull request", args.pullRequestID)
	}
	if pr.ID != args.pullRequestID {
		return prGovernanceResult{}, fmt.Errorf("Azure DevOps pull request response ID %d does not match requested ID %d", pr.ID, args.pullRequestID)
	}
	repositoryID := strings.TrimSpace(pr.Repository.ID)
	if repositoryID == "" {
		return prGovernanceResult{}, fmt.Errorf("pull request %d preflight missing repository ID", pr.ID)
	}
	status := strings.ToLower(strings.TrimSpace(pr.Status))
	switch status {
	case "active", "completed", "abandoned":
	default:
		return prGovernanceResult{}, fmt.Errorf("pull request %d has unsupported status %q", pr.ID, pr.Status)
	}

	switch action {
	case "complete":
		return executePRComplete(ctx, client, pr, status, args)
	case "auto-complete":
		return executePRAutoComplete(ctx, client, pr, status, args)
	case "cancel-auto-complete":
		return executePRCancelAutoComplete(ctx, client, pr, status)
	case "abandon":
		return executePRAbandon(ctx, client, pr, status)
	case "approve":
		return executePRVote(ctx, client, pr, status, action, 10)
	case "approve-with-suggestions":
		return executePRVote(ctx, client, pr, status, action, 5)
	case "reject":
		return executePRVote(ctx, client, pr, status, action, -10)
	default:
		return prGovernanceResult{}, fmt.Errorf("unsupported pull request governance action %q", action)
	}
}

func executePRComplete(ctx context.Context, client ADOClient, pr *ado.PullRequest, status string, args prGovernanceArgs) (prGovernanceResult, error) {
	if status == "completed" {
		return lifecycleResult(pr, "unchanged", nil), nil
	}
	if status == "abandoned" {
		return prGovernanceResult{}, fmt.Errorf("pull request %d is abandoned and cannot be completed", pr.ID)
	}
	if pr.IsDraft {
		return prGovernanceResult{}, fmt.Errorf("pull request %d is a draft and cannot be completed", pr.ID)
	}
	if pr.LastMergeSourceCommit == nil || strings.TrimSpace(pr.LastMergeSourceCommit.CommitID) == "" {
		return prGovernanceResult{}, fmt.Errorf("pull request %d preflight missing current source commit ID", pr.ID)
	}
	completed := "completed"
	updated, err := client.UpdatePullRequest(ctx, ado.PullRequestUpdateOptions{
		RepositoryID:          pr.Repository.ID,
		PullRequestID:         pr.ID,
		Status:                &completed,
		LastMergeSourceCommit: &ado.GitCommitRef{CommitID: strings.TrimSpace(pr.LastMergeSourceCommit.CommitID)},
		CompletionOptions:     mergedSafeCompletionOptions(pr.CompletionOptions, args.completion),
		EnforcePolicies:       true,
	})
	if err != nil {
		return prGovernanceResult{}, err
	}
	if err := validateLifecycleResponse(updated, pr.ID, "completed"); err != nil {
		return prGovernanceResult{}, err
	}
	return lifecycleResult(updated, "complete", nil), nil
}

func executePRAutoComplete(ctx context.Context, client ADOClient, pr *ado.PullRequest, status string, args prGovernanceArgs) (prGovernanceResult, error) {
	if status != "active" {
		return prGovernanceResult{}, fmt.Errorf("pull request %d has status %s and cannot enable auto-complete", pr.ID, status)
	}
	if pr.IsDraft {
		return prGovernanceResult{}, fmt.Errorf("pull request %d is a draft and cannot enable auto-complete", pr.ID)
	}
	desiredOptions := mergedSafeCompletionOptions(pr.CompletionOptions, args.completion)
	policyOverridesPresent := hasCompletionPolicyOverrides(pr.CompletionOptions)
	if autoCompleteEnabled(pr) {
		if !policyOverridesPresent && (!args.completion.supplied() || equalSafeCompletionOptions(safeCompletionOptions(pr.CompletionOptions), desiredOptions)) {
			enabled := true
			result := lifecycleResult(pr, "unchanged", &enabled)
			result.MergeStatus = pr.MergeStatus
			return result, nil
		}
		updated, err := client.UpdatePullRequest(ctx, ado.PullRequestUpdateOptions{
			RepositoryID:      pr.Repository.ID,
			PullRequestID:     pr.ID,
			CompletionOptions: desiredOptions,
			EnforcePolicies:   true,
		})
		if err != nil {
			return prGovernanceResult{}, err
		}
		return validateAutoCompleteResult(updated, pr.ID, "auto-complete", "")
	}

	identity, err := client.FetchAuthenticatedIdentity(ctx)
	if err != nil {
		return prGovernanceResult{}, err
	}
	if identity == nil || strings.TrimSpace(identity.ID) == "" {
		return prGovernanceResult{}, fmt.Errorf("authenticated Azure DevOps user is missing identity ID")
	}
	updated, err := client.UpdatePullRequest(ctx, ado.PullRequestUpdateOptions{
		RepositoryID:           pr.Repository.ID,
		PullRequestID:          pr.ID,
		CompletionOptions:      desiredOptions,
		EnforcePolicies:        true,
		AutoCompleteMode:       ado.PullRequestAutoCompleteSet,
		AutoCompleteIdentityID: strings.TrimSpace(identity.ID),
	})
	if err != nil {
		return prGovernanceResult{}, err
	}
	return validateAutoCompleteResult(updated, pr.ID, "auto-complete", strings.TrimSpace(identity.ID))
}

func validateAutoCompleteResult(updated *ado.PullRequest, pullRequestID int, action, expectedSetterID string) (prGovernanceResult, error) {
	if err := validateLifecycleResponseID(updated, pullRequestID); err != nil {
		return prGovernanceResult{}, err
	}
	status := strings.ToLower(strings.TrimSpace(updated.Status))
	switch {
	case status == "completed":
		enabled := false
		return lifecycleResult(updated, action, &enabled), nil
	case status == "active" && autoCompleteEnabled(updated):
		if expectedSetterID != "" && !strings.EqualFold(strings.TrimSpace(updated.AutoCompleteSetBy.ID), expectedSetterID) {
			return prGovernanceResult{}, fmt.Errorf("Azure DevOps pull request %d response auto-complete setter %q does not match authenticated user %q", pullRequestID, updated.AutoCompleteSetBy.ID, expectedSetterID)
		}
		enabled := true
		return lifecycleResult(updated, action, &enabled), nil
	default:
		return prGovernanceResult{}, fmt.Errorf("Azure DevOps pull request %d response did not prove auto-complete scheduling or completion", pullRequestID)
	}
}

func executePRCancelAutoComplete(ctx context.Context, client ADOClient, pr *ado.PullRequest, status string) (prGovernanceResult, error) {
	if status != "active" {
		return prGovernanceResult{}, fmt.Errorf("pull request %d has status %s and cannot cancel auto-complete", pr.ID, status)
	}
	disabled := false
	if !autoCompleteEnabled(pr) {
		return lifecycleResult(pr, "unchanged", &disabled), nil
	}
	updated, err := client.UpdatePullRequest(ctx, ado.PullRequestUpdateOptions{
		RepositoryID:     pr.Repository.ID,
		PullRequestID:    pr.ID,
		AutoCompleteMode: ado.PullRequestAutoCompleteClear,
	})
	if err != nil {
		return prGovernanceResult{}, err
	}
	if err := validateLifecycleResponse(updated, pr.ID, "active"); err != nil {
		return prGovernanceResult{}, err
	}
	if updated.AutoCompleteSetBy != nil {
		return prGovernanceResult{}, fmt.Errorf("Azure DevOps pull request %d response still contains an auto-complete setter", pr.ID)
	}
	return lifecycleResult(updated, "cancel-auto-complete", &disabled), nil
}

func executePRAbandon(ctx context.Context, client ADOClient, pr *ado.PullRequest, status string) (prGovernanceResult, error) {
	if status == "abandoned" {
		return lifecycleResult(pr, "unchanged", nil), nil
	}
	if status == "completed" {
		return prGovernanceResult{}, fmt.Errorf("pull request %d is completed and cannot be abandoned", pr.ID)
	}
	abandoned := "abandoned"
	updated, err := client.UpdatePullRequest(ctx, ado.PullRequestUpdateOptions{
		RepositoryID:  pr.Repository.ID,
		PullRequestID: pr.ID,
		Status:        &abandoned,
	})
	if err != nil {
		return prGovernanceResult{}, err
	}
	if err := validateLifecycleResponse(updated, pr.ID, "abandoned"); err != nil {
		return prGovernanceResult{}, err
	}
	return lifecycleResult(updated, "abandon", nil), nil
}

func executePRVote(ctx context.Context, client ADOClient, pr *ado.PullRequest, status, action string, vote int) (prGovernanceResult, error) {
	if status != "active" {
		return prGovernanceResult{}, fmt.Errorf("pull request %d has status %s and cannot be voted on", pr.ID, status)
	}
	identity, err := client.FetchAuthenticatedIdentity(ctx)
	if err != nil {
		return prGovernanceResult{}, err
	}
	if identity == nil || strings.TrimSpace(identity.ID) == "" {
		return prGovernanceResult{}, fmt.Errorf("authenticated Azure DevOps user is missing identity ID")
	}
	reviewerID := strings.TrimSpace(identity.ID)
	required := false
	for _, reviewer := range pr.Reviewers {
		if strings.EqualFold(strings.TrimSpace(reviewer.ID), reviewerID) {
			required = reviewer.IsRequired
			if reviewer.Vote == vote {
				return voteResult(pr, "unchanged", reviewerID, vote), nil
			}
			break
		}
	}
	updated, err := client.SetPullRequestReviewerVote(ctx, ado.PullRequestReviewerVoteOptions{
		RepositoryID:  pr.Repository.ID,
		PullRequestID: pr.ID,
		ReviewerID:    reviewerID,
		Vote:          vote,
		IsRequired:    required,
	})
	if err != nil {
		return prGovernanceResult{}, err
	}
	if updated == nil || !strings.EqualFold(strings.TrimSpace(updated.ID), reviewerID) || updated.Vote != vote {
		return prGovernanceResult{}, fmt.Errorf("Azure DevOps pull request %d reviewer response did not prove authenticated vote %d", pr.ID, vote)
	}
	if updated.IsRequired != required {
		return prGovernanceResult{}, fmt.Errorf("Azure DevOps pull request %d reviewer response did not preserve required reviewer designation", pr.ID)
	}
	return voteResult(pr, action, reviewerID, vote), nil
}

func validateLifecycleResponse(updated *ado.PullRequest, pullRequestID int, expectedStatus string) error {
	if err := validateLifecycleResponseID(updated, pullRequestID); err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(updated.Status), expectedStatus) {
		return fmt.Errorf("Azure DevOps pull request %d response status %q does not prove %s", pullRequestID, updated.Status, expectedStatus)
	}
	return nil
}

func validateLifecycleResponseID(updated *ado.PullRequest, pullRequestID int) error {
	if updated == nil {
		return fmt.Errorf("Azure DevOps pull request %d update returned no pull request", pullRequestID)
	}
	if updated.ID != pullRequestID {
		return fmt.Errorf("Azure DevOps pull request response ID %d does not match requested ID %d", updated.ID, pullRequestID)
	}
	return nil
}

func lifecycleResult(pr *ado.PullRequest, action string, autoCompleteEnabled *bool) prGovernanceResult {
	result := prGovernanceResult{
		PullRequestID:       pr.ID,
		Action:              action,
		Status:              strings.ToLower(strings.TrimSpace(pr.Status)),
		AutoCompleteEnabled: autoCompleteEnabled,
	}
	if action == "complete" || action == "auto-complete" || (action == "unchanged" && result.Status == "completed") {
		result.MergeStatus = pr.MergeStatus
	}
	return result
}

func voteResult(pr *ado.PullRequest, action, reviewerID string, vote int) prGovernanceResult {
	return prGovernanceResult{
		PullRequestID: pr.ID,
		Action:        action,
		Status:        strings.ToLower(strings.TrimSpace(pr.Status)),
		ReviewerID:    reviewerID,
		Vote:          &vote,
	}
}

func autoCompleteEnabled(pr *ado.PullRequest) bool {
	if pr == nil || pr.AutoCompleteSetBy == nil {
		return false
	}
	id := strings.TrimSpace(pr.AutoCompleteSetBy.ID)
	return id != "" && !strings.EqualFold(id, clearedAutoCompleteIdentityID)
}

func mergedSafeCompletionOptions(current *ado.PullRequestCompletionOptions, supplied completionPreferenceArgs) *ado.PullRequestCompletionOptions {
	result := safeCompletionOptions(current)
	if result == nil {
		result = &ado.PullRequestCompletionOptions{}
	}
	if supplied.mergeStrategy != nil {
		result.MergeStrategy = stringPointer(*supplied.mergeStrategy)
	}
	if supplied.deleteSourceBranch != nil {
		result.DeleteSourceBranch = boolPointerValue(*supplied.deleteSourceBranch)
	}
	if supplied.transitionWorkItems != nil {
		result.TransitionWorkItems = boolPointerValue(*supplied.transitionWorkItems)
	}
	if supplied.mergeCommitMessage != nil {
		result.MergeCommitMessage = stringPointer(*supplied.mergeCommitMessage)
	}
	if completionOptionsEmpty(result) {
		return nil
	}
	return result
}

func safeCompletionOptions(current *ado.PullRequestCompletionOptions) *ado.PullRequestCompletionOptions {
	if current == nil {
		return nil
	}
	result := &ado.PullRequestCompletionOptions{}
	if current.MergeStrategy != nil {
		result.MergeStrategy = stringPointer(*current.MergeStrategy)
	} else if current.SquashMerge != nil && *current.SquashMerge {
		result.MergeStrategy = stringPointer("squash")
	}
	if current.DeleteSourceBranch != nil {
		result.DeleteSourceBranch = boolPointerValue(*current.DeleteSourceBranch)
	}
	if current.TransitionWorkItems != nil {
		result.TransitionWorkItems = boolPointerValue(*current.TransitionWorkItems)
	}
	if current.MergeCommitMessage != nil {
		result.MergeCommitMessage = stringPointer(*current.MergeCommitMessage)
	}
	if completionOptionsEmpty(result) {
		return nil
	}
	return result
}

func hasCompletionPolicyOverrides(options *ado.PullRequestCompletionOptions) bool {
	return options != nil && (options.BypassPolicy != nil && *options.BypassPolicy || len(options.AutoCompleteIgnoreConfigIDs) > 0)
}

func equalSafeCompletionOptions(left, right *ado.PullRequestCompletionOptions) bool {
	return equalOptionalString(optionMergeStrategy(left), optionMergeStrategy(right)) &&
		equalOptionalBool(optionDeleteSourceBranch(left), optionDeleteSourceBranch(right)) &&
		equalOptionalBool(optionTransitionWorkItems(left), optionTransitionWorkItems(right)) &&
		equalOptionalString(optionMergeCommitMessage(left), optionMergeCommitMessage(right))
}

func completionOptionsEmpty(options *ado.PullRequestCompletionOptions) bool {
	return options == nil || options.MergeStrategy == nil && options.DeleteSourceBranch == nil && options.TransitionWorkItems == nil && options.MergeCommitMessage == nil
}

func optionMergeStrategy(options *ado.PullRequestCompletionOptions) *string {
	if options == nil {
		return nil
	}
	return options.MergeStrategy
}

func optionDeleteSourceBranch(options *ado.PullRequestCompletionOptions) *bool {
	if options == nil {
		return nil
	}
	return options.DeleteSourceBranch
}

func optionTransitionWorkItems(options *ado.PullRequestCompletionOptions) *bool {
	if options == nil {
		return nil
	}
	return options.TransitionWorkItems
}

func optionMergeCommitMessage(options *ado.PullRequestCompletionOptions) *string {
	if options == nil {
		return nil
	}
	return options.MergeCommitMessage
}

func equalOptionalString(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func equalOptionalBool(left, right *bool) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func stringPointer(value string) *string {
	return &value
}

func boolPointerValue(value bool) *bool {
	return &value
}
