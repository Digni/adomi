package ado

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientListInProgressPipelineRunsAcceptsSafeTransports(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "HTTPS", baseURL: "https://dev.azure.com/org"},
		{name: "localhost", baseURL: "http://LOCALHOST:8080/collection"},
		{name: "IPv4 loopback", baseURL: "http://127.0.0.2:8080/collection"},
		{name: "IPv6 loopback", baseURL: "http://[::1]:8080/collection"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			httpClient := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests.Add(1)
				return pipelineTestResponse(req, http.StatusOK, `{"value":[]}`), nil
			})}
			client, err := NewClient(httpClient, ClientConfig{BaseURL: tt.baseURL, Project: "Project", PAT: "secret"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}
			if _, err := client.ListInProgressPipelineRuns(context.Background()); err != nil {
				t.Fatalf("ListInProgressPipelineRuns returned error: %v", err)
			}
			if requests.Load() != 1 {
				t.Fatalf("requests = %d, want 1", requests.Load())
			}
		})
	}
}

func TestClientListInProgressPipelineRunsRejectsUnsafeTransportsBeforeRequest(t *testing.T) {
	for _, baseURL := range []string{
		"http://dev.azure.com/org",
		"http://localhost.example.com/org",
		"ftp://localhost/org",
	} {
		t.Run(baseURL, func(t *testing.T) {
			var requests atomic.Int32
			httpClient := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests.Add(1)
				if req.Header.Get("Authorization") != "" {
					t.Errorf("Authorization = %q, want no authenticated request", req.Header.Get("Authorization"))
				}
				return pipelineTestResponse(req, http.StatusInternalServerError, "unexpected request"), nil
			})}
			client, err := NewClient(httpClient, ClientConfig{BaseURL: baseURL, Project: "Project", PAT: "secret"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}
			if _, err := client.ListInProgressPipelineRuns(context.Background()); err == nil {
				t.Fatal("ListInProgressPipelineRuns error = nil, want unsafe transport error")
			}
			if requests.Load() != 0 {
				t.Fatalf("requests = %d, want 0", requests.Load())
			}
		})
	}
}

