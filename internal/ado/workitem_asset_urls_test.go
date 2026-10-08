package ado

import "testing"

func TestWorkItemAssetResolvesOnlyConfiguredAttachmentURLs(t *testing.T) {
	const base = "https://dev.azure.com/org"
	for _, tc := range []struct {
		url  string
		want string
	}{
		{"https://dev.azure.com/org/_apis/wit/attachments/a?fileName=a.png#view", "https://dev.azure.com/org/_apis/wit/attachments/a?fileName=a.png"},
		{"/org/Project/_apis/wit/attachments/a", "https://dev.azure.com/org/Project/_apis/wit/attachments/a"},
		{"_apis/wit/attachments/a", "https://dev.azure.com/org/Project/_apis/wit/attachments/a"},
		{"https://external.test/org/_apis/wit/attachments/a", ""},
		{"https://dev.azure.com/other/_apis/wit/attachments/a", ""},
		{"https://user:secret@dev.azure.com/org/_apis/wit/attachments/a", ""},
		{"/org/Project/_apis/wit/workitems/1", ""},
		{"data:image/png;base64,abc", ""},
		{"javascript:alert(1)", ""},
		{"/org/Project/../_apis/wit/attachments/a", ""},
		{"/org/Project/_apis/wit/attachments/%2e%2e", ""},
		{"/org/Project/_apis/wit/attachments/a%2fb", ""},
		{"/org/Project/_apis/wit/attachments/a%252fb", ""},
		{"/org/Project/_apis/wit/attachments/a/b", ""},
		{"", ""},
	} {
		t.Run(tc.url, func(t *testing.T) {
			got, _, reason := resolveInlineAssetURL(tc.url, base, "Project")
			if got != tc.want || (reason == "") != (tc.want != "") {
				t.Fatalf("resolved = %q, reason = %q, want %q", got, reason, tc.want)
			}
		})
	}
	_, first, _ := resolveInlineAssetURL(base+"/_apis/wit/attachments/a?fileName=one.png&download=true", base, "Project")
	_, second, _ := resolveInlineAssetURL(base+"/Project/_apis/wit/attachments/a?api-version=7.1", base, "Project")
	if first != second {
		t.Fatalf("attachment identity differs for presentation variants: %q != %q", first, second)
	}
}
