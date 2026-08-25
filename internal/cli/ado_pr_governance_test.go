package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestRunADOPullRequestGovernanceActions(t *testing.T) {
	tests := []struct {
		name             string
		verb             string
		fetched          *ado.PullRequest
		updated          *ado.PullRequest
		identity         *ado.IdentityRef
		voted            *ado.PullRequestReviewer
		wantUpdate       int
		wantIdentity     int
		wantVote         int
		wantStatus       string
		wantAutoMode     ado.PullRequestAutoCompleteMode
		wantVoteValue    int
		wantRequired     bool
		wantJSONAction   string
		wantJSONStatus   string
		wantJSONAuto     *bool
		wantJSONReviewer string
	}{
		{
			name: "complete active", verb: "complete",
			fetched:        governancePR("active"),
			updated:        governancePR("completed"),
			wantUpdate:     1,
			wantStatus:     "completed",
			wantJSONAction: "complete",
			wantJSONStatus: "completed",
		},
		{
			name: "complete already completed", verb: "complete",
			fetched:        governancePR("completed"),
			wantJSONAction: "unchanged",
			wantJSONStatus: "completed",
		},
		{
			name: "enable auto complete", verb: "auto-complete",
			fetched:        governancePR("active"),
			updated:        governancePRWithSetter("active", "caller-id"),
			identity:       &ado.IdentityRef{ID: "caller-id"},
			wantUpdate:     1,
			wantIdentity:   1,
			wantAutoMode:   ado.PullRequestAutoCompleteSet,
			wantJSONAction: "auto-complete",
			wantJSONStatus: "active",
			wantJSONAuto:   boolPointer(true),
		},
		{
			name: "auto complete finishes immediately", verb: "auto-complete",
			fetched:        governancePR("active"),
			updated:        governancePR("completed"),
			identity:       &ado.IdentityRef{ID: "caller-id"},
			wantUpdate:     1,
			wantIdentity:   1,
			wantAutoMode:   ado.PullRequestAutoCompleteSet,
			wantJSONAction: "auto-complete",
			wantJSONStatus: "completed",
			wantJSONAuto:   boolPointer(false),
		},
		{
			name: "auto complete unchanged", verb: "auto-complete",
			fetched:        governancePRWithSetter("active", "some-user"),
			wantJSONAction: "unchanged",
			wantJSONStatus: "active",
			wantJSONAuto:   boolPointer(true),
		},
		{
			name: "cancel auto complete", verb: "cancel-auto-complete",
			fetched:        governancePRWithSetter("active", "some-user"),
			updated:        governancePR("active"),
			wantUpdate:     1,
			wantAutoMode:   ado.PullRequestAutoCompleteClear,
			wantJSONAction: "cancel-auto-complete",
			wantJSONStatus: "active",
			wantJSONAuto:   boolPointer(false),
		},
		{
			name: "cancel auto complete unchanged", verb: "cancel-auto-complete",
			fetched:        governancePR("active"),
			wantJSONAction: "unchanged",
			wantJSONStatus: "active",
			wantJSONAuto:   boolPointer(false),
		},
		{
			name: "abandon active", verb: "abandon",
			fetched:        governancePRWithSetter("active", "some-user"),
			updated:        governancePR("abandoned"),
			wantUpdate:     1,
			wantStatus:     "abandoned",
			wantJSONAction: "abandon",
			wantJSONStatus: "abandoned",
		},
		{
			name: "abandon unchanged", verb: "abandon",
			fetched:        governancePR("abandoned"),
			wantJSONAction: "unchanged",
			wantJSONStatus: "abandoned",
		},
		{
			name: "approve", verb: "approve",
			fetched:          governancePR("active"),
			identity:         &ado.IdentityRef{ID: "caller-id"},
			voted:            &ado.PullRequestReviewer{ID: "caller-id", Vote: 10},
			wantIdentity:     1,
			wantVote:         1,
			wantVoteValue:    10,
			wantJSONAction:   "approve",
			wantJSONStatus:   "active",
			wantJSONReviewer: "caller-id",
		},
		{
			name: "approve with suggestions", verb: "approve-with-suggestions",
			fetched:          governancePR("active"),
			identity:         &ado.IdentityRef{ID: "caller-id"},
			voted:            &ado.PullRequestReviewer{ID: "caller-id", Vote: 5},
			wantIdentity:     1,
			wantVote:         1,
			wantVoteValue:    5,
			wantJSONAction:   "approve-with-suggestions",
			wantJSONStatus:   "active",
			wantJSONReviewer: "caller-id",
		},
		{
			name: "reject required reviewer", verb: "reject",
			fetched:          governancePRWithReviewer("active", ado.PullRequestReviewer{ID: "CALLER-ID", Vote: 0, IsRequired: true}),
			identity:         &ado.IdentityRef{ID: "caller-id"},
			voted:            &ado.PullRequestReviewer{ID: "caller-id", Vote: -10, IsRequired: true},
			wantIdentity:     1,
			wantVote:         1,
			wantVoteValue:    -10,
			wantRequired:     true,
			wantJSONAction:   "reject",
			wantJSONStatus:   "active",
			wantJSONReviewer: "caller-id",
		},
		{
			name: "vote unchanged", verb: "approve",
			fetched:          governancePRWithReviewer("active", ado.PullRequestReviewer{ID: "caller-id", Vote: 10, IsRequired: true}),
			identity:         &ado.IdentityRef{ID: "caller-id"},
			wantIdentity:     1,
			wantVoteValue:    10,
			wantJSONAction:   "unchanged",
			wantJSONStatus:   "active",
			wantJSONReviewer: "caller-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakePRMaintenanceClient{
				fetched:               tt.fetched,
				updated:               tt.updated,
				authenticatedIdentity: tt.identity,
				votedReviewer:         tt.voted,
			}
			runner := prThreadTestRunner(t, client)
			var stdout, stderr bytes.Buffer
			err := runner.Run([]string{"ado", "pr", tt.verb, "42", "--json"}, strings.NewReader(""), &stdout, &stderr)
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
			if stderr.String() != "" {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
			if client.fetchPRCalled != 1 || client.updateCalled != tt.wantUpdate || client.identityCalled != tt.wantIdentity || client.voteCalled != tt.wantVote {
				t.Fatalf("calls fetch/update/identity/vote = %d/%d/%d/%d, want 1/%d/%d/%d", client.fetchPRCalled, client.updateCalled, client.identityCalled, client.voteCalled, tt.wantUpdate, tt.wantIdentity, tt.wantVote)
			}
			if tt.wantStatus != "" && (client.updateOpts.Status == nil || *client.updateOpts.Status != tt.wantStatus) {
				t.Fatalf("update status = %v, want %q", client.updateOpts.Status, tt.wantStatus)
			}
			if client.updateCalled > 0 && client.updateOpts.AutoCompleteMode != tt.wantAutoMode {
				t.Fatalf("auto-complete mode = %v, want %v", client.updateOpts.AutoCompleteMode, tt.wantAutoMode)
			}
			if tt.wantVote > 0 && (client.voteOpts.Vote != tt.wantVoteValue || client.voteOpts.IsRequired != tt.wantRequired || client.voteOpts.ReviewerID != "caller-id") {
				t.Fatalf("vote opts = %+v, want vote %d required %v authenticated caller", client.voteOpts, tt.wantVoteValue, tt.wantRequired)
			}
			var result map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("stdout = %q, want JSON: %v", stdout.String(), err)
			}
			if result["pullRequestId"] != float64(42) || result["action"] != tt.wantJSONAction || result["status"] != tt.wantJSONStatus {
				t.Fatalf("result = %+v, want ID/action/current status", result)
			}
			if (tt.verb == "complete" || tt.verb == "auto-complete") && result["mergeStatus"] != "queued" {
				t.Fatalf("mergeStatus = %v, want returned queued state", result["mergeStatus"])
			}
			if tt.wantJSONAuto != nil && result["autoCompleteEnabled"] != *tt.wantJSONAuto {
				t.Fatalf("autoCompleteEnabled = %v, want %v", result["autoCompleteEnabled"], *tt.wantJSONAuto)
			}
			if tt.wantJSONReviewer != "" && (result["reviewerId"] != tt.wantJSONReviewer || result["vote"] != float64(tt.wantVoteValue)) {
				t.Fatalf("vote result = %+v, want reviewer/vote", result)
			}
		})
	}
}

