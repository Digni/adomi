package ado

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFetchWikiContextFetchesOnePage(t *testing.T) {
	fetcher := &fakeWikiFetcher{
		wiki: &Wiki{ID: "wiki-id", Name: "Engineering"},
		pages: map[string]*WikiPage{
			"/Guide|content": {Path: "/Guide", Content: "# Guide\n"},
		},
	}

	got, err := FetchWikiContext(context.Background(), fetcher, "Engineering", "/Guide", false)
	if err != nil {
		t.Fatalf("FetchWikiContext returned error: %v", err)
	}
	if got.Wiki == nil || got.Wiki.ID != "wiki-id" || got.RequestedPath != "/Guide" || got.Recursive {
		t.Fatalf("context = %#v, want resolved wiki and non-recursive /Guide selection", got)
	}
	if !reflect.DeepEqual(got.Pages, []WikiPage{{Path: "/Guide", Content: "# Guide\n"}}) {
		t.Fatalf("pages = %#v, want selected page", got.Pages)
	}
	wantCalls := []wikiPageFetchCall{{
		wikiIdentifier: "wiki-id",
		opts: WikiPageFetchOptions{
			Path:           "/Guide",
			IncludeContent: true,
		},
	}}
	if !reflect.DeepEqual(fetcher.pageCalls, wantCalls) {
		t.Fatalf("page calls = %#v, want %#v", fetcher.pageCalls, wantCalls)
	}
}

func TestFetchWikiContextFetchesRecursiveTreeInStablePathOrder(t *testing.T) {
	fetcher := &fakeWikiFetcher{
		wiki: &Wiki{ID: "wiki-id", Name: "Engineering"},
		pages: map[string]*WikiPage{
			"/Guide|recursion=full": {
				Path: "/Guide",
				SubPages: []WikiPage{
					{Path: "/Guide/Zeta"},
					{Path: "/Guide/Alpha", SubPages: []WikiPage{{Path: "/Guide/Alpha/Details"}}},
				},
			},
			"/Guide|content":               {Path: "/Guide", Content: "root"},
			"/Guide/Alpha|content":         {Path: "/Guide/Alpha", Content: "alpha"},
			"/Guide/Alpha/Details|content": {Path: "/Guide/Alpha/Details", Content: "details"},
			"/Guide/Zeta|content":          {Path: "/Guide/Zeta", Content: "zeta"},
		},
	}

	got, err := FetchWikiContext(context.Background(), fetcher, "Engineering", "/Guide", true)
	if err != nil {
		t.Fatalf("FetchWikiContext returned error: %v", err)
	}
	if !got.Recursive {
		t.Fatal("Recursive = false, want true")
	}
	wantPaths := []string{"/Guide", "/Guide/Alpha", "/Guide/Alpha/Details", "/Guide/Zeta"}
	if paths := wikiPagePaths(got.Pages); !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("page paths = %#v, want stable path order %#v", paths, wantPaths)
	}
	wantCalls := []wikiPageFetchCall{
		{wikiIdentifier: "wiki-id", opts: WikiPageFetchOptions{Path: "/Guide", RecursionLevel: "full"}},
		{wikiIdentifier: "wiki-id", opts: WikiPageFetchOptions{Path: "/Guide", IncludeContent: true}},
		{wikiIdentifier: "wiki-id", opts: WikiPageFetchOptions{Path: "/Guide/Alpha", IncludeContent: true}},
		{wikiIdentifier: "wiki-id", opts: WikiPageFetchOptions{Path: "/Guide/Alpha/Details", IncludeContent: true}},
		{wikiIdentifier: "wiki-id", opts: WikiPageFetchOptions{Path: "/Guide/Zeta", IncludeContent: true}},
	}
	if !reflect.DeepEqual(fetcher.pageCalls, wantCalls) {
		t.Fatalf("page calls = %#v, want metadata then sequential content calls %#v", fetcher.pageCalls, wantCalls)
	}
}

