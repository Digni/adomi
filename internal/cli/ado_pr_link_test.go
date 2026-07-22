package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

const testPullRequestArtifactURL = "vstfs:///Git/PullRequestId/project-guid%2Frepository-guid%2F42"

type prLinkCall struct {
	workItemID       int
	expectedRevision int
	artifactURL      string
}

type fakePRLinkClient struct {
	fakeADOClient
	pullRequest      *ado.PullRequest
	pullRequestErr   error
	workItems        map[int]*ado.WorkItem
	workItemErrors   map[int]error
	linkResponses    map[int]*ado.WorkItem
	linkErrors       map[int]error
	pullRequestCalls int
	workItemFetches  []int
	linkCalls        []prLinkCall
}

func (f *fakePRLinkClient) FetchPullRequest(_ context.Context, id int) (*ado.PullRequest, error) {
	f.pullRequestCalls++
	if f.pullRequestErr != nil {
		return nil, f.pullRequestErr
	}
	if f.pullRequest != nil {
		return f.pullRequest, nil
	}
	return testPullRequest(id), nil
}

func (f *fakePRLinkClient) FetchWorkItem(_ context.Context, id int) (*ado.WorkItem, error) {
	f.workItemFetches = append(f.workItemFetches, id)
	if err := f.workItemErrors[id]; err != nil {
		return nil, err
	}
	if item := f.workItems[id]; item != nil {
		return item, nil
	}
	return &ado.WorkItem{ID: id, Rev: id}, nil
}

func (f *fakePRLinkClient) LinkWorkItemToPullRequest(_ context.Context, workItemID, expectedRevision int, artifactURL string) (*ado.WorkItem, error) {
	f.linkCalls = append(f.linkCalls, prLinkCall{workItemID: workItemID, expectedRevision: expectedRevision, artifactURL: artifactURL})
	if err := f.linkErrors[workItemID]; err != nil {
		return nil, err
	}
	if item := f.linkResponses[workItemID]; item != nil {
		return item, nil
	}
	return &ado.WorkItem{ID: workItemID, Rev: expectedRevision + 1, Relations: []ado.Relation{{Rel: "ArtifactLink", URL: artifactURL}}}, nil
}

func testPullRequest(id int) *ado.PullRequest {
	return &ado.PullRequest{
		ID: id,
		Repository: ado.PullRequestRepo{
			ID:      "repository-guid",
			Project: map[string]any{"id": "project-guid", "name": "Project Name"},
		},
	}
}

func prLinkTestRunner(t *testing.T, client ADOClient) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" {
				t.Fatalf("LoadConfig locations = %q, %q", repoRoot, homeDir)
			}
			if requestedProfile != "company-cloud" {
				t.Fatalf("requested profile = %q, want company-cloud", requestedProfile)
			}
			if scope != config.GlobalScope {
				t.Fatalf("scope = %v, want global", scope)
			}
			return &config.Loaded{Profile: config.Profile{
				Name: "company-cloud", PATRef: "shared-ado", BaseURL: "https://dev.azure.com/org", Project: "Project Name", APIVersion: "7.1",
			}}, nil
		},
		NewHTTPClient: func(string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient: func(_ *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			if cfg.PAT != "secret-pat" || cfg.Project != "Project Name" {
				t.Fatalf("client config = %+v", cfg)
			}
			return client, nil
		},
	}}
}

func TestADOPullRequestLinkPlainOutputPreservesInputOrder(t *testing.T) {
	client := &fakePRLinkClient{workItems: map[int]*ado.WorkItem{
		101: {ID: 101, Rev: 7},
		102: {ID: 102, Rev: 9, Relations: []ado.Relation{{Rel: "ArtifactLink", URL: testPullRequestArtifactURL}}},
	}}
	runner := prLinkTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "link", "42", "--work-item", "101", "--work-item", "102", "--profile", "company-cloud", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got, want := stdout.String(), "101\n102\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := client.workItemFetches, []int{101, 102}; !reflect.DeepEqual(got, want) {
		t.Fatalf("work-item fetch order = %v, want %v", got, want)
	}
	if got, want := client.linkCalls, []prLinkCall{{workItemID: 101, expectedRevision: 7, artifactURL: testPullRequestArtifactURL}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("link calls = %+v, want %+v", got, want)
	}
}

