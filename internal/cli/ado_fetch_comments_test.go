package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestParseFetchArgsAcceptsCommentsOnce(t *testing.T) {
	parsed, err := parseFetchArgs([]string{"123", "--include-comments", "--json"})
	if err != nil {
		t.Fatalf("parseFetchArgs returned error: %v", err)
	}
	if !parsed.includeComments || !parsed.json {
		t.Fatalf("parsed = %+v, want comments and JSON enabled", parsed)
	}
	if _, err := parseFetchArgs([]string{"123", "--include-comments", "--include-comments"}); err == nil {
		t.Fatal("repeated --include-comments succeeded, want error")
	}
}

func TestADOFetchHelpDescribesOptionalComments(t *testing.T) {
	help := runHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "fetch", "--help"})
	for _, want := range []string{"--include-comments", "non-deleted", "comments/<work-item-id>.json", "attachment counts remain unchanged", "work-item read permission"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help = %q, want %q", help, want)
		}
	}
}

func TestADOFetchCommentsUsesDistinctTreeItemsAndPreservesAttachmentCount(t *testing.T) {
	comments := testReadComments(t, `[{"id":10,"workItemId":1,"text":"handover"}]`)
	client := &fakeFetchCommentClient{comments: map[int][]ado.WorkItemReadComment{1: comments, 2: {}, 3: comments}}
	tree := &ado.WorkItemTree{RootID: 1, WorkItems: []ado.WorkItem{{ID: 1}, {ID: 2}, {ID: 1}, {ID: 3}}}
	var gotComments map[int][]ado.WorkItemReadComment
	runner := fetchCommentsTestRunner(t, client, tree)
	runner.deps.ExportContext = func(_ context.Context, _ ado.AttachmentDownloader, opts ado.ExportOptions, _ *ado.WorkItemTree, progress ado.ProgressFunc) (string, error) {
		gotComments = opts.Comments
		progress("Downloaded attachment.txt")
		return "/repo/.adomi/context/work-items/1", nil
	}
	var stdout, stderr bytes.Buffer
	if err := runner.Run([]string{"ado", "fetch", "1", "--include-comments", "--json"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got, want := stdout.String(), `{"path":"/repo/.adomi/context/work-items/1","workItems":4,"attachments":1}`+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if len(client.requested) != 3 {
		t.Fatalf("comment requests = %v, want one per distinct item", client.requested)
	}
	if gotComments == nil || len(gotComments) != 3 || gotComments[2] == nil || len(gotComments[2]) != 0 {
		t.Fatalf("comments map = %#v, want explicit empty item entry", gotComments)
	}
	if !strings.Contains(stderr.String(), "Fetching comments for work item 1") || !strings.Contains(stderr.String(), "Downloaded attachment.txt") {
		t.Fatalf("stderr = %q, want comment and attachment feedback", stderr.String())
	}
}

func TestADOFetchCommentsFailureLeavesExporterAndStdoutUntouched(t *testing.T) {
	client := &fakeFetchCommentClient{commentErr: errors.New("comments unavailable")}
	tree := &ado.WorkItemTree{RootID: 1, WorkItems: []ado.WorkItem{{ID: 1}}}
	runner := fetchCommentsTestRunner(t, client, tree)
	exporterCalled := false
	runner.deps.ExportContext = func(context.Context, ado.AttachmentDownloader, ado.ExportOptions, *ado.WorkItemTree, ado.ProgressFunc) (string, error) {
		exporterCalled = true
		return "", nil
	}
	var stdout bytes.Buffer
	err := runner.Run([]string{"ado", "fetch", "1", "--include-comments"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "comments unavailable") {
		t.Fatalf("error = %v, want comment read error", err)
	}
	if exporterCalled || stdout.Len() != 0 {
		t.Fatalf("exporterCalled/stdout = %t/%q, want false/empty", exporterCalled, stdout.String())
	}
}

func TestADOFetchCommentsHTTPToFilesystemIncludesRootParentsAndChildren(t *testing.T) {
	requested := map[int]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/comments") {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		var id int
		if _, err := fmt.Sscanf(filepath.Base(filepath.Dir(r.URL.Path)), "%d", &id); err != nil {
			t.Fatalf("parse comment path %q: %v", r.URL.Path, err)
		}
		requested[id]++
		if id == 2 {
			fmt.Fprint(w, `{"comments":[],"totalCount":0}`)
			return
		}
		fmt.Fprintf(w, `{"comments":[{"id":%d,"workItemId":%d,"text":"<script>alert(1)</script>"}],"totalCount":1}`, id+100, id)
	}))
	t.Cleanup(server.Close)

	repoRoot := t.TempDir()
	client, err := ado.NewClient(server.Client(), ado.ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	tree := &ado.WorkItemTree{RootID: 1, WorkItems: []ado.WorkItem{{ID: 1}, {ID: 2}, {ID: 3}}}
	runner := Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"ado": "secret"}},
		Getwd:        func() (string, error) { return repoRoot, nil },
		UserHomeDir:  func() (string, error) { return t.TempDir(), nil },
		FindRepoRoot: func(string) (string, error) { return repoRoot, nil },
		LoadConfig: func(string, string, string, config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "profile", PATRef: "ado", BaseURL: server.URL, Project: "Project", APIVersion: "7.1"}}, nil
		},
		NewHTTPClient: func(string) (*http.Client, error) { return server.Client(), nil },
		NewADOClient:  func(*http.Client, ado.ClientConfig) (ADOClient, error) { return client, nil },
		FetchTree: func(context.Context, ado.WorkItemFetcher, int, ado.ProgressFunc) (*ado.WorkItemTree, error) {
			return tree, nil
		},
		Now: func() time.Time { return time.Unix(0, 0).UTC() },
	}}
	var stdout bytes.Buffer
	if err := runner.Run([]string{"ado", "fetch", "1", "--include-comments"}, strings.NewReader(""), &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != filepath.Join(repoRoot, ".adomi", "context", "work-items", "1")+"\n" {
		t.Fatalf("stdout = %q, want exported path", stdout.String())
	}
	for _, id := range []int{1, 2, 3} {
		if requested[id] != 1 {
			t.Fatalf("comment requests for %d = %d, want 1", id, requested[id])
		}
	}
	commentsJSON := filepath.Join(repoRoot, ".adomi", "context", "work-items", "1", "comments", "1.json")
	if _, err := os.Stat(commentsJSON); err != nil {
		t.Fatalf("comments artifact stat error: %v", err)
	}
	htmlPath := filepath.Join(repoRoot, ".adomi", "context", "work-items", "1", "html", "1.html")
	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("reading HTML artifact: %v", err)
	}
	html := string(htmlData)
	if strings.Contains(html, "<script>alert(1)</script>") || !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("html = %q, want escaped comment text", html)
	}
}

