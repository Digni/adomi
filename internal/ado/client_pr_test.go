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
		fmt.Fprint(w, `{"pullRequestId":42,"title":"PR title","repository":{"id":"repo-uuid","name":"adomi"}}`)
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
