package ado

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientFetchPullRequestBuildsURLAndAuth(t *testing.T) {
	var seenPath string
	var seenAPIVersion string
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"pullRequestId":42,"title":"PR title","repository":{"id":"repo-uuid","name":"adomi","project":{"id":"project-uuid","name":"My Project"}}}`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.Client(), ClientConfig{
		BaseURL:    server.URL + "/tfs/DefaultCollection",
		Project:    "My Project",
		APIVersion: "7.0",
		PAT:        "secret",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	pr, err := client.FetchPullRequest(context.Background(), 42)
	if err != nil {
		t.Fatalf("FetchPullRequest returned error: %v", err)
	}
	if pr.ID != 42 {
		t.Fatalf("PR ID = %d, want 42", pr.ID)
	}
	if pr.Repository.ID != "repo-uuid" {
		t.Fatalf("repository ID = %q, want repo-uuid", pr.Repository.ID)
	}
	if pr.Repository.ProjectID() != "project-uuid" || pr.Repository.Project["name"] != "My Project" {
		t.Fatalf("repository project = %#v, want decoded ID and preserved metadata", pr.Repository.Project)
	}
	if seenPath != "/tfs/DefaultCollection/My%20Project/_apis/git/pullrequests/42" {
		t.Fatalf("path = %q, want pullrequests path", seenPath)
	}
	if seenAPIVersion != "7.0" {
		t.Fatalf("api-version = %q, want 7.0", seenAPIVersion)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
}

func TestClientFetchPullRequestGovernanceFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"pullRequestId":42,
			"status":"active",
			"autoCompleteSetBy":{"id":"user-1","displayName":"Ada"},
			"lastMergeSourceCommit":{"commitId":"abc123"},
			"completionOptions":{
				"mergeStrategy":"squash",
				"squashMerge":false,
				"deleteSourceBranch":false,
				"transitionWorkItems":true,
				"mergeCommitMessage":"Merge safely",
				"bypassPolicy":true,
				"bypassReason":"configured elsewhere",
				"autoCompleteIgnoreConfigIds":[17]
			},
			"reviewers":[{"id":"user-1","displayName":"Ada","vote":10,"isRequired":true}],
			"repository":{"id":"repo-uuid"}
		}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	pr, err := client.FetchPullRequest(context.Background(), 42)
	if err != nil {
		t.Fatalf("FetchPullRequest returned error: %v", err)
	}
	if pr.AutoCompleteSetBy == nil || pr.AutoCompleteSetBy.ID != "user-1" {
		t.Fatalf("auto-complete setter = %+v, want user-1", pr.AutoCompleteSetBy)
	}
	if pr.LastMergeSourceCommit == nil || pr.LastMergeSourceCommit.CommitID != "abc123" {
		t.Fatalf("last merge source commit = %+v, want abc123", pr.LastMergeSourceCommit)
	}
	if pr.CompletionOptions == nil || pr.CompletionOptions.MergeStrategy == nil || *pr.CompletionOptions.MergeStrategy != "squash" {
		t.Fatalf("completion options = %+v, want squash strategy", pr.CompletionOptions)
	}
	if pr.CompletionOptions.DeleteSourceBranch == nil || *pr.CompletionOptions.DeleteSourceBranch {
		t.Fatalf("delete source branch = %+v, want explicit false", pr.CompletionOptions.DeleteSourceBranch)
	}
	if pr.CompletionOptions.SquashMerge == nil || *pr.CompletionOptions.SquashMerge {
		t.Fatalf("legacy squash merge = %+v, want explicit false", pr.CompletionOptions.SquashMerge)
	}
	if pr.CompletionOptions.TransitionWorkItems == nil || !*pr.CompletionOptions.TransitionWorkItems {
		t.Fatalf("transition work items = %+v, want explicit true", pr.CompletionOptions.TransitionWorkItems)
	}
	if pr.CompletionOptions.BypassPolicy == nil || !*pr.CompletionOptions.BypassPolicy {
		t.Fatalf("bypass policy = %+v, want decoded true", pr.CompletionOptions.BypassPolicy)
	}
	if len(pr.CompletionOptions.AutoCompleteIgnoreConfigIDs) != 1 || pr.CompletionOptions.AutoCompleteIgnoreConfigIDs[0] != 17 {
		t.Fatalf("auto-complete ignore IDs = %+v, want 17", pr.CompletionOptions.AutoCompleteIgnoreConfigIDs)
	}
	if len(pr.Reviewers) != 1 || pr.Reviewers[0].ID != "user-1" || pr.Reviewers[0].Vote != 10 || !pr.Reviewers[0].IsRequired {
		t.Fatalf("reviewers = %+v, want typed required user-1 approval", pr.Reviewers)
	}
}

func TestClientFetchAuthenticatedIdentityBuildsOrganizationURLAndAuth(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"authenticatedUser":{"id":"user-1","displayName":"Ada"}}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	identity, err := client.FetchAuthenticatedIdentity(context.Background())
	if err != nil {
		t.Fatalf("FetchAuthenticatedIdentity returned error: %v", err)
	}
	if identity.ID != "user-1" || identity.DisplayName != "Ada" {
		t.Fatalf("authenticated identity = %+v, want user-1 Ada", identity)
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/_apis/connectionData" {
		t.Fatalf("path = %q, want organization connectionData path", seenPath)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
}

func TestClientFetchAuthenticatedIdentityRejectsMissingUserID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"authenticatedUser":{"displayName":"Ada"}}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchAuthenticatedIdentity(context.Background())
	if err == nil || !strings.Contains(err.Error(), "user ID") {
		t.Fatalf("FetchAuthenticatedIdentity error = %v, want missing user ID", err)
	}
}

func TestClientFetchPullRequestRejectsMismatchedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"pullRequestId":99,"repository":{"id":"x"}}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchPullRequest(context.Background(), 42)
	if err == nil {
		t.Fatal("FetchPullRequest error = nil, want ID mismatch error")
	}
	if !strings.Contains(err.Error(), "42") || !strings.Contains(err.Error(), "99") {
		t.Fatalf("error = %q, want both IDs", err.Error())
	}
}

func TestClientFetchPullRequestReturnsNon2xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchPullRequest(context.Background(), 42)
	if err == nil {
		t.Fatal("FetchPullRequest error = nil, want error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %q, want status code", err.Error())
	}
}

func TestClientListPullRequestsBuildsRepoScopedURLQueryAndAuth(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenAuth string
	var seenQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenAuth = r.Header.Get("Authorization")
		seenQuery = r.URL.RawQuery
		fmt.Fprint(w, `{"count":1,"value":[{"pullRequestId":43,"title":"Existing PR","sourceRefName":"refs/heads/feature/x","targetRefName":"refs/heads/main","repository":{"id":"repo-uuid","name":"adomi"}}]}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "My Project", APIVersion: "7.0", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	prs, err := client.ListPullRequests(context.Background(), PullRequestListOptions{
		RepositoryID:  "repo-uuid",
		SourceRefName: "refs/heads/feature/x",
		TargetRefName: "refs/heads/main",
		Status:        "active",
	})
	if err != nil {
		t.Fatalf("ListPullRequests returned error: %v", err)
	}
	if len(prs) != 1 || prs[0].ID != 43 || prs[0].Title != "Existing PR" {
		t.Fatalf("pull requests = %+v, want one existing PR", prs)
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/My%20Project/_apis/git/repositories/repo-uuid/pullrequests" {
		t.Fatalf("path = %q, want repo-scoped pullrequests path", seenPath)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
	for _, want := range []string{
		"api-version=7.0",
		"searchCriteria.sourceRefName=refs%2Fheads%2Ffeature%2Fx",
		"searchCriteria.targetRefName=refs%2Fheads%2Fmain",
		"searchCriteria.status=active",
	} {
		if !strings.Contains(seenQuery, want) {
			t.Fatalf("query = %q, want %q", seenQuery, want)
		}
	}
}

