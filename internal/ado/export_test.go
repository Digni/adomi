package ado

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOutputPathUsesRepoLocalAdomiDirectory(t *testing.T) {
	got := OutputPath("/repo", "company-cloud", "MyProject", 12345)
	want := filepath.Join("/repo", ".adomi", "context", "work-items", "12345")
	if got != want {
		t.Fatalf("OutputPath = %q, want %q", got, want)
	}
}

func TestExportContextWritesIndexTreeItemsAndHTML(t *testing.T) {
	repoRoot := t.TempDir()
	createdAt := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	tree := &WorkItemTree{
		RootID: 12345,
		EpicID: 10000,
		WorkItems: []WorkItem{
			{
				ID: 12345,
				Fields: map[string]any{
					"System.WorkItemType": "Task",
					"System.Title":        "Implement <checkout>",
					"System.Description":  `<p>Description</p><img src="https://example.test/pixel" onerror="alert(1)">`,
				},
			},
			{
				ID: 10000,
				Fields: map[string]any{
					"System.WorkItemType": "Epic",
					"System.Title":        "Epic",
				},
			},
		},
	}

	outputDir, err := ExportContext(context.Background(), fakeDownloader{}, ExportOptions{
		RepoRoot:  repoRoot,
		Profile:   "company-cloud",
		Project:   "MyProject",
		CreatedAt: createdAt,
	}, tree)
	if err != nil {
		t.Fatalf("ExportContext returned error: %v", err)
	}

	wantOutputDir := filepath.Join(repoRoot, ".adomi", "context", "work-items", "12345")
	if outputDir != wantOutputDir {
		t.Fatalf("output dir = %q, want %q", outputDir, wantOutputDir)
	}
	assertExists(t, filepath.Join(outputDir, "index.json"))
	assertExists(t, filepath.Join(outputDir, "tree.json"))
	assertExists(t, filepath.Join(outputDir, "items", "12345.json"))
	assertExists(t, filepath.Join(outputDir, "html", "12345.html"))

	var index Index
	readJSON(t, filepath.Join(outputDir, "index.json"), &index)
	if index.Source != "azure-devops" {
		t.Fatalf("source = %q, want azure-devops", index.Source)
	}
	if index.Profile != "company-cloud" || index.Project != "MyProject" {
		t.Fatalf("profile/project = %q/%q, want company-cloud/MyProject", index.Profile, index.Project)
	}
	if index.RootWorkItemID != 12345 || index.EpicWorkItemID != 10000 {
		t.Fatalf("root/epic IDs = %d/%d, want 12345/10000", index.RootWorkItemID, index.EpicWorkItemID)
	}
	if index.CreatedAt != "2026-04-26T12:00:00Z" {
		t.Fatalf("createdAt = %q, want RFC3339 UTC", index.CreatedAt)
	}
	if got := index.WorkItems[0].Path; got != "items/12345.json" {
		t.Fatalf("item path = %q, want relative item path", got)
	}
	if got := index.WorkItems[0].HTMLPath; got != "html/12345.html" {
		t.Fatalf("html path = %q, want relative HTML path", got)
	}
	if got := index.WorkItems[0].AttachmentsPath; got != "attachments/12345" {
		t.Fatalf("attachments path = %q, want relative attachments path", got)
	}

	html := readFile(t, filepath.Join(outputDir, "html", "12345.html"))
	if !strings.Contains(html, "<h1>12345: Implement &lt;checkout&gt;</h1>") {
		t.Fatalf("html = %q, want escaped title", html)
	}
	if strings.Contains(html, `<img src="https://example.test/pixel" onerror="alert(1)">`) {
		t.Fatalf("html = %q, want executable description markup escaped", html)
	}
	if !strings.Contains(html, `&lt;p&gt;Description&lt;/p&gt;&lt;img src=&#34;https://example.test/pixel&#34; onerror=&#34;alert(1)&#34;&gt;`) {
		t.Fatalf("html = %q, want escaped description", html)
	}
}

func TestExportContextRemovesStaleFilesFromPreviousRun(t *testing.T) {
	repoRoot := t.TempDir()
	outputDir := OutputPath(repoRoot, "company-cloud", "MyProject", 12345)
	stalePath := filepath.Join(outputDir, "attachments", "12345", "old.png")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatalf("creating stale dir: %v", err)
	}
	if err := os.WriteFile(stalePath, []byte("old"), 0o644); err != nil {
		t.Fatalf("writing stale file: %v", err)
	}

	_, err := ExportContext(context.Background(), fakeDownloader{}, ExportOptions{
		RepoRoot: repoRoot,
		Profile:  "company-cloud",
		Project:  "MyProject",
	}, &WorkItemTree{RootID: 12345, WorkItems: []WorkItem{{ID: 12345}}})
	if err != nil {
		t.Fatalf("ExportContext returned error: %v", err)
	}

	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("stale file stat error = %v, want not exist", err)
	}
}

func TestExportContextRejectsNilTree(t *testing.T) {
	_, err := ExportContext(context.Background(), fakeDownloader{}, ExportOptions{RepoRoot: t.TempDir()}, nil)
	if err == nil {
		t.Fatal("ExportContext error = nil, want error")
	}
	if !strings.Contains(err.Error(), "work item tree") {
		t.Fatalf("error = %q, want work item tree context", err.Error())
	}
}

func TestExportContextDefaultsZeroCreatedAt(t *testing.T) {
	repoRoot := t.TempDir()

	outputDir, err := ExportContext(context.Background(), nil, ExportOptions{
		RepoRoot: repoRoot,
		Profile:  "company-cloud",
		Project:  "MyProject",
	}, &WorkItemTree{RootID: 12345, WorkItems: []WorkItem{{ID: 12345}}})
	if err != nil {
		t.Fatalf("ExportContext returned error: %v", err)
	}

	var index Index
	readJSON(t, filepath.Join(outputDir, "index.json"), &index)
	if index.CreatedAt == "" {
		t.Fatal("createdAt is empty, want default timestamp")
	}
	if _, err := time.Parse(time.RFC3339, index.CreatedAt); err != nil {
		t.Fatalf("createdAt = %q, want RFC3339 timestamp: %v", index.CreatedAt, err)
	}
}

func TestExportContextAllowsNilDownloader(t *testing.T) {
	repoRoot := t.TempDir()
	tree := &WorkItemTree{
		RootID: 12345,
		WorkItems: []WorkItem{{
			ID: 12345,
			Relations: []Relation{{
				Rel:        attachmentRelationType,
				URL:        "https://example.test/attachment",
				Attributes: map[string]any{"name": "note.txt"},
			}},
		}},
	}

	outputDir, err := ExportContext(context.Background(), nil, ExportOptions{
		RepoRoot: repoRoot,
		Profile:  "company-cloud",
		Project:  "MyProject",
	}, tree)
	if err != nil {
		t.Fatalf("ExportContext returned error: %v", err)
	}
	assertExists(t, filepath.Join(outputDir, "items", "12345.json"))
	if _, err := os.Stat(filepath.Join(outputDir, "attachments")); !os.IsNotExist(err) {
		t.Fatalf("attachments stat error = %v, want not exist", err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}
