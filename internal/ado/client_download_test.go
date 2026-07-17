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

func TestClientDownloadDoesNotFollowAuthenticationRedirect(t *testing.T) {
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
	client, err := NewClient(httpClient, ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "invalid"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	data, err := client.Download(context.Background(), server.URL+"/attachment")
	if err == nil {
		t.Fatal("Download error = nil, want redirect status error")
	}
	if len(data) != 0 {
		t.Errorf("Download returned %d bytes, want 0", len(data))
	}
	if signInRequests.Load() != 0 {
		t.Errorf("sign-in target requests = %d, want 0", signInRequests.Load())
	}
	errorText := err.Error()
	if !strings.Contains(errorText, "302") {
		t.Errorf("error = %q, want redirect status code", errorText)
	}
	for _, leaked := range []string{"decoding", "secret sign-in page", signIn.URL, "/signin"} {
		if strings.Contains(errorText, leaked) {
			t.Errorf("error = %q, want no %q", errorText, leaked)
		}
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

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
