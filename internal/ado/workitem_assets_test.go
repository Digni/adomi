package ado

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportContextMapsAndDeduplicatesImageEvidence(t *testing.T) {
	const base = "https://dev.azure.com/org"
	const attached = base + "/_apis/wit/attachments/a"
	const inline = base + "/Project/_apis/wit/attachments/b"
	body := assetTestPNG(t)
	downloader := &assetTestDownloader{data: map[string][]byte{attached: body, inline: body}}
	markdown := "markdown"
	item := WorkItem{ID: 1, Fields: map[string]any{
		"System.Description": `<img src="` + attached + `?fileName=shot.png&amp;download=true"><img src="_apis/wit/attachments/b"><img src="https://outside.test/pixel">`,
	}, Relations: []Relation{{Rel: "AttachedFile", URL: attached, Attributes: map[string]any{"name": "shot.png"}}}}
	comments := map[int][]WorkItemReadComment{1: {{ID: 7, Format: &markdown, Text: "![shot](" + attached + ")"}}}
	progress := 0
	dir, err := ExportContext(context.Background(), downloader, ExportOptions{RepoRoot: t.TempDir(), BaseURL: base, Project: "Project", Comments: comments}, &WorkItemTree{RootID: 1, WorkItems: []WorkItem{item}}, func(string) { progress++ })
	if err != nil {
		t.Fatal(err)
	}
	var manifest WorkItemAssets
	readJSON(t, filepath.Join(dir, "assets", "1.json"), &manifest)
	if manifest.WorkItemID != 1 || len(manifest.Assets) != 3 || progress != 2 || len(downloader.calls) != 2 {
		t.Fatalf("manifest = %+v, progress = %d, calls = %v", manifest, progress, downloader.calls)
	}
	if len(manifest.Assets[0].Sources) != 3 || manifest.Assets[0].Sources[2].CommentID != 7 {
		t.Fatalf("source mapping = %+v", manifest.Assets[0])
	}
	for _, asset := range manifest.Assets[:2] {
		if asset.Status != "downloaded" || !strings.HasSuffix(asset.Path, ".png") {
			t.Fatalf("downloaded image = %+v", asset)
		}
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(asset.Path)))
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("image bytes changed: %v", err)
		}
	}
	if manifest.Assets[2].Status != "skipped" || manifest.Assets[2].Path != "" || manifest.Assets[2].Reason == "" {
		t.Fatalf("unsupported image = %+v", manifest.Assets[2])
	}
	var saved WorkItem
	readJSON(t, filepath.Join(dir, "items", "1.json"), &saved)
	if saved.Description() != item.Description() {
		t.Fatal("raw description changed")
	}
	if strings.Contains(readFile(t, filepath.Join(dir, "html", "1.html")), "<img") {
		t.Fatal("export renders remote HTML")
	}
	var index Index
	readJSON(t, filepath.Join(dir, "index.json"), &index)
	if index.WorkItems[0].AssetsPath != "assets/1.json" {
		t.Fatalf("missing manifest index path: %+v", index)
	}
}

func assetTestPNG(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := png.Encode(&output, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestExportContextRejectsNonImageBeforePublishingMetadata(t *testing.T) {
	const base = "https://dev.azure.com/org"
	const url = base + "/_apis/wit/attachments/a"
	for _, attached := range []bool{false, true} {
		t.Run(map[bool]string{false: "inline-only", true: "attached-and-inline"}[attached], func(t *testing.T) {
			item := WorkItem{ID: 1, Fields: map[string]any{"System.Description": `<img src="` + url + `">`}}
			if attached {
				item.Relations = []Relation{{Rel: "AttachedFile", URL: url}}
			}
			opts := ExportOptions{RepoRoot: t.TempDir(), BaseURL: base, Project: "Project"}
			progress := 0
			_, err := ExportContext(context.Background(), &assetTestDownloader{data: map[string][]byte{url: []byte("<html>login</html>")}}, opts, &WorkItemTree{RootID: 1, WorkItems: []WorkItem{item}}, func(string) { progress++ })
			if err == nil || !strings.Contains(err.Error(), "recognized image") || progress != 0 {
				t.Fatalf("error = %v, progress = %d", err, progress)
			}
			for _, path := range []string{"index.json", "tree.json", "items/1.json", "assets/1.json"} {
				if _, err := os.Stat(filepath.Join(OutputPath(opts.RepoRoot, "", "", 1), path)); !os.IsNotExist(err) {
					t.Fatalf("published %s after failure: %v", path, err)
				}
			}
		})
	}
}

func TestExportContextRefreshRemovesCommentOnlyImageEvidence(t *testing.T) {
	const base = "https://dev.azure.com/org"
	const url = base + "/_apis/wit/attachments/a"
	opts := ExportOptions{RepoRoot: t.TempDir(), BaseURL: base, Comments: map[int][]WorkItemReadComment{
		1: {{ID: 7, Text: "![shot](" + url + ")"}},
	}}
	tree := &WorkItemTree{RootID: 1, WorkItems: []WorkItem{{ID: 1}, {ID: 2}}}
	download := &assetTestDownloader{data: map[string][]byte{url: assetTestPNG(t)}}
	dir, err := ExportContext(context.Background(), download, opts, tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(download.calls) != 1 {
		t.Fatalf("comment image not downloaded: %v", download.calls)
	}
	opts.Comments = nil
	if _, err := ExportContext(context.Background(), download, opts, tree, nil); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"attachments", "comments"} {
		if _, err := os.Stat(filepath.Join(dir, directory)); !os.IsNotExist(err) {
			t.Fatalf("stale %s directory: %v", directory, err)
		}
	}
	var manifest WorkItemAssets
	readJSON(t, filepath.Join(dir, "assets", "1.json"), &manifest)
	if manifest.Assets == nil || len(manifest.Assets) != 0 {
		t.Fatalf("expected explicit empty manifest: %+v", manifest)
	}
}

func TestExportContextWithoutDownloaderRecordsUnavailableEvidence(t *testing.T) {
	const base = "https://dev.azure.com/org"
	item := WorkItem{ID: 1, Fields: map[string]any{"System.Description": `<img src="` + base + `/_apis/wit/attachments/a">`}}
	dir, err := ExportContext(context.Background(), nil, ExportOptions{RepoRoot: t.TempDir(), BaseURL: base}, &WorkItemTree{RootID: 1, WorkItems: []WorkItem{item}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var manifest WorkItemAssets
	readJSON(t, filepath.Join(dir, "assets", "1.json"), &manifest)
	if len(manifest.Assets) != 1 || manifest.Assets[0].Status != "skipped" || manifest.Assets[0].Path != "" || manifest.Assets[0].Reason == "" {
		t.Fatalf("unavailable evidence = %+v", manifest)
	}
}

type assetTestDownloader struct {
	data  map[string][]byte
	calls []string
}

func (d *assetTestDownloader) Download(_ context.Context, url string) ([]byte, error) {
	d.calls = append(d.calls, url)
	return d.data[url], nil
}
