package ado

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientListInProgressPipelineRunsBuildsRequestAndDecodesResponse(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenStatusFilter string
	var seenQueryOrder string
	var seenAPIVersion string
	var seenAuth string
	var seenContentType string
	var seenBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenStatusFilter = r.URL.Query().Get("statusFilter")
		seenQueryOrder = r.URL.Query().Get("queryOrder")
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
		fmt.Fprint(w, `{"value":[{"id":17,"buildNumber":"20260715.1","status":"inProgress","result":"none","definition":{"id":23,"name":"Build and deploy"},"sourceBranch":" refs/heads/main ","sourceVersion":" abc123 ","queueTime":"2026-07-15T08:09:10.1200+02:00","startTime":"2026-07-15T06:10:11Z","finishTime":null,"url":"https://ignored.example/build/17","_links":{"web":{"href":" https://dev.azure.com/org/project/_build/results?buildId=17 "}}}]}`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.Client(), ClientConfig{
		BaseURL:    server.URL + "/tfs/Default Collection",
		Project:    "My Project/Area",
		APIVersion: "7.0",
		PAT:        "secret",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	runs, err := client.ListInProgressPipelineRuns(context.Background())
	if err != nil {
		t.Fatalf("ListInProgressPipelineRuns returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs length = %d, want 1", len(runs))
	}
	run := runs[0]
	if run.ID != 17 || run.PipelineID != 23 || run.PipelineName != "Build and deploy" || run.RunNumber != "20260715.1" || run.Status != "inProgress" || run.Result != nil {
		t.Fatalf("run = %#v, want normalized in-progress run", run)
	}
	if run.SourceBranch == nil || *run.SourceBranch != "refs/heads/main" {
		t.Fatalf("source branch = %#v, want trimmed branch", run.SourceBranch)
	}
	if run.SourceVersion == nil || *run.SourceVersion != "abc123" {
		t.Fatalf("source version = %#v, want trimmed version", run.SourceVersion)
	}
	if run.WebURL == nil || *run.WebURL != "https://dev.azure.com/org/project/_build/results?buildId=17" {
		t.Fatalf("web URL = %#v, want _links.web.href", run.WebURL)
	}
	wantQueueTime := time.Date(2026, 7, 15, 6, 9, 10, 120_000_000, time.UTC)
	if run.QueueTime == nil || !run.QueueTime.Equal(wantQueueTime) || run.QueueTime.Location() != time.UTC {
		t.Fatalf("queue time = %#v, want %s in UTC", run.QueueTime, wantQueueTime)
	}
	if run.StartTime == nil || run.StartTime.Format(time.RFC3339Nano) != "2026-07-15T06:10:11Z" || run.FinishTime != nil {
		t.Fatalf("start/finish times = %#v/%#v, want normalized start and nil finish", run.StartTime, run.FinishTime)
	}
	if seenMethod != http.MethodGet || len(seenBody) != 0 || seenContentType != "" {
		t.Fatalf("request = %s body %q content-type %q, want bodyless GET", seenMethod, seenBody, seenContentType)
	}
	if seenPath != "/tfs/Default%20Collection/My%20Project%2FArea/_apis/build/builds" {
		t.Fatalf("path = %q, want escaped collection and project path", seenPath)
	}
	if seenStatusFilter != "inProgress" || seenQueryOrder != "queueTimeDescending" || seenAPIVersion != "7.0" {
		t.Fatalf("query status/order/version = %q/%q/%q", seenStatusFilter, seenQueryOrder, seenAPIVersion)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
}

func TestClientGetPipelineRunBuildsRequestAndDecodesCompletedResponse(t *testing.T) {
	var seenPath string
	var seenAPIVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		seenAPIVersion = r.URL.Query().Get("api-version")
		fmt.Fprint(w, `{"id":2147483647,"buildNumber":"release-42","status":"completed","result":" succeeded ","definition":{"id":2147483647,"name":"Release build"},"sourceBranch":null,"sourceVersion":"","queueTime":null,"startTime":null,"finishTime":"2026-07-15T08:09:10.1234000Z","_links":{"web":{"href":null}}}`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.Client(), ClientConfig{
		BaseURL:    server.URL + "/collection",
		Project:    "Project",
		APIVersion: "7.0-preview.1",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	run, err := client.GetPipelineRun(context.Background(), 2147483647)
	if err != nil {
		t.Fatalf("GetPipelineRun returned error: %v", err)
	}
	if run.ID != 2147483647 || run.PipelineID != 2147483647 || run.Result == nil || *run.Result != "succeeded" {
		t.Fatalf("run = %#v, want completed signed-int32-boundary run", run)
	}
	if run.SourceBranch != nil || run.SourceVersion != nil || run.QueueTime != nil || run.StartTime != nil || run.WebURL != nil {
		t.Fatalf("optional values = %#v, want nil", run)
	}
	if run.FinishTime == nil || run.FinishTime.Format(time.RFC3339Nano) != "2026-07-15T08:09:10.1234Z" || run.FinishTime.Location() != time.UTC {
		t.Fatalf("finish time = %#v, want normalized fractional UTC timestamp", run.FinishTime)
	}
	if seenPath != "/collection/Project/_apis/build/builds/2147483647" {
		t.Fatalf("path = %q, want signed-int32-boundary route", seenPath)
	}
	if seenAPIVersion != "7.0-preview.1" {
		t.Fatalf("api-version = %q, want configured version", seenAPIVersion)
	}
}

func TestClientGetPipelineRunAcceptsSignedInt32RouteBoundaries(t *testing.T) {
	for _, id := range []int{1, 2147483647} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			var requests atomic.Int32
			httpClient := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests.Add(1)
				body := fmt.Sprintf(`{"id":%d,"buildNumber":"run","status":"inProgress","definition":{"id":1,"name":"Pipeline"}}`, id)
				return pipelineTestResponse(req, http.StatusOK, body), nil
			})}
			client, err := NewClient(httpClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}
			if _, err := client.GetPipelineRun(context.Background(), id); err != nil {
				t.Fatalf("GetPipelineRun(%d) returned error: %v", id, err)
			}
			if requests.Load() != 1 {
				t.Fatalf("requests = %d, want 1", requests.Load())
			}
		})
	}
}

