package ado

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type WikiContext struct {
	Wiki          *Wiki      `json:"wiki"`
	RequestedPath string     `json:"requestedPath"`
	Recursive     bool       `json:"recursive"`
	Pages         []WikiPage `json:"pages"`
}

type WikiExportOptions struct {
	RepoRoot  string
	Profile   string
	Project   string
	CreatedAt time.Time
}

type WikiIndex struct {
	Source        string          `json:"source"`
	Profile       string          `json:"profile"`
	Project       string          `json:"project"`
	WikiID        string          `json:"wikiId"`
	WikiName      string          `json:"wikiName"`
	RequestedPath string          `json:"requestedPath"`
	Recursive     bool            `json:"recursive"`
	CreatedAt     string          `json:"createdAt"`
	Pages         []WikiPageIndex `json:"pages"`
}

type WikiPageIndex struct {
	Path         string `json:"path"`
	MarkdownPath string `json:"markdownPath"`
}

func FetchWikiContext(ctx context.Context, fetcher WikiFetcher, wikiIdentifier, pagePath string, recursive bool, progress ProgressFunc) (*WikiContext, error) {
	if !strings.HasPrefix(pagePath, "/") {
		return nil, fmt.Errorf("wiki page path must be absolute")
	}

	wiki, err := fetcher.ResolveWiki(ctx, wikiIdentifier)
	if err != nil {
		return nil, fmt.Errorf("resolving wiki %q: %w", wikiIdentifier, err)
	}
	if wiki == nil || strings.TrimSpace(wiki.ID) == "" {
		return nil, fmt.Errorf("resolved wiki is missing canonical ID")
	}
	paths := []string{pagePath}
	if recursive {
		metadata, err := fetcher.FetchWikiPage(ctx, wiki.ID, WikiPageFetchOptions{
			Path:           pagePath,
			RecursionLevel: "full",
		})
		if err != nil {
			return nil, fmt.Errorf("fetching wiki page tree %q: %w", pagePath, err)
		}
		if err := validateFetchedWikiPage(metadata, pagePath); err != nil {
			return nil, err
		}
		paths, err = flattenWikiPagePaths(metadata, pagePath)
		if err != nil {
			return nil, err
		}
	}

	pages := make([]WikiPage, 0, len(paths))
	for _, path := range paths {
		progress.report(fmt.Sprintf("Fetching wiki page %q", path))
		page, err := fetcher.FetchWikiPage(ctx, wiki.ID, WikiPageFetchOptions{
			Path:           path,
			IncludeContent: true,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching wiki page %q: %w", path, err)
		}
		if err := validateFetchedWikiPage(page, path); err != nil {
			return nil, err
		}
		pages = append(pages, *page)
	}

	return &WikiContext{
		Wiki:          wiki,
		RequestedPath: pagePath,
		Recursive:     recursive,
		Pages:         pages,
	}, nil
}

func flattenWikiPagePaths(root *WikiPage, requestedPath string) ([]string, error) {
	seen := make(map[string]struct{})
	paths := make([]string, 0)
	var visit func(WikiPage) error
	visit = func(page WikiPage) error {
		if !strings.HasPrefix(page.Path, "/") {
			return fmt.Errorf("wiki page metadata missing absolute path")
		}
		if !wikiPathInSubtree(requestedPath, page.Path) {
			return fmt.Errorf("wiki page path %q is outside requested subtree %q", page.Path, requestedPath)
		}
		if _, exists := seen[page.Path]; exists {
			return fmt.Errorf("duplicate wiki page path %q", page.Path)
		}
		seen[page.Path] = struct{}{}
		paths = append(paths, page.Path)
		for _, child := range page.SubPages {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(*root); err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func validateFetchedWikiPage(page *WikiPage, requestedPath string) error {
	if page == nil || !strings.HasPrefix(page.Path, "/") {
		return fmt.Errorf("wiki page response missing absolute path")
	}
	if page.Path != requestedPath {
		return fmt.Errorf("wiki page response path %q does not match requested path %q", page.Path, requestedPath)
	}
	return nil
}

func WikiOutputPath(repoRoot, canonicalWikiID string) string {
	return filepath.Join(repoRoot, ".adomi", "context", "wikis", safeWikiComponent(canonicalWikiID))
}

func ExportWikiContext(opts WikiExportOptions, wikiContext *WikiContext) (string, error) {
	if wikiContext == nil || wikiContext.Wiki == nil {
		return "", fmt.Errorf("wiki context is required")
	}
	if strings.TrimSpace(wikiContext.Wiki.ID) == "" {
		return "", fmt.Errorf("wiki context is missing canonical wiki ID")
	}
	if !strings.HasPrefix(wikiContext.RequestedPath, "/") {
		return "", fmt.Errorf("wiki context requested path must be absolute")
	}
	if opts.CreatedAt.IsZero() {
		opts.CreatedAt = time.Now().UTC()
	}

	pages := append([]WikiPage(nil), wikiContext.Pages...)
	if len(pages) == 0 {
		return "", fmt.Errorf("wiki context must contain at least one page")
	}
	if !wikiContext.Recursive && len(pages) != 1 {
		return "", fmt.Errorf("non-recursive wiki context must contain exactly one page")
	}
	sort.SliceStable(pages, func(i, j int) bool { return pages[i].Path < pages[j].Path })
	pageInfos, err := wikiPageIndexEntries(pages)
	if err != nil {
		return "", err
	}
	requestedPageFound := false
	for _, page := range pages {
		if wikiContext.Recursive && !wikiPathInSubtree(wikiContext.RequestedPath, page.Path) {
			return "", fmt.Errorf("wiki page path %q is outside requested subtree %q", page.Path, wikiContext.RequestedPath)
		}
		if page.Path == wikiContext.RequestedPath {
			requestedPageFound = true
		}
	}
	if !requestedPageFound {
		return "", fmt.Errorf("wiki context does not contain requested page %q", wikiContext.RequestedPath)
	}

	outputDir := WikiOutputPath(opts.RepoRoot, wikiContext.Wiki.ID)
	pagesDir := filepath.Join(outputDir, "pages")
	if err := rejectWikiBundleSymlinks(opts.RepoRoot, safeWikiComponent(wikiContext.Wiki.ID)); err != nil {
		return "", err
	}
	if err := os.RemoveAll(outputDir); err != nil {
		return "", fmt.Errorf("removing previous wiki export directory: %w", err)
	}
	if err := os.MkdirAll(pagesDir, 0o755); err != nil {
		return "", fmt.Errorf("creating wiki pages directory: %w", err)
	}

	if err := writePrettyJSON(filepath.Join(outputDir, "wiki.json"), wikiContext.Wiki); err != nil {
		return "", err
	}
	if err := writePrettyJSON(filepath.Join(outputDir, "pages.json"), pages); err != nil {
		return "", err
	}
	for i, page := range pages {
		markdownPath := filepath.Join(outputDir, filepath.FromSlash(pageInfos[i].MarkdownPath))
		if err := os.WriteFile(markdownPath, []byte(page.Content), 0o644); err != nil {
			return "", fmt.Errorf("writing Markdown for wiki page %q: %w", page.Path, err)
		}
	}

	index := WikiIndex{
		Source:        "azure-devops",
		Profile:       opts.Profile,
		Project:       opts.Project,
		WikiID:        wikiContext.Wiki.ID,
		WikiName:      wikiContext.Wiki.Name,
		RequestedPath: wikiContext.RequestedPath,
		Recursive:     wikiContext.Recursive,
		CreatedAt:     opts.CreatedAt.UTC().Format(time.RFC3339),
		Pages:         pageInfos,
	}
	if err := writeWikiIndexAtomically(filepath.Join(outputDir, "index.json"), index); err != nil {
		return "", err
	}
	return outputDir, nil
}

func wikiPageIndexEntries(pages []WikiPage) ([]WikiPageIndex, error) {
	infos := make([]WikiPageIndex, 0, len(pages))
	seenPaths := make(map[string]struct{}, len(pages))
	seenTargets := make(map[string]struct{}, len(pages))
	for _, page := range pages {
		if !strings.HasPrefix(page.Path, "/") {
			return nil, fmt.Errorf("wiki page path %q must be absolute", page.Path)
		}
		if _, exists := seenPaths[page.Path]; exists {
			return nil, fmt.Errorf("duplicate wiki page path %q", page.Path)
		}
		seenPaths[page.Path] = struct{}{}

		relativePath := filepath.Join("pages", wikiMarkdownFilename(page.Path))
		if err := validateContainedPath("pages", relativePath); err != nil {
			return nil, err
		}
		normalizedTarget := strings.ToLower(filepath.Clean(relativePath))
		if _, exists := seenTargets[normalizedTarget]; exists {
			return nil, fmt.Errorf("wiki page paths collide at output path %q", filepath.ToSlash(relativePath))
		}
		seenTargets[normalizedTarget] = struct{}{}
		infos = append(infos, WikiPageIndex{
			Path:         page.Path,
			MarkdownPath: filepath.ToSlash(relativePath),
		})
	}
	return infos, nil
}

func safeWikiComponent(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func wikiMarkdownFilename(pagePath string) string {
	sum := sha256.Sum256([]byte(pagePath))
	return hex.EncodeToString(sum[:]) + ".md"
}

func validateContainedPath(parent, target string) error {
	relative, err := filepath.Rel(parent, target)
	if err != nil {
		return fmt.Errorf("validating wiki output path %q: %w", target, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("wiki output path %q escapes pages directory", target)
	}
	return nil
}

func wikiPathInSubtree(requestedPath, pagePath string) bool {
	if pagePath == requestedPath {
		return true
	}
	if requestedPath == "/" {
		return strings.HasPrefix(pagePath, "/")
	}
	return strings.HasPrefix(pagePath, requestedPath+"/")
}

func rejectWikiBundleSymlinks(repoRoot, safeWikiID string) error {
	current := repoRoot
	for _, component := range []string{".adomi", "context", "wikis", safeWikiID} {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("checking wiki export path %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("wiki export path %q contains a symlink", current)
		}
	}
	return nil
}

func writeWikiIndexAtomically(path string, index WikiIndex) (returnErr error) {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON %s: %w", path, err)
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(filepath.Dir(path), ".index.json.tmp-*")
	if err != nil {
		return fmt.Errorf("creating temporary wiki index: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
			returnErr = errors.Join(returnErr, fmt.Errorf("cleaning temporary wiki index: %w", err))
		}
	}()

	if err := temp.Chmod(0o644); err != nil {
		closeErr := temp.Close()
		return errors.Join(fmt.Errorf("setting temporary wiki index permissions: %w", err), closeWikiIndexError(closeErr))
	}
	if _, err := temp.Write(data); err != nil {
		closeErr := temp.Close()
		return errors.Join(fmt.Errorf("writing temporary wiki index: %w", err), closeWikiIndexError(closeErr))
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("closing temporary wiki index: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("publishing wiki index: %w", err)
	}
	return nil
}

func closeWikiIndexError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("closing temporary wiki index: %w", err)
}
