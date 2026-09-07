package ado

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	maxPipelineResponseBytes  int64 = 8 * 1024 * 1024
	maxPipelinePages                = 1000
	maxPipelineRuns                 = 100000
	maxPipelineRunID                = 2147483647
	maxRecentPipelineRuns           = 200
	maxInspectionRequests           = 1000
	maxInspectionBytes        int64 = 128 * 1024 * 1024
	inspectionCollectionLimit       = 100000
)

var errPipelineRunLimitExceeded = errors.New("pipeline run limit exceeded")

type PipelineRun struct {
	ID            int        `json:"id"`
	PipelineID    int        `json:"pipelineId"`
	PipelineName  string     `json:"pipelineName"`
	RunNumber     string     `json:"runNumber"`
	Status        string     `json:"status"`
	Result        *string    `json:"result"`
	SourceBranch  *string    `json:"sourceBranch"`
	SourceVersion *string    `json:"sourceVersion"`
	QueueTime     *time.Time `json:"queueTime"`
	StartTime     *time.Time `json:"startTime"`
	FinishTime    *time.Time `json:"finishTime"`
	WebURL        *string    `json:"webUrl"`
}

type PipelineRunReader interface {
	ListInProgressPipelineRuns(ctx context.Context, options ...PipelineRunListOptions) ([]PipelineRun, error)
	ListRecentPipelineRuns(ctx context.Context, n int, options ...PipelineRunListOptions) ([]PipelineRun, error)
	GetPipelineRun(ctx context.Context, id int) (*PipelineRun, error)
}

type PipelineRunListOptions struct {
	BranchName string
}

type pipelineRunResponse struct {
	ID            int                        `json:"id"`
	URI           *string                    `json:"uri"`
	BuildNumber   string                     `json:"buildNumber"`
	Status        string                     `json:"status"`
	Result        *string                    `json:"result"`
	Definition    pipelineDefinitionResponse `json:"definition"`
	SourceBranch  *string                    `json:"sourceBranch"`
	SourceVersion *string                    `json:"sourceVersion"`
	QueueTime     *string                    `json:"queueTime"`
	StartTime     *string                    `json:"startTime"`
	FinishTime    *string                    `json:"finishTime"`
	Links         pipelineLinksResponse      `json:"_links"`
}

type pipelineDefinitionResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type pipelineLinksResponse struct {
	Web pipelineWebLinkResponse `json:"web"`
}

type pipelineWebLinkResponse struct {
	Href *string `json:"href"`
}

func (c *Client) ListInProgressPipelineRuns(ctx context.Context, options ...PipelineRunListOptions) ([]PipelineRun, error) {
	if err := c.validatePipelineTransport(); err != nil {
		return nil, err
	}
	listOptions, err := pipelineRunListOptions(options)
	if err != nil {
		return nil, err
	}
	httpClient, ownsTransport := c.pipelineHTTPClient()
	if ownsTransport {
		defer httpClient.CloseIdleConnections()
	}

	return c.listPipelineRuns(ctx, httpClient, pipelineListOptions{
		maxRuns:           maxPipelineRuns,
		requireInProgress: true,
		branchName:        listOptions.BranchName,
		buildURL: func(continuationToken string, _ int) string {
			return c.pipelineRunsURL(continuationToken, listOptions.BranchName)
		},
	})
}

func (c *Client) ListRecentPipelineRuns(ctx context.Context, n int, options ...PipelineRunListOptions) ([]PipelineRun, error) {
	if n < 1 || n > maxRecentPipelineRuns {
		return nil, fmt.Errorf("recent pipeline run count must be between 1 and %d", maxRecentPipelineRuns)
	}
	if err := c.validatePipelineTransport(); err != nil {
		return nil, err
	}
	listOptions, err := pipelineRunListOptions(options)
	if err != nil {
		return nil, err
	}
	httpClient, ownsTransport := c.pipelineHTTPClient()
	if ownsTransport {
		defer httpClient.CloseIdleConnections()
	}

	return c.listPipelineRuns(ctx, httpClient, pipelineListOptions{
		maxRuns:    n,
		stopAtMax:  true,
		branchName: listOptions.BranchName,
		buildURL: func(continuationToken string, remaining int) string {
			return c.pipelineRecentRunsURL(continuationToken, remaining, listOptions.BranchName)
		},
	})
}