func TestClientListPullRequestsDefaultsStatusAndReturnsEmptySliceForNullValue(t *testing.T) {
	var seenStatus string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenStatus = r.URL.Query().Get("searchCriteria.status")
		fmt.Fprint(w, `{"count":0}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	prs, err := client.ListPullRequests(context.Background(), PullRequestListOptions{RepositoryID: "repo"})
	if err != nil {
		t.Fatalf("ListPullRequests returned error: %v", err)
	}
	if seenStatus != "active" {
		t.Fatalf("status query = %q, want active", seenStatus)
	}
	if prs == nil {
		t.Fatal("pull requests is nil, want empty slice")
	}
	if len(prs) != 0 {
		t.Fatalf("pull requests len = %d, want 0", len(prs))
	}
}

func TestClientCreatePullRequestBuildsURLAuthAndBody(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenAPIVersion string
	var seenAuth string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"pullRequestId":44,"title":"New PR","description":"Body","sourceRefName":"refs/heads/feature/x","targetRefName":"refs/heads/main","repository":{"id":"repo-uuid","name":"adomi"}}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	pr, err := client.CreatePullRequest(context.Background(), PullRequestCreateOptions{
		RepositoryID:  "repo-uuid",
		SourceRefName: "refs/heads/feature/x",
		TargetRefName: "refs/heads/main",
		Title:         "New PR",
		Description:   "Body",
	})
	if err != nil {
		t.Fatalf("CreatePullRequest returned error: %v", err)
	}
	if pr.ID != 44 || pr.Title != "New PR" {
		t.Fatalf("pull request = %+v, want created PR", pr)
	}
	if seenMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/MyProject/_apis/git/repositories/repo-uuid/pullrequests" {
		t.Fatalf("path = %q, want create pullrequests path", seenPath)
	}
	if seenAPIVersion != "7.1" {
		t.Fatalf("api-version = %q, want 7.1", seenAPIVersion)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
	for key, want := range map[string]any{"sourceRefName": "refs/heads/feature/x", "targetRefName": "refs/heads/main", "title": "New PR", "description": "Body"} {
		if body[key] != want {
			t.Fatalf("body[%s] = %#v, want %#v (body=%#v)", key, body[key], want, body)
		}
	}
}

func TestClientUpdatePullRequestBuildsURLAuthAndBody(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenAPIVersion string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenAPIVersion = r.URL.Query().Get("api-version")
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"pullRequestId":44,"title":"Updated","description":"Updated body","repository":{"id":"repo-uuid"}}`)
	}))
	t.Cleanup(server.Close)
	title := "Updated"
	description := "Updated body"
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	pr, err := client.UpdatePullRequest(context.Background(), PullRequestUpdateOptions{RepositoryID: "repo-uuid", PullRequestID: 44, Title: &title, Description: &description})
	if err != nil {
		t.Fatalf("UpdatePullRequest returned error: %v", err)
	}
	if pr.ID != 44 || pr.Title != "Updated" {
		t.Fatalf("pull request = %+v, want updated PR", pr)
	}
	if seenMethod != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/MyProject/_apis/git/repositories/repo-uuid/pullrequests/44" {
		t.Fatalf("path = %q, want update pullrequest path", seenPath)
	}
	if seenAPIVersion != "7.1" {
		t.Fatalf("api-version = %q, want 7.1", seenAPIVersion)
	}
	if body["title"] != title || body["description"] != description {
		t.Fatalf("body = %#v, want title and description", body)
	}
}

