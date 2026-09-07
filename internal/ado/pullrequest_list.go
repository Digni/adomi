package ado

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

const (
	maxPullRequestResponseBytes  int64 = 8 * 1024 * 1024
	maxPullRequestPages                = 1000
	maxPullRequests                    = 100000
	pullRequestDiscoveryPageSize       = 100
)

type PullRequestLister interface {
	ListPullRequests(context.Context, PullRequestListOptions) ([]PullRequest, error)
}

func DiscoverPullRequests(ctx context.Context, lister PullRequestLister, opts PullRequestListOptions) ([]PullRequest, error) {
	if lister == nil {
		return nil, fmt.Errorf("pull request discovery requires a lister")
	}

	discovered := make([]PullRequest, 0)
	seenIDs := make(map[int]struct{})
	skip := 0
	for page := 1; page <= maxPullRequestPages; page++ {
		pageOpts := opts
		pageOpts.Top = pullRequestDiscoveryPageSize
		pageOpts.Skip = skip
		pullRequests, err := lister.ListPullRequests(ctx, pageOpts)
		if err != nil {
			return nil, fmt.Errorf("discovering pull requests at page %d: %w", page, err)
		}
		if len(pullRequests) == 0 {
			return discovered, nil
		}
		if len(pullRequests) > maxPullRequests-len(discovered) {
			return nil, fmt.Errorf("discovering pull requests: response exceeds maximum of %d pull requests", maxPullRequests)
		}
		for i, pullRequest := range pullRequests {
			if pullRequest.ID <= 0 {
				return nil, fmt.Errorf("discovering pull requests: response item %d on page %d has invalid pull request ID %d", i, page, pullRequest.ID)
			}
			if _, exists := seenIDs[pullRequest.ID]; exists {
				return nil, fmt.Errorf("discovering pull requests: duplicate pull request ID %d", pullRequest.ID)
			}
			seenIDs[pullRequest.ID] = struct{}{}
		}

		nextSkip := skip + len(pullRequests)
		if nextSkip <= skip {
			return nil, fmt.Errorf("discovering pull requests: pagination did not advance from skip %d", skip)
		}
		discovered = append(discovered, pullRequests...)
		skip = nextSkip
		if page == maxPullRequestPages {
			return nil, fmt.Errorf("discovering pull requests: response exceeds maximum of %d pages", maxPullRequestPages)
		}
	}

	return nil, fmt.Errorf("discovering pull requests: response exceeds maximum of %d pages", maxPullRequestPages)
}

func readPullRequestListResponseBody(body io.Reader, contentLength int64) ([]byte, error) {
	if contentLength > maxPullRequestResponseBytes {
		return nil, fmt.Errorf("response exceeds maximum size of %d bytes", maxPullRequestResponseBytes)
	}
	data, err := io.ReadAll(io.LimitReader(body, maxPullRequestResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if int64(len(data)) > maxPullRequestResponseBytes {
		return nil, fmt.Errorf("response exceeds maximum size of %d bytes", maxPullRequestResponseBytes)
	}
	return data, nil
}

func decodePullRequestListResponse(body []byte) ([]PullRequest, error) {
	var response struct {
		Value json.RawMessage `json:"value"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("response body contains multiple JSON values")
		}
		return nil, err
	}
	if len(response.Value) == 0 || bytes.Equal(bytes.TrimSpace(response.Value), []byte("null")) {
		return nil, fmt.Errorf("response is missing a pull request collection")
	}
	var pullRequests []PullRequest
	if err := json.Unmarshal(response.Value, &pullRequests); err != nil {
		return nil, fmt.Errorf("response contains invalid pull request collection: %w", err)
	}
	if pullRequests == nil {
		return nil, fmt.Errorf("response contains invalid pull request collection")
	}
	for i, pullRequest := range pullRequests {
		if pullRequest.ID <= 0 {
			return nil, fmt.Errorf("response item %d missing pull request ID", i)
		}
	}
	return pullRequests, nil
}