func TestClientGetPipelineRunRejectsOutOfRangeIDBeforeRequest(t *testing.T) {
	for _, id := range []int{-1, 0, 2147483648} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			var requests atomic.Int32
			httpClient := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests.Add(1)
				return pipelineTestResponse(req, http.StatusInternalServerError, "unexpected request"), nil
			})}
			client, err := NewClient(httpClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}
			if _, err := client.GetPipelineRun(context.Background(), id); err == nil {
				t.Fatalf("GetPipelineRun(%d) error = nil, want range error", id)
			}
			if requests.Load() != 0 {
				t.Fatalf("requests = %d, want 0", requests.Load())
			}
		})
	}
}

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

func TestClientGetPipelineRunRejectsMismatchedResponseID(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return pipelineTestResponse(req, http.StatusOK, `{"id":8,"buildNumber":"run","status":"inProgress","definition":{"id":1,"name":"Pipeline"}}`), nil
	})}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.GetPipelineRun(context.Background(), 7)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %v, want mismatched ID error", err)
	}
}

func TestClientListInProgressPipelineRunsAcceptsPresentNullAndEmptyValue(t *testing.T) {
	for _, body := range []string{`{"value":null}`, `{"value":[]}`} {
		t.Run(body, func(t *testing.T) {
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineTestResponse(req, http.StatusOK, body), nil
			})

			runs, err := client.ListInProgressPipelineRuns(context.Background())
			if err != nil {
				t.Fatalf("ListInProgressPipelineRuns returned error: %v", err)
			}
			if runs == nil || len(runs) != 0 {
				t.Fatalf("runs = %#v, want non-nil empty collection", runs)
			}
		})
	}
}

