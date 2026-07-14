package ado

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientResolveWikiBuildsEncodedURLWithBasePathAndAuth(t *testing.T) {
	var seenPath string
	var seenAPIVersion string
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"id":"wiki-id","name":"Engineering/Docs","projectId":"project-id","repositoryId":"repo-id","mappedPath":"/Wiki","versions":[{"version":"wikiMaster","versionType":"branch"}],"url":"https://example.test/wiki","remoteUrl":"https://example.test/_wiki","type":"projectWiki","isDisabled":true,"properties":{"owner":"Docs"}}`)
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

	wiki, err := client.ResolveWiki(context.Background(), "Engineering/Docs")
	if err != nil {
		t.Fatalf("ResolveWiki returned error: %v", err)
	}
	if wiki.ID != "wiki-id" || wiki.Name != "Engineering/Docs" || wiki.ProjectID != "project-id" || wiki.RepositoryID != "repo-id" || wiki.MappedPath != "/Wiki" {
		t.Fatalf("wiki = %#v, want returned metadata", wiki)
	}
	if len(wiki.Versions) != 1 || wiki.Versions[0].Version != "wikiMaster" || wiki.Versions[0].VersionType != "branch" || wiki.URL == "" || wiki.RemoteURL == "" || wiki.Type != "projectWiki" || !wiki.IsDisabled || wiki.Properties["owner"] != "Docs" {
		t.Fatalf("wiki = %#v, want complete returned metadata", wiki)
	}
	if seenPath != "/tfs/DefaultCollection/My%20Project/_apis/wiki/wikis/Engineering%2FDocs" {
		t.Fatalf("path = %q, want encoded Azure DevOps wiki path", seenPath)
	}
	if seenAPIVersion != "7.0" {
		t.Fatalf("api-version = %q, want 7.0", seenAPIVersion)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
}

func TestClientResolveWikiRejectsMissingCanonicalID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"name":"Engineering"}`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.ResolveWiki(context.Background(), "Engineering")
	if err == nil {
		t.Fatal("ResolveWiki error = nil, want missing canonical ID error")
	}
	if !strings.Contains(err.Error(), "missing canonical wiki ID") {
		t.Fatalf("error = %q, want missing canonical wiki ID context", err.Error())
	}
}