func TestClientUpdatePullRequestGovernanceCompletion(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"pullRequestId":44,"status":"completed","mergeStatus":"queued","repository":{"id":"repo-uuid"}}`)
	}))
	t.Cleanup(server.Close)
	status := "completed"
	mergeStrategy := "squash"
	deleteSourceBranch := false
	transitionWorkItems := true
	mergeCommitMessage := "Merge safely"
	bypassPolicy := true
	bypassReason := "must not be sent"
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	pr, err := client.UpdatePullRequest(context.Background(), PullRequestUpdateOptions{
		RepositoryID:          "repo-uuid",
		PullRequestID:         44,
		Status:                &status,
		LastMergeSourceCommit: &GitCommitRef{CommitID: "abc123"},
		CompletionOptions: &PullRequestCompletionOptions{
			MergeStrategy:       &mergeStrategy,
			DeleteSourceBranch:  &deleteSourceBranch,
			TransitionWorkItems: &transitionWorkItems,
			MergeCommitMessage:  &mergeCommitMessage,
			BypassPolicy:        &bypassPolicy,
			BypassReason:        &bypassReason,
		},
	})
	if err != nil {
		t.Fatalf("UpdatePullRequest returned error: %v", err)
	}
	if pr.Status != "completed" || pr.MergeStatus != "queued" {
		t.Fatalf("pull request = %+v, want completed queued response", pr)
	}
	if body["status"] != "completed" {
		t.Fatalf("body status = %#v, want completed", body["status"])
	}
	commit, ok := body["lastMergeSourceCommit"].(map[string]any)
	if !ok || commit["commitId"] != "abc123" {
		t.Fatalf("lastMergeSourceCommit = %#v, want commitId abc123", body["lastMergeSourceCommit"])
	}
	completion, ok := body["completionOptions"].(map[string]any)
	if !ok {
		t.Fatalf("completionOptions = %#v, want object", body["completionOptions"])
	}
	for key, want := range map[string]any{
		"mergeStrategy":       "squash",
		"deleteSourceBranch":  false,
		"transitionWorkItems": true,
		"mergeCommitMessage":  "Merge safely",
	} {
		if completion[key] != want {
			t.Fatalf("completionOptions[%s] = %#v, want %#v", key, completion[key], want)
		}
	}
	for _, forbidden := range []string{"bypassPolicy", "bypassReason", "triggeredByAutoComplete", "ignoreConfigIds"} {
		if _, exists := completion[forbidden]; exists {
			t.Fatalf("completionOptions contains forbidden %q: %#v", forbidden, completion)
		}
	}
}

func TestClientUpdatePullRequestGovernanceClearsPolicyOverrides(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"pullRequestId":44,"status":"completed","repository":{"id":"repo-uuid"}}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	status := "completed"
	if _, err := client.UpdatePullRequest(context.Background(), PullRequestUpdateOptions{
		RepositoryID:    "repo-uuid",
		PullRequestID:   44,
		Status:          &status,
		EnforcePolicies: true,
	}); err != nil {
		t.Fatalf("UpdatePullRequest returned error: %v", err)
	}
	completion, ok := body["completionOptions"].(map[string]any)
	if !ok {
		t.Fatalf("completionOptions = %#v, want policy-enforcement object", body["completionOptions"])
	}
	if completion["bypassPolicy"] != false {
		t.Fatalf("bypassPolicy = %#v, want false", completion["bypassPolicy"])
	}
	ignoreIDs, ok := completion["autoCompleteIgnoreConfigIds"].([]any)
	if !ok || len(ignoreIDs) != 0 {
		t.Fatalf("autoCompleteIgnoreConfigIds = %#v, want empty list", completion["autoCompleteIgnoreConfigIds"])
	}
	for _, forbidden := range []string{"bypassReason", "triggeredByAutoComplete", "squashMerge"} {
		if _, exists := completion[forbidden]; exists {
			t.Fatalf("completionOptions contains forbidden %q: %#v", forbidden, completion)
		}
	}
}

func TestClientUpdatePullRequestGovernanceAutoCompleteAndAbandon(t *testing.T) {
	tests := []struct {
		name         string
		opts         PullRequestUpdateOptions
		responseBody string
		assertBody   func(*testing.T, map[string]any)
	}{
		{
			name: "set auto-complete",
			opts: PullRequestUpdateOptions{
				RepositoryID:           "repo-uuid",
				PullRequestID:          44,
				AutoCompleteMode:       PullRequestAutoCompleteSet,
				AutoCompleteIdentityID: "user-1",
			},
			responseBody: `{"pullRequestId":44,"status":"active","autoCompleteSetBy":{"id":"user-1"},"repository":{"id":"repo-uuid"}}`,
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				setter, ok := body["autoCompleteSetBy"].(map[string]any)
				if !ok || setter["id"] != "user-1" {
					t.Fatalf("autoCompleteSetBy = %#v, want user-1", body["autoCompleteSetBy"])
				}
			},
		},
		{
			name: "clear auto-complete",
			opts: PullRequestUpdateOptions{
				RepositoryID:     "repo-uuid",
				PullRequestID:    44,
				AutoCompleteMode: PullRequestAutoCompleteClear,
			},
			responseBody: `{"pullRequestId":44,"status":"active","repository":{"id":"repo-uuid"}}`,
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				setter, ok := body["autoCompleteSetBy"].(map[string]any)
				if !ok || setter["id"] != "00000000-0000-0000-0000-000000000000" {
					t.Fatalf("autoCompleteSetBy = %#v, want empty identity UUID", body["autoCompleteSetBy"])
				}
			},
		},
		{
			name: "abandon",
			opts: func() PullRequestUpdateOptions {
				status := "abandoned"
				return PullRequestUpdateOptions{RepositoryID: "repo-uuid", PullRequestID: 44, Status: &status}
			}(),
			responseBody: `{"pullRequestId":44,"status":"abandoned","repository":{"id":"repo-uuid"}}`,
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				if body["status"] != "abandoned" {
					t.Fatalf("status = %#v, want abandoned", body["status"])
				}
				if _, exists := body["autoCompleteSetBy"]; exists {
					t.Fatalf("abandon body unexpectedly changes auto-complete: %#v", body)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decoding request body: %v", err)
				}
				fmt.Fprint(w, tt.responseBody)
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			if _, err := client.UpdatePullRequest(context.Background(), tt.opts); err != nil {
				t.Fatalf("UpdatePullRequest returned error: %v", err)
			}
			tt.assertBody(t, body)
		})
	}
}

func TestClientUpdatePullRequestGovernanceNormalizesLegacySquash(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"pullRequestId":44,"status":"active","repository":{"id":"repo-uuid"}}`)
	}))
	t.Cleanup(server.Close)
	legacySquash := true
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if _, err := client.UpdatePullRequest(context.Background(), PullRequestUpdateOptions{
		RepositoryID:      "repo-uuid",
		PullRequestID:     44,
		CompletionOptions: &PullRequestCompletionOptions{SquashMerge: &legacySquash},
	}); err != nil {
		t.Fatalf("UpdatePullRequest returned error: %v", err)
	}
	completion, ok := body["completionOptions"].(map[string]any)
	if !ok || completion["mergeStrategy"] != "squash" {
		t.Fatalf("completionOptions = %#v, want explicit squash strategy", body["completionOptions"])
	}
	if _, exists := completion["squashMerge"]; exists {
		t.Fatalf("completionOptions contains deprecated squashMerge: %#v", completion)
	}
}

