package ado

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
