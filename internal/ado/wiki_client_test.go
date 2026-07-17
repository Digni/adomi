package ado

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

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