func TestFetchWikiContextRecursiveRootIncludesWholeWikiTree(t *testing.T) {
	fetcher := &fakeWikiFetcher{
		wiki: &Wiki{ID: "wiki-id"},
		pages: map[string]*WikiPage{
			"/|recursion=full":      {Path: "/", SubPages: []WikiPage{{Path: "/Guide"}, {Path: "/Other/Nested"}}},
			"/|content":             {Path: "/", Content: "root"},
			"/Guide|content":        {Path: "/Guide", Content: "guide"},
			"/Other/Nested|content": {Path: "/Other/Nested", Content: "nested"},
		},
	}

	got, err := FetchWikiContext(context.Background(), fetcher, "wiki", "/", true)
	if err != nil {
		t.Fatalf("FetchWikiContext returned error: %v", err)
	}
	wantPaths := []string{"/", "/Guide", "/Other/Nested"}
	if paths := wikiPagePaths(got.Pages); !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("page paths = %#v, want whole wiki tree %#v", paths, wantPaths)
	}
}

func TestFetchWikiContextAllowsEmptyContentAndIgnoresDescendantsWhenNotRecursive(t *testing.T) {
	fetcher := &fakeWikiFetcher{
		wiki: &Wiki{ID: "wiki-id"},
		pages: map[string]*WikiPage{
			"/Guide|content": {
				Path:     "/Guide",
				Content:  "",
				SubPages: []WikiPage{{Path: "/Guide/Child"}},
			},
		},
	}

	got, err := FetchWikiContext(context.Background(), fetcher, "wiki", "/Guide", false)
	if err != nil {
		t.Fatalf("FetchWikiContext returned error: %v", err)
	}
	if len(got.Pages) != 1 || got.Pages[0].Content != "" {
		t.Fatalf("pages = %#v, want one page with empty content", got.Pages)
	}
	if len(fetcher.pageCalls) != 1 || fetcher.pageCalls[0].opts.Path != "/Guide" {
		t.Fatalf("page calls = %#v, want only requested page", fetcher.pageCalls)
	}
}

func TestFetchWikiContextRejectsInvalidOrIncompleteResponses(t *testing.T) {
	tests := []struct {
		name        string
		pagePath    string
		recursive   bool
		fetcher     *fakeWikiFetcher
		wantError   string
		wantCalls   int
		wantResolve bool
	}{
		{
			name:        "non-absolute requested path",
			pagePath:    "Guide",
			fetcher:     &fakeWikiFetcher{},
			wantError:   "must be absolute",
			wantResolve: false,
		},
		{
			name:        "nil resolved wiki",
			pagePath:    "/Guide",
			fetcher:     &fakeWikiFetcher{},
			wantError:   "missing canonical ID",
			wantResolve: true,
		},
		{
			name:        "blank canonical ID",
			pagePath:    "/Guide",
			fetcher:     &fakeWikiFetcher{wiki: &Wiki{ID: "  "}},
			wantError:   "missing canonical ID",
			wantResolve: true,
		},
		{
			name:      "duplicate recursive metadata path",
			pagePath:  "/Guide",
			recursive: true,
			fetcher: &fakeWikiFetcher{
				wiki: &Wiki{ID: "wiki-id"},
				pages: map[string]*WikiPage{
					"/Guide|recursion=full": {Path: "/Guide", SubPages: []WikiPage{{Path: "/Guide"}}},
				},
			},
			wantError:   "duplicate wiki page path",
			wantCalls:   1,
			wantResolve: true,
		},
		{
			name:      "non-absolute recursive child path",
			pagePath:  "/Guide",
			recursive: true,
			fetcher: &fakeWikiFetcher{
				wiki: &Wiki{ID: "wiki-id"},
				pages: map[string]*WikiPage{
					"/Guide|recursion=full": {Path: "/Guide", SubPages: []WikiPage{{Path: "Child"}}},
				},
			},
			wantError:   "metadata missing absolute path",
			wantCalls:   1,
			wantResolve: true,
		},
		{
			name:      "absolute sibling outside requested subtree",
			pagePath:  "/Guide",
			recursive: true,
			fetcher: &fakeWikiFetcher{
				wiki: &Wiki{ID: "wiki-id"},
				pages: map[string]*WikiPage{
					"/Guide|recursion=full": {Path: "/Guide", SubPages: []WikiPage{{Path: "/Guidebook"}}},
				},
			},
			wantError:   "outside requested subtree",
			wantCalls:   1,
			wantResolve: true,
		},
		{
			name:     "content response path mismatch",
			pagePath: "/Guide",
			fetcher: &fakeWikiFetcher{
				wiki:  &Wiki{ID: "wiki-id"},
				pages: map[string]*WikiPage{"/Guide|content": {Path: "/Other"}},
			},
			wantError:   "does not match requested path",
			wantCalls:   1,
			wantResolve: true,
		},
		{
			name:     "nil content response",
			pagePath: "/Guide",
			fetcher: &fakeWikiFetcher{
				wiki:  &Wiki{ID: "wiki-id"},
				pages: map[string]*WikiPage{"/Guide|content": nil},
			},
			wantError:   "missing absolute path",
			wantCalls:   1,
			wantResolve: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FetchWikiContext(context.Background(), tt.fetcher, "wiki", tt.pagePath, tt.recursive)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("FetchWikiContext error = %v, want %q", err, tt.wantError)
			}
			if got != nil {
				t.Fatalf("context = %#v, want nil on failure", got)
			}
			if len(tt.fetcher.pageCalls) != tt.wantCalls {
				t.Fatalf("page calls = %d, want %d", len(tt.fetcher.pageCalls), tt.wantCalls)
			}
			if called := tt.fetcher.resolveArg != ""; called != tt.wantResolve {
				t.Fatalf("ResolveWiki called = %v, want %v", called, tt.wantResolve)
			}
		})
	}
}

