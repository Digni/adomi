package ado

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const testPullRequestArtifactURL = "vstfs:///Git/PullRequestId/project-guid%2Frepo-guid%2F42"

func TestPullRequestArtifactURLUsesProjectRepositoryAndPullRequestIDs(t *testing.T) {
	pr := PullRequest{
		ID: 42,
		Repository: PullRequestRepo{
			ID:      "repo-guid",
			Project: map[string]any{"id": "project-guid"},
		},
	}

	got, err := pr.ArtifactURL()
	if err != nil {
		t.Fatalf("ArtifactURL returned error: %v", err)
	}
	if got != testPullRequestArtifactURL {
		t.Fatalf("artifact URL = %q, want %q", got, testPullRequestArtifactURL)
	}
	if pr.Repository.ProjectID() != "project-guid" {
		t.Fatalf("project ID = %q, want project-guid", pr.Repository.ProjectID())
	}
}

func TestPullRequestArtifactURLRejectsMissingIdentifiers(t *testing.T) {
	tests := []struct {
		name string
		pr   PullRequest
		want string
	}{
		{name: "pull request ID", pr: PullRequest{Repository: PullRequestRepo{ID: "repo-guid", Project: map[string]any{"id": "project-guid"}}}, want: "pull request ID"},
		{name: "repository ID", pr: PullRequest{ID: 42, Repository: PullRequestRepo{Project: map[string]any{"id": "project-guid"}}}, want: "repository ID"},
		{name: "project ID", pr: PullRequest{ID: 42, Repository: PullRequestRepo{ID: "repo-guid"}}, want: "project ID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.pr.ArtifactURL()
			if err == nil {
				t.Fatal("ArtifactURL error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestWorkItemHasArtifactLinkRequiresExactRelationAndURL(t *testing.T) {
	item := WorkItem{Relations: []Relation{
		{Rel: "ArtifactLink", URL: "vstfs:///Git/PullRequestId/other%2Frepo-guid%2F42"},
		{Rel: "System.LinkTypes.Related", URL: testPullRequestArtifactURL},
		{Rel: "ArtifactLink", URL: testPullRequestArtifactURL},
	}}

	if !item.HasArtifactLink(testPullRequestArtifactURL) {
		t.Fatal("HasArtifactLink = false, want exact ArtifactLink match")
	}
	if item.HasArtifactLink("vstfs:///Git/PullRequestId/project-guid%2Frepo-guid%2F43") {
		t.Fatal("HasArtifactLink = true for a different URL")
	}
}

func TestClientLinkWorkItemToPullRequestBuildsRevisionGuardedPatch(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenExpand string
	var seenAPIVersion string
	var seenContentType string
	var seenAuth string
	var seenBody []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenExpand = r.URL.Query().Get("$expand")
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenContentType = r.Header.Get("Content-Type")
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprintf(w, `{"id":101,"rev":8,"relations":[{"rel":"ArtifactLink","url":%q,"attributes":{"name":"Pull Request"}}]}`, testPullRequestArtifactURL)
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

	item, err := client.LinkWorkItemToPullRequest(context.Background(), 101, 7, testPullRequestArtifactURL)
	if err != nil {
		t.Fatalf("LinkWorkItemToPullRequest returned error: %v", err)
	}
	if item.ID != 101 || item.Rev != 8 || !item.HasArtifactLink(testPullRequestArtifactURL) {
		t.Fatalf("work item = %+v, want validated linked response", item)
	}
	if seenMethod != http.MethodPatch {
		t.Fatalf("method = %q, want PATCH", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/My%20Project/_apis/wit/workitems/101" {
		t.Fatalf("path = %q, want work item path", seenPath)
	}
	if seenExpand != "relations" {
		t.Fatalf("$expand = %q, want relations", seenExpand)
	}
	if seenAPIVersion != "7.0" {
		t.Fatalf("api-version = %q, want configured version", seenAPIVersion)
	}
	if seenContentType != "application/json-patch+json" {
		t.Fatalf("Content-Type = %q, want application/json-patch+json", seenContentType)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
	if len(seenBody) != 2 {
		t.Fatalf("patch operations = %#v, want two operations", seenBody)
	}
	if seenBody[0]["op"] != "test" || seenBody[0]["path"] != "/rev" || seenBody[0]["value"] != float64(7) {
		t.Fatalf("revision operation = %#v, want revision test", seenBody[0])
	}
	if seenBody[1]["op"] != "add" || seenBody[1]["path"] != "/relations/-" {
		t.Fatalf("relation operation = %#v, want relation append", seenBody[1])
	}
	value, ok := seenBody[1]["value"].(map[string]any)
	if !ok {
		t.Fatalf("relation value = %#v, want object", seenBody[1]["value"])
	}
	attributes, ok := value["attributes"].(map[string]any)
	if !ok || attributes["name"] != "Pull Request" || value["rel"] != "ArtifactLink" || value["url"] != testPullRequestArtifactURL {
		t.Fatalf("relation value = %#v, want Pull Request ArtifactLink", value)
	}
}

func TestClientLinkWorkItemToPullRequestValidatesArgumentsBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	tests := []struct {
		name        string
		workItemID  int
		revision    int
		artifactURL string
		want        string
	}{
		{name: "work item ID", workItemID: 0, revision: 7, artifactURL: testPullRequestArtifactURL, want: "work item ID"},
		{name: "revision", workItemID: 101, revision: 0, artifactURL: testPullRequestArtifactURL, want: "revision"},
		{name: "artifact URL", workItemID: 101, revision: 7, artifactURL: "", want: "artifact URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.LinkWorkItemToPullRequest(context.Background(), tt.workItemID, tt.revision, tt.artifactURL)
			if err == nil {
				t.Fatal("LinkWorkItemToPullRequest error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want none", requests.Load())
	}
}

func TestClientLinkWorkItemToPullRequestValidatesResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "ID", body: `{"id":102,"rev":8,"relations":[{"rel":"ArtifactLink","url":"` + testPullRequestArtifactURL + `"}]}`, want: "102"},
		{name: "revision", body: `{"id":101,"relations":[{"rel":"ArtifactLink","url":"` + testPullRequestArtifactURL + `"}]}`, want: "revision"},
		{name: "relation", body: `{"id":101,"rev":8,"relations":[]}`, want: "missing pull request ArtifactLink"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tt.body)
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			_, err = client.LinkWorkItemToPullRequest(context.Background(), 101, 7, testPullRequestArtifactURL)
			if err == nil {
				t.Fatal("LinkWorkItemToPullRequest error = nil, want response validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestClientLinkWorkItemToPullRequestRejectsMultipleJSONValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"id":101,"rev":8,"relations":[{"rel":"ArtifactLink","url":%q}]} {}`, testPullRequestArtifactURL)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.LinkWorkItemToPullRequest(context.Background(), 101, 7, testPullRequestArtifactURL)
	if err == nil {
		t.Fatal("LinkWorkItemToPullRequest error = nil, want strict JSON error")
	}
	if !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("error = %q, want multiple JSON values", err.Error())
	}
}

func TestClientLinkWorkItemToPullRequestDoesNotFollowRedirect(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "/signin", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	httpClient, err := NewHTTPClient("")
	if err != nil {
		t.Fatalf("NewHTTPClient returned error: %v", err)
	}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.LinkWorkItemToPullRequest(context.Background(), 101, 7, testPullRequestArtifactURL)
	if err == nil {
		t.Fatal("LinkWorkItemToPullRequest error = nil, want redirect status error")
	}
	if !strings.Contains(err.Error(), "302") || !strings.Contains(err.Error(), "redirected") {
		t.Fatalf("error = %q, want redirect status and guidance", err.Error())
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want redirect not followed", requests.Load())
	}
}

func TestClientLinkWorkItemToPullRequestReturnsNetworkFailureContext(t *testing.T) {
	client, err := NewClient(&http.Client{Transport: failingRoundTripper{err: errors.New("transport unavailable")}}, ClientConfig{
		BaseURL:    "https://example.invalid",
		Project:    "Project",
		APIVersion: "7.1",
		PAT:        "secret",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.LinkWorkItemToPullRequest(context.Background(), 101, 7, testPullRequestArtifactURL)
	if err == nil {
		t.Fatal("LinkWorkItemToPullRequest error = nil, want network failure")
	}
	if !strings.Contains(err.Error(), "linking Azure DevOps work item 101") || !strings.Contains(err.Error(), "transport unavailable") {
		t.Fatalf("error = %q, want operation and network failure context", err.Error())
	}
}

func TestClientLinkWorkItemToPullRequestReturnsNon2xxDiagnostics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "revision conflict", http.StatusConflict)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.LinkWorkItemToPullRequest(context.Background(), 101, 7, testPullRequestArtifactURL)
	if err == nil {
		t.Fatal("LinkWorkItemToPullRequest error = nil, want HTTP status error")
	}
	if !strings.Contains(err.Error(), "linking Azure DevOps work item 101") || !strings.Contains(err.Error(), "409") || !strings.Contains(err.Error(), "revision conflict") {
		t.Fatalf("error = %q, want operation, status, and response diagnostics", err.Error())
	}
}
