package ado

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientFetchWikiPageBuildsEncodedURLWithContentAndAuth(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenRawQuery string
	var seenPagePath string
	var seenIncludeContent string
	var seenAPIVersion string
	var seenAuth string
	var seenContentType string
	var seenBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenRawQuery = r.URL.RawQuery
		seenPagePath = r.URL.Query().Get("path")
		seenIncludeContent = r.URL.Query().Get("includeContent")
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		seenContentType = r.Header.Get("Content-Type")
		if r.Body != nil {
			var err error
			seenBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("reading request body: %v", err)
			}
		}
		fmt.Fprint(w, `{"id":51,"path":"/Guide & Setup/Intro?","order":2,"isParentPage":true,"isNonConformant":true,"gitItemPath":"/Guide-and-Setup/Intro-.md","content":"# Intro\n","url":"https://example.test/page","remoteUrl":"https://example.test/wiki/intro"}`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.Client(), ClientConfig{
		BaseURL:    server.URL + "/tfs/DefaultCollection",
		Project:    "My Project",
		APIVersion: "7.0",
		PAT:        "secret",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	page, err := client.FetchWikiPage(context.Background(), "wiki/id", WikiPageFetchOptions{
		Path:           "/Guide & Setup/Intro?",
		IncludeContent: true,
	})
	if err != nil {
		t.Fatalf("FetchWikiPage returned error: %v", err)
	}
	if page.ID != 51 || page.Path != "/Guide & Setup/Intro?" || page.Order != 2 || !page.IsParentPage || !page.IsNonConformant || page.GitItemPath != "/Guide-and-Setup/Intro-.md" || page.Content != "# Intro\n" || page.URL == "" || page.RemoteURL == "" {
		t.Fatalf("page = %#v, want returned metadata and content", page)
	}
	if seenMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", seenMethod)
	}
	if len(seenBody) != 0 {
		t.Fatalf("request body = %q, want empty", seenBody)
	}
	if seenContentType != "" {
		t.Fatalf("Content-Type = %q, want empty", seenContentType)
	}
	if seenPath != "/tfs/DefaultCollection/My%20Project/_apis/wiki/wikis/wiki%2Fid/pages" {
		t.Fatalf("path = %q, want encoded Azure DevOps wiki page path", seenPath)
	}
	if seenPagePath != "/Guide & Setup/Intro?" {
		t.Fatalf("page path query = %q, want original page path", seenPagePath)
	}
	if !strings.Contains(seenRawQuery, "path=%2FGuide+%26+Setup%2FIntro%3F") {
		t.Fatalf("raw query = %q, want encoded page path", seenRawQuery)
	}
	if seenIncludeContent != "true" {
		t.Fatalf("includeContent = %q, want true", seenIncludeContent)
	}
	if seenAPIVersion != "7.0" {
		t.Fatalf("api-version = %q, want 7.0", seenAPIVersion)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
}

func TestClientFetchWikiPageBuildsRecursiveMetadataQuery(t *testing.T) {
	var seenRecursionLevel string
	var seenIncludeContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRecursionLevel = r.URL.Query().Get("recursionLevel")
		seenIncludeContent = r.URL.Query().Get("includeContent")
		fmt.Fprint(w, `{"path":"/Guide","subPages":[{"path":"/Guide/Child","subPages":[]}]}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	page, err := client.FetchWikiPage(context.Background(), "wiki-id", WikiPageFetchOptions{
		Path:           "/Guide",
		RecursionLevel: "full",
		IncludeContent: false,
	})
	if err != nil {
		t.Fatalf("FetchWikiPage returned error: %v", err)
	}
	if seenRecursionLevel != "full" {
		t.Fatalf("recursionLevel = %q, want full", seenRecursionLevel)
	}
	if seenIncludeContent != "false" {
		t.Fatalf("includeContent = %q, want false", seenIncludeContent)
	}
	if len(page.SubPages) != 1 || page.SubPages[0].Path != "/Guide/Child" || page.SubPages[0].SubPages == nil {
		t.Fatalf("subPages = %#v, want decoded child and empty leaf list", page.SubPages)
	}
}

func TestClientFetchWikiPageRejectsMissingOrNonAbsoluteRequestPath(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"path":"/Guide","content":"content"}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	for _, pagePath := range []string{"", "Guide"} {
		t.Run(fmt.Sprintf("path_%q", pagePath), func(t *testing.T) {
			_, err := client.FetchWikiPage(context.Background(), "wiki-id", WikiPageFetchOptions{Path: pagePath, IncludeContent: true})
			if err == nil {
				t.Fatal("FetchWikiPage error = nil, want absolute path error")
			}
			if !strings.Contains(err.Error(), "absolute path") {
				t.Fatalf("error = %q, want absolute path context", err.Error())
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("requests = %d, want 0", requests.Load())
	}
}

func TestClientFetchWikiPageRejectsMissingOrNonAbsoluteResponsePath(t *testing.T) {
	for _, responsePath := range []string{"", "Guide"} {
		t.Run(fmt.Sprintf("path_%q", responsePath), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"path":%q,"content":"content"}`, responsePath)
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "secret"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			_, err = client.FetchWikiPage(context.Background(), "wiki-id", WikiPageFetchOptions{Path: "/Guide", IncludeContent: true})
			if err == nil {
				t.Fatal("FetchWikiPage error = nil, want absolute response path error")
			}
			if !strings.Contains(err.Error(), "response missing absolute path") {
				t.Fatalf("error = %q, want absolute response path context", err.Error())
			}
		})
	}
}

func TestClientFetchWikiPageRejectsMismatchedResponsePath(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"path":"/Other","content":"content"}`)),
			Request:    req,
		}, nil
	})}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWikiPage(context.Background(), "wiki-id", WikiPageFetchOptions{Path: "/Guide", IncludeContent: true})
	if err == nil {
		t.Fatal("FetchWikiPage error = nil, want path mismatch error")
	}
	if !strings.Contains(err.Error(), `response path "/Other" does not match requested path "/Guide"`) {
		t.Fatalf("error = %q, want requested and response paths", err.Error())
	}
}

func TestClientFetchWikiPageAcceptsEmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"path":"/Empty","content":""}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	page, err := client.FetchWikiPage(context.Background(), "wiki-id", WikiPageFetchOptions{Path: "/Empty", IncludeContent: true})
	if err != nil {
		t.Fatalf("FetchWikiPage returned error: %v", err)
	}
	if page.Content != "" {
		t.Fatalf("content = %q, want empty", page.Content)
	}
}
