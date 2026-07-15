package cli

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestADOPipelineListRealWiringReturnsMultiPageCompactJSON(t *testing.T) {
	const pat = "real-pipeline-pat"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := requests.Add(1)
		assertRealPipelineRequest(t, r, pat)
		if r.URL.EscapedPath() != "/Collection%20Root/My%20Project%2FArea/_apis/build/builds" {
			t.Fatalf("request %d path = %q, want encoded Build list route", request, r.URL.EscapedPath())
		}
		query := r.URL.Query()
		if query.Get("statusFilter") != "inProgress" || query.Get("queryOrder") != "queueTimeDescending" || query.Get("api-version") != "7.0" {
			t.Fatalf("request %d query = %v, want Build list query", request, query)
		}
		switch request {
		case 1:
			if query.Get("continuationToken") != "" {
				t.Fatalf("first continuation token = %q, want absent", query.Get("continuationToken"))
			}
			w.Header().Set("X-MS-ContinuationToken", "opaque token/+?=")
			fmt.Fprint(w, `{"value":[{"id":29,"buildNumber":"20260715.2","status":"inProgress","result":"none","definition":{"id":3,"name":"Deploy API"},"sourceBranch":"refs/heads/main","sourceVersion":"abc123","queueTime":"2026-07-15T08:01:02Z","startTime":"2026-07-15T08:02:03Z","finishTime":null,"_links":{"web":{"href":"https://dev.azure.com/org/project/_build/results?buildId=29"}}}]}`)
		case 2:
			if query.Get("continuationToken") != "opaque token/+?=" {
				t.Fatalf("second continuation token = %q, want exact opaque token", query.Get("continuationToken"))
			}
			fmt.Fprint(w, `{"value":[{"id":11,"buildNumber":"20260715.1","status":"inProgress","definition":{"id":2,"name":"Build API"},"sourceBranch":null,"sourceVersion":"","queueTime":null,"startTime":null,"finishTime":null,"_links":{"web":{"href":null}}}]}`)
		default:
			t.Fatalf("unexpected request %d", request)
		}
	}))
	t.Cleanup(server.Close)
	runner := pipelineRealRunner(t, server.URL+"/Collection Root", pat)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pipeline", "list", "--profile", "pipeline-profile"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := "{\"runs\":[{\"id\":29,\"pipelineId\":3,\"pipelineName\":\"Deploy API\",\"runNumber\":\"20260715.2\",\"status\":\"inProgress\",\"result\":null,\"sourceBranch\":\"refs/heads/main\",\"sourceVersion\":\"abc123\",\"queueTime\":\"2026-07-15T08:01:02Z\",\"startTime\":\"2026-07-15T08:02:03Z\",\"finishTime\":null,\"webUrl\":\"https://dev.azure.com/org/project/_build/results?buildId=29\"},{\"id\":11,\"pipelineId\":2,\"pipelineName\":\"Build API\",\"runNumber\":\"20260715.1\",\"status\":\"inProgress\",\"result\":null,\"sourceBranch\":null,\"sourceVersion\":null,\"queueTime\":null,\"startTime\":null,\"finishTime\":null,\"webUrl\":null}]}\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}
}

func TestADOPipelineGetRealWiringReturnsCompletedCompactJSON(t *testing.T) {
	const pat = "real-get-pat"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRealPipelineRequest(t, r, pat)
		if r.URL.EscapedPath() != "/Collection%20Root/My%20Project%2FArea/_apis/build/builds/2147483647" {
			t.Fatalf("path = %q, want encoded Build get route", r.URL.EscapedPath())
		}
		if r.URL.Query().Get("api-version") != "7.0" {
			t.Fatalf("query = %v, want configured API version", r.URL.Query())
		}
		fmt.Fprint(w, `{"id":2147483647,"buildNumber":"release-42","status":"completed","result":"succeeded","definition":{"id":7,"name":"Release build"},"sourceBranch":null,"sourceVersion":null,"queueTime":"2026-07-15T08:00:00+02:00","startTime":"2026-07-15T06:01:00Z","finishTime":"2026-07-15T06:02:03.4500Z","_links":{"web":{"href":null}}}`)
	}))
	t.Cleanup(server.Close)
	runner := pipelineRealRunner(t, server.URL+"/Collection Root", pat)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pipeline", "get", "2147483647"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := "{\"id\":2147483647,\"pipelineId\":7,\"pipelineName\":\"Release build\",\"runNumber\":\"release-42\",\"status\":\"completed\",\"result\":\"succeeded\",\"sourceBranch\":null,\"sourceVersion\":null,\"queueTime\":\"2026-07-15T06:00:00Z\",\"startTime\":\"2026-07-15T06:01:00Z\",\"finishTime\":\"2026-07-15T06:02:03.45Z\",\"webUrl\":null}\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestADOPipelineListLaterPageFailureLeavesStdoutEmpty(t *testing.T) {
	const (
		pat        = "LATER_PAGE_PAT_MARKER"
		bodyMarker = "LATER_PAGE_BODY_MARKER"
	)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("X-MS-ContinuationToken", "next")
			fmt.Fprint(w, `{"value":[{"id":1,"buildNumber":"run","status":"inProgress","definition":{"id":1,"name":"Pipeline"}}]}`)
			return
		}
		http.Error(w, bodyMarker+" <html>failure</html>", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	runner := pipelineRealRunner(t, server.URL, pat)
	assertRealPipelineFailure(t, runner, []string{"ado", "pipeline", "list"}, http.StatusInternalServerError, pat, bodyMarker, "<html>")
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}
}