func TestFetchWikiContextReturnsNilOnMidFetchError(t *testing.T) {
	fetcher := &fakeWikiFetcher{
		wiki: &Wiki{ID: "wiki-id"},
		pages: map[string]*WikiPage{
			"/Guide|recursion=full": {Path: "/Guide", SubPages: []WikiPage{{Path: "/Guide/A"}, {Path: "/Guide/B"}}},
			"/Guide|content":        {Path: "/Guide", Content: "root"},
		},
		pageErrs: map[string]error{"/Guide/A|content": errors.New("boom")},
	}

	got, err := FetchWikiContext(context.Background(), fetcher, "wiki", "/Guide", true)
	if err == nil || !strings.Contains(err.Error(), `fetching wiki page "/Guide/A"`) {
		t.Fatalf("FetchWikiContext error = %v, want failing page context", err)
	}
	if got != nil {
		t.Fatalf("context = %#v, want nil on incomplete fetch", got)
	}
	if paths := calledWikiPagePaths(fetcher.pageCalls); !reflect.DeepEqual(paths, []string{"/Guide", "/Guide", "/Guide/A"}) {
		t.Fatalf("page calls = %#v, want stop before /Guide/B", fetcher.pageCalls)
	}
}

func TestExportWikiContextWritesDeterministicBundle(t *testing.T) {
	repoRoot := t.TempDir()
	createdAt := time.Date(2026, 7, 13, 9, 30, 0, 0, time.FixedZone("offset", 2*60*60))
	wikiContext := &WikiContext{
		Wiki:          &Wiki{ID: "wiki/id", Name: "Engineering", ProjectID: "project-id"},
		RequestedPath: "/Guide",
		Recursive:     true,
		Pages: []WikiPage{
			{Path: "/Guide/Zeta", Content: "# Zeta\n"},
			{Path: "/Guide", Content: "# Guide\n"},
			{Path: "/Guide/Alpha", Content: "# Alpha\n"},
		},
	}

	outputDir, err := ExportWikiContext(WikiExportOptions{
		RepoRoot:  repoRoot,
		Profile:   "company-cloud",
		Project:   "MyProject",
		CreatedAt: createdAt,
	}, wikiContext)
	if err != nil {
		t.Fatalf("ExportWikiContext returned error: %v", err)
	}
	wantOutputDir := filepath.Join(repoRoot, ".adomi", "context", "wikis", base64.RawURLEncoding.EncodeToString([]byte("wiki/id")))
	if outputDir != wantOutputDir {
		t.Fatalf("output dir = %q, want %q", outputDir, wantOutputDir)
	}

	var wiki Wiki
	readWikiJSON(t, filepath.Join(outputDir, "wiki.json"), &wiki)
	if !reflect.DeepEqual(wiki, *wikiContext.Wiki) {
		t.Fatalf("wiki.json = %#v, want %#v", wiki, *wikiContext.Wiki)
	}
	var pages []WikiPage
	readWikiJSON(t, filepath.Join(outputDir, "pages.json"), &pages)
	wantPaths := []string{"/Guide", "/Guide/Alpha", "/Guide/Zeta"}
	if paths := wikiPagePaths(pages); !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("pages.json paths = %#v, want %#v", paths, wantPaths)
	}

	var index WikiIndex
	readWikiJSON(t, filepath.Join(outputDir, "index.json"), &index)
	assertNoWikiIndexTemps(t, outputDir)
	if index.Source != "azure-devops" || index.Profile != "company-cloud" || index.Project != "MyProject" {
		t.Fatalf("index source/profile/project = %q/%q/%q", index.Source, index.Profile, index.Project)
	}
	if index.WikiID != "wiki/id" || index.WikiName != "Engineering" || index.RequestedPath != "/Guide" || !index.Recursive {
		t.Fatalf("index selection = %#v, want wiki and recursive /Guide", index)
	}
	if index.CreatedAt != "2026-07-13T07:30:00Z" {
		t.Fatalf("createdAt = %q, want UTC RFC3339", index.CreatedAt)
	}
	if len(index.Pages) != len(wantPaths) {
		t.Fatalf("index pages = %d, want %d", len(index.Pages), len(wantPaths))
	}
	for i, pagePath := range wantPaths {
		wantRelative := filepath.ToSlash(filepath.Join("pages", expectedWikiMarkdownFilename(pagePath)))
		if index.Pages[i].Path != pagePath || index.Pages[i].MarkdownPath != wantRelative {
			t.Fatalf("index page %d = %#v, want %q -> %q", i, index.Pages[i], pagePath, wantRelative)
		}
		data, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(wantRelative)))
		if err != nil {
			t.Fatalf("reading Markdown for %q: %v", pagePath, err)
		}
		if got, want := string(data), pages[i].Content; got != want {
			t.Fatalf("Markdown for %q = %q, want %q", pagePath, got, want)
		}
	}
}

