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

func TestClientListRecentPipelineRunsBuildsRequestAndDecodesMixedStatuses(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenTop string
	var seenStatusFilterPresent bool
	var seenQueryOrder string
	var seenAPIVersion string
	var seenAuth string
	var seenContentType string
	var seenBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenTop = r.URL.Query().Get("$top")
		seenStatusFilterPresent = r.URL.Query().Has("statusFilter")
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
		fmt.Fprint(w, `{"value":[`+
			`{"id":30,"buildNumber":"20260720.3","status":"completed","result":"succeeded","definition":{"id":7,"name":"CI"},"sourceBranch":" refs/heads/feature-x ","sourceVersion":" def456 ","queueTime":"2026-07-20T10:00:00.500+02:00","startTime":"2026-07-20T08:01:00Z","finishTime":"2026-07-20T08:05:00Z","url":"https://ignored.example/build/30","_links":{"web":{"href":" https://dev.azure.com/org/project/_build/results?buildId=30 "}}},`+
			`{"id":29,"buildNumber":"20260720.2","status":"inProgress","result":"none","definition":{"id":7,"name":"CI"},"queueTime":"2026-07-20T07:59:00Z","_links":{}},`+
			`{"id":28,"buildNumber":"20260720.1","status":"notStarted","definition":{"id":8,"name":"Release"},"_links":{}}`+
			`]}`)
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

	runs, err := client.ListRecentPipelineRuns(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecentPipelineRuns returned error: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("runs length = %d, want 3", len(runs))
	}

	completed := runs[0]
	if completed.ID != 30 || completed.PipelineID != 7 || completed.PipelineName != "CI" || completed.RunNumber != "20260720.3" || completed.Status != "completed" {
		t.Fatalf("completed run = %#v, want normalized completed run", completed)
	}
	if completed.Result == nil || *completed.Result != "succeeded" {
		t.Fatalf("completed result = %#v, want succeeded", completed.Result)
	}
	if completed.SourceBranch == nil || *completed.SourceBranch != "refs/heads/feature-x" {
		t.Fatalf("source branch = %#v, want trimmed branch", completed.SourceBranch)
	}
	if completed.SourceVersion == nil || *completed.SourceVersion != "def456" {
		t.Fatalf("source version = %#v, want trimmed version", completed.SourceVersion)
	}
	if completed.QueueTime == nil || completed.QueueTime.Format("2006-01-02T15:04:05.000Z07:00") != "2026-07-20T08:00:00.500Z" {
		t.Fatalf("queue time = %#v, want UTC-normalized offset timestamp", completed.QueueTime)
	}
	if completed.FinishTime == nil {
		t.Fatal("finish time = nil, want normalized finish time")
	}
	if completed.WebURL == nil || *completed.WebURL != "https://dev.azure.com/org/project/_build/results?buildId=30" {
		t.Fatalf("web URL = %#v, want trimmed _links.web.href", completed.WebURL)
	}

	inProgress := runs[1]
	if inProgress.ID != 29 || inProgress.Status != "inProgress" || inProgress.Result != nil {
		t.Fatalf("in-progress run = %#v, want status inProgress with null result", inProgress)
	}
	notStarted := runs[2]
	if notStarted.ID != 28 || notStarted.Status != "notStarted" || notStarted.Result != nil {
		t.Fatalf("not-started run = %#v, want status notStarted with null result", notStarted)
	}

	if seenMethod != http.MethodGet || len(seenBody) != 0 || seenContentType != "" {
		t.Fatalf("request = %s body %q content-type %q, want bodyless GET", seenMethod, seenBody, seenContentType)
	}
	if seenPath != "/tfs/Default%20Collection/My%20Project%2FArea/_apis/build/builds" {
		t.Fatalf("path = %q, want escaped collection and project path", seenPath)
	}
	if seenTop != "10" || seenStatusFilterPresent || seenQueryOrder != "queueTimeDescending" || seenAPIVersion != "7.0" {
		t.Fatalf("query top/filter/order/version = %q/%v/%q/%q, want $top 10, no statusFilter, descending queue time, 7.0", seenTop, seenStatusFilterPresent, seenQueryOrder, seenAPIVersion)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
}

func TestClientListRecentPipelineRunsAcceptsPresentNullAndEmptyValue(t *testing.T) {
	for _, body := range []string{`{"value":null}`, `{"value":[]}`} {
		t.Run(body, func(t *testing.T) {
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineTestResponse(req, http.StatusOK, body), nil
			})

			runs, err := client.ListRecentPipelineRuns(context.Background(), 5)
			if err != nil {
				t.Fatalf("ListRecentPipelineRuns returned error: %v", err)
			}
			if runs == nil || len(runs) != 0 {
				t.Fatalf("runs = %#v, want non-nil empty collection", runs)
			}
		})
	}
}

