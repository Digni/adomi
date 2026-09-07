package ado

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const (
	inspectionRootTimelineID   = "11111111-1111-1111-1111-111111111111"
	inspectionDetailTimelineID = "22222222-2222-2222-2222-222222222222"
	inspectionRootRecordID     = "33333333-3333-3333-3333-333333333333"
	inspectionSecondRecordID   = "44444444-4444-4444-4444-444444444444"
	inspectionDetailRecordID   = "55555555-5555-5555-5555-555555555555"
	inspectionNoLogRecordID    = "66666666-6666-6666-6666-666666666666"
	inspectionUnknownRecordID  = "77777777-7777-7777-7777-777777777777"
)

func TestInspectPipelineRunReadsCurrentTimelinesAndDeduplicatesFailedLogs(t *testing.T) {
	var logRequests int
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/org/Project/_apis/build/builds/42":
			return pipelineTestResponse(req, http.StatusOK, `{"id":42,"uri":"vstfs:///Build/Build/42","buildNumber":"run-42","status":"inProgress","definition":{"id":7,"name":"Pipeline"}}`), nil
		case req.URL.Path == "/org/Project/_apis/build/builds/42/timeline":
			return pipelineTestResponse(req, http.StatusOK, fmt.Sprintf(`{"id":%q,"records":[{"id":%q,"type":"Task","name":"compile","result":"failed","log":{"id":7},"details":{"id":%q}},{"id":%q,"type":"Task","name":"same log","result":"failed","log":{"id":7}},{"id":%q,"type":"Task","name":"skipped","result":"skipped","log":{"id":8}},{"id":%q,"type":"Task","name":"no log","result":"failed"},{"id":%q,"type":"mystery","name":"pending","state":"pending","result":"future"}]}`, inspectionRootTimelineID, inspectionRootRecordID, inspectionDetailTimelineID, inspectionSecondRecordID, inspectionDetailRecordID, inspectionNoLogRecordID, inspectionUnknownRecordID)), nil
		case req.URL.Path == "/org/Project/_apis/build/builds/42/timeline/"+inspectionDetailTimelineID:
			return pipelineTestResponse(req, http.StatusOK, fmt.Sprintf(`{"id":%q,"records":[{"id":%q,"type":"Job","state":"completed","previousAttempts":[{"recordId":%q,"attempt":1,"timelineId":%q}]}]}`, inspectionDetailTimelineID, inspectionDetailRecordID, inspectionRootRecordID, inspectionDetailTimelineID)), nil
		case req.URL.Path == "/org/Project/_apis/build/builds/42/logs/7":
			logRequests++
			if got := req.Header.Get("Accept"); got != "text/plain" {
				t.Fatalf("log Accept = %q, want text/plain", got)
			}
			return pipelineTestResponse(req, http.StatusOK, "compiler output\n"), nil
		case req.URL.Path == "/org/Project/_apis/test/runs":
			return pipelineTestResponse(req, http.StatusOK, `{"value":[]}`), nil
		default:
			return nil, fmt.Errorf("unexpected request path %s", req.URL.Path)
		}
	})

	inspection, err := client.InspectPipelineRun(context.Background(), 42)
	if err != nil {
		t.Fatalf("InspectPipelineRun returned error: %v", err)
	}
	if inspection.BuildURI != "vstfs:///Build/Build/42" || len(inspection.Timelines) != 2 {
		t.Fatalf("inspection identity/timelines = %#v, want build URI and root/detail timelines", inspection)
	}
	if logRequests != 1 || string(inspection.Logs[7]) != "compiler output\n" {
		t.Fatalf("logs = %#v, requests = %d, want one shared downloaded log", inspection.Logs, logRequests)
	}
	if got := inspection.Timelines[0].Records[2].LogAvailability; got != "notRequested" {
		t.Fatalf("skipped task log availability = %q, want notRequested", got)
	}
	if got := inspection.Timelines[0].Records[3].LogAvailability; got != "notPublished" {
		t.Fatalf("failed task without log availability = %q, want notPublished", got)
	}
	if got := inspection.Timelines[0].Records[4].Result; got == nil || *got != "future" {
		t.Fatalf("unknown record result = %#v, want preserved value", got)
	}
	if got := inspection.Timelines[1].Records[0].PreviousAttempts[0].TimelineID; got == nil || *got != inspectionDetailTimelineID {
		t.Fatalf("previous attempt timeline ID = %#v, want retained reference", got)
	}
}

