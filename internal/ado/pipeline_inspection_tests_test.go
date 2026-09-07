package ado

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func newInspectionSession(t *testing.T, handler http.Handler) (*pipelineInspectionSession, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	client, err := NewClient(server.Client(), ClientConfig{
		BaseURL:    server.URL,
		Project:    "Project",
		APIVersion: "7.1",
	})
	if err != nil {
		server.Close()
		t.Fatalf("NewClient returned error: %v", err)
	}
	return &pipelineInspectionSession{client: client, httpClient: server.Client()}, server.Close
}

func TestPipelineInspectionReadTestsPagesRunsAndResults(t *testing.T) {
	const buildURI = "vstfs:///Build/Build/42"
	var requests []string
	session, closeServer := newInspectionSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		if r.URL.Path == "/Project/_apis/test/runs" {
			if got := r.URL.Query().Get("buildUri"); got != buildURI {
				t.Errorf("buildUri = %q, want %q", got, buildURI)
			}
			if got := r.URL.Query().Get("includeRunDetails"); got != "true" {
				t.Errorf("includeRunDetails = %q, want true", got)
			}
			switch r.URL.Query().Get("$skip") {
			case "0":
				fmt.Fprint(w, `{"count":3,"value":[`+
					`{"id":10,"name":"first","state":"Completed","build":{"id":"42","uri":"`+buildURI+`"},"buildConfiguration":{"id":"42","uri":"`+buildURI+`"},"totalTests":2,"passedTests":1,"completedTests":2},`+
					`{"id":11,"name":"second","state":"InProgress","build":{"id":42}}]}`)
			case "2":
				fmt.Fprint(w, `{"value":[{"id":12,"name":"third","state":"Completed","build":{"id":"42"}}]}`)
			case "3":
				fmt.Fprint(w, `{"value":[]}`)
			default:
				t.Errorf("unexpected test-run offset %q", r.URL.Query().Get("$skip"))
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/Project/_apis/test/runs/") {
			if got := r.URL.Query().Get("$top"); got != "1000" {
				t.Errorf("result $top = %q, want 1000", got)
			}
			if got := r.URL.Query().Get("detailsToInclude"); got != "none" {
				t.Errorf("detailsToInclude = %q, want none", got)
			}
			runID := strings.Split(strings.TrimPrefix(r.URL.Path, "/Project/_apis/test/runs/"), "/")[0]
			switch runID + ":" + r.URL.Query().Get("$skip") {
			case "10:0":
				fmt.Fprint(w, `{"value":[{"id":1,"testRun":{"id":"10"},"testCaseTitle":"case one","automatedTestName":"A.One","state":"Completed","outcome":"Passed","durationInMs":12.5}]}`)
			case "10:1", "11:1", "12:0":
				fmt.Fprint(w, `{"value":[]}`)
			case "11:0":
				fmt.Fprint(w, `{"value":[{"id":1,"testRun":{"id":"11"},"testCaseTitle":"case two","state":"Completed","outcome":"Failed","errorMessage":"boom","stackTrace":"trace"}]}`)
			default:
				t.Errorf("unexpected result request %s", r.URL.RequestURI())
			}
			return
		}
		http.Error(w, "unexpected path", http.StatusNotFound)
	}))
	defer closeServer()

	runs, results, err := session.readTests(context.Background(), 42, buildURI)
	if err != nil {
		t.Fatalf("readTests returned error: %v", err)
	}
	if got := []int{runs[0].ID, runs[1].ID, runs[2].ID}; !reflect.DeepEqual(got, []int{10, 11, 12}) {
		t.Fatalf("run IDs = %v, want [10 11 12]", got)
	}
	if runs[0].ReportedTotalTests == nil || *runs[0].ReportedTotalTests != 2 || runs[0].CalculatedResultCount != 1 || runs[0].CalculatedOutcomeCounts["Passed"] != 1 {
		t.Fatalf("first run totals = %+v", runs[0])
	}
	if got := results[10][0]; got.TestRunID != 10 || got.AutomatedTestName != "A.One" || got.DurationInMs == nil || *got.DurationInMs != 12.5 {
		t.Fatalf("first result = %+v", got)
	}
	if got := results[11][0]; got.ID != 1 || got.TestRunID != 11 || got.ErrorMessage == nil || *got.ErrorMessage != "boom" || got.StackTrace == nil || *got.StackTrace != "trace" {
		t.Fatalf("second result = %+v", got)
	}
	if len(results[12]) != 0 || runs[2].CalculatedResultCount != 0 {
		t.Fatalf("empty third run = %+v, %+v", runs[2], results[12])
	}
	if len(requests) != 8 {
		t.Fatalf("request count = %d, want 8 (%v)", len(requests), requests)
	}
}

