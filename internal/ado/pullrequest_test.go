package ado

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestClientFetchPullRequestThreadsBuildsRepoScopedURL(t *testing.T) {
	var seenPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		fmt.Fprint(w, `{"count":1,"value":[{"id":1,"status":"active","comments":[{"id":1,"content":"hi"}]}]}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{
		BaseURL:    server.URL + "/tfs/DefaultCollection",
		Project:    "MyProject",
		APIVersion: "7.1",
		PAT:        "secret",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	threads, err := client.FetchPullRequestThreads(context.Background(), "repo-uuid", 42)
	if err != nil {
		t.Fatalf("FetchPullRequestThreads returned error: %v", err)
	}
	if len(threads) != 1 || threads[0].ID != 1 {
		t.Fatalf("threads = %+v, want one thread", threads)
	}
	if seenPath != "/tfs/DefaultCollection/MyProject/_apis/git/repositories/repo-uuid/pullrequests/42/threads" {
		t.Fatalf("path = %q, want repo-scoped threads path", seenPath)
	}
}

func TestClientFetchPullRequestThreadsRequiresRepoID(t *testing.T) {
	client, err := NewClient(http.DefaultClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	_, err = client.FetchPullRequestThreads(context.Background(), "  ", 42)
	if err == nil {
		t.Fatal("FetchPullRequestThreads error = nil, want repo ID error")
	}
	if !strings.Contains(err.Error(), "repository ID") {
		t.Fatalf("error = %q, want repository ID context", err.Error())
	}
}

func TestClientFetchPullRequestThreadsReturnsEmptySliceForNullValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"count":0}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	threads, err := client.FetchPullRequestThreads(context.Background(), "repo", 42)
	if err != nil {
		t.Fatalf("FetchPullRequestThreads returned error: %v", err)
	}
	if threads == nil {
		t.Fatal("threads is nil, want empty slice")
	}
	if len(threads) != 0 {
		t.Fatalf("threads len = %d, want 0", len(threads))
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

func TestClientPullRequestIterationsBuildsURLAndAuth(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenAPIVersion string
	var seenContentType string
	var seenBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenContentType = r.Header.Get("Content-Type")
		if r.Body != nil {
			var err error
			seenBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading request body: %v", err)
			}
		}
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		fmt.Fprint(w, `{"count":2,"value":[{"id":1},{"id":3}]}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	iterations, err := client.ListPullRequestIterations(context.Background(), "repo-uuid", 44)
	if err != nil {
		t.Fatalf("ListPullRequestIterations returned error: %v", err)
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/MyProject/_apis/git/repositories/repo-uuid/pullrequests/44/iterations" {
		t.Fatalf("path = %q, want iterations path", seenPath)
	}
	if seenAPIVersion != "7.1" {
		t.Fatalf("api-version = %q, want 7.1", seenAPIVersion)
	}
	if string(seenBody) != "null" {
		t.Fatalf("request body = %q, want null", seenBody)
	}
	if seenContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", seenContentType)
	}
	if len(iterations) != 2 || iterations[0].ID != 1 || iterations[1].ID != 3 {
		t.Fatalf("iterations = %+v, want IDs 1/3", iterations)
	}
}

