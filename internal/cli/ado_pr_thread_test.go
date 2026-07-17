package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Digni/adomi/internal/ado"
)

func TestADOPullRequestReplyCreatesCommentFromInlineMessage(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdComment: &ado.PullRequestComment{ID: 8}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "Fixed this", "--profile", "company-cloud"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "8\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if client.fetchPRCalled != 1 || client.fetchedPRID != 42 {
		t.Fatalf("fetch PR called/id = %d/%d, want 1/42", client.fetchPRCalled, client.fetchedPRID)
	}
	if client.commentCalled != 1 || client.commentOpts.RepositoryID != "repo-uuid" || client.commentOpts.PullRequestID != 42 || client.commentOpts.ThreadID != 7 || client.commentOpts.Content != "Fixed this" {
		t.Fatalf("comment called/opts = %d/%+v, want repo/pr/thread/content", client.commentCalled, client.commentOpts)
	}
	if client.threadUpdateCalled != 0 {
		t.Fatalf("threadUpdateCalled = %d, want 0", client.threadUpdateCalled)
	}
}

func TestADOPullRequestReplyCreatesCommentFromMessageFile(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "reply.md")
	if err := os.WriteFile(messagePath, []byte("Done\n"), 0o644); err != nil {
		t.Fatalf("writing reply: %v", err)
	}
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdComment: &ado.PullRequestComment{ID: 9}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "9\n" {
		t.Fatalf("stdout = %q, want comment ID", stdout.String())
	}
	if client.commentOpts.Content != "Done\n" {
		t.Fatalf("comment content = %q, want file content", client.commentOpts.Content)
	}
}

func TestADOPullRequestReplyJSONOutput(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, createdComment: &ado.PullRequestComment{ID: 8}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "Fixed this", "--json"}, strings.NewReader(""), &stdout, &stderr)
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
	if payload["pullRequestId"] != float64(42) || payload["threadId"] != float64(7) || payload["commentId"] != float64(8) || payload["action"] != "replied" {
		t.Fatalf("payload = %+v, want reply summary", payload)
	}
}

func TestADOPullRequestResolveUpdatesThreadFixed(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, updatedThread: &ado.PullRequestThread{ID: 7, Status: "fixed"}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "resolve", "42", "--thread", "7"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "7\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if client.threadUpdateCalled != 1 || client.threadUpdateOpts.RepositoryID != "repo-uuid" || client.threadUpdateOpts.PullRequestID != 42 || client.threadUpdateOpts.ThreadID != 7 || client.threadUpdateOpts.Status != "fixed" {
		t.Fatalf("thread update called/opts = %d/%+v, want fixed", client.threadUpdateCalled, client.threadUpdateOpts)
	}
	if client.commentCalled != 0 {
		t.Fatalf("commentCalled = %d, want 0", client.commentCalled)
	}
}

func TestADOPullRequestReopenUpdatesThreadActive(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, updatedThread: &ado.PullRequestThread{ID: 7, Status: "active"}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reopen", "42", "--thread", "7"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stdout.String() != "7\n" {
		t.Fatalf("stdout = %q, want thread ID", stdout.String())
	}
	if client.threadUpdateOpts.Status != "active" {
		t.Fatalf("thread update status = %q, want active", client.threadUpdateOpts.Status)
	}
}

func TestADOPullRequestThreadStatusJSONOutput(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, updatedThread: &ado.PullRequestThread{ID: 7, Status: "fixed"}}
	runner := prThreadTestRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "resolve", "42", "--thread", "7", "--json"}, strings.NewReader(""), &stdout, &stderr)
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
	if payload["pullRequestId"] != float64(42) || payload["threadId"] != float64(7) || payload["status"] != "fixed" || payload["action"] != "resolved" {
		t.Fatalf("payload = %+v, want resolve summary", payload)
	}
}

func TestADOPullRequestReplyRejectsInvalidArgsBeforeCredentials(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing PR ID", args: []string{"ado", "pr", "reply"}, want: "usage"},
		{name: "non-positive PR ID", args: []string{"ado", "pr", "reply", "0", "--thread", "7", "--message", "x"}, want: "pull request ID"},
		{name: "missing thread", args: []string{"ado", "pr", "reply", "42", "--message", "x"}, want: "--thread"},
		{name: "non-positive thread", args: []string{"ado", "pr", "reply", "42", "--thread", "0", "--message", "x"}, want: "thread ID"},
		{name: "missing message source", args: []string{"ado", "pr", "reply", "42", "--thread", "7"}, want: "--message"},
		{name: "conflicting message sources", args: []string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "x", "--message-file", "reply.md"}, want: "exactly one"},
		{name: "empty message", args: []string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "   "}, want: "--message cannot be empty"},
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

