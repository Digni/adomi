package ado

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestExportContextWritesExplicitCommentArtifactsAndEscapedDiscussion(t *testing.T) {
	repoRoot := t.TempDir()
	displayName := "<Reviewer>"
	createdDate := "2026-09-07T10:00:00Z"
	modifiedDate := "2026-09-07T10:01:00Z"
	tree := &WorkItemTree{RootID: 1, WorkItems: []WorkItem{{ID: 1}, {ID: 2}}}
	comments := map[int][]WorkItemReadComment{
		1: {},
		2: {{
			ID:           10,
			WorkItemID:   2,
			Text:         `<script>alert("x")</script>`,
			Author:       &WorkItemCommentAuthor{DisplayName: &displayName},
			CreatedDate:  &createdDate,
			ModifiedDate: &modifiedDate,
		}},
	}

	outputDir, err := ExportContext(context.Background(), nil, ExportOptions{RepoRoot: repoRoot, Comments: comments}, tree, nil)
	if err != nil {
		t.Fatalf("ExportContext returned error: %v", err)
	}

	for _, id := range []int{1, 2} {
		path := filepath.Join(outputDir, "comments", strconv.Itoa(id)+".json")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("comment artifact %d stat error: %v", id, err)
		}
	}
	var empty WorkItemComments
	readJSON(t, filepath.Join(outputDir, "comments", "1.json"), &empty)
	if empty.WorkItemID != 1 || empty.Comments == nil || len(empty.Comments) != 0 {
		t.Fatalf("empty comments = %+v, want explicit empty array", empty)
	}
	htmlContent := readFile(t, filepath.Join(outputDir, "html", "2.html"))
	if strings.Contains(htmlContent, `<script>alert("x")</script>`) || !strings.Contains(htmlContent, "&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;") || !strings.Contains(htmlContent, "&lt;Reviewer&gt;") {
		t.Fatalf("html = %q, want escaped comment data", htmlContent)
	}
	var index Index
	readJSON(t, filepath.Join(outputDir, "index.json"), &index)
	if index.WorkItems[0].CommentsPath != "comments/1.json" || index.WorkItems[1].CommentsPath != "comments/2.json" {
		t.Fatalf("comments paths = %+v, want relative comment paths", index.WorkItems)
	}
}

func TestExportContextWithoutCommentsOmitsCommentArtifactsAndIndexFields(t *testing.T) {
	repoRoot := t.TempDir()
	tree := &WorkItemTree{RootID: 1, WorkItems: []WorkItem{{ID: 1}}}
	commentText := "old discussion"
	if _, err := ExportContext(context.Background(), nil, ExportOptions{RepoRoot: repoRoot, Comments: map[int][]WorkItemReadComment{
		1: {{ID: 10, WorkItemID: 1, Text: commentText}},
	}}, tree, nil); err != nil {
		t.Fatalf("initial enriched ExportContext returned error: %v", err)
	}
	outputDir, err := ExportContext(context.Background(), nil, ExportOptions{RepoRoot: repoRoot}, tree, nil)
	if err != nil {
		t.Fatalf("ExportContext returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "comments")); !os.IsNotExist(err) {
		t.Fatalf("comments directory stat error = %v, want absent", err)
	}
	var index Index
	readJSON(t, filepath.Join(outputDir, "index.json"), &index)
	if index.WorkItems[0].CommentsPath != "" {
		t.Fatalf("comments path = %q, want omitted", index.WorkItems[0].CommentsPath)
	}
	htmlContent := readFile(t, filepath.Join(outputDir, "html", "1.html"))
	if strings.Contains(htmlContent, "Discussion") || strings.Contains(htmlContent, commentText) {
		t.Fatalf("html = %q, want stale discussion removed", htmlContent)
	}
}