func TestClientListInProgressPipelineRunsPreservesObservedOrderAcrossStateChangesAndEmptyPage(t *testing.T) {
	opaqueToken := " 01+/%?=&opaque "
	var requests atomic.Int32
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		request := int(requests.Add(1))
		if got := req.URL.Query().Get("statusFilter"); got != "inProgress" {
			t.Fatalf("request %d statusFilter = %q, want inProgress", request, got)
		}
		if got := req.URL.Query().Get("queryOrder"); got != "queueTimeDescending" {
			t.Fatalf("request %d queryOrder = %q, want queueTimeDescending", request, got)
		}
		response := pipelineTestResponse(req, http.StatusOK, `{"value":[]}`)
		switch request {
		case 1:
			if got := req.URL.Query().Get("continuationToken"); got != "" {
				t.Fatalf("first continuationToken = %q, want absent", got)
			}
			response.Body = io.NopCloser(strings.NewReader(pipelineRunListJSON(30)))
			response.Header.Set("X-MS-ContinuationToken", opaqueToken)
		case 2:
			if got := req.URL.Query().Get("continuationToken"); got != opaqueToken {
				t.Fatalf("second continuationToken = %q, want exact opaque token %q", got, opaqueToken)
			}
			response.Header.Set("X-MS-ContinuationToken", "next-token")
		case 3:
			if got := req.URL.Query().Get("continuationToken"); got != "next-token" {
				t.Fatalf("third continuationToken = %q, want next-token", got)
			}
			response.Body = io.NopCloser(strings.NewReader(pipelineRunListJSON(10, 40)))
		default:
			t.Fatalf("unexpected request %d to %s", request, req.URL)
		}
		response.ContentLength = -1
		return response, nil
	})

	runs, err := client.ListInProgressPipelineRuns(context.Background())
	if err != nil {
		t.Fatalf("ListInProgressPipelineRuns returned error: %v", err)
	}
	if requests.Load() != 3 {
		t.Fatalf("requests = %d, want one finite three-page traversal", requests.Load())
	}
	if len(runs) != 3 || runs[0].ID != 30 || runs[1].ID != 10 || runs[2].ID != 40 {
		t.Fatalf("run IDs = %v, want received order [30 10 40]", pipelineRunIDs(runs))
	}
}

func TestClientListInProgressPipelineRunsRejectsInvalidContinuationSequences(t *testing.T) {
	tests := []struct {
		name         string
		tokens       [][]string
		wantRequests int32
	}{
		{name: "blank", tokens: [][]string{{""}}, wantRequests: 1},
		{name: "whitespace", tokens: [][]string{{" \t "}}, wantRequests: 1},
		{name: "multiple", tokens: [][]string{{"first", "second"}}, wantRequests: 1},
		{name: "immediate repetition", tokens: [][]string{{"cycle"}, {"cycle"}}, wantRequests: 2},
		{name: "earlier repetition", tokens: [][]string{{"first"}, {"second"}, {"first"}}, wantRequests: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				request := int(requests.Add(1))
				if request > len(tt.tokens) {
					t.Fatalf("unexpected request %d", request)
				}
				response := pipelineTestResponse(req, http.StatusOK, `{"value":[]}`)
				for _, token := range tt.tokens[request-1] {
					response.Header.Add("X-MS-ContinuationToken", token)
				}
				return response, nil
			})

			if _, err := client.ListInProgressPipelineRuns(context.Background()); err == nil {
				t.Fatal("ListInProgressPipelineRuns error = nil, want invalid continuation error")
			}
			if requests.Load() != tt.wantRequests {
				t.Fatalf("requests = %d, want %d", requests.Load(), tt.wantRequests)
			}
		})
	}
}

func TestClientListInProgressPipelineRunsRejectsDuplicateIDsAcrossPages(t *testing.T) {
	var requests atomic.Int32
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		request := requests.Add(1)
		response := pipelineTestResponse(req, http.StatusOK, pipelineRunListJSON(17))
		if request == 1 {
			response.Header.Set("X-MS-ContinuationToken", "second-page")
		}
		return response, nil
	})

	if _, err := client.ListInProgressPipelineRuns(context.Background()); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %v, want duplicate run ID error", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2 and no further request", requests.Load())
	}
}

func TestClientListInProgressPipelineRunsEnforcesPageCeiling(t *testing.T) {
	tests := []struct {
		name        string
		lastHasNext bool
		wantError   bool
	}{
		{name: "exactly at limit", lastHasNext: false},
		{name: "over limit", lastHasNext: true, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				request := requests.Add(1)
				if request > maxPipelinePages {
					t.Fatalf("unexpected request %d beyond page ceiling", request)
				}
				response := pipelineTestResponse(req, http.StatusOK, `{"value":[]}`)
				if request < maxPipelinePages || tt.lastHasNext {
					response.Header.Set("X-MS-ContinuationToken", fmt.Sprintf("page-%d", request+1))
				}
				return response, nil
			})

			runs, err := client.ListInProgressPipelineRuns(context.Background())
			if tt.wantError {
				if err == nil || !strings.Contains(err.Error(), "1000 pages") {
					t.Fatalf("error = %v, want page-ceiling error", err)
				}
			} else {
				if err != nil {
					t.Fatalf("ListInProgressPipelineRuns returned error: %v", err)
				}
				if runs == nil || len(runs) != 0 {
					t.Fatalf("runs = %#v, want non-nil empty result", runs)
				}
			}
			if requests.Load() != maxPipelinePages {
				t.Fatalf("requests = %d, want exactly %d", requests.Load(), maxPipelinePages)
			}
		})
	}
}