func TestWikiOutputPathUsesSafeCanonicalIDComponent(t *testing.T) {
	got := WikiOutputPath("/repo", "../wiki/id:CON")
	wantComponent := base64.RawURLEncoding.EncodeToString([]byte("../wiki/id:CON"))
	want := filepath.Join("/repo", ".adomi", "context", "wikis", wantComponent)
	if got != want {
		t.Fatalf("WikiOutputPath = %q, want %q", got, want)
	}
	if strings.Contains(filepath.Base(got), "/") || strings.Contains(filepath.Base(got), "\\") {
		t.Fatalf("safe wiki component = %q, want one path component", filepath.Base(got))
	}
}

func TestExportWikiContextRejectsInvalidContext(t *testing.T) {
	validWiki := &Wiki{ID: "wiki-id"}
	validPage := WikiPage{Path: "/Guide"}
	tests := []struct {
		name    string
		context *WikiContext
	}{
		{name: "nil context"},
		{name: "nil wiki", context: &WikiContext{RequestedPath: "/Guide", Pages: []WikiPage{validPage}}},
		{name: "blank canonical wiki ID", context: &WikiContext{Wiki: &Wiki{ID: " "}, RequestedPath: "/Guide", Pages: []WikiPage{validPage}}},
		{name: "relative requested path", context: &WikiContext{Wiki: validWiki, RequestedPath: "Guide", Pages: []WikiPage{validPage}}},
		{name: "empty page set", context: &WikiContext{Wiki: validWiki, RequestedPath: "/Guide"}},
		{name: "relative page path", context: &WikiContext{Wiki: validWiki, RequestedPath: "/Guide", Pages: []WikiPage{{Path: "Guide"}}}},
		{name: "duplicate page path", context: &WikiContext{Wiki: validWiki, RequestedPath: "/Guide", Recursive: true, Pages: []WikiPage{validPage, validPage}}},
		{name: "requested page absent", context: &WikiContext{Wiki: validWiki, RequestedPath: "/Guide", Recursive: true, Pages: []WikiPage{{Path: "/Other"}}}},
		{name: "non-recursive selection has descendants", context: &WikiContext{Wiki: validWiki, RequestedPath: "/Guide", Pages: []WikiPage{validPage, {Path: "/Guide/Child"}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			outputDir, err := ExportWikiContext(WikiExportOptions{RepoRoot: repoRoot}, tt.context)
			if err == nil {
				t.Fatal("ExportWikiContext error = nil, want invalid context error")
			}
			if outputDir != "" {
				t.Fatalf("output dir = %q, want empty on failure", outputDir)
			}
		})
	}
}

func TestExportWikiContextMapsUnsafePathsAndEmptyMarkdownSafely(t *testing.T) {
	repoRoot := t.TempDir()
	pagePath := `/../CON:<unsafe>?\trail`
	wikiContext := &WikiContext{
		Wiki:          &Wiki{ID: `../wiki\CON:name`, Name: "Unsafe names"},
		RequestedPath: pagePath,
		Pages:         []WikiPage{{Path: pagePath, Content: ""}},
	}

	outputDir, err := ExportWikiContext(WikiExportOptions{RepoRoot: repoRoot}, wikiContext)
	if err != nil {
		t.Fatalf("ExportWikiContext returned error: %v", err)
	}
	var index WikiIndex
	readWikiJSON(t, filepath.Join(outputDir, "index.json"), &index)
	if len(index.Pages) != 1 {
		t.Fatalf("index pages = %d, want 1", len(index.Pages))
	}
	markdownPath := index.Pages[0].MarkdownPath
	if strings.Contains(markdownPath, "\\") || !strings.HasPrefix(markdownPath, "pages/") {
		t.Fatalf("Markdown path = %q, want slash-normalized path below pages/", markdownPath)
	}
	wantFilename := expectedWikiMarkdownFilename(pagePath)
	if filepath.Base(filepath.FromSlash(markdownPath)) != wantFilename {
		t.Fatalf("Markdown filename = %q, want SHA-256 %q", filepath.Base(filepath.FromSlash(markdownPath)), wantFilename)
	}
	data, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(markdownPath)))
	if err != nil {
		t.Fatalf("reading empty Markdown: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("empty Markdown bytes = %q, want empty", data)
	}
}