func TestADOPullRequestLinkPlainOutputSingleItem(t *testing.T) {
	client := &fakePRLinkClient{workItems: map[int]*ado.WorkItem{101: {ID: 101, Rev: 7}}}
	runner := prLinkTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "link", "42", "--work-item", "101", "--profile", "company-cloud", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got, want := stdout.String(), "101\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := len(client.linkCalls), 1; got != want {
		t.Fatalf("link calls = %+v, want %d", client.linkCalls, want)
	}
}

func TestADOPullRequestLinkJSONReportsMixedResults(t *testing.T) {
	client := &fakePRLinkClient{workItems: map[int]*ado.WorkItem{
		101: {ID: 101, Rev: 7},
		102: {ID: 102, Rev: 9, Relations: []ado.Relation{{Rel: "ArtifactLink", URL: testPullRequestArtifactURL}}},
	}}
	runner := prLinkTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "link", "42", "--work-item", "101", "--work-item", "102", "--profile", "company-cloud", "--global", "--json"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := "{\"pullRequestId\":42,\"workItemIds\":[101,102],\"linkedWorkItemIds\":[101],\"alreadyLinkedWorkItemIds\":[102],\"action\":\"linked\"}\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestADOPullRequestLinkJSONReportsUnchanged(t *testing.T) {
	client := &fakePRLinkClient{workItems: map[int]*ado.WorkItem{
		101: {ID: 101, Rev: 7, Relations: []ado.Relation{{Rel: "ArtifactLink", URL: testPullRequestArtifactURL}}},
	}}
	runner := prLinkTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "link", "42", "--work-item", "101", "--profile", "company-cloud", "--global", "--json"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := "{\"pullRequestId\":42,\"workItemIds\":[101],\"linkedWorkItemIds\":[],\"alreadyLinkedWorkItemIds\":[101],\"action\":\"unchanged\"}\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if len(client.linkCalls) != 0 {
		t.Fatalf("link calls = %+v, want none", client.linkCalls)
	}
}

