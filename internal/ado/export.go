package ado

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type ExportOptions struct {
	RepoRoot  string
	Profile   string
	Project   string
	CreatedAt time.Time
}

type Index struct {
	Source         string      `json:"source"`
	Profile        string      `json:"profile"`
	Project        string      `json:"project"`
	RootWorkItemID int         `json:"rootWorkItemId"`
	EpicWorkItemID int         `json:"epicWorkItemId"`
	CreatedAt      string      `json:"createdAt"`
	WorkItems      []IndexItem `json:"workItems"`
}

type IndexItem struct {
	ID              int    `json:"id"`
	Type            string `json:"type"`
	Title           string `json:"title"`
	Path            string `json:"path"`
	HTMLPath        string `json:"htmlPath"`
	AttachmentsPath string `json:"attachmentsPath"`
}

func OutputPath(repoRoot, profile, project string, rootID int) string {
	return filepath.Join(repoRoot, ".adomi", "context", "work-items", strconv.Itoa(rootID))
}

func ExportContext(ctx context.Context, downloader AttachmentDownloader, opts ExportOptions, tree *WorkItemTree) (string, error) {
	if tree == nil {
		return "", fmt.Errorf("work item tree is required")
	}
	if opts.CreatedAt.IsZero() {
		opts.CreatedAt = time.Now().UTC()
	}

	outputDir := OutputPath(opts.RepoRoot, opts.Profile, opts.Project, tree.RootID)
	if err := os.RemoveAll(outputDir); err != nil {
		return "", fmt.Errorf("removing previous export directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "items"), 0o755); err != nil {
		return "", fmt.Errorf("creating work item export directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "html"), 0o755); err != nil {
		return "", fmt.Errorf("creating HTML export directory: %w", err)
	}

	if downloader != nil {
		for _, item := range tree.WorkItems {
			if _, err := DownloadAttachments(ctx, downloader, outputDir, item); err != nil {
				return "", err
			}
		}
	}

	for _, item := range tree.WorkItems {
		itemPath := filepath.Join(outputDir, "items", strconv.Itoa(item.ID)+".json")
		if err := writePrettyJSON(itemPath, item); err != nil {
			return "", err
		}
		htmlPath := filepath.Join(outputDir, "html", strconv.Itoa(item.ID)+".html")
		if err := os.WriteFile(htmlPath, []byte(renderHTML(item)), 0o644); err != nil {
			return "", fmt.Errorf("writing HTML for work item %d: %w", item.ID, err)
		}
	}

	if err := writePrettyJSON(filepath.Join(outputDir, "tree.json"), tree); err != nil {
		return "", err
	}
	if err := writePrettyJSON(filepath.Join(outputDir, "index.json"), newIndex(opts, tree)); err != nil {
		return "", err
	}

	return outputDir, nil
}

func newIndex(opts ExportOptions, tree *WorkItemTree) Index {
	items := make([]IndexItem, 0, len(tree.WorkItems))
	for _, item := range tree.WorkItems {
		id := strconv.Itoa(item.ID)
		items = append(items, IndexItem{
			ID:              item.ID,
			Type:            item.Type(),
			Title:           item.Title(),
			Path:            filepath.ToSlash(filepath.Join("items", id+".json")),
			HTMLPath:        filepath.ToSlash(filepath.Join("html", id+".html")),
			AttachmentsPath: filepath.ToSlash(filepath.Join("attachments", id)),
		})
	}

	return Index{
		Source:         "azure-devops",
		Profile:        opts.Profile,
		Project:        opts.Project,
		RootWorkItemID: tree.RootID,
		EpicWorkItemID: tree.EpicID,
		CreatedAt:      opts.CreatedAt.UTC().Format(time.RFC3339),
		WorkItems:      items,
	}
}

func writePrettyJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing JSON %s: %w", path, err)
	}
	return nil
}

func renderHTML(item WorkItem) string {
	return fmt.Sprintf(
		"<h1>%d: %s</h1>\n<p><strong>Type:</strong> %s</p>\n<div>%s</div>\n",
		item.ID,
		html.EscapeString(item.Title()),
		html.EscapeString(item.Type()),
		html.EscapeString(item.Description()),
	)
}
