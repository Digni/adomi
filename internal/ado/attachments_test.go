package ado

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFilenameForAttachmentUsesAttributeName(t *testing.T) {
	relation := Relation{
		URL:        "https://dev.azure.com/org/_apis/wit/attachments/file-from-url.png",
		Attributes: map[string]any{"name": "screenshot.png"},
	}

	got := FilenameForAttachment(relation, 1)
	if got != "screenshot.png" {
		t.Fatalf("filename = %q, want screenshot.png", got)
	}
}

func TestFilenameForAttachmentFallsBackToURLBasename(t *testing.T) {
	relation := Relation{URL: "https://dev.azure.com/org/_apis/wit/attachments/spec%20file.pdf?download=true"}

	got := FilenameForAttachment(relation, 2)
	if got != "spec file.pdf" {
		t.Fatalf("filename = %q, want URL basename", got)
	}
}

func TestFilenameForAttachmentUsesQueryNameAfterRelationName(t *testing.T) {
	for _, tc := range []struct {
		name     string
		relation Relation
		want     string
	}{
		{name: "query filename", relation: Relation{URL: "https://example.test/opaque-id?fileName=screen+shot%20%C3%A5.png"}, want: "screen shot å.png"},
		{name: "relation takes priority", relation: Relation{URL: "https://example.test/opaque-id?fileName=query.png", Attributes: map[string]any{"name": "relation.png"}}, want: "relation.png"},
		{name: "blank query falls back", relation: Relation{URL: "https://example.test/basename.png?fileName=+"}, want: "basename.png"},
		{name: "query filename sanitized", relation: Relation{URL: "https://example.test/opaque-id?fileName=folder%2Fscreen.png"}, want: "folder_screen.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := FilenameForAttachment(tc.relation, 1); got != tc.want {
				t.Fatalf("filename = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFilenameForAttachmentFallsBackToOrdinal(t *testing.T) {
	relation := Relation{URL: "https://dev.azure.com/"}

	got := FilenameForAttachment(relation, 3)
	if got != "attachment-3" {
		t.Fatalf("filename = %q, want attachment-3", got)
	}
}

func TestFilenameForAttachmentSanitizesPathSeparators(t *testing.T) {
	relation := Relation{Attributes: map[string]any{"name": `folder\unsafe/name.png`}}

	got := FilenameForAttachment(relation, 1)
	if got != "folder_unsafe_name.png" {
		t.Fatalf("filename = %q, want sanitized filename", got)
	}
}

func TestFilenameForAttachmentSanitizesUnsafeCharacters(t *testing.T) {
	relation := Relation{Attributes: map[string]any{"name": "bad\x00name\r\n?.txt"}}

	got := FilenameForAttachment(relation, 1)
	if got != "bad_name___.txt" {
		t.Fatalf("filename = %q, want control characters sanitized", got)
	}
}

func TestFilenameForAttachmentAvoidsReservedDeviceNames(t *testing.T) {
	relation := Relation{Attributes: map[string]any{"name": "CON.txt"}}

	got := FilenameForAttachment(relation, 1)
	if got != "_CON.txt" {
		t.Fatalf("filename = %q, want reserved device name prefixed", got)
	}
}

func TestFilenameForAttachmentTruncatesLongNamesPreservingExtension(t *testing.T) {
	relation := Relation{Attributes: map[string]any{"name": strings.Repeat("a", 300) + ".txt"}}

	got := FilenameForAttachment(relation, 1)
	if len(got) > maxFilenameLength {
		t.Fatalf("filename length = %d, want <= %d", len(got), maxFilenameLength)
	}
	if !strings.HasSuffix(got, ".txt") {
		t.Fatalf("filename = %q, want .txt extension", got)
	}
}

func TestFilenameForAttachmentTruncatesLongUnicodeNamesAtRuneBoundary(t *testing.T) {
	relation := Relation{Attributes: map[string]any{"name": strings.Repeat("å", 300) + ".txt"}}

	got := FilenameForAttachment(relation, 1)
	if len(got) > maxFilenameLength {
		t.Fatalf("filename length = %d, want <= %d", len(got), maxFilenameLength)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("filename = %q, want valid UTF-8", got)
	}
	if !strings.HasSuffix(got, ".txt") {
		t.Fatalf("filename = %q, want .txt extension", got)
	}
}

func TestDownloadAttachmentsKeepsDuplicateSuffixesWithinLengthLimit(t *testing.T) {
	outputDir := t.TempDir()
	longName := strings.Repeat("a", maxFilenameLength-4) + ".txt"
	item := WorkItem{
		ID: 12345,
		Relations: []Relation{
			{Rel: attachmentRelationType, URL: "https://example.test/a", Attributes: map[string]any{"name": longName}},
			{Rel: attachmentRelationType, URL: "https://example.test/b", Attributes: map[string]any{"name": longName}},
		},
	}
	downloader := fakeDownloader{data: map[string][]byte{
		"https://example.test/a": []byte("a"),
		"https://example.test/b": []byte("b"),
	}}

	attachments, err := DownloadAttachments(context.Background(), downloader, outputDir, item, nil)
	if err != nil {
		t.Fatalf("DownloadAttachments returned error: %v", err)
	}
	for _, attachment := range attachments {
		if len(attachment.Name) > maxFilenameLength {
			t.Fatalf("attachment name %q length = %d, want <= %d", attachment.Name, len(attachment.Name), maxFilenameLength)
		}
	}
	if attachments[0].Name == attachments[1].Name {
		t.Fatalf("duplicate attachment names were not uniqued: %q", attachments[0].Name)
	}
}

func TestDownloadAttachmentsWritesFilesAndSuffixesDuplicates(t *testing.T) {
	outputDir := t.TempDir()
	item := WorkItem{
		ID: 12345,
		Relations: []Relation{
			{Rel: attachmentRelationType, URL: "https://example.test/a", Attributes: map[string]any{"name": "same.png"}},
			{Rel: attachmentRelationType, URL: "https://example.test/b", Attributes: map[string]any{"name": "same.png"}},
		},
	}
	downloader := fakeDownloader{data: map[string][]byte{
		"https://example.test/a": []byte("a"),
		"https://example.test/b": []byte("b"),
	}}

	attachments, err := DownloadAttachments(context.Background(), downloader, outputDir, item, nil)
	if err != nil {
		t.Fatalf("DownloadAttachments returned error: %v", err)
	}
	if len(attachments) != 2 {
		t.Fatalf("attachments len = %d, want 2", len(attachments))
	}
	assertFileContent(t, filepath.Join(outputDir, "attachments", "12345", "same.png"), "a")
	assertFileContent(t, filepath.Join(outputDir, "attachments", "12345", "same-2.png"), "b")
}

func TestDownloadAttachmentsReturnsDownloadError(t *testing.T) {
	outputDir := t.TempDir()
	item := WorkItem{ID: 12345, Relations: []Relation{{Rel: attachmentRelationType, URL: "https://example.test/a"}}}
	downloader := fakeDownloader{errs: map[string]error{"https://example.test/a": errors.New("boom")}}

	_, err := DownloadAttachments(context.Background(), downloader, outputDir, item, nil)
	if err == nil {
		t.Fatal("DownloadAttachments error = nil, want error")
	}
}

func TestDownloadAttachmentsPreservesBytesForCollidingNames(t *testing.T) {
	for _, tc := range []struct {
		name  string
		names []string
	}{
		{name: "generated suffix", names: []string{"same.png", "same.png", "same-2.png"}},
		{name: "case differences", names: []string{"Same.png", "same.png", "SAME-2.PNG"}},
		{name: "unicode length limit", names: []string{strings.Repeat("å", 100) + ".png", strings.Repeat("å", 100) + ".png", strings.Repeat("å", 100) + ".png"}},
		{name: "long extension", names: []string{"a." + strings.Repeat("x", 178), "a." + strings.Repeat("x", 178)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outputDir := t.TempDir()
			item := WorkItem{ID: 12345}
			downloader := fakeDownloader{data: map[string][]byte{}}
			for i, name := range tc.names {
				url := fmt.Sprintf("https://example.test/file-%d", i)
				item.Relations = append(item.Relations, Relation{Rel: attachmentRelationType, URL: url, Attributes: map[string]any{"name": name}})
				downloader.data[url] = []byte(fmt.Sprintf("evidence %d", i))
			}
			attachments, err := DownloadAttachments(context.Background(), downloader, outputDir, item, nil)
			if err != nil {
				t.Fatalf("DownloadAttachments returned error: %v", err)
			}
			if len(attachments) != len(tc.names) {
				t.Fatalf("attachments count = %d, want %d", len(attachments), len(tc.names))
			}
			used := map[string]bool{}
			for i, attachment := range attachments {
				if used[strings.ToLower(attachment.Name)] {
					t.Errorf("attachment name %q collides case-insensitively", attachment.Name)
				}
				used[strings.ToLower(attachment.Name)] = true
				if len(attachment.Name) > maxFilenameLength || !utf8.ValidString(attachment.Name) {
					t.Errorf("attachment name %q exceeds byte limit or is invalid UTF-8", attachment.Name)
				}
				assertFileContent(t, filepath.Join(outputDir, filepath.FromSlash(attachment.Path)), fmt.Sprintf("evidence %d", i))
			}
		})
	}
}

type fakeDownloader struct {
	data map[string][]byte
	errs map[string]error
}

func (f fakeDownloader) Download(ctx context.Context, rawURL string) ([]byte, error) {
	if err := f.errs[rawURL]; err != nil {
		return nil, err
	}
	return f.data[rawURL], nil
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, data, want)
	}
}