func TestClientUpdatePullRequestGovernanceRejectsInvalidInputAndResponse(t *testing.T) {
	t.Run("set auto-complete without identity", func(t *testing.T) {
		client, err := NewClient(http.DefaultClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project", APIVersion: "7.1", PAT: "secret"})
		if err != nil {
			t.Fatalf("NewClient returned error: %v", err)
		}
		_, err = client.UpdatePullRequest(context.Background(), PullRequestUpdateOptions{
			RepositoryID:     "repo-uuid",
			PullRequestID:    44,
			AutoCompleteMode: PullRequestAutoCompleteSet,
		})
		if err == nil || !strings.Contains(err.Error(), "identity ID") {
			t.Fatalf("UpdatePullRequest error = %v, want identity ID", err)
		}
	})

	t.Run("mismatched response ID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"pullRequestId":99,"status":"abandoned","repository":{"id":"repo-uuid"}}`)
		}))
		t.Cleanup(server.Close)
		client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
		if err != nil {
			t.Fatalf("NewClient returned error: %v", err)
		}
		status := "abandoned"
		_, err = client.UpdatePullRequest(context.Background(), PullRequestUpdateOptions{
			RepositoryID:  "repo-uuid",
			PullRequestID: 44,
			Status:        &status,
		})
		if err == nil || !strings.Contains(err.Error(), "99") || !strings.Contains(err.Error(), "44") {
			t.Fatalf("UpdatePullRequest error = %v, want both response and requested IDs", err)
		}
	})
}