func TestClientListInProgressPipelineRunsEnforcesRunCeiling(t *testing.T) {
	tests := []struct {
		name         string
		totalRuns    int
		wantError    bool
		wantRequests int32
	}{
		{name: "exactly at limit", totalRuns: maxPipelineRuns, wantRequests: 100},
		{name: "over limit", totalRuns: maxPipelineRuns + 1, wantError: true, wantRequests: 101},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const pageSize = 1000
			var requests atomic.Int32
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				request := int(requests.Add(1))
				startID := (request-1)*pageSize + 1
				remaining := tt.totalRuns - startID + 1
				count := pageSize
				if remaining < count {
					count = remaining
				}
				response := pipelineTestResponse(req, http.StatusOK, pipelineRunRangeListJSON(startID, count))
				if startID+count-1 < tt.totalRuns {
					response.Header.Set("X-MS-ContinuationToken", fmt.Sprintf("run-page-%d", request+1))
				}
				return response, nil
			})

			runs, err := client.ListInProgressPipelineRuns(context.Background())
			if tt.wantError {
				if err == nil || !strings.Contains(err.Error(), "100000 runs") {
					t.Fatalf("error = %v, want run-ceiling error", err)
				}
			} else {
				if err != nil {
					t.Fatalf("ListInProgressPipelineRuns returned error: %v", err)
				}
				if len(runs) != maxPipelineRuns || runs[0].ID != 1 || runs[len(runs)-1].ID != maxPipelineRuns {
					t.Fatalf("runs length/endpoints = %d/%v, want 100000 ordered runs", len(runs), pipelineRunIDsAtEnds(runs))
				}
			}
			if requests.Load() != tt.wantRequests {
				t.Fatalf("requests = %d, want %d", requests.Load(), tt.wantRequests)
			}
		})
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

func TestClientPipelineRunOperationsRejectInvalidJSON(t *testing.T) {
	validGet := pipelineRunJSON(1, 1, "inProgress", "null")
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
		for _, tc := range []struct {
			name string
			body func(string) string
		}{
			{name: "empty", body: func(string) string { return "" }},
			{name: "malformed", body: func(string) string { return "{" }},
			{name: "multiple values", body: func(valid string) string { return valid + `{}` }},
		} {
			t.Run(operation.name+"/"+tc.name, func(t *testing.T) {
				valid := validGet
				if operation.name == "list" {
					valid = `{"value":[]}`
				}
				client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
					return pipelineTestResponse(req, http.StatusOK, tc.body(valid)), nil
				})
				if err := operation.invoke(client); err == nil {
					t.Fatal("operation error = nil, want strict JSON error")
				}
			})
		}
	}
}

