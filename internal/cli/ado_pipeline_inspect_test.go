package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestADOPipelineInspectHTTPToCLIToFilesystem(t *testing.T) {
	root := t.TempDir()
	var logAccept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Project/_apis/build/builds/42" {
			fmt.Fprint(w, `{"id":42,"uri":"vstfs:///Build/Build/42","buildNumber":"2026.09.07.42","status":"completed","result":"succeeded","definition":{"id":7,"name":"CI"},"sourceBranch":"refs/heads/main","sourceVersion":"abc","queueTime":"2026-09-07T10:00:00Z","_links":{"web":{"href":"https://dev.azure.com/org/Project/_build/results?buildId=42"}}}`)
			return
		}
		if r.URL.Path == "/Project/_apis/build/builds/42/timeline" {
			fmt.Fprint(w, `{"id":"11111111-1111-1111-1111-111111111111","records":[{"id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","type":"Stage","name":"stage","state":"completed","result":"succeeded"},{"id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","type":"Job","name":"job","state":"completed"},{"id":"cccccccc-cccc-cccc-cccc-cccccccccccc","type":"Task","name":"test","state":"completed","result":"failed","log":{"id":7}}]}`)
			return
		}
		if r.URL.Path == "/Project/_apis/build/builds/42/logs/7" {
			logAccept = r.Header.Get("Accept")
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "test log\n")
			return
		}
		if r.URL.Path == "/Project/_apis/test/runs" {
			if r.URL.Query().Get("buildUri") != "vstfs:///Build/Build/42" || r.URL.Query().Get("includeRunDetails") != "true" {
				t.Errorf("test-run query = %v", r.URL.Query())
			}
			if r.URL.Query().Get("$skip") == "0" {
				fmt.Fprint(w, `{"value":[{"id":10,"name":"tests","state":"Completed","build":{"id":"42","uri":"vstfs:///Build/Build/42"},"totalTests":1,"passedTests":0,"completedTests":1}]}`)
			} else {
				fmt.Fprint(w, `{"value":[]}`)
			}
			return
		}
		if r.URL.Path == "/Project/_apis/test/runs/10/results" {
			if r.URL.Query().Get("$top") != "1000" {
				t.Errorf("result query = %v", r.URL.Query())
			}
			if r.URL.Query().Get("$skip") == "0" {
				fmt.Fprint(w, `{"value":[{"id":100,"testRun":{"id":"10"},"testCaseTitle":"fails","automatedTestName":"Tests.Fails","state":"Completed","outcome":"Failed","durationInMs":20,"errorMessage":"failure","stackTrace":"stack"}]}`)
			} else {
				fmt.Fprint(w, `{"value":[]}`)
			}
			return
		}
		http.Error(w, "unexpected request", http.StatusNotFound)
	}))
	defer server.Close()

	runner := inspectionCLIRunner(t, root, server.URL)
	var stdout, stderr bytes.Buffer
	if err := runner.Run([]string{"ado", "pipeline", "inspect", "42", "--profile", "ci", "--global", "--json"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, want := range []string{"Inspecting Azure DevOps pipeline run 42", "3 timeline records", "1 failed task logs", "1 test runs", "1 test results"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	var result struct {
		Path            string `json:"path"`
		RunID           int    `json:"runId"`
		TimelineRecords int    `json:"timelineRecords"`
		FailedTaskLogs  int    `json:"failedTaskLogs"`
		TestRuns        int    `json:"testRuns"`
		TestResults     int    `json:"testResults"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout = %q: %v", stdout.String(), err)
	}
	wantJSON := fmt.Sprintf(`{"path":%q,"runId":42,"timelineRecords":3,"failedTaskLogs":1,"testRuns":1,"testResults":1}`+"\n", result.Path)
	if stdout.String() != wantJSON {
		t.Fatalf("stdout = %q, want compact exact result %q", stdout.String(), wantJSON)
	}
	if result.RunID != 42 || result.TimelineRecords != 3 || result.FailedTaskLogs != 1 || result.TestRuns != 1 || result.TestResults != 1 {
		t.Fatalf("JSON result = %+v", result)
	}
	for _, relative := range []string{"index.json", "run.json", "timeline.json", "logs/7.txt", "tests/runs.json", "tests/10/results.json"} {
		if _, err := os.Stat(filepath.Join(result.Path, relative)); err != nil {
			t.Fatalf("missing %s: %v", relative, err)
		}
	}
	if logAccept != "text/plain" {
		t.Fatalf("log Accept = %q, want text/plain", logAccept)
	}
	logBody, err := os.ReadFile(filepath.Join(result.Path, "logs/7.txt"))
	if err != nil || string(logBody) != "test log\n" {
		t.Fatalf("log body = %q, err=%v", logBody, err)
	}
	timeline, err := os.ReadFile(filepath.Join(result.Path, "timeline.json"))
	if err != nil {
		t.Fatalf("read timeline: %v", err)
	}
	for _, id := range []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "cccccccc-cccc-cccc-cccc-cccccccccccc"} {
		if !strings.Contains(string(timeline), id) {
			t.Fatalf("timeline = %s, missing record %s", timeline, id)
		}
	}
	if !strings.Contains(string(timeline), `"result": "failed"`) {
		t.Fatalf("timeline = %s, want failed task result", timeline)
	}
	results, err := os.ReadFile(filepath.Join(result.Path, "tests/10/results.json"))
	if err != nil {
		t.Fatalf("read results: %v", err)
	}
	for _, marker := range []string{`"outcome": "Failed"`, `"errorMessage": "failure"`, `"stackTrace": "stack"`} {
		if !strings.Contains(string(results), marker) {
			t.Fatalf("results = %s, missing %s", results, marker)
		}
	}
	var plainStdout, plainStderr bytes.Buffer
	if err := runner.Run([]string{"ado", "pipeline", "inspect", "42"}, strings.NewReader(""), &plainStdout, &plainStderr); err != nil {
		t.Fatalf("plain Run returned error: %v", err)
	}
	if !strings.HasSuffix(plainStdout.String(), "\n") || strings.Contains(plainStdout.String(), "{\"path\"") {
		t.Fatalf("plain stdout = %q, want only a path line", plainStdout.String())
	}
}

func TestADOPipelineInspectFailurePreservesPriorSnapshotAndStdout(t *testing.T) {
	root := t.TempDir()
	prior := filepath.Join(root, ".adomi", "context", "pipelines", "42", "prior")
	if err := os.MkdirAll(prior, 0o755); err != nil {
		t.Fatal(err)
	}
	priorFile := filepath.Join(prior, "index.json")
	if err := os.WriteFile(priorFile, []byte("prior"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Project/_apis/build/builds/42":
			fmt.Fprint(w, `{"id":42,"uri":"vstfs:///Build/Build/42","buildNumber":"42","status":"completed","result":"succeeded","definition":{"id":7,"name":"CI"}}`)
		case "/Project/_apis/build/builds/42/timeline":
			fmt.Fprint(w, `{"id":"11111111-1111-1111-1111-111111111111","records":[]}`)
		case "/Project/_apis/test/runs":
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "forbidden remote body")
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	runner := inspectionCLIRunner(t, root, server.URL)
	var stdout bytes.Buffer
	err := runner.Run([]string{"ado", "pipeline", "inspect", "42"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "HTTP status 403") {
		t.Fatalf("error = %v, want safe 403 error", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if content, err := os.ReadFile(priorFile); err != nil || string(content) != "prior" {
		t.Fatalf("prior snapshot = %q, err=%v", content, err)
	}
}

func TestADOPipelineInspectRejectsFlagsBeforeCredentials(t *testing.T) {
	for _, args := range [][]string{
		{"ado", "pipeline", "inspect"},
		{"ado", "pipeline", "inspect", "0"},
		{"ado", "pipeline", "inspect", "42", "--branch", "main"},
		{"ado", "pipeline", "inspect", "42", "--profile", " \t"},
		{"ado", "pipeline", "inspect", "42", "--json", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout bytes.Buffer
			runner := pipelineFailOnDependencyRunner(t)
			if err := runner.Run(args, strings.NewReader(""), &stdout, &bytes.Buffer{}); err == nil {
				t.Fatal("Run returned nil, want argument error")
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPipelineInspectHelpIsSideEffectFree(t *testing.T) {
	stderr := pipelineRunHelp(t, pipelineFailOnDependencyRunner(t), []string{"ado", "pipeline", "inspect", "--help"})
	for _, want := range []string{"adomi ado pipeline inspect", "vso.build", "vso.test", "1000", "128 MiB", "100000", "--json"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
}

func inspectionCLIRunner(t *testing.T, root, baseURL string) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		PATStore:     pipelinePATStoreFunc(func(string) (string, error) { return "pat", nil }),
		Getwd:        func() (string, error) { return root, nil },
		UserHomeDir:  func() (string, error) { return filepath.Join(root, "home"), nil },
		FindRepoRoot: func(string) (string, error) { return root, nil },
		LoadConfig: func(string, string, string, config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{Name: "ci", PATRef: "pat", BaseURL: baseURL, Project: "Project", APIVersion: "7.1"}}, nil
		},
		NewHTTPClient: func(string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient: func(httpClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			return ado.NewClient(httpClient, cfg)
		},
	}}
}