func TestClientSetPullRequestReviewerVoteBuildsURLAuthAndBody(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenAPIVersion string
	var seenAuth string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"id":"user-1","displayName":"Ada","vote":10,"isRequired":true}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	reviewer, err := client.SetPullRequestReviewerVote(context.Background(), PullRequestReviewerVoteOptions{
		RepositoryID:  "repo-uuid",
		PullRequestID: 44,
		ReviewerID:    "user-1",
		Vote:          10,
		IsRequired:    true,
	})
	if err != nil {
		t.Fatalf("SetPullRequestReviewerVote returned error: %v", err)
	}
	if reviewer.ID != "user-1" || reviewer.Vote != 10 || !reviewer.IsRequired {
		t.Fatalf("reviewer = %+v, want required user-1 vote 10", reviewer)
	}
	if seenMethod != http.MethodPut {
		t.Fatalf("method = %q, want PUT", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/MyProject/_apis/git/repositories/repo-uuid/pullrequests/44/reviewers/user-1" {
		t.Fatalf("path = %q, want reviewer resource path", seenPath)
	}
	if seenAPIVersion != "7.1" {
		t.Fatalf("api-version = %q, want 7.1", seenAPIVersion)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
	for key, want := range map[string]any{"id": "user-1", "vote": float64(10), "isRequired": true} {
		if body[key] != want {
			t.Fatalf("body[%s] = %#v, want %#v (body=%#v)", key, body[key], want, body)
		}
	}
}

func TestClientSetPullRequestReviewerVoteSupportsExplicitVotes(t *testing.T) {
	for _, vote := range []int{5, -10} {
		t.Run(fmt.Sprintf("vote %d", vote), func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decoding request body: %v", err)
				}
				fmt.Fprintf(w, `{"id":"user-1","vote":%d,"isRequired":false}`, vote)
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			reviewer, err := client.SetPullRequestReviewerVote(context.Background(), PullRequestReviewerVoteOptions{
				RepositoryID:  "repo-uuid",
				PullRequestID: 44,
				ReviewerID:    "user-1",
				Vote:          vote,
			})
			if err != nil {
				t.Fatalf("SetPullRequestReviewerVote returned error: %v", err)
			}
			if reviewer.Vote != vote {
				t.Fatalf("response vote = %d, want %d", reviewer.Vote, vote)
			}
			if body["vote"] != float64(vote) || body["isRequired"] != false {
				t.Fatalf("body = %#v, want vote %d and non-required reviewer", body, vote)
			}
		})
	}
}