func TestADOPullRequestLinkValidationRejectsInvalidArgumentsBeforeDependencies(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing pull request ID", args: nil, want: "usage"},
		{name: "non-integer pull request ID", args: []string{"abc", "--work-item", "101"}, want: "pull request ID"},
		{name: "non-positive pull request ID", args: []string{"0", "--work-item", "101"}, want: "pull request ID"},
		{name: "missing work item", args: []string{"42"}, want: "--work-item"},
		{name: "missing work item value", args: []string{"42", "--work-item"}, want: "--work-item requires"},
		{name: "non-integer work item", args: []string{"42", "--work-item", "abc"}, want: "work item ID"},
		{name: "non-positive work item", args: []string{"42", "--work-item", "-1"}, want: "work item ID"},
		{name: "duplicate work item", args: []string{"42", "--work-item", "101", "--work-item", "101"}, want: "duplicate work item ID 101"},
		{name: "missing profile value", args: []string{"42", "--work-item", "101", "--profile"}, want: "--profile requires"},
		{name: "unknown flag", args: []string{"42", "--work-item", "101", "--unknown"}, want: "unknown argument"},
		{name: "unexpected positional", args: []string{"42", "--work-item", "101", "102"}, want: "unknown argument"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depsCalled := false
			runner := Runner{deps: Dependencies{Getwd: func() (string, error) {
				depsCalled = true
				return "", errors.New("must not be called")
			}}}
			var stdout bytes.Buffer
			err := runner.Run(append([]string{"ado", "pr", "link"}, tt.args...), strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if depsCalled {
				t.Fatal("dependencies were loaded before argument validation completed")
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPullRequestLinkPreflightsEveryWorkItemBeforeWrites(t *testing.T) {
	client := &fakePRLinkClient{
		workItems:      map[int]*ado.WorkItem{101: {ID: 101, Rev: 3}},
		workItemErrors: map[int]error{102: errors.New("not found")},
	}
	runner := prLinkTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "link", "42", "--work-item", "101", "--work-item", "102", "--profile", "company-cloud", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "102") {
		t.Fatalf("error = %v, want work item 102", err)
	}
	if len(client.linkCalls) != 0 {
		t.Fatalf("link calls = %+v, want none", client.linkCalls)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPullRequestLinkRejectsIncompletePullRequestIdentityBeforeWorkItemFetch(t *testing.T) {
	tests := []struct {
		name string
		pr   *ado.PullRequest
		want string
	}{
		{name: "missing project", pr: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repository-guid"}}, want: "project ID"},
		{name: "missing repository", pr: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{Project: map[string]any{"id": "project-guid"}}}, want: "repository ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakePRLinkClient{pullRequest: tt.pr}
			runner := prLinkTestRunner(t, client)
			var stdout bytes.Buffer
			err := runner.Run([]string{"ado", "pr", "link", "42", "--work-item", "101", "--profile", "company-cloud", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if len(client.workItemFetches) != 0 || len(client.linkCalls) != 0 {
				t.Fatalf("work-item calls = %v/%v, want none", client.workItemFetches, client.linkCalls)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPullRequestLinkRejectsInvalidPreflightWorkItem(t *testing.T) {
	tests := []struct {
		name string
		item *ado.WorkItem
		want string
	}{
		{name: "mismatched ID", item: &ado.WorkItem{ID: 999, Rev: 3}, want: "does not match"},
		{name: "missing revision", item: &ado.WorkItem{ID: 101}, want: "revision"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakePRLinkClient{workItems: map[int]*ado.WorkItem{101: tt.item}}
			runner := prLinkTestRunner(t, client)
			var stdout bytes.Buffer
			err := runner.Run([]string{"ado", "pr", "link", "42", "--work-item", "101", "--profile", "company-cloud", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if len(client.linkCalls) != 0 {
				t.Fatalf("link calls = %+v, want none", client.linkCalls)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPullRequestLinkFailureStopsWithoutRetryAndReportsPriorLinks(t *testing.T) {
	client := &fakePRLinkClient{
		workItems:  map[int]*ado.WorkItem{101: {ID: 101, Rev: 3}, 102: {ID: 102, Rev: 4}, 103: {ID: 103, Rev: 5}},
		linkErrors: map[int]error{102: errors.New("revision conflict")},
	}
	runner := prLinkTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "link", "42", "--work-item", "101", "--work-item", "102", "--work-item", "103", "--profile", "company-cloud", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want failure")
	}
	for _, want := range []string{"work item 102", "101", "revision conflict"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	}
	if got, want := len(client.linkCalls), 2; got != want {
		t.Fatalf("link calls = %+v, want %d calls", client.linkCalls, want)
	}
	if client.linkCalls[0].workItemID != 101 || client.linkCalls[1].workItemID != 102 {
		t.Fatalf("link calls = %+v, want 101 then 102 only", client.linkCalls)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPullRequestLinkRejectsUnprovenUpdateResponse(t *testing.T) {
	tests := []struct {
		name     string
		response *ado.WorkItem
		want     string
	}{
		{name: "mismatched ID", response: &ado.WorkItem{ID: 999, Rev: 4, Relations: []ado.Relation{{Rel: "ArtifactLink", URL: testPullRequestArtifactURL}}}, want: "does not match"},
		{name: "missing revision", response: &ado.WorkItem{ID: 101, Relations: []ado.Relation{{Rel: "ArtifactLink", URL: testPullRequestArtifactURL}}}, want: "revision"},
		{name: "missing relation", response: &ado.WorkItem{ID: 101, Rev: 4}, want: "ArtifactLink"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakePRLinkClient{
				workItems:     map[int]*ado.WorkItem{101: {ID: 101, Rev: 3}},
				linkResponses: map[int]*ado.WorkItem{101: tt.response},
			}
			runner := prLinkTestRunner(t, client)
			var stdout bytes.Buffer
			err := runner.Run([]string{"ado", "pr", "link", "42", "--work-item", "101", "--profile", "company-cloud", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if len(client.linkCalls) != 1 {
				t.Fatalf("link calls = %+v, want one", client.linkCalls)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPullRequestLinkFetchFailureLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRLinkClient{pullRequestErr: fmt.Errorf("fetch failed")}
	runner := prLinkTestRunner(t, client)
	var stdout bytes.Buffer
	err := runner.Run([]string{"ado", "pr", "link", "42", "--work-item", "101", "--profile", "company-cloud", "--global"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("error = %v, want fetch failed", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