func TestClientListRecentPipelineRunsReturnsFewerRunsThanRequested(t *testing.T) {
	var requests atomic.Int32
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		if got := req.URL.Query().Get("$top"); got != "10" {
			t.Fatalf("$top = %q, want 10", got)
		}
		return pipelineTestResponse(req, http.StatusOK, pipelineRecentRunListJSON(3, 2, 1)), nil
	})

	runs, err := client.ListRecentPipelineRuns(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecentPipelineRuns returned error: %v", err)
	}
	if got := pipelineRunIDs(runs); len(got) != 3 || got[0] != 3 || got[1] != 2 || got[2] != 1 {
		t.Fatalf("run IDs = %v, want received order [3 2 1]", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want exactly 1", requests.Load())
	}
}

func TestClientListRecentPipelineRunsValidatesCountBeforeTransport(t *testing.T) {
	for _, n := range []int{0, -1, 201, 2147483647} {
		t.Run(fmt.Sprintf("invalid %d", n), func(t *testing.T) {
			var requests atomic.Int32
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				requests.Add(1)
				return pipelineTestResponse(req, http.StatusOK, `{"value":[]}`), nil
			})

			if _, err := client.ListRecentPipelineRuns(context.Background(), n); err == nil {
				t.Fatalf("ListRecentPipelineRuns(%d) error = nil, want range error", n)
			}
			if requests.Load() != 0 {
				t.Fatalf("requests = %d, want no request before validation", requests.Load())
			}
		})
	}
	for _, n := range []int{1, 200} {
		t.Run(fmt.Sprintf("boundary %d", n), func(t *testing.T) {
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineTestResponse(req, http.StatusOK, `{"value":[]}`), nil
			})

			if _, err := client.ListRecentPipelineRuns(context.Background(), n); err != nil {
				t.Fatalf("ListRecentPipelineRuns(%d) returned error: %v", n, err)
			}
		})
	}
}

func TestClientListRecentPipelineRunsFollowsContinuationWithDecrementingTop(t *testing.T) {
	opaqueToken := " 01+/%?=&opaque "
	var requests atomic.Int32
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch request := int(requests.Add(1)); request {
		case 1:
			if got := req.URL.Query().Get("$top"); got != "10" {
				t.Fatalf("request 1 $top = %q, want 10", got)
			}
			if got := req.URL.Query().Get("continuationToken"); got != "" {
				t.Fatalf("request 1 continuationToken = %q, want absent", got)
			}
			return pipelineRecentPageResponse(req, pipelineRecentRunListJSON(60, 59, 58, 57, 56, 55), opaqueToken), nil
		case 2:
			if got := req.URL.Query().Get("$top"); got != "4" {
				t.Fatalf("request 2 $top = %q, want remaining budget 4", got)
			}
			if got := req.URL.Query().Get("continuationToken"); got != opaqueToken {
				t.Fatalf("request 2 continuationToken = %q, want exact opaque token", got)
			}
			return pipelineRecentPageResponse(req, pipelineRecentRunListJSON(54, 53, 52, 51), "another-token"), nil
		default:
			t.Fatalf("unexpected request %d; traversal must stop once 10 runs accumulated", request)
			return nil, nil
		}
	})

	runs, err := client.ListRecentPipelineRuns(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecentPipelineRuns returned error: %v", err)
	}
	want := []int{60, 59, 58, 57, 56, 55, 54, 53, 52, 51}
	got := pipelineRunIDs(runs)
	if len(got) != len(want) {
		t.Fatalf("run IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("run IDs = %v, want received order %v", got, want)
		}
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want exactly 2", requests.Load())
	}
}

