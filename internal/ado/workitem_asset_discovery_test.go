package ado

import (
	"reflect"
	"testing"
)

func TestWorkItemAssetDiscoversFieldsAndMarkdownComments(t *testing.T) {
	markdown := "markdown"
	item := WorkItem{ID: 1, Fields: map[string]any{
		"System.Description":            `<p><IMG SRC="/org/_apis/wit/attachments/a?fileName=a.png&amp;download=true"></p>`,
		"Microsoft.VSTS.TCM.ReproSteps": `<img src='/org/_apis/wit/attachments/b'>`,
		"Custom.Note":                   `&lt;img src="not-an-image"&gt;`,
	}}
	comments := []WorkItemReadComment{{ID: 7, Format: &markdown, Text: "![shot][ref]\n\n[ref]: /org/_apis/wit/attachments/c\n\n`![example](/ignored)`\n\n```html\n<img src='/ignored-too'>\n```\n\n<img src='/org/_apis/wit/attachments/d'>"}}
	got := workItemImageSources(item, comments)
	want := []AssetSource{
		{Kind: "field", Field: "Microsoft.VSTS.TCM.ReproSteps", URL: "/org/_apis/wit/attachments/b"},
		{Kind: "field", Field: "System.Description", URL: "/org/_apis/wit/attachments/a?fileName=a.png&download=true"},
		{Kind: "comment", CommentID: 7, URL: "/org/_apis/wit/attachments/c"},
		{Kind: "comment", CommentID: 7, URL: "/org/_apis/wit/attachments/d"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %#v, want %#v", got, want)
	}
}

func TestWorkItemAssetMarkdownDecodesImageDestination(t *testing.T) {
	text := `![shot](/org/_apis/wit/attachments/a?fileName=shot.png&amp;download=true)`
	got := workItemImageSources(WorkItem{}, []WorkItemReadComment{{ID: 1, Text: text}})
	if len(got) != 1 || got[0].URL != "/org/_apis/wit/attachments/a?fileName=shot.png&download=true" {
		t.Fatalf("image sources = %+v", got)
	}
}
