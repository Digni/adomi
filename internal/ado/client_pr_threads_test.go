package ado

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
