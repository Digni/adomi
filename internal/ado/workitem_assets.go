package ado

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type WorkItemAssets struct {
	WorkItemID int             `json:"workItemId"`
	Assets     []WorkItemAsset `json:"assets"`
}

type WorkItemAsset struct {
	URL     string        `json:"url"`
	Status  string        `json:"status"`
	Name    string        `json:"name,omitempty"`
	Path    string        `json:"path,omitempty"`
	Reason  string        `json:"reason,omitempty"`
	Sources []AssetSource `json:"sources"`
}

type workItemAssetCandidate struct {
	asset       WorkItemAsset
	downloadURL string
	filename    string
	isImage     bool
}

func collectWorkItemAssets(item WorkItem, comments []WorkItemReadComment, opts ExportOptions) []workItemAssetCandidate {
	var candidates []workItemAssetCandidate
	indices := map[string]int{}
	add := func(key string, candidate workItemAssetCandidate, source AssetSource) {
		if index, exists := indices[key]; exists {
			current := &candidates[index]
			current.isImage = current.isImage || candidate.isImage
			for _, existing := range current.asset.Sources {
				if existing == source {
					return
				}
			}
			current.asset.Sources = append(current.asset.Sources, source)
			return
		}
		indices[key] = len(candidates)
		candidate.asset.URL = source.URL
		candidate.asset.Sources = []AssetSource{source}
		candidates = append(candidates, candidate)
	}
	for i, relation := range item.AttachmentRelations() {
		_, key, reason := resolveInlineAssetURL(relation.URL, opts.BaseURL, opts.Project)
		if reason != "" {
			key = relation.URL
		}
		add(key, workItemAssetCandidate{downloadURL: relation.URL, filename: FilenameForAttachment(relation, i+1)}, AssetSource{Kind: "attachment", URL: relation.URL})
	}
	for _, source := range workItemImageSources(item, comments) {
		resolved, key, reason := resolveInlineAssetURL(source.URL, opts.BaseURL, opts.Project)
		candidate := workItemAssetCandidate{downloadURL: resolved, isImage: true}
		if reason != "" {
			key = "skipped:" + source.URL
			candidate.asset.Status, candidate.asset.Reason = "skipped", reason
		} else {
			candidate.filename = FilenameForAttachment(Relation{URL: resolved}, len(candidates)+1)
		}
		add(key, candidate, source)
	}
	return candidates
}

func downloadWorkItemAssets(ctx context.Context, downloader AttachmentDownloader, outputDir string, item WorkItem, comments []WorkItemReadComment, opts ExportOptions, progress ProgressFunc) (WorkItemAssets, error) {
	manifest := WorkItemAssets{WorkItemID: item.ID, Assets: []WorkItemAsset{}}
	used := map[string]int{}
	for _, candidate := range collectWorkItemAssets(item, comments, opts) {
		asset := candidate.asset
		if asset.Status == "skipped" {
			manifest.Assets = append(manifest.Assets, asset)
			continue
		}
		if downloader == nil {
			asset.Status, asset.Reason = "skipped", "no downloader configured"
			manifest.Assets = append(manifest.Assets, asset)
			continue
		}
		data, err := downloader.Download(ctx, candidate.downloadURL)
		if err != nil {
			return manifest, fmt.Errorf("downloading attachment %q for work item %d: %w", candidate.filename, item.ID, err)
		}
		filename := candidate.filename
		if candidate.isImage {
			extension := assetImageExtension(data)
			if extension == "" {
				return manifest, fmt.Errorf("attachment %q for work item %d did not contain a recognized image", filename, item.ID)
			}
			if !imageExtensionMatches(filepath.Ext(filename), extension) {
				filename = sanitizeFilename(filename+extension, len(manifest.Assets)+1)
			}
		}
		filename = uniqueFilename(filename, used)
		relativePath := filepath.Join("attachments", strconv.Itoa(item.ID), filename)
		diskPath := filepath.Join(outputDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(diskPath), 0o755); err != nil {
			return manifest, fmt.Errorf("creating attachment directory for work item %d: %w", item.ID, err)
		}
		if err := os.WriteFile(diskPath, data, 0o644); err != nil {
			return manifest, fmt.Errorf("writing attachment %q for work item %d: %w", filename, item.ID, err)
		}
		asset.Status, asset.Name, asset.Path = "downloaded", filename, filepath.ToSlash(relativePath)
		manifest.Assets = append(manifest.Assets, asset)
		progress.report(fmt.Sprintf("Downloaded attachment %s for work item %d", filename, item.ID))
	}
	return manifest, nil
}

func imageExtensionMatches(actual, detected string) bool {
	actual = strings.ToLower(actual)
	return actual == detected || (actual == ".jpeg" && detected == ".jpg") || (actual == ".tiff" && detected == ".tif")
}

func assetImageExtension(data []byte) string {
	switch http.DetectContentType(data) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/x-icon":
		return ".ico"
	}
	if bytes.HasPrefix(data, []byte("II\x2a\x00")) || bytes.HasPrefix(data, []byte("MM\x00\x2a")) {
		return ".tif"
	}
	// SVG is stored as evidence only, never inserted into rendered HTML.
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		if element, ok := token.(xml.StartElement); ok {
			if element.Name.Local == "svg" && (element.Name.Space == "" || element.Name.Space == "http://www.w3.org/2000/svg") {
				return ".svg"
			}
			return ""
		}
	}
}
