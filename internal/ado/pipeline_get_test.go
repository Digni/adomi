package ado

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

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
