package ado

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNewHTTPClientDoesNotFollowAuthenticationRedirectForFetchWorkItem(t *testing.T) {
	for _, status := range []int{
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect,
	} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var signInRequests atomic.Int32
			signIn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				signInRequests.Add(1)
				fmt.Fprint(w, "<html>secret sign-in page</html>")
			}))
			t.Cleanup(signIn.Close)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, signIn.URL+"/signin", status)
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

			_, err = client.FetchWorkItem(context.Background(), 1)
			if err == nil {
				t.Fatal("FetchWorkItem error = nil, want redirect status error")
			}
			if signInRequests.Load() != 0 {
				t.Errorf("sign-in target requests = %d, want 0", signInRequests.Load())
			}
			errorText := err.Error()
			if !strings.Contains(errorText, fmt.Sprint(status)) {
				t.Errorf("error = %q, want redirect status code", errorText)
			}
			for _, leaked := range []string{"decoding", "secret sign-in page", signIn.URL, "/signin"} {
				if strings.Contains(errorText, leaked) {
					t.Errorf("error = %q, want no %q", errorText, leaked)
				}
			}
		})
	}
}

func TestNewHTTPClientReturnsStatusForMalformedRedirectLocation(t *testing.T) {
	const malformedLocation = "http://[::1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", malformedLocation)
		w.WriteHeader(http.StatusFound)
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

	_, err = client.FetchWorkItem(context.Background(), 1)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want redirect status error")
	}
	errorText := err.Error()
	if !strings.Contains(errorText, "302") {
		t.Errorf("error = %q, want redirect status code", errorText)
	}
	for _, leaked := range []string{"failed to parse Location", malformedLocation} {
		if strings.Contains(errorText, leaked) {
			t.Errorf("error = %q, want no %q", errorText, leaked)
		}
	}
}

func TestNewHTTPClientReturnsStatusForRedirectWithoutLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
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

	_, err = client.FetchWorkItem(context.Background(), 1)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want redirect status error")
	}
	errorText := err.Error()
	if !strings.Contains(errorText, "302") {
		t.Errorf("error = %q, want redirect status code", errorText)
	}
	if strings.Contains(errorText, "decoding") {
		t.Errorf("error = %q, want no decode error", errorText)
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