func TestADOPullRequestThreadStatusRejectsInvalidArgsBeforeCredentials(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "resolve missing PR ID", args: []string{"ado", "pr", "resolve"}, want: "usage: adomi ado pr resolve <pull-request-id> --thread <thread-id>"},
		{name: "reopen missing PR ID", args: []string{"ado", "pr", "reopen"}, want: "usage: adomi ado pr reopen <pull-request-id> --thread <thread-id>"},
		{name: "non-positive PR ID", args: []string{"ado", "pr", "resolve", "0", "--thread", "7"}, want: "pull request ID"},
		{name: "missing thread", args: []string{"ado", "pr", "resolve", "42"}, want: "--thread"},
		{name: "non-positive thread", args: []string{"ado", "pr", "resolve", "42", "--thread", "0"}, want: "thread ID"},
		{name: "message is rejected", args: []string{"ado", "pr", "resolve", "42", "--thread", "7", "--message", "x"}, want: "unknown"},
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

func TestADOPullRequestReplyRejectsEmptyMessageFileBeforeNetwork(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "empty.md")
	if err := os.WriteFile(messagePath, nil, 0o644); err != nil {
		t.Fatalf("writing reply: %v", err)
	}
	client := &fakePRMaintenanceClient{}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--message-file cannot be empty") {
		t.Fatalf("error = %v, want empty message-file", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 0 || client.commentCalled != 0 {
		t.Fatalf("calls fetch/comment = %d/%d, want none", client.fetchPRCalled, client.commentCalled)
	}
}

func TestADOPullRequestReplyRejectsWhitespaceOnlyMessageFileBeforeNetwork(t *testing.T) {
	messagePath := filepath.Join(t.TempDir(), "blank.md")
	if err := os.WriteFile(messagePath, []byte(" \n\t"), 0o644); err != nil {
		t.Fatalf("writing reply: %v", err)
	}
	client := &fakePRMaintenanceClient{}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message-file", messagePath}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--message-file cannot be empty") {
		t.Fatalf("error = %v, want empty message-file", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 0 || client.commentCalled != 0 {
		t.Fatalf("calls fetch/comment = %d/%d, want none", client.fetchPRCalled, client.commentCalled)
	}
}

func TestADOPullRequestReplyRejectsMissingRepositoryIDBeforeComment(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "Fixed"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "repository ID") {
		t.Fatalf("error = %v, want repository ID", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.commentCalled != 0 {
		t.Fatalf("calls fetch/comment = %d/%d, want fetch only", client.fetchPRCalled, client.commentCalled)
	}
}

func TestADOPullRequestResolveRejectsMissingRepositoryIDBeforeThreadUpdate(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42}}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "resolve", "42", "--thread", "7"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "repository ID") {
		t.Fatalf("error = %v, want repository ID", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.threadUpdateCalled != 0 {
		t.Fatalf("calls fetch/threadUpdate = %d/%d, want fetch only", client.fetchPRCalled, client.threadUpdateCalled)
	}
}

func TestADOPullRequestReplyMutationErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, commentErr: errors.New("comment failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "Fixed"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "comment failed") {
		t.Fatalf("error = %v, want comment failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestADOPullRequestReplyFetchErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetchPRErr: errors.New("fetch failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "reply", "42", "--thread", "7", "--message", "Fixed"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("error = %v, want fetch failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.fetchedPRID != 42 || client.commentCalled != 0 || client.threadUpdateCalled != 0 {
		t.Fatalf("calls fetch/id/comment/threadUpdate = %d/%d/%d/%d, want 1/42/0/0", client.fetchPRCalled, client.fetchedPRID, client.commentCalled, client.threadUpdateCalled)
	}
}

func TestADOPullRequestResolveFetchErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetchPRErr: errors.New("fetch failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "resolve", "42", "--thread", "7"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("error = %v, want fetch failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if client.fetchPRCalled != 1 || client.fetchedPRID != 42 || client.commentCalled != 0 || client.threadUpdateCalled != 0 {
		t.Fatalf("calls fetch/id/comment/threadUpdate = %d/%d/%d/%d, want 1/42/0/0", client.fetchPRCalled, client.fetchedPRID, client.commentCalled, client.threadUpdateCalled)
	}
}

func TestADOPullRequestResolveMutationErrorLeavesStdoutEmpty(t *testing.T) {
	client := &fakePRMaintenanceClient{fetched: &ado.PullRequest{ID: 42, Repository: ado.PullRequestRepo{ID: "repo-uuid"}}, threadUpdateErr: errors.New("thread failed")}
	runner := prThreadTestRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pr", "resolve", "42", "--thread", "7"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "thread failed") {
		t.Fatalf("error = %v, want thread failure", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