func TestClientListInProgressPipelineRunsBypassesConfiguredProxyForLoopbackHTTP(t *testing.T) {
	var originRequests atomic.Int32
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := originRequests.Add(1)
		if request == 1 {
			w.Header().Set("X-MS-ContinuationToken", "second-page")
		}
		fmt.Fprint(w, `{"value":[]}`)
	}))
	var originConnections atomic.Int32
	origin.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			originConnections.Add(1)
		}
	}
	origin.Start()
	t.Cleanup(origin.Close)

	var proxyRequests atomic.Int32
	var proxyAuthorization string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyRequests.Add(1)
		proxyAuthorization = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"value":[]}`)
	}))
	t.Cleanup(proxy.Close)

	httpClient, err := NewHTTPClient(proxy.URL)
	if err != nil {
		t.Fatalf("NewHTTPClient returned error: %v", err)
	}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: origin.URL, Project: "Project", PAT: "loopback-secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if _, err := client.ListInProgressPipelineRuns(context.Background()); err != nil {
		t.Fatalf("ListInProgressPipelineRuns returned error: %v", err)
	}
	if originRequests.Load() != 2 {
		t.Fatalf("origin requests = %d, want 2 direct loopback requests", originRequests.Load())
	}
	if originConnections.Load() != 1 {
		t.Fatalf("origin connections = %d, want one reused connection", originConnections.Load())
	}
	if proxyRequests.Load() != 0 || proxyAuthorization != "" {
		t.Fatalf("proxy requests/Authorization = %d/%q, want no proxy exposure", proxyRequests.Load(), proxyAuthorization)
	}
}

func TestClientPipelineRunSuccessResponsesEnforceSizeLimit(t *testing.T) {
	operations := []struct {
		name      string
		validBody string
		invoke    func(*Client) error
	}{
		{
			name:      "list",
			validBody: `{"value":[]}`,
			invoke: func(client *Client) error {
				_, err := client.ListInProgressPipelineRuns(context.Background())
				return err
			},
		},
		{
			name:      "get",
			validBody: pipelineRunJSON(1, 1, "inProgress", "null"),
			invoke: func(client *Client) error {
				_, err := client.GetPipelineRun(context.Background(), 1)
				return err
			},
		},
	}
	cases := []struct {
		name           string
		size           int64
		declaredLength int64
		wantError      bool
	}{
		{name: "exactly 8 MiB", size: maxPipelineResponseBytes, declaredLength: maxPipelineResponseBytes},
		{name: "declared over limit", size: 0, declaredLength: maxPipelineResponseBytes + 1, wantError: true},
		{name: "streamed over limit without length", size: maxPipelineResponseBytes + 1, declaredLength: -1, wantError: true},
		{name: "streamed over limit with incorrect length", size: maxPipelineResponseBytes + 1, declaredLength: 1, wantError: true},
	}
	for _, operation := range operations {
		for _, tc := range cases {
			t.Run(operation.name+"/"+tc.name, func(t *testing.T) {
				client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
					size := tc.size
					if size == 0 {
						size = int64(len(operation.validBody))
					}
					return pipelineSizedResponse(req, operation.validBody, size, tc.declaredLength), nil
				})

				err := operation.invoke(client)
				if tc.wantError {
					if err == nil || !strings.Contains(err.Error(), "maximum size") {
						t.Fatalf("error = %v, want size-limit error", err)
					}
				} else if err != nil {
					t.Fatalf("operation returned error for exact-size valid response: %v", err)
				}
			})
		}
	}
}

func TestClientPipelineRunHTTPFailuresSuppressConfidentialAndUntrustedData(t *testing.T) {
	const (
		patMarker      = "UNIQUE_PAT_MARKER"
		bodyMarker     = "UNIQUE_BODY_MARKER"
		htmlMarker     = "UNIQUE_HTML_MARKER"
		locationMarker = "UNIQUE_LOCATION_MARKER"
	)
	authorizationMarker := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+patMarker))
	operations := []struct {
		name   string
		invoke func(*Client) error
	}{
		{
			name: "list",
			invoke: func(client *Client) error {
				_, err := client.ListInProgressPipelineRuns(context.Background())
				return err
			},
		},
		{
			name: "get",
			invoke: func(client *Client) error {
				_, err := client.GetPipelineRun(context.Background(), 1)
				return err
			},
		},
	}
	for _, operation := range operations {
		for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
			t.Run(fmt.Sprintf("%s/status %d", operation.name, status), func(t *testing.T) {
				client := newPipelineTestClientWithPAT(t, patMarker, func(req *http.Request) (*http.Response, error) {
					body := bodyMarker + `<html>` + htmlMarker + `</html>` + authorizationMarker + patMarker
					response := pipelineTestResponse(req, status, body)
					response.Header.Set("Location", "https://example.invalid/"+locationMarker)
					return response, nil
				})

				err := operation.invoke(client)
				if err == nil {
					t.Fatal("operation error = nil, want HTTP status error")
				}
				errorText := err.Error()
				if !strings.Contains(errorText, fmt.Sprint(status)) || !strings.Contains(errorText, "Azure DevOps") {
					t.Fatalf("error = %q, want safe operation/status guidance", errorText)
				}
				assertPipelineErrorOmits(t, errorText, patMarker, authorizationMarker, bodyMarker, htmlMarker, locationMarker, "<html>")
			})
		}
	}
}

func TestClientPipelineRunRedirectsSuppressLocationAndDoNotFollow(t *testing.T) {
	const (
		patMarker      = "REDIRECT_PAT_MARKER"
		locationMarker = "REDIRECT_LOCATION_MARKER"
	)
	for _, operation := range []struct {
		name   string
		invoke func(*Client) error
	}{
		{
			name: "list",
			invoke: func(client *Client) error {
				_, err := client.ListInProgressPipelineRuns(context.Background())
				return err
			},
		},
		{
			name: "get",
			invoke: func(client *Client) error {
				_, err := client.GetPipelineRun(context.Background(), 1)
				return err
			},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) > 1 {
					t.Fatalf("redirect was followed to %s", r.URL)
				}
				http.Redirect(w, r, "/"+locationMarker, http.StatusFound)
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: patMarker})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			err = operation.invoke(client)
			if err == nil || !strings.Contains(err.Error(), "302") {
				t.Fatalf("error = %v, want redirect status and safe guidance", err)
			}
			authorizationMarker := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+patMarker))
			assertPipelineErrorOmits(t, err.Error(), patMarker, authorizationMarker, locationMarker, "<a href")
			if requests.Load() != 1 {
				t.Fatalf("requests = %d, want redirect not followed", requests.Load())
			}
		})
	}
}

func TestClientPipelineRunNetworkFailuresSuppressTransportDiagnostics(t *testing.T) {
	const (
		patMarker     = "NETWORK_PAT_MARKER"
		networkMarker = "NETWORK_ERROR_MARKER"
	)
	for _, operation := range []struct {
		name   string
		invoke func(*Client) error
	}{
		{
			name: "list",
			invoke: func(client *Client) error {
				_, err := client.ListInProgressPipelineRuns(context.Background())
				return err
			},
		},
		{
			name: "get",
			invoke: func(client *Client) error {
				_, err := client.GetPipelineRun(context.Background(), 1)
				return err
			},
		},
	} {
		t.Run(operation.name, func(t *testing.T) {
			client := newPipelineTestClientWithPAT(t, patMarker, func(req *http.Request) (*http.Response, error) {
				return nil, errors.New(networkMarker + " " + req.Header.Get("Authorization"))
			})
			err := operation.invoke(client)
			if err == nil || !strings.Contains(err.Error(), "connectivity") {
				t.Fatalf("error = %v, want safe network guidance", err)
			}
			authorizationMarker := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+patMarker))
			assertPipelineErrorOmits(t, err.Error(), patMarker, authorizationMarker, networkMarker)
		})
	}
}
