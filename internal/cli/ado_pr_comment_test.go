package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Digni/adomi/internal/ado"
)

func TestADOPullRequestCommentCreatesThreadFromInlineMessage(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdThread: &ado.PullRequestThread{ID: 14, Comments: []ado.PullRequestComment{{ID: 1}}}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "14\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if client.fetchPRCalled != 1 || client.fetchedPRID != 42 {
		t.Fatalf("fetch PR called/id = %d/%d, want 1/42", client.fetchPRCalled, client.fetchedPRID)
	}
	if client.threadCreateCalled != 1 || client.threadCreateOpts.RepositoryID != "repo-uuid" || client.threadCreateOpts.PullRequestID != 42 || client.threadCreateOpts.Content != "Looks good" {
		t.Fatalf("thread create called/opts = %d/%+v, want repo/pr/content", client.threadCreateCalled, client.threadCreateOpts)
	}
	if client.commentCalled != 0 || client.threadUpdateCalled != 0 {
		t.Fatalf("calls comment/threadUpdate = %d/%d, want none", client.commentCalled, client.threadUpdateCalled)
	}
}

func TestADOPullRequestCommentCreatesThreadFromMessageFile(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "comment.md")
	if err := os.WriteFile(messagePath, []byte("Looks good\n"), 0o644); err != nil {
		t.Fatalf("writing comment: %v", err)
	}
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdThread: &ado.PullRequestThread{ID: 15, Comments: []ado.PullRequestComment{{ID: 2}}}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "15\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	if client.threadCreateOpts.Content != "Looks good\n" {
		t.Fatalf("thread content = %q, want file content", client.threadCreateOpts.Content)
	}
}

func TestADOPullRequestCommentCreatesInlineThreadFromInlineMessage(t *testing.T) {
	client := &fakePRMaintenanceClient{
		fetched:    &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}},
		iterations: []ado.PullRequestIteration{{ID: 1}, {ID: 3}},
		iterationChanges: []ado.PullRequestIterationChange{{
			ChangeTrackingID: 77,
			ChangeType:       "edit",
			Item:             ado.PullRequestChangedItem{Path: "/src/app.go"},
		}},
		createdThread: &ado.PullRequestThread{ID: 16, Comments: []ado.PullRequestComment{{ID: 4}}},
	}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--file", "src/app.go", "--line", "42", "--message", "Nit"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "16\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if client.iterationsCalled != 1 || client.iterationChangesCalled != 1 {
		t.Fatalf("iterations/changes calls = %d/%d, want 1/1", client.iterationsCalled, client.iterationChangesCalled)
	}
	if client.iterationChangesOpts.RepositoryID != "repo-uuid" || client.iterationChangesOpts.PullRequestID != 42 || client.iterationChangesOpts.IterationID != 3 || client.iterationChangesOpts.CompareTo != 0 {
		t.Fatalf("iteration changes opts = %+v, want latest iteration common-base lookup", client.iterationChangesOpts)
	}
	assertInlineThreadCreateOpts(t, client.threadCreateOpts, "repo-uuid", 42, "Nit", "/src/app.go", 42, 77, 3, 3)
}

func TestADOPullRequestCommentCreatesInlineThreadFromMessageFile(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "comment.md")
	if err := os.WriteFile(messagePath, []byte("Inline note\n"), 0o644); err != nil {
		t.Fatalf("writing comment: %v", err)
	}
	client := &fakePRMaintenanceClient{
		fetched:          &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}},
		iterations:       []ado.PullRequestIteration{{ID: 2}},
		iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 8, ChangeType: "add", Item: ado.PullRequestChangedItem{Path: "/README.md"}}},
		createdThread:    &ado.PullRequestThread{ID: 17, Comments: []ado.PullRequestComment{{ID: 5}}},
	}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--file", "/README.md", "--line", "1", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "17\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	assertInlineThreadCreateOpts(t, client.threadCreateOpts, "repo-uuid", 42, "Inline note\n", "/README.md", 1, 8, 2, 2)
}