func TestExportWikiContextDistinguishesFormerCaseInsensitiveBase64Collision(t *testing.T) {
	wikiContext := &WikiContext{
		Wiki:          &Wiki{ID: "wiki-id"},
		RequestedPath: "/",
		Recursive:     true,
		Pages: []WikiPage{
			{Path: "/"},
			{Path: "/aa"},
			{Path: "/aG"},
		},
	}
	first := strings.ToLower(base64.RawURLEncoding.EncodeToString([]byte(wikiContext.Pages[1].Path)))
	second := strings.ToLower(base64.RawURLEncoding.EncodeToString([]byte(wikiContext.Pages[2].Path)))
	if first != second {
		t.Fatalf("test paths encode to %q and %q, want a case-insensitive collision", first, second)
	}

	outputDir, err := ExportWikiContext(WikiExportOptions{RepoRoot: t.TempDir()}, wikiContext)
	if err != nil {
		t.Fatalf("ExportWikiContext returned error: %v", err)
	}
	var index WikiIndex
	readWikiJSON(t, filepath.Join(outputDir, "index.json"), &index)
	paths := make(map[string]string, len(index.Pages))
	for _, page := range index.Pages {
		paths[page.Path] = page.MarkdownPath
	}
	if paths["/aa"] == paths["/aG"] {
		t.Fatalf("SHA-256 mappings collide: /aa=%q /aG=%q", paths["/aa"], paths["/aG"])
	}
}

