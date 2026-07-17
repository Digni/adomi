package ado

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
