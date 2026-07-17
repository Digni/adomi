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