func TestClientListInProgressPipelineRunsRejectsInvalidTopLevelShape(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing value", body: `{}`},
		{name: "incorrectly cased value", body: `{"Value":[]}`},
		{name: "object value", body: `{"value":{}}`},
		{name: "string value", body: `{"value":"not-an-array"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineTestResponse(req, http.StatusOK, tt.body), nil
			})
			if _, err := client.ListInProgressPipelineRuns(context.Background()); err == nil {
				t.Fatal("ListInProgressPipelineRuns error = nil, want invalid shape error")
			}
		})
	}
}

func TestDecodePipelineRunListEnforcesLimitBeforeDecodingExcessItem(t *testing.T) {
	body := []byte(`{"value":[` + pipelineRunJSON(1, 1, "inProgress", "null") + `,{"id":"must-not-decode"}]}`)

	_, err := decodePipelineRunList(body, 1)
	if !errors.Is(err, errPipelineRunLimitExceeded) {
		t.Fatalf("error = %v, want run limit before decoding excess item", err)
	}
}

func TestClientListInProgressPipelineRunsValidatesEveryItem(t *testing.T) {
	tests := []struct {
		name string
		item string
	}{
		{name: "non in progress status", item: pipelineRunJSON(1, 1, "completed", `"succeeded"`)},
		{name: "zero run ID", item: pipelineRunJSON(0, 1, "inProgress", "null")},
		{name: "negative run ID", item: pipelineRunJSON(-1, 1, "inProgress", "null")},
		{name: "run ID over signed int32", item: pipelineRunJSON(2147483648, 1, "inProgress", "null")},
		{name: "zero pipeline ID", item: pipelineRunJSON(1, 0, "inProgress", "null")},
		{name: "negative pipeline ID", item: pipelineRunJSON(1, -1, "inProgress", "null")},
		{name: "pipeline ID over signed int32", item: pipelineRunJSON(1, 2147483648, "inProgress", "null")},
		{name: "blank pipeline name", item: `{"id":1,"buildNumber":"run","status":"inProgress","definition":{"id":1,"name":" \t "}}`},
		{name: "missing run number", item: `{"id":1,"status":"inProgress","definition":{"id":1,"name":"Pipeline"}}`},
		{name: "blank status", item: `{"id":1,"buildNumber":"run","status":" ","definition":{"id":1,"name":"Pipeline"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineTestResponse(req, http.StatusOK, `{"value":[`+tt.item+`]}`), nil
			})
			if _, err := client.ListInProgressPipelineRuns(context.Background()); err == nil {
				t.Fatal("ListInProgressPipelineRuns error = nil, want invalid item error")
			}
		})
	}
}

func TestClientListInProgressPipelineRunsAcceptsSignedInt32ResponseBoundaries(t *testing.T) {
	body := `{"value":[` +
		pipelineRunJSON(1, 2147483647, "inProgress", "null") + `,` +
		pipelineRunJSON(2147483647, 1, "inProgress", `"none"`) + `]}`
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		return pipelineTestResponse(req, http.StatusOK, body), nil
	})

	runs, err := client.ListInProgressPipelineRuns(context.Background())
	if err != nil {
		t.Fatalf("ListInProgressPipelineRuns returned error: %v", err)
	}
	if len(runs) != 2 || runs[0].ID != 1 || runs[0].PipelineID != 2147483647 || runs[1].ID != 2147483647 || runs[1].PipelineID != 1 {
		t.Fatalf("runs = %#v, want signed-int32 response boundaries", runs)
	}
}