func TestPipelineInspectionReadTestsRejectsUnavailableOrMismatchedData(t *testing.T) {
	buildURI := "vstfs:///Build/Build/42"
	tests := []struct {
		name string
		body string
		code int
		want string
	}{
		{name: "forbidden", code: http.StatusForbidden, body: `{"message":"secret"}`, want: "HTTP status 403"},
		{name: "build mismatch", body: `{"value":[{"id":10,"build":{"id":"99"}}]}`, want: "does not match requested build"},
		{name: "build configuration URI mismatch", body: `{"value":[{"id":10,"buildConfiguration":{"id":"42","uri":"other"}}]}`, want: "build configuration URI"},
		{name: "duplicate run", body: `{"value":[{"id":10},{"id":10}]}`, want: "duplicate test run ID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session, closeServer := newInspectionSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.code != 0 {
					w.WriteHeader(test.code)
					fmt.Fprint(w, test.body)
					return
				}
				if r.URL.Path == "/Project/_apis/test/runs" && r.URL.Query().Get("$skip") != "0" {
					fmt.Fprint(w, `{"value":[]}`)
					return
				}
				fmt.Fprint(w, test.body)
			}))
			defer closeServer()
			_, _, err := session.readTests(context.Background(), 42, buildURI)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want marker %q", err, test.want)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("error leaked response body: %v", err)
			}
		})
	}
}

func TestPipelineInspectionReadTestsRejectsResultRunMismatchAndMissingBuildURI(t *testing.T) {
	session, closeServer := newInspectionSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Project/_apis/test/runs" && r.URL.Query().Get("$skip") != "0" {
			fmt.Fprint(w, `{"value":[]}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/results") {
			fmt.Fprint(w, `{"value":[{"id":1,"testRun":{"id":"999"}}]}`)
			return
		}
		fmt.Fprint(w, `{"value":[{"id":10,"build":{"id":"42"}}]}`)
	}))
	defer closeServer()
	_, _, err := session.readTests(context.Background(), 42, "vstfs:///Build/Build/42")
	if err == nil || !strings.Contains(err.Error(), "does not match requested test run") {
		t.Fatalf("error = %v, want test-run mismatch", err)
	}
	if _, _, err := session.readTests(context.Background(), 42, "  "); err == nil || !strings.Contains(err.Error(), "requires a build URI") {
		t.Fatalf("missing URI error = %v", err)
	}
}

func TestPipelineInspectionReadTestsRejectsDuplicateResultWithinRun(t *testing.T) {
	session, closeServer := newInspectionSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Project/_apis/test/runs" && r.URL.Query().Get("$skip") != "0" {
			fmt.Fprint(w, `{"value":[]}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/results") {
			fmt.Fprint(w, `{"value":[{"id":1,"testRun":{"id":"10"}},{"id":1,"testRun":{"id":"10"}}]}`)
			return
		}
		fmt.Fprint(w, `{"value":[{"id":10,"build":{"id":"42"}}]}`)
	}))
	defer closeServer()
	_, _, err := session.readTests(context.Background(), 42, "vstfs:///Build/Build/42")
	if err == nil || !strings.Contains(err.Error(), "duplicate test result ID") {
		t.Fatalf("error = %v, want duplicate result", err)
	}
}

func TestPipelineInspectionReadTestsAcceptsBuildConfigurationIdentity(t *testing.T) {
	const buildURI = "vstfs:///Build/Build/42"
	session, closeServer := newInspectionSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Project/_apis/test/runs" && r.URL.Query().Get("$skip") != "0" {
			fmt.Fprint(w, `{"value":[]}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/results") {
			fmt.Fprint(w, `{"value":[]}`)
			return
		}
		fmt.Fprint(w, `{"value":[{"id":10,"buildConfiguration":{"id":"42","uri":"`+buildURI+`"}}]}`)
	}))
	defer closeServer()
	if _, _, err := session.readTests(context.Background(), 42, buildURI); err != nil {
		t.Fatalf("readTests returned error: %v", err)
	}
}

func TestPipelineInspectionReadTestsUsesEscapedBuildURIQuery(t *testing.T) {
	const buildURI = "vstfs:///Build/Build/42?x=1&y=2"
	session, closeServer := newInspectionSession(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("$skip") != "0" {
			fmt.Fprint(w, `{"value":[]}`)
			return
		}
		if r.URL.Path != "/Project/_apis/test/runs" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("buildUri"); got != buildURI {
			t.Errorf("buildUri = %q, want %q", got, buildURI)
		}
		if _, err := url.QueryUnescape(r.URL.RawQuery); err != nil {
			t.Errorf("invalid escaped query: %v", err)
		}
		fmt.Fprint(w, `{"value":[]}`)
	}))
	defer closeServer()
	if _, _, err := session.readTests(context.Background(), 42, buildURI); err != nil {
		t.Fatalf("readTests returned error: %v", err)
	}
}
