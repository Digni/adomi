package ado

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	maxPipelineResponseBytes int64 = 8 * 1024 * 1024
	maxPipelinePages               = 1000
	maxPipelineRuns                = 100000
	maxPipelineRunID               = 2147483647
)

var errPipelineRunLimitExceeded = errors.New("pipeline run limit exceeded")

type PipelineRun struct {
	ID            int
	PipelineID    int
	PipelineName  string
	RunNumber     string
	Status        string
	Result        *string
	SourceBranch  *string
	SourceVersion *string
	QueueTime     *time.Time
	StartTime     *time.Time
	FinishTime    *time.Time
	WebURL        *string
}

type PipelineRunReader interface {
	ListInProgressPipelineRuns(ctx context.Context) ([]PipelineRun, error)
	GetPipelineRun(ctx context.Context, id int) (*PipelineRun, error)
}

type pipelineRunResponse struct {
	ID            int                        `json:"id"`
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

func (c *Client) ListInProgressPipelineRuns(ctx context.Context) ([]PipelineRun, error) {
	if err := c.validatePipelineTransport(); err != nil {
		return nil, err
	}
	httpClient, ownsTransport := c.pipelineHTTPClient()
	if ownsTransport {
		defer httpClient.CloseIdleConnections()
	}

	runs := make([]PipelineRun, 0)
	seenRunIDs := make(map[int]struct{})
	seenTokens := make(map[string]struct{})
	continuationToken := ""
	for page := 1; page <= maxPipelinePages; page++ {
		body, header, err := c.pipelineGET(ctx, httpClient, c.pipelineRunsURL(continuationToken), "listing Azure DevOps pipeline runs")
		if err != nil {
			return nil, err
		}

		pageRuns, err := decodePipelineRunList(body, maxPipelineRuns-len(runs))
		if err != nil {
			if errors.Is(err, errPipelineRunLimitExceeded) {
				return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response exceeds maximum of %d runs", maxPipelineRuns)
			}
			return nil, err
		}
		for i := range pageRuns {
			run, err := normalizePipelineRun(pageRuns[i])
			if err != nil {
				return nil, fmt.Errorf("listing Azure DevOps pipeline runs: invalid run at index %d: %w", i, err)
			}
			if run.Status != "inProgress" {
				return nil, fmt.Errorf("listing Azure DevOps pipeline runs: run at index %d is not in progress", i)
			}
			if _, exists := seenRunIDs[run.ID]; exists {
				return nil, fmt.Errorf("listing Azure DevOps pipeline runs: duplicate run ID %d", run.ID)
			}
			seenRunIDs[run.ID] = struct{}{}
			runs = append(runs, run)
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