func TestExportWikiContextExportsLongPathsWithBoundedSHA256Filenames(t *testing.T) {
	tests := []struct {
		name     string
		pagePath string
	}{
		{name: "long ASCII", pagePath: "/" + strings.Repeat("a", 220)},
		{name: "long multibyte", pagePath: "/" + strings.Repeat("界", 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wikiContext := &WikiContext{
				Wiki:          &Wiki{ID: "wiki-id"},
				RequestedPath: tt.pagePath,
				Pages:         []WikiPage{{Path: tt.pagePath, Content: "content"}},
			}
			outputDir, err := ExportWikiContext(WikiExportOptions{RepoRoot: t.TempDir()}, wikiContext)
			if err != nil {
				t.Fatalf("ExportWikiContext returned error: %v", err)
			}
			var index WikiIndex
			readWikiJSON(t, filepath.Join(outputDir, "index.json"), &index)
			if len(index.Pages) != 1 {
				t.Fatalf("index pages = %d, want 1", len(index.Pages))
			}
			wantRelative := "pages/" + expectedWikiMarkdownFilename(tt.pagePath)
			if index.Pages[0].Path != tt.pagePath || index.Pages[0].MarkdownPath != wantRelative {
				t.Fatalf("index mapping = %#v, want %q -> %q", index.Pages[0], tt.pagePath, wantRelative)
			}
			filename := filepath.Base(filepath.FromSlash(index.Pages[0].MarkdownPath))
			if len(filename) != 64+len(".md") {
				t.Fatalf("Markdown filename length = %d, want 67", len(filename))
			}
			data, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(index.Pages[0].MarkdownPath)))
			if err != nil || string(data) != "content" {
				t.Fatalf("Markdown = %q, error = %v; want content", data, err)
			}
		})
	}
}

func TestExportWikiContextRejectsPageOutsideRequestedSubtree(t *testing.T) {
	wikiContext := &WikiContext{
		Wiki:          &Wiki{ID: "wiki-id"},
		RequestedPath: "/Guide",
		Recursive:     true,
		Pages: []WikiPage{
			{Path: "/Guide"},
			{Path: "/Guidebook"},
		},
	}

	outputDir, err := ExportWikiContext(WikiExportOptions{RepoRoot: t.TempDir()}, wikiContext)
	if err == nil || !strings.Contains(err.Error(), "outside requested subtree") {
		t.Fatalf("ExportWikiContext error = %v, want out-of-subtree error", err)
	}
	if outputDir != "" {
		t.Fatalf("output dir = %q, want empty on invalid subtree", outputDir)
	}
}

func TestExportWikiContextRejectsExistingSymlinkInBundlePath(t *testing.T) {
	wikiID := "wiki-id"
	safeID := base64.RawURLEncoding.EncodeToString([]byte(wikiID))
	tests := []struct {
		name       string
		linkParts  []string
		afterParts []string
	}{
		{name: ".adomi", linkParts: []string{".adomi"}, afterParts: []string{"context", "wikis", safeID}},
		{name: "context", linkParts: []string{".adomi", "context"}, afterParts: []string{"wikis", safeID}},
		{name: "wikis", linkParts: []string{".adomi", "context", "wikis"}, afterParts: []string{safeID}},
		{name: "wiki bundle", linkParts: []string{".adomi", "context", "wikis", safeID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			outside := t.TempDir()
			linkPath := filepath.Join(append([]string{repoRoot}, tt.linkParts...)...)
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatalf("creating symlink parent: %v", err)
			}
			if err := os.Symlink(outside, linkPath); err != nil {
				t.Fatalf("creating symlink: %v", err)
			}
			sentinel := filepath.Join(outside, "sentinel.txt")
			if err := os.WriteFile(sentinel, []byte("untouched"), 0o644); err != nil {
				t.Fatalf("writing outside sentinel: %v", err)
			}

			wikiContext := &WikiContext{
				Wiki:          &Wiki{ID: wikiID},
				RequestedPath: "/Guide",
				Pages:         []WikiPage{{Path: "/Guide", Content: "content"}},
			}
			outputDir, err := ExportWikiContext(WikiExportOptions{RepoRoot: repoRoot}, wikiContext)
			if err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("ExportWikiContext error = %v, want symlink boundary error", err)
			}
			if outputDir != "" {
				t.Fatalf("output dir = %q, want empty on symlink boundary failure", outputDir)
			}
			data, readErr := os.ReadFile(sentinel)
			if readErr != nil || string(data) != "untouched" {
				t.Fatalf("outside sentinel = %q, error = %v; want untouched", data, readErr)
			}
			outsideOutput := filepath.Join(append([]string{outside}, tt.afterParts...)...)
			if _, statErr := os.Stat(filepath.Join(outsideOutput, "index.json")); !os.IsNotExist(statErr) {
				t.Fatalf("outside index stat error = %v, want no outside write", statErr)
			}
		})
	}
}

