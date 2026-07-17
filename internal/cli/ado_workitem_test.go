package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestADOWorkItemCommentCreatesCommentFromInlineMessage(t *testing.T) {
	var requestCount int
	var seenMethod, seenPath, seenAPIVersion, seenAuth string
	var seenBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"commentId":55,"workItemId":12345,"url":"https://dev.azure.com/my-org/MyProject/_workitems/edit/12345#comment-55"}`)
	}))
	t.Cleanup(server.Close)
	runner := workItemCommentHTTPRunner(t, server.Client(), server.URL)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "comment", "12345", "--message", "- Done", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "55\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	if seenMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", seenMethod)
	}
	if seenPath != "/MyProject/_apis/wit/workitems/12345/comments" {
		t.Fatalf("path = %q, want work item comments endpoint", seenPath)
	}
	if seenAPIVersion != "7.0-preview.3" {
		t.Fatalf("api-version = %q, want preview comments API", seenAPIVersion)
	}
	if seenAuth == "" {
		t.Fatal("Authorization header is empty")
	}
	if seenBody["text"] != "- Done" {
		t.Fatalf("request text = %q, want message", seenBody["text"])
	}
}

func TestADOWorkItemCommentCreatesCommentFromMessageFile(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "comment.md")
	if err := os.WriteFile(messagePath, []byte("Done from file\n"), 0o644); err != nil {
		t.Fatalf("writing comment: %v", err)
	}
	var seenBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"id":56,"workItemId":12345}`)
	}))
	t.Cleanup(server.Close)
	runner := workItemCommentHTTPRunner(t, server.Client(), server.URL)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "comment", "12345", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "56\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
	if seenBody["text"] != "Done from file\n" {
		t.Fatalf("request text = %q, want file content", seenBody["text"])
	}
}

func TestADOWorkItemCommentAliasCreatesComment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":57,"workItemId":12345}`)
	}))
	t.Cleanup(server.Close)
	runner := workItemCommentHTTPRunner(t, server.Client(), server.URL)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "work-item", "comment", "12345", "--message", "Done"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "57\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
}

func TestADOWorkItemCommentJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":58,"workItemId":12345,"url":"https://dev.azure.com/my-org/MyProject/_workitems/edit/12345#comment-58"}`)
	}))
	t.Cleanup(server.Close)
	runner := workItemCommentHTTPRunner(t, server.Client(), server.URL)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "comment", "12345", "--message", "Done", "--json"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.HasSuffix(stdout.String(), "\n") || strings.HasSuffix(strings.TrimSuffix(stdout.String(), "\n"), "\n") {
		t.Fatalf("stdout = %q, want exactly one trailing newline", stdout.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if payload["workItemId"] != float64(12345) || payload["commentId"] != float64(58) || payload["action"] != "commented" || payload["url"] == "" {
		t.Fatalf("payload = %+v, want work item comment summary", payload)
	}
}

func TestADOWorkItemCommentGlobalUsesGlobalConfigScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":60,"workItemId":12345}`)
	}))
	t.Cleanup(server.Close)
	runner := workItemCommentHTTPRunnerWithScope(t, server.Client(), server.URL, config.GlobalScope)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "comment", "12345", "--message", "Done", "--global", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "60\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
}