func TestClientPullRequestIterationChangesBuildsURLAuthAndPaginates(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		seen = append(seen, r.URL.EscapedPath()+"?"+r.URL.RawQuery)
		skip := r.URL.Query().Get("$skip")
		if skip == "0" {
			fmt.Fprint(w, `{"changeEntries":[{"changeTrackingId":7,"changeId":1,"changeType":"edit","item":{"path":"/src/app.go"}}],"nextSkip":1,"nextTop":50}`)
			return
		}
		fmt.Fprint(w, `{"changeEntries":[{"changeTrackingId":8,"changeId":2,"changeType":"add","item":{"path":"/README.md"},"originalPath":"/OLD.md"}],"nextSkip":0,"nextTop":0}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	changes, err := client.ListPullRequestIterationChanges(context.Background(), PullRequestIterationChangesOptions{RepositoryID: "repo-uuid", PullRequestID: 44, IterationID: 3, CompareTo: 0, Top: 100})
	if err != nil {
		t.Fatalf("ListPullRequestIterationChanges returned error: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %v, want two pages", seen)
	}
	for _, raw := range seen {
		if !strings.Contains(raw, "/tfs/DefaultCollection/MyProject/_apis/git/repositories/repo-uuid/pullrequests/44/iterations/3/changes?") || !strings.Contains(raw, "api-version=7.1") || !strings.Contains(raw, "%24compareTo=0") {
			t.Fatalf("request = %q, want iteration changes path/query", raw)
		}
	}
	if !strings.Contains(seen[0], "%24top=100") || !strings.Contains(seen[0], "%24skip=0") || !strings.Contains(seen[1], "%24top=50") || !strings.Contains(seen[1], "%24skip=1") {
		t.Fatalf("requests = %v, want paginated top/skip", seen)
	}
	if len(changes) != 2 || changes[0].ChangeTrackingID != 7 || changes[0].Item.Path != "/src/app.go" || changes[1].ChangeTrackingID != 8 || changes[1].OriginalPath != "/OLD.md" {
		t.Fatalf("changes = %+v, want decoded changes", changes)
	}
}

func TestClientPullRequestIterationChangesRejectsNonAdvancingPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("$skip") == "0" {
			fmt.Fprint(w, `{"changeEntries":[],"nextSkip":1,"nextTop":100}`)
			return
		}
		fmt.Fprint(w, `{"changeEntries":[],"nextSkip":1,"nextTop":100}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.ListPullRequestIterationChanges(context.Background(), PullRequestIterationChangesOptions{RepositoryID: "repo", PullRequestID: 1, IterationID: 1, CompareTo: 0})
	if err == nil || !strings.Contains(err.Error(), "pagination did not advance") {
		t.Fatalf("error = %v, want pagination guard", err)
	}
}

func TestClientCreatePullRequestThreadRejectsPartialInlineContext(t *testing.T) {
	client, err := NewClient(http.DefaultClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.CreatePullRequestThread(context.Background(), PullRequestThreadCreateOptions{RepositoryID: "repo", PullRequestID: 1, Content: "x", ThreadContext: &ThreadContext{FilePath: "/x.go"}})
	if err == nil || !strings.Contains(err.Error(), "requires both threadContext and pullRequestThreadContext") {
		t.Fatalf("error = %v, want partial inline context rejection", err)
	}
	_, err = client.CreatePullRequestThread(context.Background(), PullRequestThreadCreateOptions{RepositoryID: "repo", PullRequestID: 1, Content: "x", PullRequestThreadContext: &PullRequestThreadContext{ChangeTrackingID: 1}})
	if err == nil || !strings.Contains(err.Error(), "requires both threadContext and pullRequestThreadContext") {
		t.Fatalf("error = %v, want partial inline context rejection", err)
	}
}

func TestClientCreatePullRequestThreadBuildsURLAuthAndBody(t *testing.T) {
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
		fmt.Fprint(w, `{"id":14,"status":"active","comments":[{"id":1,"content":"Looks good","commentType":"text"}]}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	thread, err := client.CreatePullRequestThread(context.Background(), PullRequestThreadCreateOptions{RepositoryID: "repo-uuid", PullRequestID: 44, Content: "Looks good"})
	if err != nil {
		t.Fatalf("CreatePullRequestThread returned error: %v", err)
	}
	if thread.ID != 14 || thread.Status != "active" || len(thread.Comments) != 1 || thread.Comments[0].ID != 1 {
		t.Fatalf("thread = %+v, want created thread", thread)
	}
	if seenMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/MyProject/_apis/git/repositories/repo-uuid/pullrequests/44/threads" {
		t.Fatalf("path = %q, want thread collection path", seenPath)
	}
	if seenAPIVersion != "7.1" {
		t.Fatalf("api-version = %q, want 7.1", seenAPIVersion)
	}
	comments, ok := body["comments"].([]any)
	if !ok || len(comments) != 1 {
		t.Fatalf("body = %#v, want one comment", body)
	}
	comment, ok := comments[0].(map[string]any)
	if !ok {
		t.Fatalf("comments[0] = %#v, want object", comments[0])
	}
	if comment["parentCommentId"] != float64(0) || comment["content"] != "Looks good" || comment["commentType"] != "text" || body["status"] != "active" {
		t.Fatalf("body = %#v, want active text comment", body)
	}
	if _, ok := body["threadContext"]; ok {
		t.Fatalf("body = %#v, want no threadContext for PR-level comment", body)
	}
	if _, ok := body["pullRequestThreadContext"]; ok {
		t.Fatalf("body = %#v, want no pullRequestThreadContext for PR-level comment", body)
	}
}

func TestClientCreatePullRequestThreadBuildsInlineBody(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"id":14,"status":"active","comments":[{"id":1,"content":"Nit","commentType":"text"}]}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.CreatePullRequestThread(context.Background(), PullRequestThreadCreateOptions{
		RepositoryID:  "repo-uuid",
		PullRequestID: 44,
		Content:       "Nit",
		ThreadContext: &ThreadContext{
			FilePath:       "/src/app.go",
			RightFileStart: &FilePosition{Line: 42, Offset: 1},
			RightFileEnd:   &FilePosition{Line: 42, Offset: 1},
		},
		PullRequestThreadContext: &PullRequestThreadContext{
			ChangeTrackingID: 77,
			IterationContext: &CommentIterationContext{FirstComparingIteration: 3, SecondComparingIteration: 3},
		},
	})
	if err != nil {
		t.Fatalf("CreatePullRequestThread returned error: %v", err)
	}
	threadContext, ok := body["threadContext"].(map[string]any)
	if !ok || threadContext["filePath"] != "/src/app.go" {
		t.Fatalf("threadContext = %#v, want file path", body["threadContext"])
	}
	rightStart, ok := threadContext["rightFileStart"].(map[string]any)
	if !ok || rightStart["line"] != float64(42) || rightStart["offset"] != float64(1) {
		t.Fatalf("rightFileStart = %#v, want line 42 offset 1", threadContext["rightFileStart"])
	}
	rightEnd, ok := threadContext["rightFileEnd"].(map[string]any)
	if !ok || rightEnd["line"] != float64(42) || rightEnd["offset"] != float64(1) {
		t.Fatalf("rightFileEnd = %#v, want line 42 offset 1", threadContext["rightFileEnd"])
	}
	if _, ok := threadContext["leftFileStart"]; ok {
		t.Fatalf("threadContext = %#v, want no leftFileStart", threadContext)
	}
	prContext, ok := body["pullRequestThreadContext"].(map[string]any)
	if !ok || prContext["changeTrackingId"] != float64(77) {
		t.Fatalf("pullRequestThreadContext = %#v, want changeTrackingId", body["pullRequestThreadContext"])
	}
	iterationContext, ok := prContext["iterationContext"].(map[string]any)
	if !ok || iterationContext["firstComparingIteration"] != float64(3) || iterationContext["secondComparingIteration"] != float64(3) {
		t.Fatalf("iterationContext = %#v, want 3/3", prContext["iterationContext"])
	}
}

func TestClientCreatePullRequestThreadCommentBuildsURLAuthAndBody(t *testing.T) {
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
		fmt.Fprint(w, `{"id":7,"content":"Done","commentType":"text"}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	comment, err := client.CreatePullRequestThreadComment(context.Background(), PullRequestThreadCommentCreateOptions{RepositoryID: "repo-uuid", PullRequestID: 44, ThreadID: 12, Content: "Done"})
	if err != nil {
		t.Fatalf("CreatePullRequestThreadComment returned error: %v", err)
	}
	if comment.ID != 7 || comment.Content != "Done" {
		t.Fatalf("comment = %+v, want created comment", comment)
	}
	if seenMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/MyProject/_apis/git/repositories/repo-uuid/pullrequests/44/threads/12/comments" {
		t.Fatalf("path = %q, want thread comments path", seenPath)
	}
	if seenAPIVersion != "7.1" {
		t.Fatalf("api-version = %q, want 7.1", seenAPIVersion)
	}
	if body["content"] != "Done" || body["commentType"] != "text" {
		t.Fatalf("body = %#v, want content and text commentType", body)
	}
}

func TestClientUpdatePullRequestThreadBuildsURLAuthAndBody(t *testing.T) {
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
		fmt.Fprint(w, `{"id":12,"status":"fixed"}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "MyProject", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	thread, err := client.UpdatePullRequestThread(context.Background(), PullRequestThreadUpdateOptions{RepositoryID: "repo-uuid", PullRequestID: 44, ThreadID: 12, Status: "fixed"})
	if err != nil {
		t.Fatalf("UpdatePullRequestThread returned error: %v", err)
	}
	if thread.ID != 12 || thread.Status != "fixed" {
		t.Fatalf("thread = %+v, want fixed thread", thread)
	}
	if seenMethod != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/MyProject/_apis/git/repositories/repo-uuid/pullrequests/44/threads/12" {
		t.Fatalf("path = %q, want thread update path", seenPath)
	}
	if seenAPIVersion != "7.1" {
		t.Fatalf("api-version = %q, want 7.1", seenAPIVersion)
	}
	if body["status"] != "fixed" {
		t.Fatalf("body = %#v, want fixed status", body)
	}
}

func TestPullRequestMaintenanceMethodsRequireRepositoryID(t *testing.T) {
	client, err := NewClient(http.DefaultClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	ctx := context.Background()
	tests := []struct {
		name string
		call func() error
	}{
		{name: "list", call: func() error { _, err := client.ListPullRequests(ctx, PullRequestListOptions{}); return err }},
		{name: "create", call: func() error { _, err := client.CreatePullRequest(ctx, PullRequestCreateOptions{}); return err }},
		{name: "update", call: func() error {
			_, err := client.UpdatePullRequest(ctx, PullRequestUpdateOptions{PullRequestID: 1})
			return err
		}},
		{name: "iterations", call: func() error {
			_, err := client.ListPullRequestIterations(ctx, "", 1)
			return err
		}},
		{name: "iteration changes", call: func() error {
			_, err := client.ListPullRequestIterationChanges(ctx, PullRequestIterationChangesOptions{PullRequestID: 1, IterationID: 1})
			return err
		}},
		{name: "create thread", call: func() error {
			_, err := client.CreatePullRequestThread(ctx, PullRequestThreadCreateOptions{PullRequestID: 1, Content: "x"})
			return err
		}},
		{name: "comment", call: func() error {
			_, err := client.CreatePullRequestThreadComment(ctx, PullRequestThreadCommentCreateOptions{PullRequestID: 1, ThreadID: 1, Content: "x"})
			return err
		}},
		{name: "thread", call: func() error {
			_, err := client.UpdatePullRequestThread(ctx, PullRequestThreadUpdateOptions{PullRequestID: 1, ThreadID: 1, Status: "fixed"})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("error = nil, want repository ID error")
			}
			if !strings.Contains(err.Error(), "repository ID") {
				t.Fatalf("error = %q, want repository ID context", err.Error())
			}
		})
	}
}

func TestPullRequestMaintenanceMethodsReturnNon2xxErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	ctx := context.Background()
	title := "Updated"
	tests := []struct {
		name string
		call func() error
	}{
		{name: "list", call: func() error {
			_, err := client.ListPullRequests(ctx, PullRequestListOptions{RepositoryID: "repo"})
			return err
		}},
		{name: "create", call: func() error {
			_, err := client.CreatePullRequest(ctx, PullRequestCreateOptions{RepositoryID: "repo", SourceRefName: "refs/heads/a", TargetRefName: "refs/heads/b", Title: "PR"})
			return err
		}},
		{name: "update", call: func() error {
			_, err := client.UpdatePullRequest(ctx, PullRequestUpdateOptions{RepositoryID: "repo", PullRequestID: 1, Title: &title})
			return err
		}},
		{name: "iterations", call: func() error {
			_, err := client.ListPullRequestIterations(ctx, "repo", 1)
			return err
		}},
		{name: "iteration changes", call: func() error {
			_, err := client.ListPullRequestIterationChanges(ctx, PullRequestIterationChangesOptions{RepositoryID: "repo", PullRequestID: 1, IterationID: 1})
			return err
		}},
		{name: "create thread", call: func() error {
			_, err := client.CreatePullRequestThread(ctx, PullRequestThreadCreateOptions{RepositoryID: "repo", PullRequestID: 1, Content: "x"})
			return err
		}},
		{name: "comment", call: func() error {
			_, err := client.CreatePullRequestThreadComment(ctx, PullRequestThreadCommentCreateOptions{RepositoryID: "repo", PullRequestID: 1, ThreadID: 2, Content: "x"})
			return err
		}},
		{name: "thread", call: func() error {
			_, err := client.UpdatePullRequestThread(ctx, PullRequestThreadUpdateOptions{RepositoryID: "repo", PullRequestID: 1, ThreadID: 2, Status: "fixed"})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("error = nil, want non-2xx error")
			}
			if !strings.Contains(err.Error(), "502") || !strings.Contains(err.Error(), "nope") {
				t.Fatalf("error = %q, want status and body context", err.Error())
			}
		})
	}
}

func TestPullRequestMaintenanceMethodsWrapDecodeErrors(t *testing.T) {
	assertPullRequestMaintenanceDecodeErrors(t, `{`)
}

func TestPullRequestMaintenanceMethodsWrapEmptyBodyDecodeErrors(t *testing.T) {
	assertPullRequestMaintenanceDecodeErrors(t, "")
}

func TestPullRequestMaintenanceMethodsRejectMissingResponseIDs(t *testing.T) {
	ctx := context.Background()
	title := "Updated"
	tests := []struct {
		name         string
		responseBody string
		want         string
		call         func(*Client) error
	}{
		{name: "list item", responseBody: `{"count":1,"value":[{}]}`, want: "pull request ID", call: func(client *Client) error {
			_, err := client.ListPullRequests(ctx, PullRequestListOptions{RepositoryID: "repo"})
			return err
		}},
		{name: "create", responseBody: `{}`, want: "pull request ID", call: func(client *Client) error {
			_, err := client.CreatePullRequest(ctx, PullRequestCreateOptions{RepositoryID: "repo", SourceRefName: "refs/heads/a", TargetRefName: "refs/heads/b", Title: "PR"})
			return err
		}},
		{name: "update", responseBody: `{}`, want: "pull request ID", call: func(client *Client) error {
			_, err := client.UpdatePullRequest(ctx, PullRequestUpdateOptions{RepositoryID: "repo", PullRequestID: 1, Title: &title})
			return err
		}},
		{name: "create thread", responseBody: `{}`, want: "thread ID", call: func(client *Client) error {
			_, err := client.CreatePullRequestThread(ctx, PullRequestThreadCreateOptions{RepositoryID: "repo", PullRequestID: 1, Content: "x"})
			return err
		}},
		{name: "comment", responseBody: `{}`, want: "comment ID", call: func(client *Client) error {
			_, err := client.CreatePullRequestThreadComment(ctx, PullRequestThreadCommentCreateOptions{RepositoryID: "repo", PullRequestID: 1, ThreadID: 2, Content: "x"})
			return err
		}},
		{name: "thread", responseBody: `{}`, want: "thread ID", call: func(client *Client) error {
			_, err := client.UpdatePullRequestThread(ctx, PullRequestThreadUpdateOptions{RepositoryID: "repo", PullRequestID: 1, ThreadID: 2, Status: "fixed"})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tt.responseBody)
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			err = tt.call(client)
			if err == nil {
				t.Fatal("error = nil, want semantic response error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func assertPullRequestMaintenanceDecodeErrors(t *testing.T, responseBody string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, responseBody)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	ctx := context.Background()
	title := "Updated"
	tests := []struct {
		name string
		want string
		call func() error
	}{
		{name: "list", want: "decoding Azure DevOps pull requests", call: func() error {
			_, err := client.ListPullRequests(ctx, PullRequestListOptions{RepositoryID: "repo"})
			return err
		}},
		{name: "create", want: "decoding Azure DevOps pull request create response", call: func() error {
			_, err := client.CreatePullRequest(ctx, PullRequestCreateOptions{RepositoryID: "repo", SourceRefName: "refs/heads/a", TargetRefName: "refs/heads/b", Title: "PR"})
			return err
		}},
		{name: "update", want: "decoding Azure DevOps pull request update response", call: func() error {
			_, err := client.UpdatePullRequest(ctx, PullRequestUpdateOptions{RepositoryID: "repo", PullRequestID: 1, Title: &title})
			return err
		}},
		{name: "iterations", want: "decoding Azure DevOps pull request iterations", call: func() error {
			_, err := client.ListPullRequestIterations(ctx, "repo", 1)
			return err
		}},
		{name: "iteration changes", want: "decoding Azure DevOps pull request iteration changes", call: func() error {
			_, err := client.ListPullRequestIterationChanges(ctx, PullRequestIterationChangesOptions{RepositoryID: "repo", PullRequestID: 1, IterationID: 1})
			return err
		}},
		{name: "create thread", want: "decoding Azure DevOps pull request thread create response", call: func() error {
			_, err := client.CreatePullRequestThread(ctx, PullRequestThreadCreateOptions{RepositoryID: "repo", PullRequestID: 1, Content: "x"})
			return err
		}},
		{name: "comment", want: "decoding Azure DevOps pull request thread comment", call: func() error {
			_, err := client.CreatePullRequestThreadComment(ctx, PullRequestThreadCommentCreateOptions{RepositoryID: "repo", PullRequestID: 1, ThreadID: 2, Content: "x"})
			return err
		}},
		{name: "thread", want: "decoding Azure DevOps pull request thread update", call: func() error {
			_, err := client.UpdatePullRequestThread(ctx, PullRequestThreadUpdateOptions{RepositoryID: "repo", PullRequestID: 1, ThreadID: 2, Status: "fixed"})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("error = nil, want decode error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestFetchPullRequestBundleReturnsPRWithThreads(t *testing.T) {
	pr := &PullRequest{ID: 42, Title: "PR", Repository: PullRequestRepo{ID: "repo-uuid", Name: "adomi"}}
	threads := []PullRequestThread{{ID: 1, Status: "active", Comments: []PullRequestComment{{ID: 100, Content: "Looks good"}}}}
	fetcher := &fakePullRequestFetcher{pr: pr, threads: threads}

	bundle, err := FetchPullRequestBundle(context.Background(), fetcher, 42)
	if err != nil {
		t.Fatalf("FetchPullRequestBundle returned error: %v", err)
	}
	if bundle.PullRequest != pr {
		t.Fatalf("bundle PR = %+v, want pointer equality", bundle.PullRequest)
	}
	if len(bundle.Threads) != 1 || bundle.Threads[0].ID != 1 {
		t.Fatalf("bundle threads = %+v, want one thread", bundle.Threads)
	}
	if fetcher.threadsRepoID != "repo-uuid" || fetcher.threadsPRID != 42 {
		t.Fatalf("threads called with %q/%d, want repo-uuid/42", fetcher.threadsRepoID, fetcher.threadsPRID)
	}
}

func TestFetchPullRequestBundleRequiresRepositoryID(t *testing.T) {
	pr := &PullRequest{ID: 42, Repository: PullRequestRepo{}}
	fetcher := &fakePullRequestFetcher{pr: pr}

	_, err := FetchPullRequestBundle(context.Background(), fetcher, 42)
	if err == nil {
		t.Fatal("FetchPullRequestBundle error = nil, want missing repo ID error")
	}
	if !strings.Contains(err.Error(), "repository ID") {
		t.Fatalf("error = %q, want repository ID context", err.Error())
	}
	if fetcher.threadsRepoID != "" {
		t.Fatal("threads were fetched even though repository ID was missing")
	}
}

func TestFetchPullRequestBundleWrapsPRError(t *testing.T) {
	fetcher := &fakePullRequestFetcher{prErr: errors.New("boom")}

	_, err := FetchPullRequestBundle(context.Background(), fetcher, 42)
	if err == nil {
		t.Fatal("FetchPullRequestBundle error = nil, want PR fetch error")
	}
	if !strings.Contains(err.Error(), "fetching pull request 42") {
		t.Fatalf("error = %q, want PR fetch context", err.Error())
	}
}

func TestFetchPullRequestBundleWrapsThreadsError(t *testing.T) {
	fetcher := &fakePullRequestFetcher{
		pr:         &PullRequest{ID: 42, Repository: PullRequestRepo{ID: "repo"}},
		threadsErr: errors.New("boom"),
	}

	_, err := FetchPullRequestBundle(context.Background(), fetcher, 42)
	if err == nil {
		t.Fatal("FetchPullRequestBundle error = nil, want threads error")
	}
	if !strings.Contains(err.Error(), "fetching pull request 42 threads") {
		t.Fatalf("error = %q, want threads fetch context", err.Error())
	}
}

func TestPullRequestOutputPathUsesRepoLocalAdomiDirectory(t *testing.T) {
	got := PullRequestOutputPath("/repo", "company-cloud", "MyProject", 42)
	want := filepath.Join("/repo", ".adomi", "context", "pull-requests", "42")
	if got != want {
		t.Fatalf("PullRequestOutputPath = %q, want %q", got, want)
	}
}

func TestExportPullRequestWritesCanonicalLayout(t *testing.T) {
	repoRoot := t.TempDir()
	createdAt := time.Date(2026, 4, 28, 9, 30, 0, 0, time.UTC)
	bundle := &PullRequestBundle{
		PullRequest: &PullRequest{
			ID:            42,
			Title:         "Improve checkout",
			Status:        "active",
			SourceRefName: "refs/heads/feature/x",
			TargetRefName: "refs/heads/main",
			Repository:    PullRequestRepo{ID: "repo-uuid", Name: "adomi"},
		},
		Threads: []PullRequestThread{
			{
				ID:     2,
				Status: "active",
				Comments: []PullRequestComment{
					{ID: 200, Author: map[string]any{"displayName": "Alice"}, Content: "Nit", PublishedDate: "2026-04-28T09:00:00Z"},
				},
				ThreadContext: &ThreadContext{FilePath: "/cmd/adomi/main.go"},
			},
			{
				ID:     1,
				Status: "fixed",
				Comments: []PullRequestComment{
					{ID: 100, Author: map[string]any{"displayName": "Bob"}, Content: "LGTM", PublishedDate: "2026-04-28T08:00:00Z"},
					{ID: 101, Author: map[string]any{"displayName": "Bob"}, Content: "deleted", IsDeleted: true},
				},
			},
		},
	}

	outputDir, err := ExportPullRequest(PullRequestExportOptions{
		RepoRoot:  repoRoot,
		Profile:   "company-cloud",
		Project:   "MyProject",
		CreatedAt: createdAt,
	}, bundle)
	if err != nil {
		t.Fatalf("ExportPullRequest returned error: %v", err)
	}

	wantOutputDir := filepath.Join(repoRoot, ".adomi", "context", "pull-requests", "42")
	if outputDir != wantOutputDir {
		t.Fatalf("output dir = %q, want %q", outputDir, wantOutputDir)
	}
	for _, f := range []string{
		"index.json",
		"pull-request.json",
		"threads.json",
		filepath.Join("threads", "1.json"),
		filepath.Join("threads", "2.json"),
		"comments.md",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, f)); err != nil {
			t.Fatalf("expected %s to exist: %v", f, err)
		}
	}

	var index PullRequestIndex
	readPullRequestJSON(t, filepath.Join(outputDir, "index.json"), &index)
	if index.Source != "azure-devops" {
		t.Fatalf("source = %q, want azure-devops", index.Source)
	}
	if index.PullRequestID != 42 || index.Title != "Improve checkout" {
		t.Fatalf("index PR fields = %d/%q, want 42/Improve checkout", index.PullRequestID, index.Title)
	}
	if index.Repository != "adomi" || index.RepositoryID != "repo-uuid" {
		t.Fatalf("repository = %q/%q, want adomi/repo-uuid", index.Repository, index.RepositoryID)
	}
	if index.ThreadCount != 2 || index.CommentCount != 2 {
		t.Fatalf("counts = %d/%d, want 2/2 (deleted comments excluded)", index.ThreadCount, index.CommentCount)
	}
	if index.CreatedAt != "2026-04-28T09:30:00Z" {
		t.Fatalf("createdAt = %q, want RFC3339 UTC", index.CreatedAt)
	}
	if len(index.Threads) != 2 {
		t.Fatalf("index threads = %d, want 2", len(index.Threads))
	}
	if index.Threads[0].ID != 1 || index.Threads[1].ID != 2 {
		t.Fatalf("index thread order = %d,%d, want 1,2", index.Threads[0].ID, index.Threads[1].ID)
	}
	if index.Threads[1].FilePath != "/cmd/adomi/main.go" {
		t.Fatalf("thread filePath = %q, want /cmd/adomi/main.go", index.Threads[1].FilePath)
	}
	if index.Threads[1].Path != "threads/2.json" {
		t.Fatalf("thread path = %q, want threads/2.json", index.Threads[1].Path)
	}
	if index.Threads[0].CommentCount != 1 {
		t.Fatalf("thread 1 comment count = %d, want 1 (deleted excluded)", index.Threads[0].CommentCount)
	}

	commentsMD, err := os.ReadFile(filepath.Join(outputDir, "comments.md"))
	if err != nil {
		t.Fatalf("reading comments.md: %v", err)
	}
	commentsText := string(commentsMD)
	for _, want := range []string{"# PR 42: Improve checkout", "## Thread 1", "## Thread 2", "**Alice**", "**Bob**", "LGTM", "Nit", "/cmd/adomi/main.go"} {
		if !strings.Contains(commentsText, want) {
			t.Fatalf("comments.md missing %q in:\n%s", want, commentsText)
		}
	}
	if strings.Contains(commentsText, "deleted") {
		t.Fatalf("comments.md contains deleted comment text:\n%s", commentsText)
	}
}

func TestExportPullRequestEscapesMarkdownMetadata(t *testing.T) {
	repoRoot := t.TempDir()
	bundle := &PullRequestBundle{
		PullRequest: &PullRequest{
			ID:            42,
			Title:         "Fix [bug] *now*",
			Status:        "active",
			SourceRefName: "refs/heads/feature/_x_",
			TargetRefName: "refs/heads/main",
			Repository:    PullRequestRepo{ID: "repo", Name: "ado_mi"},
		},
		Threads: []PullRequestThread{{
			ID:     1,
			Status: "active",
			Comments: []PullRequestComment{{
				ID:            10,
				Author:        map[string]any{"displayName": "Eve_Polastri"},
				Content:       "raw *content* should remain raw",
				PublishedDate: "2026-04-28T08:00:00Z",
			}},
			ThreadContext: &ThreadContext{FilePath: "/path/with_underscores/main.go"},
		}},
	}

	outputDir, err := ExportPullRequest(PullRequestExportOptions{
		RepoRoot: repoRoot,
		Profile:  "company-cloud",
		Project:  "MyProject",
	}, bundle)
	if err != nil {
		t.Fatalf("ExportPullRequest returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "comments.md"))
	if err != nil {
		t.Fatalf("reading comments.md: %v", err)
	}
	got := string(data)

	if strings.Contains(got, "# PR 42: Fix [bug] *now*") {
		t.Fatalf("title was not escaped:\n%s", got)
	}
	if !strings.Contains(got, `Fix \[bug\] \*now\*`) {
		t.Fatalf("title escape sequence missing:\n%s", got)
	}
	if !strings.Contains(got, "**Eve\\_Polastri**") {
		t.Fatalf("author escape missing:\n%s", got)
	}
	if !strings.Contains(got, "`/path/with_underscores/main.go`") {
		t.Fatalf("file path inline code missing:\n%s", got)
	}
	if !strings.Contains(got, "`refs/heads/feature/_x_`") {
		t.Fatalf("source ref inline code missing:\n%s", got)
	}
	if !strings.Contains(got, "`ado_mi`") {
		t.Fatalf("repository inline code missing:\n%s", got)
	}
	if !strings.Contains(got, "raw *content* should remain raw") {
		t.Fatalf("comment body should not be escaped:\n%s", got)
	}
}

func TestExportPullRequestRemovesStaleFilesFromPreviousRun(t *testing.T) {
	repoRoot := t.TempDir()
	outputDir := PullRequestOutputPath(repoRoot, "company-cloud", "MyProject", 42)
	stalePath := filepath.Join(outputDir, "threads", "999.json")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatalf("creating stale dir: %v", err)
	}
	if err := os.WriteFile(stalePath, []byte("old"), 0o644); err != nil {
		t.Fatalf("writing stale file: %v", err)
	}

	_, err := ExportPullRequest(PullRequestExportOptions{
		RepoRoot: repoRoot,
		Profile:  "company-cloud",
		Project:  "MyProject",
	}, &PullRequestBundle{PullRequest: &PullRequest{ID: 42, Repository: PullRequestRepo{ID: "repo"}}})
	if err != nil {
		t.Fatalf("ExportPullRequest returned error: %v", err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("stale file stat error = %v, want not exist", err)
	}
}

func TestExportPullRequestRejectsNilBundle(t *testing.T) {
	_, err := ExportPullRequest(PullRequestExportOptions{RepoRoot: t.TempDir()}, nil)
	if err == nil {
		t.Fatal("ExportPullRequest error = nil, want error")
	}
	if !strings.Contains(err.Error(), "pull request bundle") {
		t.Fatalf("error = %q, want bundle context", err.Error())
	}
}

func TestExportPullRequestDefaultsZeroCreatedAt(t *testing.T) {
	repoRoot := t.TempDir()

	outputDir, err := ExportPullRequest(PullRequestExportOptions{
		RepoRoot: repoRoot,
		Profile:  "company-cloud",
		Project:  "MyProject",
	}, &PullRequestBundle{PullRequest: &PullRequest{ID: 42, Repository: PullRequestRepo{ID: "repo"}}})
	if err != nil {
		t.Fatalf("ExportPullRequest returned error: %v", err)
	}

	var index PullRequestIndex
	readPullRequestJSON(t, filepath.Join(outputDir, "index.json"), &index)
	if index.CreatedAt == "" {
		t.Fatal("createdAt is empty, want default timestamp")
	}
	if _, err := time.Parse(time.RFC3339, index.CreatedAt); err != nil {
		t.Fatalf("createdAt = %q, want RFC3339 timestamp: %v", index.CreatedAt, err)
	}
}

type fakePullRequestFetcher struct {
	pr            *PullRequest
	prErr         error
	threads       []PullRequestThread
	threadsErr    error
	threadsRepoID string
	threadsPRID   int
}

func (f *fakePullRequestFetcher) FetchPullRequest(ctx context.Context, id int) (*PullRequest, error) {
	if f.prErr != nil {
		return nil, f.prErr
	}
	return f.pr, nil
}

func (f *fakePullRequestFetcher) FetchPullRequestThreads(ctx context.Context, repositoryID string, pullRequestID int) ([]PullRequestThread, error) {
	f.threadsRepoID = repositoryID
	f.threadsPRID = pullRequestID
	if f.threadsErr != nil {
		return nil, f.threadsErr
	}
	return f.threads, nil
}

func readPullRequestJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
}