func TestClientGetPipelineRunValidatesRequiredShape(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing run ID", body: `{"buildNumber":"run","status":"inProgress","definition":{"id":1,"name":"Pipeline"}}`},
		{name: "negative run ID", body: pipelineRunJSON(-1, 1, "inProgress", "null")},
		{name: "run ID over signed int32", body: pipelineRunJSON(2147483648, 1, "inProgress", "null")},
		{name: "missing definition", body: `{"id":1,"buildNumber":"run","status":"inProgress"}`},
		{name: "negative pipeline ID", body: pipelineRunJSON(1, -1, "inProgress", "null")},
		{name: "pipeline ID over signed int32", body: pipelineRunJSON(1, 2147483648, "inProgress", "null")},
		{name: "blank pipeline name", body: `{"id":1,"buildNumber":"run","status":"inProgress","definition":{"id":1,"name":" \n "}}`},
		{name: "missing run number", body: `{"id":1,"status":"inProgress","definition":{"id":1,"name":"Pipeline"}}`},
		{name: "blank run number", body: `{"id":1,"buildNumber":" \t","status":"inProgress","definition":{"id":1,"name":"Pipeline"}}`},
		{name: "missing status", body: `{"id":1,"buildNumber":"run","definition":{"id":1,"name":"Pipeline"}}`},
		{name: "blank status", body: `{"id":1,"buildNumber":"run","status":"  ","definition":{"id":1,"name":"Pipeline"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineTestResponse(req, http.StatusOK, tt.body), nil
			})
			if _, err := client.GetPipelineRun(context.Background(), 1); err == nil {
				t.Fatal("GetPipelineRun error = nil, want required-shape error")
			}
		})
	}
}

func TestClientGetPipelineRunEnforcesStatusResultConsistency(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		resultJSON string
		wantResult *string
		wantError  bool
	}{
		{name: "in progress missing result", status: "inProgress", resultJSON: ""},
		{name: "in progress null result", status: "inProgress", resultJSON: "null"},
		{name: "in progress blank result", status: "inProgress", resultJSON: `" \t "`},
		{name: "in progress none result", status: "inProgress", resultJSON: `" none "`},
		{name: "unknown status without result", status: "futureStatus", resultJSON: "null"},
		{name: "completed known result", status: "completed", resultJSON: `" succeeded "`, wantResult: pipelineString("succeeded")},
		{name: "completed unknown result", status: "completed", resultJSON: `" futureResult "`, wantResult: pipelineString("futureResult")},
		{name: "completed missing result", status: "completed", resultJSON: "", wantError: true},
		{name: "completed null result", status: "completed", resultJSON: "null", wantError: true},
		{name: "completed blank result", status: "completed", resultJSON: `"  "`, wantError: true},
		{name: "completed none result", status: "completed", resultJSON: `"none"`, wantError: true},
		{name: "non completed terminal result", status: "cancelling", resultJSON: `"failed"`, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := pipelineRunJSON(1, 1, tt.status, tt.resultJSON)
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineTestResponse(req, http.StatusOK, body), nil
			})

			run, err := client.GetPipelineRun(context.Background(), 1)
			if tt.wantError {
				if err == nil {
					t.Fatal("GetPipelineRun error = nil, want inconsistent status/result error")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetPipelineRun returned error: %v", err)
			}
			if run.Status != tt.status {
				t.Fatalf("status = %q, want exact %q", run.Status, tt.status)
			}
			if !equalPipelineString(run.Result, tt.wantResult) {
				t.Fatalf("result = %#v, want %#v", run.Result, tt.wantResult)
			}
		})
	}
}

func TestClientGetPipelineRunNormalizesOptionalSourceAndLinkStrings(t *testing.T) {
	tests := []struct {
		name         string
		optionalJSON string
		want         *string
	}{
		{name: "missing", optionalJSON: ""},
		{name: "null", optionalJSON: `,"sourceBranch":null,"sourceVersion":null,"_links":{"web":{"href":null}}`},
		{name: "empty", optionalJSON: `,"sourceBranch":"","sourceVersion":"","_links":{"web":{"href":""}}`},
		{name: "whitespace", optionalJSON: `,"sourceBranch":" \t ","sourceVersion":" \n ","_links":{"web":{"href":"  "}}`},
		{name: "trimmed", optionalJSON: `,"sourceBranch":" refs/heads/main ","sourceVersion":" abc123 ","_links":{"web":{"href":" https://example.invalid/run/1 "}}`, want: pipelineString("available")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.TrimSuffix(pipelineRunJSON(1, 1, "inProgress", "null"), "}") + tt.optionalJSON + "}"
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineTestResponse(req, http.StatusOK, body), nil
			})

			run, err := client.GetPipelineRun(context.Background(), 1)
			if err != nil {
				t.Fatalf("GetPipelineRun returned error: %v", err)
			}
			if tt.want == nil {
				if run.SourceBranch != nil || run.SourceVersion != nil || run.WebURL != nil {
					t.Fatalf("optional strings = %#v/%#v/%#v, want nil", run.SourceBranch, run.SourceVersion, run.WebURL)
				}
				return
			}
			if run.SourceBranch == nil || *run.SourceBranch != "refs/heads/main" {
				t.Fatalf("source branch = %#v, want trimmed value", run.SourceBranch)
			}
			if run.SourceVersion == nil || *run.SourceVersion != "abc123" {
				t.Fatalf("source version = %#v, want trimmed value", run.SourceVersion)
			}
			if run.WebURL == nil || *run.WebURL != "https://example.invalid/run/1" {
				t.Fatalf("web URL = %#v, want trimmed _links.web.href", run.WebURL)
			}
		})
	}
}

func TestClientGetPipelineRunNormalizesAvailableTimesToUTC(t *testing.T) {
	body := strings.TrimSuffix(pipelineRunJSON(1, 1, "completed", `"succeeded"`), "}") +
		`,"queueTime":"2026-07-15T10:09:08.1200+02:00","startTime":"2026-07-15T08:09:09Z","finishTime":"2026-07-15T08:09:10.1234000Z"}`
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		return pipelineTestResponse(req, http.StatusOK, body), nil
	})

	run, err := client.GetPipelineRun(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetPipelineRun returned error: %v", err)
	}
	wants := []string{"2026-07-15T08:09:08.12Z", "2026-07-15T08:09:09Z", "2026-07-15T08:09:10.1234Z"}
	got := []*time.Time{run.QueueTime, run.StartTime, run.FinishTime}
	for i := range wants {
		if got[i] == nil || got[i].Location() != time.UTC || got[i].Format(time.RFC3339Nano) != wants[i] {
			t.Fatalf("time %d = %#v, want %s in UTC", i, got[i], wants[i])
		}
	}
}

func TestClientGetPipelineRunRejectsInvalidTimes(t *testing.T) {
	for _, field := range []string{"queueTime", "startTime", "finishTime"} {
		t.Run(field, func(t *testing.T) {
			body := strings.TrimSuffix(pipelineRunJSON(1, 1, "inProgress", "null"), "}") +
				fmt.Sprintf(`,%q:"not-a-date-time"}`, field)
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineTestResponse(req, http.StatusOK, body), nil
			})
			if _, err := client.GetPipelineRun(context.Background(), 1); err == nil {
				t.Fatal("GetPipelineRun error = nil, want invalid date-time error")
			}
		})
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

func newPipelineTestClient(t *testing.T, transport roundTripperFunc) *Client {
	t.Helper()
	return newPipelineTestClientWithPAT(t, "", transport)
}

func newPipelineTestClientWithPAT(t *testing.T, pat string, transport roundTripperFunc) *Client {
	t.Helper()
	client, err := NewClient(&http.Client{Transport: transport}, ClientConfig{
		BaseURL: "https://dev.azure.com/org",
		Project: "Project",
		PAT:     pat,
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}

func pipelineRunJSON(id, pipelineID int, status, resultJSON string) string {
	result := ""
	if resultJSON != "" {
		result = `,"result":` + resultJSON
	}
	return fmt.Sprintf(
		`{"id":%d,"buildNumber":"run-%d","status":%q%s,"definition":{"id":%d,"name":"Pipeline"}}`,
		id,
		id,
		status,
		result,
		pipelineID,
	)
}

func pipelineRunListJSON(ids ...int) string {
	var body strings.Builder
	body.WriteString(`{"value":[`)
	for i, id := range ids {
		if i > 0 {
			body.WriteByte(',')
		}
		body.WriteString(pipelineRunJSON(id, id, "inProgress", "null"))
	}
	body.WriteString(`]}`)
	return body.String()
}

func pipelineRunRangeListJSON(startID, count int) string {
	var body strings.Builder
	body.Grow(count * 100)
	body.WriteString(`{"value":[`)
	for offset := 0; offset < count; offset++ {
		if offset > 0 {
			body.WriteByte(',')
		}
		id := startID + offset
		body.WriteString(pipelineRunJSON(id, 1, "inProgress", "null"))
	}
	body.WriteString(`]}`)
	return body.String()
}

func pipelineRunIDs(runs []PipelineRun) []int {
	ids := make([]int, len(runs))
	for i := range runs {
		ids[i] = runs[i].ID
	}
	return ids
}

func pipelineRunIDsAtEnds(runs []PipelineRun) []int {
	if len(runs) == 0 {
		return nil
	}
	return []int{runs[0].ID, runs[len(runs)-1].ID}
}

func pipelineString(value string) *string {
	return &value
}

func equalPipelineString(got, want *string) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}

func pipelineSizedResponse(req *http.Request, validJSON string, size, declaredLength int64) *http.Response {
	remaining := size - int64(len(validJSON))
	if remaining < 0 {
		remaining = 0
	}
	body := io.MultiReader(
		strings.NewReader(validJSON),
		io.LimitReader(pipelineRepeatingReader(' '), remaining),
	)
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Header:        make(http.Header),
		Body:          io.NopCloser(body),
		ContentLength: declaredLength,
		Request:       req,
	}
}

type pipelineRepeatingReader byte

func (r pipelineRepeatingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

func assertPipelineErrorOmits(t *testing.T, errorText string, markers ...string) {
	t.Helper()
	for _, marker := range markers {
		if strings.Contains(errorText, marker) {
			t.Errorf("error = %q, want marker %q suppressed", errorText, marker)
		}
	}
}

func pipelineTestResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}