func TestADOWorkItemCommentRejectsInvalidArgsBeforeCredentials(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing ID", args: []string{"ado", "comment"}, want: "usage"},
		{name: "non integer ID", args: []string{"ado", "comment", "abc", "--message", "x"}, want: "work item ID"},
		{name: "zero ID", args: []string{"ado", "comment", "0", "--message", "x"}, want: "positive"},
		{name: "negative ID", args: []string{"ado", "comment", "-1", "--message", "x"}, want: "positive"},
		{name: "missing message source", args: []string{"ado", "comment", "12345"}, want: "--message"},
		{name: "conflicting message sources", args: []string{"ado", "comment", "12345", "--message", "x", "--message-file", "comment.md"}, want: "exactly one"},
		{name: "blank inline message", args: []string{"ado", "comment", "12345", "--message", " \t"}, want: "--message cannot be empty"},
		{name: "missing message value", args: []string{"ado", "comment", "12345", "--message"}, want: "--message requires a value"},
		{name: "missing message-file value", args: []string{"ado", "comment", "12345", "--message-file"}, want: "--message-file requires a value"},
		{name: "missing profile value", args: []string{"ado", "comment", "12345", "--message", "x", "--profile"}, want: "--profile requires a value"},
		{name: "unknown flag", args: []string{"ado", "comment", "12345", "--message", "x", "--unknown"}, want: "unknown"},
		{name: "unsupported work item command", args: []string{"ado", "work-item", "update", "12345"}, want: "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := workItemCommentFailFastRunner(t)
			var stdout bytes.Buffer
			err := runner.Run(tt.args, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOWorkItemCommentRejectsEmptyMessageFileBeforeNetwork(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
	}{
		{name: "empty", content: nil},
		{name: "blank", content: []byte(" \n\t")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messagePath := filepath.Join(t.TempDir(), "comment.md")
			if err := os.WriteFile(messagePath, tt.content, 0o644); err != nil {
				t.Fatalf("writing comment: %v", err)
			}
			runner := workItemCommentFailFastRunner(t)
			var stdout bytes.Buffer
			err := runner.Run([]string{"ado", "comment", "12345", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), "--message-file cannot be empty") {
				t.Fatalf("error = %v, want empty message-file", err)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOWorkItemCommentAzureDevOpsFailuresLeaveStdoutEmpty(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "write failure", status: http.StatusUnauthorized, body: "nope", want: "401"},
		{name: "malformed response", status: http.StatusOK, body: `{"id":`, want: "decoding Azure DevOps work item comment"},
		{name: "empty response", status: http.StatusOK, body: ``, want: "decoding Azure DevOps work item comment"},
		{name: "missing comment ID", status: http.StatusOK, body: `{"workItemId":12345}`, want: "missing comment ID"},
		{name: "mismatched work item ID", status: http.StatusOK, body: `{"id":59,"workItemId":999}`, want: "does not match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			}))
			t.Cleanup(server.Close)
			runner := workItemCommentHTTPRunner(t, server.Client(), server.URL)
			var stdout bytes.Buffer

			err := runner.Run([]string{"ado", "comment", "12345", "--message", "Done"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOWorkItemCommentNetworkErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakeWorkItemCommentClient{createErr: errors.New("network down")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "comment", "12345", "--message", "Done"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("error = %v, want network error", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.createCalled != 1 || client.createOpts.WorkItemID != 12345 || client.createOpts.Text != "Done" {
		t.Fatalf("create called/opts = %d/%+v, want work item comment request", client.createCalled, client.createOpts)
	}
}

func workItemCommentHTTPRunner(t *testing.T, httpClient *http.Client, baseURL string) Runner {
	t.Helper()
	return workItemCommentHTTPRunnerWithScope(t, httpClient, baseURL, config.DefaultScope)
}

func workItemCommentHTTPRunnerWithScope(t *testing.T, httpClient *http.Client, baseURL string, expectedScope config.Scope) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		PATStore:     &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" {
				t.Fatalf("LoadConfig args = %q %q", repoRoot, homeDir)
			}
			if requestedProfile != "" && requestedProfile != "company-cloud" {
				t.Fatalf("requested profile = %q, want empty or company-cloud", requestedProfile)
			}
			if scope != expectedScope {
				t.Fatalf("scope = %v, want %v", scope, expectedScope)
			}
			return &config.Loaded{Profile: config.Profile{Name: "company-cloud", PATRef: "shared-ado", BaseURL: baseURL, Project: "MyProject", APIVersion: "7.1"}}, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) { return httpClient, nil },
	}}
}

func TestADOWorkItemNamespaceHelpListsSupportedOperations(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "work-item", "--help"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	for _, want := range []string{"comment", "Supported operations"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), want)
		}
	}
	for _, notWant := range []string{"update", "assign", "state", "relation", "attachment", "delete", "reaction"} {
		if strings.Contains(stderr.String(), notWant) {
			t.Fatalf("stderr = %q, want no unsupported operation %q", stderr.String(), notWant)
		}
	}
}

type fakeWorkItemCommentClient struct {
	fakeADOClient
	created      *ado.WorkItemComment
	createErr    error
	createCalled int
	createOpts   ado.WorkItemCommentCreateOptions
}

func (f *fakeWorkItemCommentClient) CreateWorkItemComment(ctx context.Context, opts ado.WorkItemCommentCreateOptions) (*ado.WorkItemComment, error) {
	f.createCalled++
	f.createOpts = opts
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.created != nil {
		return f.created, nil
	}
	return &ado.WorkItemComment{ID: 1, WorkItemID: opts.WorkItemID}, nil
}