func pipelineRunListOptions(options []PipelineRunListOptions) (PipelineRunListOptions, error) {
	if len(options) > 1 {
		return PipelineRunListOptions{}, fmt.Errorf("pipeline run list options cannot be repeated")
	}
	if len(options) == 0 {
		return PipelineRunListOptions{}, nil
	}
	return options[0], nil
}

type pipelineListOptions struct {
	maxRuns           int
	requireInProgress bool
	stopAtMax         bool
	branchName        string
	buildURL          func(continuationToken string, remaining int) string
}

func (c *Client) listPipelineRuns(ctx context.Context, httpClient *http.Client, opts pipelineListOptions) ([]PipelineRun, error) {
	runs := make([]PipelineRun, 0)
	seenRunIDs := make(map[int]struct{})
	seenTokens := make(map[string]struct{})
	continuationToken := ""
	for page := 1; page <= maxPipelinePages; page++ {
		body, header, err := c.pipelineGET(ctx, httpClient, opts.buildURL(continuationToken, opts.maxRuns-len(runs)), "listing Azure DevOps pipeline runs")
		if err != nil {
			return nil, err
		}

		pageRuns, err := decodePipelineRunList(body, opts.maxRuns-len(runs))
		if err != nil {
			if errors.Is(err, errPipelineRunLimitExceeded) {
				if opts.stopAtMax {
					return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response page exceeds the requested top of %d runs", opts.maxRuns-len(runs))
				}
				return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response exceeds maximum of %d runs", maxPipelineRuns)
			}
			return nil, err
		}
		for i := range pageRuns {
			run, err := normalizePipelineRun(pageRuns[i])
			if err != nil {
				return nil, fmt.Errorf("listing Azure DevOps pipeline runs: invalid run at index %d: %w", i, err)
			}
			if opts.requireInProgress && run.Status != "inProgress" {
				return nil, fmt.Errorf("listing Azure DevOps pipeline runs: run at index %d is not in progress", i)
			}
			if opts.branchName != "" && (run.SourceBranch == nil || *run.SourceBranch != opts.branchName) {
				return nil, fmt.Errorf("listing Azure DevOps pipeline runs: run at index %d source branch does not match requested branch %q", i, opts.branchName)
			}
			if _, exists := seenRunIDs[run.ID]; exists {
				return nil, fmt.Errorf("listing Azure DevOps pipeline runs: duplicate run ID %d", run.ID)
			}
			seenRunIDs[run.ID] = struct{}{}
			runs = append(runs, run)
		}

		if opts.stopAtMax && len(runs) == opts.maxRuns {
			return runs, nil
		}
		token, hasToken, err := pipelineContinuationToken(header)
		if err != nil {
			return nil, err
		}
		if !hasToken {
			return runs, nil
		}
		if page == maxPipelinePages {
			return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response exceeds maximum of %d pages", maxPipelinePages)
		}
		if _, exists := seenTokens[token]; exists {
			return nil, fmt.Errorf("listing Azure DevOps pipeline runs: continuation token repeated")
		}
		seenTokens[token] = struct{}{}
		continuationToken = token
	}

	return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response exceeds maximum of %d pages", maxPipelinePages)
}

func (c *Client) GetPipelineRun(ctx context.Context, id int) (*PipelineRun, error) {
	if id < 1 || id > maxPipelineRunID {
		return nil, fmt.Errorf("pipeline run ID must be between 1 and %d", maxPipelineRunID)
	}
	if err := c.validatePipelineTransport(); err != nil {
		return nil, err
	}
	httpClient, ownsTransport := c.pipelineHTTPClient()
	if ownsTransport {
		defer httpClient.CloseIdleConnections()
	}

	body, _, err := c.pipelineGET(ctx, httpClient, c.pipelineRunURL(id), "getting Azure DevOps pipeline run")
	if err != nil {
		return nil, err
	}
	var response pipelineRunResponse
	if err := decodePipelineJSON(body, &response, "getting Azure DevOps pipeline run"); err != nil {
		return nil, err
	}
	run, err := normalizePipelineRun(response)
	if err != nil {
		return nil, fmt.Errorf("getting Azure DevOps pipeline run: invalid response: %w", err)
	}
	if run.ID != id {
		return nil, fmt.Errorf("getting Azure DevOps pipeline run: response ID does not match requested ID")
	}
	return &run, nil
}