func assertInlineThreadCreateOpts(t *testing.T, opts ado.PullRequestThreadCreateOptions, repositoryID string, pullRequestID int, content string, filePath string, line int, changeTrackingID int, firstIteration int, secondIteration int) {
	t.Helper()
	if opts.RepositoryID != repositoryID || opts.PullRequestID != pullRequestID || opts.Content != content {
		t.Fatalf("thread create opts = %+v, want repo/pr/content", opts)
	}
	if opts.ThreadContext == nil {
		t.Fatal("ThreadContext = nil, want inline context")
	}
	if opts.ThreadContext.FilePath != filePath {
		t.Fatalf("FilePath = %q, want %q", opts.ThreadContext.FilePath, filePath)
	}
	if opts.ThreadContext.LeftFileStart != nil || opts.ThreadContext.LeftFileEnd != nil {
		t.Fatalf("left positions = %+v/%+v, want nil", opts.ThreadContext.LeftFileStart, opts.ThreadContext.LeftFileEnd)
	}
	if opts.ThreadContext.RightFileStart == nil || opts.ThreadContext.RightFileEnd == nil {
		t.Fatalf("right positions = %+v/%+v, want populated", opts.ThreadContext.RightFileStart, opts.ThreadContext.RightFileEnd)
	}
	if *opts.ThreadContext.RightFileStart != (ado.FilePosition{Line: line, Offset: 1}) || *opts.ThreadContext.RightFileEnd != (ado.FilePosition{Line: line, Offset: 1}) {
		t.Fatalf("right positions = %+v/%+v, want line %d offset 1", opts.ThreadContext.RightFileStart, opts.ThreadContext.RightFileEnd, line)
	}
	if opts.PullRequestThreadContext == nil || opts.PullRequestThreadContext.ChangeTrackingID != changeTrackingID || opts.PullRequestThreadContext.IterationContext == nil {
		t.Fatalf("PullRequestThreadContext = %+v, want change tracking", opts.PullRequestThreadContext)
	}
	if opts.PullRequestThreadContext.IterationContext.FirstComparingIteration != firstIteration || opts.PullRequestThreadContext.IterationContext.SecondComparingIteration != secondIteration {
		t.Fatalf("IterationContext = %+v, want %d/%d", opts.PullRequestThreadContext.IterationContext, firstIteration, secondIteration)
	}
}

func TestADOPullRequestCommentJSONOutput(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdThread: &ado.PullRequestThread{ID: 14, Comments: []ado.PullRequestComment{{ID: 3}}}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good", "--json"}, strings.NewReader(""), &stdout, &stderr)
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
	if payload["pullRequestId"] != float64(42) || payload["threadId"] != float64(14) || payload["commentId"] != float64(3) || payload["action"] != "commented" {
		t.Fatalf("payload = %+v, want comment summary", payload)
	}
}

func TestADOPullRequestCommentInlineJSONOutput(t *testing.T) {
	client := &fakePRMaintenanceClient{
		fetched:          &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}},
		iterations:       []ado.PullRequestIteration{{ID: 2}},
		iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 9, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
		createdThread:    &ado.PullRequestThread{ID: 18, Comments: []ado.PullRequestComment{{ID: 6}}},
	}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--file", "/src/app.go", "--line", "42", "--message", "Nit", "--json"}, strings.NewReader(""), &stdout, &stderr)
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
	if payload["pullRequestId"] != float64(42) || payload["threadId"] != float64(18) || payload["commentId"] != float64(6) || payload["filePath"] != "/src/app.go" || payload["line"] != float64(42) || payload["action"] != "commented" {
		t.Fatalf("payload = %+v, want inline comment summary", payload)
	}
}

func TestADOPullRequestCommentJSONOmitsMissingInitialCommentID(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdThread: &ado.PullRequestThread{ID: 14}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good", "--json"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if _, ok := payload["commentId"]; ok {
		t.Fatalf("payload = %+v, want omitted commentId", payload)
	}
}