func TestExportWikiContextSuccessfulRefetchRemovesStaleBundleFiles(t *testing.T) {
	repoRoot := t.TempDir()
	first := &WikiContext{
		Wiki:          &Wiki{ID: "wiki-id"},
		RequestedPath: "/Guide",
		Recursive:     true,
		Pages: []WikiPage{
			{Path: "/Guide", Content: "old root"},
			{Path: "/Guide/Old", Content: "stale"},
		},
	}
	outputDir, err := ExportWikiContext(WikiExportOptions{RepoRoot: repoRoot}, first)
	if err != nil {
		t.Fatalf("first ExportWikiContext returned error: %v", err)
	}
	staleMarkdown := filepath.Join(outputDir, "pages", expectedWikiMarkdownFilename("/Guide/Old"))
	staleExtra := filepath.Join(outputDir, "stale.txt")
	if err := os.WriteFile(staleExtra, []byte("stale"), 0o644); err != nil {
		t.Fatalf("writing stale file: %v", err)
	}

	second := &WikiContext{
		Wiki:          &Wiki{ID: "wiki-id"},
		RequestedPath: "/Guide",
		Pages:         []WikiPage{{Path: "/Guide", Content: "new root"}},
	}
	gotOutputDir, err := ExportWikiContext(WikiExportOptions{RepoRoot: repoRoot}, second)
	if err != nil {
		t.Fatalf("second ExportWikiContext returned error: %v", err)
	}
	if gotOutputDir != outputDir {
		t.Fatalf("second output dir = %q, want %q", gotOutputDir, outputDir)
	}
	for _, stalePath := range []string{staleMarkdown, staleExtra} {
		if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
			t.Fatalf("stale path %s stat error = %v, want not exist", stalePath, err)
		}
	}
}

func TestExportWikiContextDefaultsZeroCreatedAt(t *testing.T) {
	wikiContext := &WikiContext{
		Wiki:          &Wiki{ID: "wiki-id"},
		RequestedPath: "/Guide",
		Pages:         []WikiPage{{Path: "/Guide"}},
	}
	outputDir, err := ExportWikiContext(WikiExportOptions{RepoRoot: t.TempDir()}, wikiContext)
	if err != nil {
		t.Fatalf("ExportWikiContext returned error: %v", err)
	}
	var index WikiIndex
	readWikiJSON(t, filepath.Join(outputDir, "index.json"), &index)
	if _, err := time.Parse(time.RFC3339, index.CreatedAt); err != nil {
		t.Fatalf("createdAt = %q, want RFC3339 timestamp: %v", index.CreatedAt, err)
	}
}

func TestExportWikiContextFilesystemFailureReturnsNoPathOrIndex(t *testing.T) {
	t.Run("directory creation", func(t *testing.T) {
		repoRoot := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(repoRoot, []byte("file"), 0o644); err != nil {
			t.Fatalf("writing repo root file: %v", err)
		}
		wikiContext := &WikiContext{
			Wiki:          &Wiki{ID: "wiki-id"},
			RequestedPath: "/Guide",
			Pages:         []WikiPage{{Path: "/Guide"}},
		}
		outputDir, err := ExportWikiContext(WikiExportOptions{RepoRoot: repoRoot}, wikiContext)
		if err == nil {
			t.Fatal("ExportWikiContext error = nil, want filesystem error")
		}
		if outputDir != "" {
			t.Fatalf("output dir = %q, want empty on failure", outputDir)
		}
	})
}

