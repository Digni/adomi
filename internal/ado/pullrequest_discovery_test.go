package ado

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverPullRequestsTraversesShortPagesAndPreservesFilters(t *testing.T) {
	var skips []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("$top") != "100" {
			t.Errorf("$top = %q, want 100", query.Get("$top"))
		}
		if query.Get("searchCriteria.status") != "all" {
			t.Errorf("status = %q, want all", query.Get("searchCriteria.status"))
		}
		if query.Get("searchCriteria.sourceRefName") != "refs/heads/feature/example" {
			t.Errorf("source = %q, want normalized source", query.Get("searchCriteria.sourceRefName"))
		}
		if query.Get("searchCriteria.targetRefName") != "refs/heads/main" {
			t.Errorf("target = %q, want target", query.Get("searchCriteria.targetRefName"))
		}
		skip := query.Get("$skip")
		skips = append(skips, skip)
		switch skip {
		case "":
			fmt.Fprint(w, `{"count":2,"value":[{"pullRequestId":1,"repository":{"id":"repo"}},{"pullRequestId":2,"repository":{"id":"repo"}}]}`)
		case "2":
			fmt.Fprint(w, `{"count":1,"value":[{"pullRequestId":3,"repository":{"id":"repo"}}]}`)
		case "3":
			fmt.Fprint(w, `{"count":0,"value":[]}`)
		default:
			http.Error(w, "unexpected skip", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	got, err := DiscoverPullRequests(context.Background(), client, PullRequestListOptions{
		RepositoryID:  "repo",
		SourceRefName: "refs/heads/feature/example",
		TargetRefName: "refs/heads/main",
		Status:        "all",
	})
	if err != nil {
		t.Fatalf("DiscoverPullRequests returned error: %v", err)
	}
	if len(got) != 3 || got[0].ID != 1 || got[1].ID != 2 || got[2].ID != 3 {
		t.Fatalf("pull requests = %#v, want IDs 1, 2, 3", got)
	}
	if len(skips) != 3 || skips[0] != "" || skips[1] != "2" || skips[2] != "3" {
		t.Fatalf("skips = %#v, want empty, 2, 3", skips)
	}
}

func TestDiscoverPullRequestsRejectsDuplicateIDs(t *testing.T) {
	lister := pullRequestListFunc(func(_ context.Context, opts PullRequestListOptions) ([]PullRequest, error) {
		if opts.Top != 100 {
			t.Fatalf("Top = %d, want 100", opts.Top)
		}
		if opts.Skip == 0 {
			return []PullRequest{{ID: 1}, {ID: 2}}, nil
		}
		return []PullRequest{{ID: 2}}, nil
	})

	_, err := DiscoverPullRequests(context.Background(), lister, PullRequestListOptions{RepositoryID: "repo", Top: 7, Skip: 9})
	if err == nil {
		t.Fatal("DiscoverPullRequests error = nil, want duplicate ID error")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "duplicate") || !strings.Contains(got, "2") {
		t.Fatalf("error = %q, want duplicate ID context", got)
	}
}

func TestDiscoverPullRequestsReturnsEmptyForEmptyInitialPage(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.URL.Query().Get("$top"); got != "100" {
			t.Errorf("$top = %q, want 100", got)
		}
		fmt.Fprint(w, `{"count":0,"value":[]}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	got, err := DiscoverPullRequests(context.Background(), client, PullRequestListOptions{RepositoryID: "repo"})
	if err != nil {
		t.Fatalf("DiscoverPullRequests returned error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("pull requests = %#v, want non-nil empty slice", got)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestDiscoverPullRequestsRejectsMalformedPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"count":1}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	got, err := DiscoverPullRequests(context.Background(), client, PullRequestListOptions{RepositoryID: "repo"})
	if err == nil {
		t.Fatal("DiscoverPullRequests error = nil, want malformed collection error")
	}
	if got != nil {
		t.Fatalf("pull requests = %#v, want nil on error", got)
	}
	if !strings.Contains(err.Error(), "collection") {
		t.Fatalf("error = %q, want collection context", err)
	}
}

func TestDiscoverPullRequestsRejectsPageCeiling(t *testing.T) {
	calls := 0
	lister := pullRequestListFunc(func(_ context.Context, opts PullRequestListOptions) ([]PullRequest, error) {
		calls++
		if opts.Skip != calls-1 {
			t.Fatalf("Skip = %d on call %d, want %d", opts.Skip, calls, calls-1)
		}
		return []PullRequest{{ID: calls}}, nil
	})

	got, err := DiscoverPullRequests(context.Background(), lister, PullRequestListOptions{RepositoryID: "repo"})
	if err == nil {
		t.Fatal("DiscoverPullRequests error = nil, want page ceiling error")
	}
	if got != nil {
		t.Fatalf("pull requests = %#v, want nil on error", got)
	}
	if calls != maxPullRequestPages {
		t.Fatalf("calls = %d, want %d", calls, maxPullRequestPages)
	}
	if !strings.Contains(err.Error(), "maximum of 1000 pages") {
		t.Fatalf("error = %q, want page ceiling context", err)
	}
}

type pullRequestListFunc func(context.Context, PullRequestListOptions) ([]PullRequest, error)

func (f pullRequestListFunc) ListPullRequests(ctx context.Context, opts PullRequestListOptions) ([]PullRequest, error) {
	return f(ctx, opts)
}
