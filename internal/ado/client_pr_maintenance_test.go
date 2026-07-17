package ado

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