func TestADOPullRequestCommentRejectsInvalidArgsBeforeCredentials(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing PR ID", args: []string{"ado", "pr", "comment"}, want: "usage"},
		{name: "non-positive PR ID", args: []string{"ado", "pr", "comment", "0", "--message", "x"}, want: "pull request ID"},
		{name: "missing message source", args: []string{"ado", "pr", "comment", "42"}, want: "--message"},
		{name: "conflicting message sources", args: []string{"ado", "pr", "comment", "42", "--message", "x", "--message-file", "comment.md"}, want: "exactly one"},
		{name: "empty message", args: []string{"ado", "pr", "comment", "42", "--message", "   "}, want: "--message cannot be empty"},
		{name: "thread is rejected", args: []string{"ado", "pr", "comment", "42", "--thread", "7", "--message", "x"}, want: "unknown"},
		{name: "file requires line", args: []string{"ado", "pr", "comment", "42", "--file", "/main.go", "--message", "x"}, want: "--file and --line"},
		{name: "line requires file", args: []string{"ado", "pr", "comment", "42", "--line", "12", "--message", "x"}, want: "--file and --line"},
		{name: "line must be positive", args: []string{"ado", "pr", "comment", "42", "--file", "/main.go", "--line", "0", "--message", "x"}, want: "--line must be a positive integer"},
		{name: "negative line must be positive", args: []string{"ado", "pr", "comment", "42", "--file", "/main.go", "--line", "-1", "--message", "x"}, want: "--line must be a positive integer"},
		{name: "line must be integer", args: []string{"ado", "pr", "comment", "42", "--file", "/main.go", "--line", "abc", "--message", "x"}, want: "--line must be a positive integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := Runner{deps: Dependencies{PATStore: &fakePATStore{}}}
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

func TestADOPullRequestCommentInlineTargetErrorsLeaveStdoutEmpty(t *testing.T) {
	tests := []struct {
		name   string
		client *fakePRMaintenanceClient
		want   string
	}{
		{name: "iterations error", client: &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, iterationsErr: errors.New("iterations failed")}, want: "iterations failed"},
		{name: "changes error", client: &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, iterations: []ado.PullRequestIteration{{ID: 1}}, iterationChangesErr: errors.New("changes failed")}, want: "changes failed"},
		{name: "no match", client: &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, iterations: []ado.PullRequestIteration{{ID: 1}}, iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/other.go"}}}}, want: "was not found"},
		{name: "unsupported", client: &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, iterations: []ado.PullRequestIteration{{ID: 1}}, iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "delete", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}}}, want: "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := prThreadTestRunner(t, tt.client)
			var stdout bytes.Buffer
			err := runner.Run([]string{"ado", "pr", "comment", "42", "--file", "/src/app.go", "--line", "42", "--message", "Nit"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if tt.client.threadCreateCalled != 0 {
				t.Fatalf("threadCreateCalled = %d, want 0", tt.client.threadCreateCalled)
			}
		})
	}
}