func TestClientListRecentPipelineRunsStopsAtRequestedCountOnFirstPage(t *testing.T) {
	var requests atomic.Int32
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		return pipelineRecentPageResponse(req, pipelineRecentRunListJSON(9, 8), "unconsumed-token"), nil
	})

	runs, err := client.ListRecentPipelineRuns(context.Background(), 2)
	if err != nil {
		t.Fatalf("ListRecentPipelineRuns returned error: %v", err)
	}
	if got := pipelineRunIDs(runs); len(got) != 2 || got[0] != 9 || got[1] != 8 {
		t.Fatalf("run IDs = %v, want [9 8]", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want no request beyond the accumulated count", requests.Load())
	}
}

func TestClientListRecentPipelineRunsFailsWhenPageExceedsRequestedTop(t *testing.T) {
	var requests atomic.Int32
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		return pipelineTestResponse(req, http.StatusOK, pipelineRecentRunListJSON(4, 3, 2, 1)), nil
	})

	_, err := client.ListRecentPipelineRuns(context.Background(), 3)
	if err == nil || !strings.Contains(err.Error(), "exceeds the requested top") {
		t.Fatalf("error = %v, want requested-top violation", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want exactly 1", requests.Load())
	}
}

func TestClientListRecentPipelineRunsRejectsInvalidContinuationTokens(t *testing.T) {
	tests := []struct {
		name  string
		pages [][]string
		want  string
	}{
		{name: "blank token", pages: [][]string{{"   "}}, want: "blank continuation token"},
		{name: "multiple tokens", pages: [][]string{{"one", "two"}}, want: "multiple continuation tokens"},
		{name: "cyclic token", pages: [][]string{{"cycle"}, {"cycle"}}, want: "continuation token repeated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				page := int(requests.Add(1)) - 1
				if page >= len(tt.pages) {
					t.Fatalf("unexpected request %d", page+1)
				}
				return pipelineRecentPageResponse(req, pipelineRecentRunListJSON(page+1), tt.pages[page]...), nil
			})

			_, err := client.ListRecentPipelineRuns(context.Background(), 10)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestClientListRecentPipelineRunsRejectsDuplicateRunIDsAcrossPages(t *testing.T) {
	var requests atomic.Int32
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch request := int(requests.Add(1)); request {
		case 1:
			return pipelineRecentPageResponse(req, pipelineRecentRunListJSON(5, 4), "next"), nil
		case 2:
			return pipelineTestResponse(req, http.StatusOK, pipelineRecentRunListJSON(3, 5)), nil
		default:
			t.Fatalf("unexpected request %d", request)
			return nil, nil
		}
	})

	_, err := client.ListRecentPipelineRuns(context.Background(), 10)
	if err == nil || !strings.Contains(err.Error(), "duplicate run ID 5") {
		t.Fatalf("error = %v, want duplicate run ID failure", err)
	}
}