func TestClientSetPullRequestReviewerVoteRejectsInvalidInputAndResponse(t *testing.T) {
	client, err := NewClient(http.DefaultClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	for _, tt := range []struct {
		name string
		opts PullRequestReviewerVoteOptions
		want string
	}{
		{name: "missing repository", opts: PullRequestReviewerVoteOptions{PullRequestID: 44, ReviewerID: "user-1", Vote: 10}, want: "repository ID"},
		{name: "missing reviewer", opts: PullRequestReviewerVoteOptions{RepositoryID: "repo", PullRequestID: 44, Vote: 10}, want: "reviewer ID"},
		{name: "unsupported raw vote", opts: PullRequestReviewerVoteOptions{RepositoryID: "repo", PullRequestID: 44, ReviewerID: "user-1", Vote: 0}, want: "unsupported"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.SetPullRequestReviewerVote(context.Background(), tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("SetPullRequestReviewerVote error = %v, want %q", err, tt.want)
			}
		})
	}

	for _, tt := range []struct {
		name            string
		responseBody    string
		requestRequired bool
		want            string
	}{
		{name: "mismatched reviewer", responseBody: `{"id":"user-2","vote":10}`, want: "user-2"},
		{name: "mismatched vote", responseBody: `{"id":"user-1","vote":5}`, want: "requested vote"},
		{name: "required flag lost", responseBody: `{"id":"user-1","vote":10,"isRequired":false}`, requestRequired: true, want: "required reviewer"},
		{name: "required flag added", responseBody: `{"id":"user-1","vote":10,"isRequired":true}`, want: "required reviewer"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tt.responseBody)
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			_, err = client.SetPullRequestReviewerVote(context.Background(), PullRequestReviewerVoteOptions{
				RepositoryID:  "repo",
				PullRequestID: 44,
				ReviewerID:    "user-1",
				Vote:          10,
				IsRequired:    tt.requestRequired,
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("SetPullRequestReviewerVote error = %v, want %q", err, tt.want)
			}
		})
	}
}