func TestWriteWikiIndexAtomicallyPublishesCompleteIndexAndCleansTemp(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.json")
	want := WikiIndex{
		Source:        "azure-devops",
		WikiID:        "wiki-id",
		RequestedPath: "/Guide",
		CreatedAt:     "2026-07-13T12:00:00Z",
		Pages: []WikiPageIndex{{
			Path:         "/Guide",
			MarkdownPath: "pages/hash.md",
		}},
	}

	if err := writeWikiIndexAtomically(indexPath, want); err != nil {
		t.Fatalf("writeWikiIndexAtomically returned error: %v", err)
	}
	var got WikiIndex
	readWikiJSON(t, indexPath, &got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("published index = %#v, want %#v", got, want)
	}
	assertNoWikiIndexTemps(t, dir)
}

func TestWriteWikiIndexAtomicallyCleansTempWhenRenameFails(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.json")
	if err := os.Mkdir(indexPath, 0o755); err != nil {
		t.Fatalf("creating final index directory: %v", err)
	}

	err := writeWikiIndexAtomically(indexPath, WikiIndex{Source: "azure-devops"})
	if err == nil {
		t.Fatal("writeWikiIndexAtomically error = nil, want rename failure")
	}
	info, statErr := os.Lstat(indexPath)
	if statErr != nil {
		t.Fatalf("stat final index path: %v", statErr)
	}
	if info.Mode().IsRegular() {
		t.Fatal("final index is a regular file after failed publication")
	}
	assertNoWikiIndexTemps(t, dir)
}

type wikiPageFetchCall struct {
	wikiIdentifier string
	opts           WikiPageFetchOptions
}

type fakeWikiFetcher struct {
	wiki       *Wiki
	wikiErr    error
	pages      map[string]*WikiPage
	pageErrs   map[string]error
	pageCalls  []wikiPageFetchCall
	resolveArg string
}

func (f *fakeWikiFetcher) ResolveWiki(_ context.Context, identifier string) (*Wiki, error) {
	f.resolveArg = identifier
	if f.wikiErr != nil {
		return nil, f.wikiErr
	}
	return f.wiki, nil
}

func (f *fakeWikiFetcher) FetchWikiPage(_ context.Context, wikiIdentifier string, opts WikiPageFetchOptions) (*WikiPage, error) {
	f.pageCalls = append(f.pageCalls, wikiPageFetchCall{wikiIdentifier: wikiIdentifier, opts: opts})
	key := wikiPageFetchKey(opts)
	if err := f.pageErrs[key]; err != nil {
		return nil, err
	}
	page, ok := f.pages[key]
	if !ok {
		return nil, errors.New("missing fake wiki page")
	}
	return page, nil
}

func wikiPageFetchKey(opts WikiPageFetchOptions) string {
	parts := []string{opts.Path}
	if opts.RecursionLevel != "" {
		parts = append(parts, "recursion="+opts.RecursionLevel)
	}
	if opts.IncludeContent {
		parts = append(parts, "content")
	}
	return strings.Join(parts, "|")
}

func wikiPagePaths(pages []WikiPage) []string {
	paths := make([]string, 0, len(pages))
	for _, page := range pages {
		paths = append(paths, page.Path)
	}
	return paths
}

func calledWikiPagePaths(calls []wikiPageFetchCall) []string {
	paths := make([]string, 0, len(calls))
	for _, call := range calls {
		paths = append(paths, call.opts.Path)
	}
	return paths
}

func readWikiJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
}

func expectedWikiMarkdownFilename(pagePath string) string {
	sum := sha256.Sum256([]byte(pagePath))
	return hex.EncodeToString(sum[:]) + ".md"
}

func assertNoWikiIndexTemps(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".index.json.tmp-*"))
	if err != nil {
		t.Fatalf("globbing temporary index files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary index files remain: %v", matches)
	}
}