func TestClientListRecentPipelineRunsRejectsInvalidResponseShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "omitted value", body: `{}`, want: "missing value"},
		{name: "malformed JSON", body: `not json`, want: "invalid JSON"},
		{name: "empty body", body: ``, want: "invalid JSON"},
		{name: "multiple JSON values", body: `{"value":[]} {}`, want: "exactly one JSON value"},
		{name: "completed without result", body: `{"value":[{"id":1,"buildNumber":"run-1","status":"completed","definition":{"id":1,"name":"Pipeline"}}]}`, want: "completed run requires a result"},
		{name: "non-completed with result", body: `{"value":[{"id":1,"buildNumber":"run-1","status":"inProgress","result":"failed","definition":{"id":1,"name":"Pipeline"}}]}`, want: "non-completed run must not contain a result"},
		{name: "invalid timestamp", body: `{"value":[{"id":1,"buildNumber":"run-1","status":"notStarted","definition":{"id":1,"name":"Pipeline"},"queueTime":"not-a-time"}]}`, want: "queue time is invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineTestResponse(req, http.StatusOK, tt.body), nil
			})

			_, err := client.ListRecentPipelineRuns(context.Background(), 10)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestClientListRecentPipelineRunsPreservesUnknownConsistentStatusAndResult(t *testing.T) {
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		return pipelineTestResponse(req, http.StatusOK, `{"value":[`+
			`{"id":2,"buildNumber":"run-2","status":"cancelling","definition":{"id":1,"name":"Pipeline"}},`+
			`{"id":1,"buildNumber":"run-1","status":"completed","result":" customResult ","definition":{"id":1,"name":"Pipeline"}}`+
			`]}`), nil
	})

	runs, err := client.ListRecentPipelineRuns(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecentPipelineRuns returned error: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("runs length = %d, want 2", len(runs))
	}
	if runs[0].Status != "cancelling" || runs[0].Result != nil {
		t.Fatalf("cancelling run = %#v, want preserved unknown status with null result", runs[0])
	}
	if runs[1].Result == nil || *runs[1].Result != "customResult" {
		t.Fatalf("completed result = %#v, want trimmed preserved unknown result", runs[1].Result)
	}
}

func TestClientListRecentPipelineRunsSuppressesConfidentialDiagnostics(t *testing.T) {
	const patMarker = "recent-secret-pat-marker"
	const bodyMarker = "recent-response-body-marker"
	const locationMarker = "recent-redirect-location-marker"
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError, http.StatusFound} {
		t.Run(fmt.Sprintf("status %d", status), func(t *testing.T) {
			client := newPipelineTestClientWithPAT(t, patMarker, func(req *http.Request) (*http.Response, error) {
				resp := pipelineTestResponse(req, status, "<html>"+bodyMarker+patMarker+"</html>")
				if status == http.StatusFound {
					resp.Header.Set("Location", locationMarker)
				}
				return resp, nil
			})

			_, err := client.ListRecentPipelineRuns(context.Background(), 10)
			if err == nil {
				t.Fatal("error = nil, want HTTP failure")
			}
			assertPipelineErrorOmits(t, err.Error(), patMarker, bodyMarker, locationMarker, "<html>")
		})
	}
}

func TestClientListRecentPipelineRunsEnforcesResponseSizeLimit(t *testing.T) {
	tests := []struct {
		name           string
		size           int64
		declaredLength int64
		wantError      bool
	}{
		{name: "exactly at limit", size: maxPipelineResponseBytes, declaredLength: maxPipelineResponseBytes, wantError: false},
		{name: "declared over limit", size: 64, declaredLength: maxPipelineResponseBytes + 1, wantError: true},
		{name: "streamed over limit", size: maxPipelineResponseBytes + 1, declaredLength: -1, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				return pipelineSizedResponse(req, `{"value":[]}`, tt.size, tt.declaredLength), nil
			})

			_, err := client.ListRecentPipelineRuns(context.Background(), 10)
			if tt.wantError && err == nil {
				t.Fatal("error = nil, want size-limit failure")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("ListRecentPipelineRuns returned error: %v", err)
			}
		})
	}
}

func pipelineRecentPageResponse(req *http.Request, body string, continuationTokens ...string) *http.Response {
	resp := pipelineTestResponse(req, http.StatusOK, body)
	for _, token := range continuationTokens {
		resp.Header.Add("X-MS-ContinuationToken", token)
	}
	return resp
}

func pipelineRecentRunListJSON(ids ...int) string {
	body := `{"value":[`
	for i, id := range ids {
		if i > 0 {
			body += ","
		}
		body += pipelineRunJSON(id, id, "completed", `"succeeded"`)
	}
	return body + `]}`
}