func TestRunADOPullRequestGovernanceCompletionPreferences(t *testing.T) {
	legacySquash := true
	deleteSource := false
	transition := true
	oldMessage := "old"
	client := &fakePRMaintenanceClient{
		fetched: governancePRWithOptions("active", &ado.PullRequestCompletionOptions{
			SquashMerge:         &legacySquash,
			DeleteSourceBranch:  &deleteSource,
			TransitionWorkItems: &transition,
			MergeCommitMessage:  &oldMessage,
			BypassPolicy:        boolPointer(true),
		}),
		updated: governancePR("completed"),
	}
	runner := prThreadTestRunner(t, client)
	loadConfig := runner.deps.LoadConfig
	runner.deps.LoadConfig = func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
		if requestedProfile != "company-cloud" || scope != config.GlobalScope {
			t.Fatalf("profile/scope = %q/%v, want company-cloud/global", requestedProfile, scope)
		}
		return loadConfig(repoRoot, homeDir, requestedProfile, scope)
	}
	var stdout bytes.Buffer
	err := runner.Run([]string{
		"ado", "pr", "complete", "42",
		"--merge-strategy", "rebase-merge",
		"--delete-source-branch", "true",
		"--transition-work-items", "false",
		"--merge-commit-message", "new message",
		"--profile", "company-cloud",
		"--global",
	}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "42\n" {
		t.Fatalf("stdout = %q, want PR ID only", stdout.String())
	}
	if client.updateOpts.RepositoryID != "repo-uuid" || client.updateOpts.PullRequestID != 42 || client.updateOpts.LastMergeSourceCommit == nil || client.updateOpts.LastMergeSourceCommit.CommitID != "source-sha" {
		t.Fatalf("update identity/source = %+v, want fetched repo/PR/source", client.updateOpts)
	}
	options := client.updateOpts.CompletionOptions
	if options == nil || options.MergeStrategy == nil || *options.MergeStrategy != "rebaseMerge" || options.DeleteSourceBranch == nil || !*options.DeleteSourceBranch || options.TransitionWorkItems == nil || *options.TransitionWorkItems || options.MergeCommitMessage == nil || *options.MergeCommitMessage != "new message" {
		t.Fatalf("completion options = %+v, want explicit overlays", options)
	}
	if options.SquashMerge != nil || options.BypassPolicy != nil || options.BypassReason != nil || options.TriggeredByAutoComplete != nil || options.AutoCompleteIgnoreConfigIDs != nil {
		t.Fatalf("unsafe/deprecated options leaked: %+v", options)
	}
}