func TestADOPipelineMalformedCompletedRunFailureLeavesStdoutEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":42,"buildNumber":"run","status":"completed","result":null,"definition":{"id":1,"name":"Pipeline"}}`)
	}))
	t.Cleanup(server.Close)
	runner := pipelineRealRunner(t, server.URL, "malformed-pat")
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pipeline", "get", "42"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "completed run requires a result") {
		t.Fatalf("error = %v, want completed-result validation error", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPipelineHTTPFailureSuppressesConfidentialDiagnostics(t *testing.T) {
	const (
		pat            = "HTTP_STATUS_PAT_MARKER"
		bodyMarker     = "HTTP_STATUS_BODY_MARKER"
		locationMarker = "HTTP_STATUS_LOCATION_MARKER"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/"+locationMarker)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, bodyMarker+" <html>forbidden</html>")
	}))
	t.Cleanup(server.Close)
	runner := pipelineRealRunner(t, server.URL, pat)
	assertRealPipelineFailure(t, runner, []string{"ado", "pipeline", "get", "42"}, http.StatusForbidden, pat, bodyMarker, locationMarker, "<html>")
}

func TestADOPipelineRedirectFailureSuppressesConfidentialDiagnostics(t *testing.T) {
	const (
		pat            = "REDIRECT_PAT_MARKER"
		locationMarker = "REDIRECT_LOCATION_MARKER"
	)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) > 1 {
			t.Fatalf("redirect followed to %s", r.URL)
		}
		http.Redirect(w, r, "/signin/"+locationMarker, http.StatusFound)
	}))
	t.Cleanup(server.Close)
	runner := pipelineRealRunner(t, server.URL, pat)
	assertRealPipelineFailure(t, runner, []string{"ado", "pipeline", "list"}, http.StatusFound, pat, locationMarker, "<a href")
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want redirect not followed", requests.Load())
	}
}

func pipelineRealRunner(t *testing.T, baseURL, pat string) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		PATStore: pipelinePATStoreFunc(func(profile string) (string, error) {
			if profile != "pipeline-ref" {
				t.Fatalf("credential ref = %q, want pipeline-ref", profile)
			}
			return pat, nil
		}),
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" || scope != config.DefaultScope {
				t.Fatalf("LoadConfig args = %q %q %q %v", repoRoot, homeDir, requestedProfile, scope)
			}
			if requestedProfile != "" && requestedProfile != "pipeline-profile" {
				t.Fatalf("requested profile = %q", requestedProfile)
			}
			return &config.Loaded{Profile: config.Profile{
				Name:       "pipeline-profile",
				PATRef:     "pipeline-ref",
				BaseURL:    baseURL,
				Project:    "My Project/Area",
				APIVersion: "7.0",
			}}, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) {
			if proxy != "" {
				t.Fatalf("proxy = %q, want empty", proxy)
			}
			return ado.NewHTTPClient(proxy)
		},
	}}
}

func assertRealPipelineRequest(t *testing.T, r *http.Request, pat string) {
	t.Helper()
	if r.Method != http.MethodGet {
		t.Fatalf("method = %q, want GET", r.Method)
	}
	if r.Header.Get("Content-Type") != "" {
		t.Fatalf("Content-Type = %q, want empty", r.Header.Get("Content-Type"))
	}
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("request body = %q, want empty", body)
		}
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+pat))
	if r.Header.Get("Authorization") != wantAuth {
		t.Fatalf("Authorization = %q, want %q", r.Header.Get("Authorization"), wantAuth)
	}
}

func assertRealPipelineFailure(t *testing.T, runner Runner, args []string, status int, markers ...string) {
	t.Helper()
	var stdout bytes.Buffer
	err := runner.Run(args, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), fmt.Sprint(status)) {
		t.Fatalf("error = %v, want safe status %d error", err, status)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	errorText := err.Error()
	for _, marker := range markers {
		authorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+marker))
		if strings.Contains(errorText, marker) || strings.Contains(errorText, authorization) {
			t.Fatalf("error = %q, want no confidential marker derived from %q", errorText, marker)
		}
	}
}