func TestResolvePullRequestInlineCommentTarget(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		client  *fakePRMaintenanceClient
		want    prInlineCommentTarget
		wantErr string
	}{
		{
			name: "matches normalized latest iteration change",
			file: "src/app.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}, {ID: 4}, {ID: 2}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			want: prInlineCommentTarget{FilePath: "/src/app.go", ChangeTrackingID: 77, FirstComparingIteration: 4, SecondComparingIteration: 4},
		},
		{
			name:    "no iterations",
			file:    "/src/app.go",
			client:  &fakePRMaintenanceClient{},
			wantErr: "no iterations",
		},
		{
			name: "missing file",
			file: "/missing.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "was not found",
		},
		{
			name: "duplicate match",
			file: "/src/app.go",
			client: &fakePRMaintenanceClient{
				iterations: []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{
					{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}},
					{ChangeTrackingID: 78, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}},
				},
			},
			wantErr: "multiple",
		},
		{
			name: "delete unsupported",
			file: "/src/app.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "delete", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "unsupported",
		},
		{
			name: "rename unsupported",
			file: "/src/app.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "rename", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "unsupported",
		},
		{
			name: "missing tracking unsupported",
			file: "/src/app.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "unsupported",
		},
		{
			name: "dot segment path rejected",
			file: "../src/app.go",
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "was not found",
		},
		{
			name: "backslash path rejected",
			file: `src\app.go`,
			client: &fakePRMaintenanceClient{
				iterations:       []ado.PullRequestIteration{{ID: 1}},
				iterationChanges: []ado.PullRequestIterationChange{{ChangeTrackingID: 77, ChangeType: "edit", Item: ado.PullRequestChangedItem{Path: "/src/app.go"}}},
			},
			wantErr: "was not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolvePullRequestInlineCommentTarget(context.Background(), tt.client, "repo-uuid", 42, tt.file)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("target = %+v, want %+v", got, tt.want)
			}
			if tt.client.iterationChangesOpts.IterationID != tt.want.SecondComparingIteration || tt.client.iterationChangesOpts.CompareTo != 0 {
				t.Fatalf("iteration changes opts = %+v, want latest/common-base", tt.client.iterationChangesOpts)
			}
		})
	}
}

func TestADOPullRequestCommentRejectsEmptyMessageFileBeforeNetwork(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "empty.md")
	if err := os.WriteFile(messagePath, nil, 0o644); err != nil {
		t.Fatalf("writing comment: %v", err)
	}
	client := &fakePRMaintenanceClient{}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--message-file cannot be empty") {
		t.Fatalf("error = %v, want empty message-file", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 0 || client.threadCreateCalled != 0 {
		t.Fatalf("calls fetch/threadCreate = %d/%d, want none", client.fetchPRCalled, client.threadCreateCalled)
	}
}

func TestADOPullRequestCommentRejectsWhitespaceOnlyMessageFileBeforeNetwork(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "blank.md")
	if err := os.WriteFile(messagePath, []byte(" \n\t"), 0o644); err != nil {
		t.Fatalf("writing comment: %v", err)
	}
	client := &fakePRMaintenanceClient{}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--message-file cannot be empty") {
		t.Fatalf("error = %v, want empty message-file", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 0 || client.threadCreateCalled != 0 {
		t.Fatalf("calls fetch/threadCreate = %d/%d, want none", client.fetchPRCalled, client.threadCreateCalled)
	}
}

func TestADOPullRequestCommentRejectsMissingRepositoryIDBeforeThreadCreate(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "repository ID") {
		t.Fatalf("error = %v, want repository ID", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.threadCreateCalled != 0 {
		t.Fatalf("calls fetch/threadCreate = %d/%d, want fetch only", client.fetchPRCalled, client.threadCreateCalled)
	}
}

func TestADOPullRequestCommentMutationErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, threadCreateErr: errors.New("thread failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "thread failed") {
		t.Fatalf("error = %v, want thread failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPullRequestCommentFetchErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetchPRErr: errors.New("fetch failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("error = %v, want fetch failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.fetchedPRID != 42 || client.threadCreateCalled != 0 || client.commentCalled != 0 || client.threadUpdateCalled != 0 {
		t.Fatalf("calls fetch/id/threadCreate/comment/threadUpdate = %d/%d/%d/%d/%d, want 1/42/0/0/0", client.fetchPRCalled, client.fetchedPRID, client.threadCreateCalled, client.commentCalled, client.threadUpdateCalled)
	}
}

func TestADOPullRequestCommentRejectsMissingThreadIDResponse(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdThread: &ado.PullRequestThread{}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "comment", "42", "--message", "Looks good"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "thread ID") {
		t.Fatalf("error = %v, want thread ID", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