func TestInspectPipelineRunRejectsReturnedDetailTimelineIdentityMismatch(t *testing.T) {
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/org/Project/_apis/build/builds/42":
			return pipelineTestResponse(req, http.StatusOK, `{"id":42,"uri":"build-uri","buildNumber":"run","status":"inProgress","definition":{"id":1,"name":"Pipeline"}}`), nil
		case "/org/Project/_apis/build/builds/42/timeline":
			return pipelineTestResponse(req, http.StatusOK, fmt.Sprintf(`{"id":%q,"records":[{"id":%q,"type":"Job","details":{"id":%q}}]}`, inspectionRootTimelineID, inspectionRootRecordID, inspectionDetailTimelineID)), nil
		case "/org/Project/_apis/build/builds/42/timeline/" + inspectionDetailTimelineID:
			return pipelineTestResponse(req, http.StatusOK, fmt.Sprintf(`{"id":%q,"records":[]}`, inspectionRootTimelineID)), nil
		default:
			return nil, fmt.Errorf("unexpected request path %s", req.URL.Path)
		}
	})

	_, err := client.InspectPipelineRun(context.Background(), 42)
	if err == nil || !strings.Contains(err.Error(), "does not match requested ID") {
		t.Fatalf("InspectPipelineRun error = %v, want returned timeline identity error", err)
	}
}

func TestInspectPipelineRunRequiresBuildURI(t *testing.T) {
	var requests int
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/org/Project/_apis/build/builds/42" {
			return nil, fmt.Errorf("unexpected request path %s", req.URL.Path)
		}
		return pipelineTestResponse(req, http.StatusOK, `{"id":42,"buildNumber":"run","status":"inProgress","definition":{"id":1,"name":"Pipeline"}}`), nil
	})

	_, err := client.InspectPipelineRun(context.Background(), 42)
	if err == nil || !strings.Contains(err.Error(), "missing build URI") {
		t.Fatalf("InspectPipelineRun error = %v, want missing URI error", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want build-only read before URI validation", requests)
	}
}

func TestPipelineInspectionSessionStopsBeforeRequestAndAggregateBytesLimit(t *testing.T) {
	requests := 0
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		requests++
		return pipelineTestResponse(req, http.StatusOK, `{"value":[]}`), nil
	})
	session := &pipelineInspectionSession{client: client, httpClient: client.httpClient, budgets: pipelineInspectionBudgets{requests: maxInspectionRequests}}
	if _, err := session.get(context.Background(), "https://dev.azure.com/org/Project/_apis/test/runs", "testing request budget", "application/json"); err == nil || !strings.Contains(err.Error(), "HTTP requests") {
		t.Fatalf("request budget error = %v, want bounded request error", err)
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want no request after budget exhaustion", requests)
	}
	session.budgets = pipelineInspectionBudgets{bytes: maxInspectionBytes}
	if _, err := session.get(context.Background(), "https://dev.azure.com/org/Project/_apis/test/runs", "testing byte budget", "application/json"); err == nil || !strings.Contains(err.Error(), "response bytes") {
		t.Fatalf("byte budget error = %v, want bounded byte error", err)
	}
	firstBody := `{"value":[]}`
	session.budgets = pipelineInspectionBudgets{bytes: maxInspectionBytes - int64(len(firstBody)) - 1}
	if body, err := session.get(context.Background(), "https://dev.azure.com/org/Project/_apis/test/runs", "testing remaining byte budget", "application/json"); err != nil || string(body) != firstBody {
		t.Fatalf("first near-limit read = %q, %v, want successful bounded read", body, err)
	}
	if _, err := session.get(context.Background(), "https://dev.azure.com/org/Project/_apis/test/runs", "testing exhausted remaining bytes", "application/json"); err == nil || !strings.Contains(err.Error(), "response exceeds maximum size") {
		t.Fatalf("second near-limit read error = %v, want response limit before success", err)
	}
}

func TestInspectPipelineRunDoesNotEchoForbiddenLogBody(t *testing.T) {
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/org/Project/_apis/build/builds/42":
			return pipelineTestResponse(req, http.StatusOK, `{"id":42,"uri":"build-uri","buildNumber":"run","status":"inProgress","definition":{"id":1,"name":"Pipeline"}}`), nil
		case "/org/Project/_apis/build/builds/42/timeline":
			return pipelineTestResponse(req, http.StatusOK, fmt.Sprintf(`{"id":%q,"records":[{"id":%q,"type":"Task","result":"failed","log":{"id":7}}]}`, inspectionRootTimelineID, inspectionRootRecordID)), nil
		case "/org/Project/_apis/build/builds/42/logs/7":
			return pipelineTestResponse(req, http.StatusForbidden, "secret remote body"), nil
		default:
			return nil, fmt.Errorf("unexpected request path %s", req.URL.Path)
		}
	})

	_, err := client.InspectPipelineRun(context.Background(), 42)
	if err == nil || !strings.Contains(err.Error(), "HTTP status 403") || strings.Contains(err.Error(), "secret remote body") {
		t.Fatalf("InspectPipelineRun error = %v, want safe forbidden diagnostic", err)
	}
}

func TestDecodeInspectionCollectionRejectsNullAndChecksLimitBeforeDecodingExcess(t *testing.T) {
	if _, err := decodeInspectionCollection[map[string]any]([]byte(`{"value":null}`), "value", 10, "testing collection"); err == nil {
		t.Fatal("decodeInspectionCollection accepted null collection")
	}
	if _, err := decodeInspectionCollection[map[string]any]([]byte(`{"value":[{"id":1},7]}`), "value", 1, "testing collection"); err == nil || !strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("decodeInspectionCollection error = %v, want pre-decode collection limit error", err)
	}
}