func TestClientResolveWikiUsesGetWithoutRequestBody(t *testing.T) {
	var seenMethod string
	var seenBody []byte
	var seenContentType string
	httpClient := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		seenMethod = req.Method
		seenContentType = req.Header.Get("Content-Type")
		if req.Body != nil {
			var err error
			seenBody, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"wiki-id","name":"Engineering"}`)),
			Request:    req,
		}, nil
	})}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.ResolveWiki(context.Background(), "Engineering")
	if err != nil {
		t.Fatalf("ResolveWiki returned error: %v", err)
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
}

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

func TestClientWikiMethodsReturnStatusErrors(t *testing.T) {
	operations := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "resolve",
			call: func(client *Client) error {
				_, err := client.ResolveWiki(context.Background(), "wiki-id")
				return err
			},
		},
		{
			name: "fetch page",
			call: func(client *Client) error {
				_, err := client.FetchWikiPage(context.Background(), "wiki-id", WikiPageFetchOptions{Path: "/Guide", IncludeContent: true})
				return err
			},
		},
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError} {
		for _, operation := range operations {
			t.Run(fmt.Sprintf("%s_%d", operation.name, status), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "remote failure", status)
				}))
				t.Cleanup(server.Close)
				client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "super-secret"})
				if err != nil {
					t.Fatalf("NewClient returned error: %v", err)
				}

				err = operation.call(client)
				if err == nil {
					t.Fatal("wiki operation error = nil, want status error")
				}
				if !strings.Contains(err.Error(), fmt.Sprint(status)) || !strings.Contains(err.Error(), "remote failure") {
					t.Fatalf("error = %q, want status and bounded response context", err.Error())
				}
				if strings.Contains(err.Error(), "super-secret") {
					t.Fatalf("error = %q, want no PAT", err.Error())
				}
			})
		}
	}
}

func TestClientWikiMethodsExcludeDirectHTMLStatusBodies(t *testing.T) {
	operations := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "resolve",
			call: func(client *Client) error {
				_, err := client.ResolveWiki(context.Background(), "wiki-id")
				return err
			},
		},
		{
			name: "fetch page",
			call: func(client *Client) error {
				_, err := client.FetchWikiPage(context.Background(), "wiki-id", WikiPageFetchOptions{Path: "/Guide", IncludeContent: true})
				return err
			},
		},
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusInternalServerError} {
		for _, operation := range operations {
			t.Run(fmt.Sprintf("%s_%d", operation.name, status), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(status)
					fmt.Fprint(w, "<html><body>secret identity error details</body></html>")
				}))
				t.Cleanup(server.Close)
				client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "super-secret"})
				if err != nil {
					t.Fatalf("NewClient returned error: %v", err)
				}

				err = operation.call(client)
				if err == nil {
					t.Fatal("wiki operation error = nil, want status error")
				}
				errorText := err.Error()
				if !strings.Contains(errorText, fmt.Sprint(status)) {
					t.Fatalf("error = %q, want status", errorText)
				}
				for _, leaked := range []string{"<html", "secret identity error details", "super-secret"} {
					if strings.Contains(errorText, leaked) {
						t.Fatalf("error = %q, want no %q", errorText, leaked)
					}
				}
			})
		}
	}
}

func TestClientResolveWikiClassifiesDirectStatusBodies(t *testing.T) {
	tests := []struct {
		name          string
		contentType   string
		body          string
		bodyInError   string
		wantBodyError bool
	}{
		{
			name:        "HTML without content type",
			body:        "<html><body>untyped identity details</body></html>",
			bodyInError: "untyped identity details",
		},
		{
			name:        "HTML with misleading content type",
			contentType: "application/json",
			body:        "<html><body>mislabelled identity details</body></html>",
			bodyInError: "mislabelled identity details",
		},
		{
			name:        "mixed-case HTML content type with parameters",
			contentType: "Text/HTML; Charset=UTF-8",
			body:        "<html><body>mixed-case identity details</body></html>",
			bodyInError: "mixed-case identity details",
		},
		{
			name:          "non-HTML response context remains available",
			contentType:   "application/json; charset=utf-8",
			body:          `{"message":"useful remote failure"}`,
			bodyInError:   "useful remote failure",
			wantBodyError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, tt.body)
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "super-secret"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			_, err = client.ResolveWiki(context.Background(), "wiki-id")
			if err == nil {
				t.Fatal("ResolveWiki error = nil, want status error")
			}
			errorText := err.Error()
			if !strings.Contains(errorText, "500") {
				t.Fatalf("error = %q, want status", errorText)
			}
			if got := strings.Contains(errorText, tt.bodyInError); got != tt.wantBodyError {
				t.Fatalf("error = %q, body context present = %t, want %t", errorText, got, tt.wantBodyError)
			}
			if strings.Contains(errorText, "super-secret") {
				t.Fatalf("error = %q, want no PAT", errorText)
			}
		})
	}
}

func TestClientWikiMethodsRejectInvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed", body: `{"id":`},
		{name: "empty", body: ``},
		{name: "multiple values", body: `{"id":"wiki-id","path":"/Guide"} {"id":"other"}`},
	}
	operations := []struct {
		name        string
		decodeError string
		call        func(*Client) error
	}{
		{
			name:        "resolve",
			decodeError: "decoding Azure DevOps wiki",
			call: func(client *Client) error {
				_, err := client.ResolveWiki(context.Background(), "wiki-id")
				return err
			},
		},
		{
			name:        "fetch page",
			decodeError: "decoding Azure DevOps wiki page",
			call: func(client *Client) error {
				_, err := client.FetchWikiPage(context.Background(), "wiki-id", WikiPageFetchOptions{Path: "/Guide", IncludeContent: true})
				return err
			},
		},
	}
	for _, operation := range operations {
		for _, tt := range tests {
			t.Run(operation.name+"_"+tt.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, tt.body)
				}))
				t.Cleanup(server.Close)
				client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "super-secret"})
				if err != nil {
					t.Fatalf("NewClient returned error: %v", err)
				}

				err = operation.call(client)
				if err == nil {
					t.Fatal("wiki operation error = nil, want decode error")
				}
				if !strings.Contains(err.Error(), operation.decodeError) {
					t.Fatalf("error = %q, want %q", err.Error(), operation.decodeError)
				}
				if strings.Contains(err.Error(), "super-secret") {
					t.Fatalf("error = %q, want no PAT", err.Error())
				}
			})
		}
	}
}

func TestClientWikiMethodsWrapNetworkErrorsWithoutPAT(t *testing.T) {
	httpClient := &http.Client{Transport: failingRoundTripper{err: errors.New("network down")}}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project", PAT: "super-secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	operations := []struct {
		name string
		call func() error
	}{
		{
			name: "resolve",
			call: func() error {
				_, err := client.ResolveWiki(context.Background(), "wiki-id")
				return err
			},
		},
		{
			name: "fetch page",
			call: func() error {
				_, err := client.FetchWikiPage(context.Background(), "wiki-id", WikiPageFetchOptions{Path: "/Guide", IncludeContent: true})
				return err
			},
		},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			err := operation.call()
			if err == nil {
				t.Fatal("wiki operation error = nil, want network error")
			}
			if !strings.Contains(err.Error(), "network down") {
				t.Fatalf("error = %q, want network context", err.Error())
			}
			if strings.Contains(err.Error(), "super-secret") {
				t.Fatalf("error = %q, want no PAT", err.Error())
			}
		})
	}
}

func TestClientWikiMethodsDoNotFollowAuthenticationRedirect(t *testing.T) {
	var signInRequests atomic.Int32
	signIn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signInRequests.Add(1)
		fmt.Fprint(w, "<html>secret sign-in page</html>")
	}))
	t.Cleanup(signIn.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, signIn.URL+"/signin", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	httpClient, err := NewHTTPClient("")
	if err != nil {
		t.Fatalf("NewHTTPClient returned error: %v", err)
	}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "super-secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	operations := []struct {
		name string
		call func() error
	}{
		{
			name: "resolve",
			call: func() error {
				_, err := client.ResolveWiki(context.Background(), "wiki-id")
				return err
			},
		},
		{
			name: "fetch page",
			call: func() error {
				_, err := client.FetchWikiPage(context.Background(), "wiki-id", WikiPageFetchOptions{Path: "/Guide", IncludeContent: true})
				return err
			},
		},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			err := operation.call()
			if err == nil {
				t.Fatal("wiki operation error = nil, want redirect status error")
			}
			errorText := err.Error()
			if !strings.Contains(errorText, "302") || !strings.Contains(errorText, "PAT") || !strings.Contains(errorText, "base URL") {
				t.Fatalf("error = %q, want redirect status and guidance", errorText)
			}
			for _, leaked := range []string{"super-secret", "secret sign-in page", signIn.URL, "/signin", "Location"} {
				if strings.Contains(errorText, leaked) {
					t.Fatalf("error = %q, want no %q", errorText, leaked)
				}
			}
		})
	}
	if signInRequests.Load() != 0 {
		t.Fatalf("sign-in target requests = %d, want 0", signInRequests.Load())
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

type failingRoundTripper struct {
	err error
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (f failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, f.err
}
