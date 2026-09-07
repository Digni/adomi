package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Digni/adomi/internal/ado"
)

func TestParsePipelineListBranchNormalizesRefs(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{name: "short branch", arg: " feature/example ", want: "refs/heads/feature/example"},
		{name: "full ref", arg: " refs/pull/42/merge ", want: "refs/pull/42/merge"},
		{name: "remote prefix stays part of branch", arg: "origin/feature/example", want: "refs/heads/origin/feature/example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parsePipelineListArgs([]string{"--branch", tt.arg})
			if err != nil {
				t.Fatalf("parsePipelineListArgs returned error: %v", err)
			}
			if parsed.branch != tt.want {
				t.Fatalf("branch = %q, want %q", parsed.branch, tt.want)
			}
		})
	}
}

func TestADOPipelineListRejectsInvalidBranchArgumentsBeforeDependencies(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing branch value", args: []string{"ado", "pipeline", "list", "--branch"}, want: "--branch requires a value"},
		{name: "blank branch value", args: []string{"ado", "pipeline", "list", "--branch", " \t"}, want: "--branch requires a non-empty value"},
		{name: "repeated branch", args: []string{"ado", "pipeline", "list", "--branch", "one", "--branch", "two"}, want: "--branch cannot be repeated"},
		{name: "branch on get", args: []string{"ado", "pipeline", "get", "1", "--branch", "main"}, want: "unknown argument"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPipelineRunRejectsBeforeDependencies(t, tt.args, tt.want)
		})
	}
}

func TestADOPipelineListPassesNormalizedBranchToInProgressReader(t *testing.T) {
	client := &pipelineBranchADOClient{}
	runner := pipelineFakeRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pipeline", "list", "--branch", "feature/example"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if client.inProgressBranch != "refs/heads/feature/example" {
		t.Fatalf("in-progress branch = %q, want normalized branch", client.inProgressBranch)
	}
	if client.recentBranch != "" {
		t.Fatalf("recent branch = %q, want empty", client.recentBranch)
	}
	if stdout.String() != "{\"runs\":[]}\n" {
		t.Fatalf("stdout = %q, want empty runs result", stdout.String())
	}
}

func TestADOPipelineListPassesFullBranchToRecentReader(t *testing.T) {
	client := &pipelineBranchADOClient{}
	runner := pipelineFakeRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pipeline", "list", "--branch", "refs/pull/42/merge", "--last", "10"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if client.recentBranch != "refs/pull/42/merge" || client.recentN != 10 {
		t.Fatalf("recent branch/count = %q/%d, want full ref/10", client.recentBranch, client.recentN)
	}
	if client.inProgressBranch != "" {
		t.Fatalf("in-progress branch = %q, want empty", client.inProgressBranch)
	}
	if stdout.String() != "{\"runs\":[]}\n" {
		t.Fatalf("stdout = %q, want empty runs result", stdout.String())
	}
}

func TestADOPipelineHelpDocumentsBranchFilter(t *testing.T) {
	stderr := pipelineRunHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "pipeline", "list", "--help"})
	for _, want := range []string{"--branch <branch>", "branchName", "refs/heads/", "does not infer pull-request validation refs"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
}

func TestADOPipelineListRealWiringFiltersBranchAcrossRecentPages(t *testing.T) {
	const pat = "branch-pipeline-pat"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := int(requests.Add(1))
		assertRealPipelineRequest(t, r, pat)
		query := r.URL.Query()
		if query.Get("branchName") != "refs/heads/feature/example" {
			t.Fatalf("request %d branchName = %q, want normalized branch", request, query.Get("branchName"))
		}
		if query.Has("statusFilter") {
			t.Fatalf("request %d statusFilter = %q, want absent in recent mode", request, query.Get("statusFilter"))
		}
		response := `{"value":[]}`
		switch request {
		case 1:
			if query.Get("$top") != "3" {
				t.Fatalf("request 1 $top = %q, want 3", query.Get("$top"))
			}
			w.Header().Set("X-MS-ContinuationToken", "next")
			response = fmt.Sprintf(`{"value":[%s,%s]}`, realBranchPipelineRunJSON(31, "refs/heads/feature/example", "completed", `"succeeded"`), realBranchPipelineRunJSON(30, "refs/heads/feature/example", "inProgress", `null`))
		case 2:
			if query.Get("$top") != "1" || query.Get("continuationToken") != "next" {
				t.Fatalf("request 2 query = %v, want top 1 and continuation next", query)
			}
			response = fmt.Sprintf(`{"value":[%s]}`, realBranchPipelineRunJSON(29, "refs/heads/feature/example", "notStarted", `null`))
		default:
			t.Fatalf("unexpected request %d", request)
		}
		fmt.Fprint(w, response)
	}))
	t.Cleanup(server.Close)
	runner := pipelineRealRunner(t, server.URL, pat)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pipeline", "list", "--branch", " feature/example ", "--last", "3", "--profile", "pipeline-profile"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := `{"runs":[{"id":31,"pipelineId":1,"pipelineName":"Pipeline","runNumber":"run-31","status":"completed","result":"succeeded","sourceBranch":"refs/heads/feature/example","sourceVersion":null,"queueTime":null,"startTime":null,"finishTime":null,"webUrl":null},{"id":30,"pipelineId":1,"pipelineName":"Pipeline","runNumber":"run-30","status":"inProgress","result":null,"sourceBranch":"refs/heads/feature/example","sourceVersion":null,"queueTime":null,"startTime":null,"finishTime":null,"webUrl":null},{"id":29,"pipelineId":1,"pipelineName":"Pipeline","runNumber":"run-29","status":"notStarted","result":null,"sourceBranch":"refs/heads/feature/example","sourceVersion":null,"queueTime":null,"startTime":null,"finishTime":null,"webUrl":null}]}` + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.String() != "" || requests.Load() != 2 {
		t.Fatalf("stderr/requests = %q/%d, want empty/2", stderr.String(), requests.Load())
	}
}

func realBranchPipelineRunJSON(id int, branch, status, result string) string {
	return fmt.Sprintf(`{"id":%d,"buildNumber":"run-%d","status":%q,"result":%s,"definition":{"id":1,"name":"Pipeline"},"sourceBranch":%q}`, id, id, status, result, branch)
}

type pipelineBranchADOClient struct {
	fakeADOClient
	inProgressBranch string
	recentBranch     string
	recentN          int
}

func (c *pipelineBranchADOClient) ListInProgressPipelineRuns(_ context.Context, options ...ado.PipelineRunListOptions) ([]ado.PipelineRun, error) {
	if len(options) == 1 {
		c.inProgressBranch = options[0].BranchName
	}
	return []ado.PipelineRun{}, nil
}

func (c *pipelineBranchADOClient) ListRecentPipelineRuns(_ context.Context, n int, options ...ado.PipelineRunListOptions) ([]ado.PipelineRun, error) {
	c.recentN = n
	if len(options) == 1 {
		c.recentBranch = options[0].BranchName
	}
	return []ado.PipelineRun{}, nil
}
