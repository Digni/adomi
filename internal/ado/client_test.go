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

func TestClientFetchWorkItemBuildsURLWithBasePathAndAuth(t *testing.T) {
	var seenPath string
	var seenExpand string
	var seenAPIVersion string
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		seenExpand = r.URL.Query().Get("$expand")
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"id":12345,"fields":{"System.Title":"Title","System.WorkItemType":"Task"}}`)
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

	item, err := client.FetchWorkItem(context.Background(), 12345)
	if err != nil {
		t.Fatalf("FetchWorkItem returned error: %v", err)
	}
	if item.ID != 12345 {
		t.Fatalf("item ID = %d, want 12345", item.ID)
	}
	if seenPath != "/tfs/DefaultCollection/My%20Project/_apis/wit/workitems/12345" {
		t.Fatalf("path = %q, want Azure DevOps work item path", seenPath)
	}
	if seenExpand != "all" {
		t.Fatalf("$expand = %q, want all", seenExpand)
	}
	if seenAPIVersion != "7.0" {
		t.Fatalf("api-version = %q, want 7.0", seenAPIVersion)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
}

func TestClientFetchWorkItemReturnsNon2xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 1)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %q, want status code", err.Error())
	}
}

func TestClientFetchWorkItemReturnsCleanNon2xxErrorWithEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 1)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want error")
	}
	if strings.HasSuffix(err.Error(), ":") {
		t.Fatalf("error = %q, want no dangling colon", err.Error())
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %q, want status code", err.Error())
	}
}

func TestClientFetchWorkItemRejectsMismatchedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":999,"fields":{"System.Title":"Wrong"}}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 12345)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want ID mismatch error")
	}
	if !strings.Contains(err.Error(), "12345") || !strings.Contains(err.Error(), "999") {
		t.Fatalf("error = %q, want requested and response IDs", err.Error())
	}
}

func TestClientFetchWorkItemReturnsMalformedJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 12345)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want decode error")
	}
	if !strings.Contains(err.Error(), "decoding Azure DevOps work item 12345") {
		t.Fatalf("error = %q, want decode context", err.Error())
	}
}

func TestClientFetchWorkItemReturnsEmptyBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 12345)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want decode error")
	}
	if !strings.Contains(err.Error(), "decoding Azure DevOps work item 12345") {
		t.Fatalf("error = %q, want decode context", err.Error())
	}
}

func TestClientDownloadUsesAuthAndReturnsBytes(t *testing.T) {
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, "attachment bytes")
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	got, err := client.Download(context.Background(), server.URL+"/file")
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	if string(got) != "attachment bytes" {
		t.Fatalf("download = %q, want attachment bytes", got)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
}

func TestClientDownloadReturnsNon2xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.Download(context.Background(), server.URL+"/file")
	if err == nil {
		t.Fatal("Download error = nil, want error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %q, want status code", err.Error())
	}
}

func TestClientDownloadRejectsAttachmentOverMaxSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(maxAttachmentBytes+1))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.Download(context.Background(), server.URL+"/large.bin")
	if err == nil {
		t.Fatal("Download error = nil, want size limit error")
	}
	if !strings.Contains(err.Error(), "attachment exceeds") {
		t.Fatalf("error = %q, want size limit context", err.Error())
	}
}

func TestReadAttachmentBodyRejectsStreamOverMaxSize(t *testing.T) {
	_, err := readAttachmentBody(strings.NewReader("123456789"), 8)
	if err != nil {
		if strings.Contains(err.Error(), "attachment exceeds") {
			return
		}
		t.Fatalf("error = %q, want size limit context", err.Error())
	}
	t.Fatal("readAttachmentBody error = nil, want size limit error")
}

func TestReadAttachmentBodyWrapsReadError(t *testing.T) {
	_, err := readAttachmentBody(errorReader{}, 8)
	if err == nil {
		t.Fatal("readAttachmentBody error = nil, want read error")
	}
	if !strings.Contains(err.Error(), "reading Azure DevOps attachment") {
		t.Fatalf("error = %q, want read context", err.Error())
	}
}

func TestClientDownloadRejectsExternalHostBeforeAuth(t *testing.T) {
	var externalRequests atomic.Int32
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		externalRequests.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("external request received Authorization header %q", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, "external")
	}))
	t.Cleanup(external.Close)
	base := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "base")
	}))
	t.Cleanup(base.Close)
	client, err := NewClient(base.Client(), ClientConfig{BaseURL: base.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.Download(context.Background(), external.URL+"/file")
	if err == nil {
		t.Fatal("Download error = nil, want same-origin error")
	}
	if externalRequests.Load() != 0 {
		t.Fatalf("external requests = %d, want 0", externalRequests.Load())
	}
}

func TestNewHTTPClientRejectsInvalidProxy(t *testing.T) {
	_, err := NewHTTPClient("://bad proxy")
	if err == nil {
		t.Fatal("NewHTTPClient error = nil, want error")
	}
}

func TestNewHTTPClientRejectsProxyWithoutScheme(t *testing.T) {
	_, err := NewHTTPClient("proxy.company.local:8080")
	if err == nil {
		t.Fatal("NewHTTPClient error = nil, want error")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