type fakeFetchCommentClient struct {
	fakeADOClient
	comments   map[int][]ado.WorkItemReadComment
	commentErr error
	requested  []int
}

func (c *fakeFetchCommentClient) ListWorkItemComments(_ context.Context, id int) ([]ado.WorkItemReadComment, error) {
	c.requested = append(c.requested, id)
	if c.commentErr != nil {
		return nil, c.commentErr
	}
	return c.comments[id], nil
}

func fetchCommentsTestRunner(t *testing.T, client ADOClient, tree *ado.WorkItemTree) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"ado": "pat"}},
		Getwd:        func() (string, error) { return "/repo", nil },
		UserHomeDir:  func() (string, error) { return "/home", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(string, string, string, config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "profile", PATRef: "ado", BaseURL: "https://dev.azure.com/org", Project: "Project"}}, nil
		},
		NewHTTPClient: func(string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient:  func(*http.Client, ado.ClientConfig) (ADOClient, error) { return client, nil },
		FetchTree: func(context.Context, ado.WorkItemFetcher, int, ado.ProgressFunc) (*ado.WorkItemTree, error) {
			return tree, nil
		},
	}}
}

func testReadComments(t *testing.T, raw string) []ado.WorkItemReadComment {
	t.Helper()
	var comments []ado.WorkItemReadComment
	if err := json.Unmarshal([]byte(raw), &comments); err != nil {
		t.Fatalf("unmarshal test comments: %v", err)
	}
	return comments
}
