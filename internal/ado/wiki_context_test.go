package ado

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFetchWikiContextFetchesOnePage(t *testing.T) {
	fetcher := &fakeWikiFetcher{
		wiki: &Wiki{ID: "wiki-id", Name: "Engineering"},
		pages: map[string]*WikiPage{
			"/Guide|content": {Path: "/Guide", Content: "# Guide\n"},
		},
	}

	got, err := FetchWikiContext(context.Background(), fetcher, "Engineering", "/Guide", false, nil)
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

	var progress []string
	got, err := FetchWikiContext(context.Background(), fetcher, "Engineering", "/Guide", true, func(message string) {
		progress = append(progress, message)
	})
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
	wantProgress := []string{
		`Fetching wiki page "/Guide"`,
		`Fetching wiki page "/Guide/Alpha"`,
		`Fetching wiki page "/Guide/Alpha/Details"`,
		`Fetching wiki page "/Guide/Zeta"`,
	}
	if !reflect.DeepEqual(progress, wantProgress) {
		t.Fatalf("progress = %q, want %q", progress, wantProgress)
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

	got, err := FetchWikiContext(context.Background(), fetcher, "wiki", "/", true, nil)
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

	got, err := FetchWikiContext(context.Background(), fetcher, "wiki", "/Guide", false, nil)
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
			got, err := FetchWikiContext(context.Background(), tt.fetcher, "wiki", tt.pagePath, tt.recursive, nil)
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

	got, err := FetchWikiContext(context.Background(), fetcher, "wiki", "/Guide", true, nil)
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
