package ado

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientListInProgressPipelineRunsFiltersBranchOnEveryPage(t *testing.T) {
	const branch = "refs/heads/feature/example"
	var requests atomic.Int32
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		request := int(requests.Add(1))
		if got := req.URL.Query().Get("branchName"); got != branch {
			t.Fatalf("request %d branchName = %q, want %q", request, got, branch)
		}
		if got := req.URL.Query().Get("statusFilter"); got != "inProgress" {
			t.Fatalf("request %d statusFilter = %q, want inProgress", request, got)
		}
		response := pipelineTestResponse(req, http.StatusOK, `{"value":[]}`)
		if request == 1 {
			response.Header.Set("X-MS-ContinuationToken", "next")
			response.Body = ioNopCloserString(fmt.Sprintf(`{"value":[%s]}`, branchPipelineRunJSON(17, branch)))
		} else if request == 2 {
			response.Body = ioNopCloserString(fmt.Sprintf(`{"value":[%s]}`, branchPipelineRunJSON(18, branch)))
		}
		response.ContentLength = -1
		return response, nil
	})

	runs, err := client.ListInProgressPipelineRuns(context.Background(), PipelineRunListOptions{BranchName: branch})
	if err != nil {
		t.Fatalf("ListInProgressPipelineRuns returned error: %v", err)
	}
	if requests.Load() != 2 || len(runs) != 2 || runs[0].ID != 17 || runs[1].ID != 18 {
		t.Fatalf("requests/runs = %d/%v, want 2/[17 18]", requests.Load(), pipelineRunIDs(runs))
	}
}

func TestClientListRecentPipelineRunsFiltersBranchAndPreservesRemainingTop(t *testing.T) {
	const branch = "refs/pull/42/merge"
	var requests atomic.Int32
	client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
		request := int(requests.Add(1))
		if got := req.URL.Query().Get("branchName"); got != branch {
			t.Fatalf("request %d branchName = %q, want %q", request, got, branch)
		}
		if req.URL.Query().Has("statusFilter") {
			t.Fatalf("request %d has statusFilter %q, want absent", request, req.URL.Query().Get("statusFilter"))
		}
		wantTop := []string{"3", "1"}[request-1]
		if got := req.URL.Query().Get("$top"); got != wantTop {
			t.Fatalf("request %d $top = %q, want %q", request, got, wantTop)
		}
		response := pipelineTestResponse(req, http.StatusOK, `{"value":[]}`)
		if request == 1 {
			response.Header.Set("X-MS-ContinuationToken", "next")
			response.Body = ioNopCloserString(fmt.Sprintf(`{"value":[%s,%s]}`, branchPipelineRunJSON(30, branch), branchPipelineRunJSON(29, branch)))
		} else {
			response.Body = ioNopCloserString(fmt.Sprintf(`{"value":[%s]}`, branchPipelineRunJSON(28, branch)))
		}
		response.ContentLength = -1
		return response, nil
	})

	runs, err := client.ListRecentPipelineRuns(context.Background(), 3, PipelineRunListOptions{BranchName: branch})
	if err != nil {
		t.Fatalf("ListRecentPipelineRuns returned error: %v", err)
	}
	if requests.Load() != 2 || len(runs) != 3 || runs[0].ID != 30 || runs[1].ID != 29 || runs[2].ID != 28 {
		t.Fatalf("requests/runs = %d/%v, want 2/[30 29 28]", requests.Load(), pipelineRunIDs(runs))
	}
}

func TestClientListPipelineRunsRejectsBranchMismatchOrMissing(t *testing.T) {
	for _, sourceBranch := range []string{"refs/heads/other", ""} {
		t.Run(sourceBranch, func(t *testing.T) {
			client := newPipelineTestClient(t, func(req *http.Request) (*http.Response, error) {
				body := branchPipelineRunJSON(17, sourceBranch)
				if sourceBranch == "" {
					body = pipelineRunJSON(17, 1, "inProgress", "null")
				}
				return pipelineTestResponse(req, http.StatusOK, fmt.Sprintf(`{"value":[%s]}`, body)), nil
			})

			runs, err := client.ListInProgressPipelineRuns(context.Background(), PipelineRunListOptions{BranchName: "refs/heads/feature/example"})
			if err == nil || !strings.Contains(err.Error(), "source branch") {
				t.Fatalf("runs/error = %v/%v, want source branch validation error", runs, err)
			}
			if runs != nil {
				t.Fatalf("runs = %#v, want nil on mismatch", runs)
			}
		})
	}
}

func branchPipelineRunJSON(id int, branch string) string {
	return fmt.Sprintf(`{"id":%d,"buildNumber":"run-%d","status":"inProgress","result":"none","definition":{"id":1,"name":"Pipeline"},"sourceBranch":%q}`, id, id, branch)
}

func ioNopCloserString(value string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(value))
}
