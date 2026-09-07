package ado

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientListPullRequestsIncludesOptionalPageControls(t *testing.T) {
	var seenTop, seenSkip string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTop = r.URL.Query().Get("$top")
		seenSkip = r.URL.Query().Get("$skip")
		fmt.Fprint(w, `{"count":1,"value":[{"pullRequestId":43,"repository":{"id":"repo"}}]}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.ListPullRequests(context.Background(), PullRequestListOptions{RepositoryID: "repo", Top: 100, Skip: 200})
	if err != nil {
		t.Fatalf("ListPullRequests returned error: %v", err)
	}
	if seenTop != "100" || seenSkip != "200" {
		t.Fatalf("pagination query = top %q, skip %q; want 100 and 200", seenTop, seenSkip)
	}
}

func TestClientListPullRequestsStrictPageRejectsMissingCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"count":0}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	got, err := client.ListPullRequests(context.Background(), PullRequestListOptions{RepositoryID: "repo", Top: 100})
	if err == nil {
		t.Fatal("ListPullRequests error = nil, want missing collection error")
	}
	if got != nil {
		t.Fatalf("pull requests = %#v, want nil on error", got)
	}
	if !strings.Contains(err.Error(), "collection") {
		t.Fatalf("error = %q, want collection context", err)
	}
}

func TestClientListPullRequestsStrictPageEnforcesResponseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", int(maxPullRequestResponseBytes)+1)))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.ListPullRequests(context.Background(), PullRequestListOptions{RepositoryID: "repo", Top: 100})
	if err == nil {
		t.Fatal("ListPullRequests error = nil, want response size error")
	}
	if !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("error = %q, want maximum size context", err)
	}
}