func TestRunADOPullRequestGovernancePreservesOmittedCompletionPreferences(t *testing.T) {
	legacySquash := true
	deleteSource := false
	transition := true
	message := "keep"
	client := &fakePRMaintenanceClient{
		fetched: governancePRWithOptions("active", &ado.PullRequestCompletionOptions{
			SquashMerge:         &legacySquash,
			DeleteSourceBranch:  &deleteSource,
			TransitionWorkItems: &transition,
			MergeCommitMessage:  &message,
		}),
		updated: governancePR("completed"),
	}
	err := prThreadTestRunner(t, client).Run([]string{"ado", "pr", "complete", "42"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	options := client.updateOpts.CompletionOptions
	if options == nil || options.MergeStrategy == nil || *options.MergeStrategy != "squash" || options.DeleteSourceBranch == nil || *options.DeleteSourceBranch || options.TransitionWorkItems == nil || !*options.TransitionWorkItems || options.MergeCommitMessage == nil || *options.MergeCommitMessage != "keep" {
		t.Fatalf("completion options = %+v, want safe fetched preferences with legacy squash normalized", options)
	}
}

func TestRunADOPullRequestGovernanceUpdatesScheduledPreferencesWithoutIdentityLookup(t *testing.T) {
	strategy := "squash"
	fetched := governancePRWithSetter("active", "setter-id")
	fetched.CompletionOptions = &ado.PullRequestCompletionOptions{MergeStrategy: &strategy}
	updated := governancePRWithSetter("active", "setter-id")
	client := &fakePRMaintenanceClient{fetched: fetched, updated: updated}
	var stdout bytes.Buffer
	err := prThreadTestRunner(t, client).Run([]string{"ado", "pr", "auto-complete", "42", "--merge-strategy", "rebase", "--json"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if client.identityCalled != 0 || client.updateCalled != 1 || client.updateOpts.AutoCompleteMode != ado.PullRequestAutoCompleteUnchanged || client.updateOpts.CompletionOptions == nil || client.updateOpts.CompletionOptions.MergeStrategy == nil || *client.updateOpts.CompletionOptions.MergeStrategy != "rebase" {
		t.Fatalf("calls/options = identity %d update %d %+v, want preference-only update", client.identityCalled, client.updateCalled, client.updateOpts)
	}
}

func TestRunADOPullRequestGovernanceSanitizesScheduledPolicyOverrides(t *testing.T) {
	bypass := true
	bypassReason := "outside Adomi"
	fetched := governancePRWithSetter("active", "setter-id")
	fetched.CompletionOptions = &ado.PullRequestCompletionOptions{
		BypassPolicy:                &bypass,
		BypassReason:                &bypassReason,
		AutoCompleteIgnoreConfigIDs: []int{17},
	}
	updated := governancePRWithSetter("active", "setter-id")
	client := &fakePRMaintenanceClient{fetched: fetched, updated: updated}
	var stdout bytes.Buffer
	err := prThreadTestRunner(t, client).Run([]string{"ado", "pr", "auto-complete", "42", "--json"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if client.identityCalled != 0 || client.updateCalled != 1 || client.updateOpts.AutoCompleteMode != ado.PullRequestAutoCompleteUnchanged || !client.updateOpts.EnforcePolicies {
		t.Fatalf("calls/options = identity %d update %d %+v, want policy-only sanitizing update", client.identityCalled, client.updateCalled, client.updateOpts)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout = %q, want JSON: %v", stdout.String(), err)
	}
	if result["action"] != "auto-complete" || result["autoCompleteEnabled"] != true {
		t.Fatalf("result = %+v, want performed scheduled update", result)
	}
}

func TestRunADOPullRequestGovernanceValidationBeforeCredentials(t *testing.T) {
	tests := [][]string{
		{"ado", "pr", "complete"},
		{"ado", "pr", "complete", "0"},
		{"ado", "pr", "complete", "-1"},
		{"ado", "pr", "complete", "nope"},
		{"ado", "pr", "complete", "42", "43"},
		{"ado", "pr", "complete", "42", "--unknown"},
		{"ado", "pr", "complete", "42", "--merge-strategy", "merge-commit"},
		{"ado", "pr", "complete", "42", "--delete-source-branch", "yes"},
		{"ado", "pr", "complete", "42", "--transition-work-items", "1"},
		{"ado", "pr", "complete", "42", "--merge-commit-message", "   "},
		{"ado", "pr", "abandon", "42", "--merge-strategy", "squash"},
		{"ado", "pr", "approve", "42", "--reviewer", "someone"},
		{"ado", "pr", "reject", "42", "--vote", "-10"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args[2:], "_"), func(t *testing.T) {
			runner := prThreadTestRunner(t, &fakePRMaintenanceClient{})
			runner.deps.PATStore = failOnGetPATStore{t: t}
			var stdout bytes.Buffer
			if err := runner.Run(args, strings.NewReader(""), &stdout, &bytes.Buffer{}); err == nil {
				t.Fatalf("Run(%v) error = nil, want validation error", args)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestRunADOPullRequestGovernanceRejectsUnsafeStatesAndUnprovenResponses(t *testing.T) {
	tests := []struct {
		name         string
		verb         string
		fetched      *ado.PullRequest
		updated      *ado.PullRequest
		identity     *ado.IdentityRef
		updateErr    error
		voteErr      error
		wantUpdate   int
		wantIdentity int
		wantVote     int
	}{
		{name: "complete abandoned", verb: "complete", fetched: governancePR("abandoned")},
		{name: "complete draft", verb: "complete", fetched: governanceDraftPR()},
		{name: "complete missing repository", verb: "complete", fetched: governancePRWithoutRepository("active")},
		{name: "complete missing source", verb: "complete", fetched: governancePRWithoutSource("active")},
		{name: "complete source race", verb: "complete", fetched: governancePR("active"), updateErr: errors.New("source commit changed"), wantUpdate: 1},
		{name: "complete unproven status", verb: "complete", fetched: governancePR("active"), updated: governancePR("active"), wantUpdate: 1},
		{name: "auto complete completed", verb: "auto-complete", fetched: governancePR("completed")},
		{name: "auto complete abandoned", verb: "auto-complete", fetched: governancePR("abandoned")},
		{name: "auto complete draft", verb: "auto-complete", fetched: governanceDraftPR()},
		{name: "auto complete missing identity", verb: "auto-complete", fetched: governancePR("active"), identity: &ado.IdentityRef{}, wantIdentity: 1},
		{name: "auto complete unproven", verb: "auto-complete", fetched: governancePR("active"), identity: &ado.IdentityRef{ID: "caller-id"}, updated: governancePR("active"), wantUpdate: 1, wantIdentity: 1},
		{name: "cancel completed", verb: "cancel-auto-complete", fetched: governancePRWithSetter("completed", "setter")},
		{name: "cancel abandoned", verb: "cancel-auto-complete", fetched: governancePRWithSetter("abandoned", "setter")},
		{name: "cancel setter remains", verb: "cancel-auto-complete", fetched: governancePRWithSetter("active", "setter"), updated: governancePRWithSetter("active", "setter"), wantUpdate: 1},
		{name: "cancel empty UUID setter remains", verb: "cancel-auto-complete", fetched: governancePRWithSetter("active", "setter"), updated: governancePRWithSetter("active", "00000000-0000-0000-0000-000000000000"), wantUpdate: 1},
		{name: "abandon completed", verb: "abandon", fetched: governancePR("completed")},
		{name: "abandon unproven", verb: "abandon", fetched: governancePR("active"), updated: governancePR("active"), wantUpdate: 1},
		{name: "vote completed", verb: "approve", fetched: governancePR("completed")},
		{name: "vote abandoned", verb: "reject", fetched: governancePR("abandoned")},
		{name: "vote missing identity", verb: "approve", fetched: governancePR("active"), identity: &ado.IdentityRef{}, wantIdentity: 1},
		{name: "vote write failure", verb: "reject", fetched: governancePR("active"), identity: &ado.IdentityRef{ID: "caller-id"}, voteErr: errors.New("vote rejected"), wantIdentity: 1, wantVote: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakePRMaintenanceClient{fetched: tt.fetched, updated: tt.updated, authenticatedIdentity: tt.identity, updateErr: tt.updateErr, voteErr: tt.voteErr}
			var stdout bytes.Buffer
			err := prThreadTestRunner(t, client).Run([]string{"ado", "pr", tt.verb, "42"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil {
				t.Fatal("Run error = nil, want failure")
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if client.updateCalled != tt.wantUpdate || client.identityCalled != tt.wantIdentity || client.voteCalled != tt.wantVote {
				t.Fatalf("calls update/identity/vote = %d/%d/%d, want %d/%d/%d", client.updateCalled, client.identityCalled, client.voteCalled, tt.wantUpdate, tt.wantIdentity, tt.wantVote)
			}
		})
	}
}

func TestRunADOPullRequestGovernanceRejectsMismatchedPreflightID(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: governancePR("active")}
	client.fetched.ID = 43
	var stdout bytes.Buffer
	err := prThreadTestRunner(t, client).Run([]string{"ado", "pr", "abandon", "42"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || client.updateCalled != 0 || stdout.String() != "" {
		t.Fatalf("error/update/stdout = %v/%d/%q, want fail closed before update", err, client.updateCalled, stdout.String())
	}
}

func TestRunADOPullRequestGovernanceRejectsUnprovenMutationIdentity(t *testing.T) {
	tests := []struct {
		name     string
		verb     string
		fetched  *ado.PullRequest
		updated  *ado.PullRequest
		identity *ado.IdentityRef
		voted    *ado.PullRequestReviewer
	}{
		{name: "complete response PR", verb: "complete", fetched: governancePR("active"), updated: governancePR("completed")},
		{name: "auto response PR", verb: "auto-complete", fetched: governancePR("active"), updated: governancePRWithSetter("active", "caller-id"), identity: &ado.IdentityRef{ID: "caller-id"}},
		{name: "cancel response PR", verb: "cancel-auto-complete", fetched: governancePRWithSetter("active", "setter"), updated: governancePR("active")},
		{name: "abandon response PR", verb: "abandon", fetched: governancePR("active"), updated: governancePR("abandoned")},
		{name: "vote response reviewer", verb: "approve", fetched: governancePR("active"), identity: &ado.IdentityRef{ID: "caller-id"}, voted: &ado.PullRequestReviewer{ID: "other-id", Vote: 10}},
		{name: "vote response value", verb: "reject", fetched: governancePR("active"), identity: &ado.IdentityRef{ID: "caller-id"}, voted: &ado.PullRequestReviewer{ID: "caller-id", Vote: 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.updated != nil {
				tt.updated.ID = 43
			}
			client := &fakePRMaintenanceClient{fetched: tt.fetched, updated: tt.updated, authenticatedIdentity: tt.identity, votedReviewer: tt.voted}
			var stdout bytes.Buffer
			err := prThreadTestRunner(t, client).Run([]string{"ado", "pr", tt.verb, "42"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || stdout.String() != "" {
				t.Fatalf("error/stdout = %v/%q, want fail closed", err, stdout.String())
			}
		})
	}
}

func TestRunADOPullRequestGovernanceRejectsWrongAutoCompleteSetter(t *testing.T) {
	client := &fakePRMaintenanceClient{
		fetched:               governancePR("active"),
		updated:               governancePRWithSetter("active", "other-id"),
		authenticatedIdentity: &ado.IdentityRef{ID: "caller-id"},
	}
	var stdout bytes.Buffer
	err := prThreadTestRunner(t, client).Run([]string{"ado", "pr", "auto-complete", "42"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || stdout.String() != "" {
		t.Fatalf("error/stdout = %v/%q, want setter mismatch failure", err, stdout.String())
	}
}

func TestRunADOPullRequestGovernanceRejectsChangedRequiredReviewerFlag(t *testing.T) {
	client := &fakePRMaintenanceClient{
		fetched:               governancePR("active"),
		authenticatedIdentity: &ado.IdentityRef{ID: "caller-id"},
		votedReviewer:         &ado.PullRequestReviewer{ID: "caller-id", Vote: 10, IsRequired: true},
	}
	var stdout bytes.Buffer
	err := prThreadTestRunner(t, client).Run([]string{"ado", "pr", "approve", "42"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || stdout.String() != "" {
		t.Fatalf("error/stdout = %v/%q, want required-reviewer mismatch failure", err, stdout.String())
	}
}

func governancePR(status string) *ado.PullRequest {
	return &ado.PullRequest{
		ID:                    42,
		Status:                status,
		MergeStatus:           "queued",
		Repository:            ado.PullRequestRepo{ID: "repo-uuid"},
		LastMergeSourceCommit: &ado.GitCommitRef{CommitID: "source-sha"},
	}
}

func governancePRWithSetter(status, identityID string) *ado.PullRequest {
	pr := governancePR(status)
	pr.AutoCompleteSetBy = &ado.IdentityRef{ID: identityID}
	return pr
}

func governancePRWithReviewer(status string, reviewer ado.PullRequestReviewer) *ado.PullRequest {
	pr := governancePR(status)
	pr.Reviewers = []ado.PullRequestReviewer{reviewer}
	return pr
}

func governancePRWithOptions(status string, options *ado.PullRequestCompletionOptions) *ado.PullRequest {
	pr := governancePR(status)
	pr.CompletionOptions = options
	return pr
}

func governanceDraftPR() *ado.PullRequest {
	pr := governancePR("active")
	pr.IsDraft = true
	return pr
}

func governancePRWithoutRepository(status string) *ado.PullRequest {
	pr := governancePR(status)
	pr.Repository.ID = ""
	return pr
}

func governancePRWithoutSource(status string) *ado.PullRequest {
	pr := governancePR(status)
	pr.LastMergeSourceCommit = nil
	return pr
}

func boolPointer(value bool) *bool {
	return &value
}
