package ado

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

const maxFilenameLength = 180

type AttachmentDownloader interface {
	Download(ctx context.Context, rawURL string) ([]byte, error)
}

type AttachmentSummary struct {
	WorkItemID int    `json:"workItemId"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	URL        string `json:"url"`
}

func FilenameForAttachment(relation Relation, ordinal int) string {
	name := attributeName(relation)
	if name == "" {
		name = urlBasename(relation.URL)
	}
	if name == "" {
		name = fmt.Sprintf("attachment-%d", ordinal)
	}
	return sanitizeFilename(name, ordinal)
}

func DownloadAttachments(ctx context.Context, downloader AttachmentDownloader, outputDir string, item WorkItem) ([]AttachmentSummary, error) {
	relations := item.AttachmentRelations()
	if len(relations) == 0 {
		return nil, nil
	}

	itemDir := filepath.Join(outputDir, "attachments", strconv.Itoa(item.ID))
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating attachment directory: %w", err)
	}

	used := map[string]int{}
	summaries := make([]AttachmentSummary, 0, len(relations))
	for i, relation := range relations {
		filename := uniqueFilename(FilenameForAttachment(relation, i+1), used)
		data, err := downloader.Download(ctx, relation.URL)
		if err != nil {
			return nil, fmt.Errorf("downloading attachment %q for work item %d: %w", filename, item.ID, err)
		}

		relativePath := filepath.ToSlash(filepath.Join("attachments", strconv.Itoa(item.ID), filename))
		if err := os.WriteFile(filepath.Join(outputDir, relativePath), data, 0o644); err != nil {
			return nil, fmt.Errorf("writing attachment %q for work item %d: %w", filename, item.ID, err)
		}
		summaries = append(summaries, AttachmentSummary{
			WorkItemID: item.ID,
			Name:       filename,
			Path:       relativePath,
			URL:        relation.URL,
		})
	}

	return summaries, nil
}

func attributeName(relation Relation) string {
	value, ok := relation.Attributes["name"]
	if !ok || value == nil {
		return ""
	}
	name, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(name)
}

func urlBasename(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	base := path.Base(parsed.Path)
	if base == "." || base == "/" {
		return ""
	}
	if unescaped, err := url.PathUnescape(base); err == nil {
		base = unescaped
	}
	return strings.TrimSpace(base)
}

func sanitizeFilename(name string, ordinal int) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\' || r == '?' || r == '*' || r == ':' || r == '"' || r == '<' || r == '>' || r == '|':
			return '_'
		case r == 0 || unicode.IsControl(r):
			return '_'
		default:
			return r
		}
	}, name)
	name = strings.Trim(name, ". ")
	if name == "" {
		return fmt.Sprintf("attachment-%d", ordinal)
	}
	name = avoidReservedDeviceName(name)
	name = truncateFilename(name, maxFilenameLength)
	return name
}

func avoidReservedDeviceName(name string) string {
	stem := name
	if ext := filepath.Ext(name); ext != "" {
		stem = strings.TrimSuffix(name, ext)
	}
	switch strings.ToUpper(stem) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return "_" + name
	default:
		return name
	}
}

func truncateFilename(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	ext := filepath.Ext(name)
	if ext == name || len(ext) >= limit {
		return truncateStringBytes(name, limit)
	}
	stemLimit := limit - len(ext)
	if stemLimit <= 0 {
		return truncateStringBytes(name, limit)
	}
	return truncateStringBytes(strings.TrimSuffix(name, ext), stemLimit) + ext
}

func uniqueFilename(filename string, used map[string]int) string {
	used[filename]++
	if used[filename] == 1 {
		return filename
	}

	ext := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, ext)
	suffix := fmt.Sprintf("-%d", used[filename])
	candidate := fmt.Sprintf("%s%s%s", stem, suffix, ext)
	if len(candidate) <= maxFilenameLength {
		return candidate
	}
	stemLimit := maxFilenameLength - len(suffix) - len(ext)
	if stemLimit <= 0 {
		return truncateFilename(candidate, maxFilenameLength)
	}
	return fmt.Sprintf("%s%s%s", truncateStringBytes(stem, stemLimit), suffix, ext)
}

func truncateStringBytes(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	end := 0
	for index := range value {
		if index > limit {
			break
		}
		end = index
	}
	if end == 0 {
		return ""
	}
	return value[:end]
}
