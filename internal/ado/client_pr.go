package ado

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

func (c *Client) FetchPullRequest(ctx context.Context, id int) (*PullRequest, error) {
	requestURL := c.pullRequestURL(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating pull request request: %w", err)
	}
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching Azure DevOps pull request %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, responseError("fetching Azure DevOps pull request", id, resp)
	}

	var pr PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decoding Azure DevOps pull request %d: %w", id, err)
	}
	if pr.ID != id {
		return nil, fmt.Errorf("Azure DevOps pull request response ID %d does not match requested ID %d", pr.ID, id)
	}
	return &pr, nil
}

func (c *Client) FetchPullRequestThreads(ctx context.Context, repositoryID string, pullRequestID int) ([]PullRequestThread, error) {
	if strings.TrimSpace(repositoryID) == "" {
		return nil, fmt.Errorf("pull request threads request requires repository ID")
	}
	requestURL := c.pullRequestThreadsURL(repositoryID, pullRequestID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating pull request threads request: %w", err)
	}
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching Azure DevOps pull request %d threads: %w", pullRequestID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, responseError("fetching Azure DevOps pull request threads", pullRequestID, resp)
	}

	var threads PullRequestThreadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&threads); err != nil {
		return nil, fmt.Errorf("decoding Azure DevOps pull request %d threads: %w", pullRequestID, err)
	}
	if threads.Value == nil {
		return []PullRequestThread{}, nil
	}
	return threads.Value, nil
}

func (c *Client) ListPullRequests(ctx context.Context, opts PullRequestListOptions) ([]PullRequest, error) {
	if strings.TrimSpace(opts.RepositoryID) == "" {
		return nil, fmt.Errorf("pull request list request requires repository ID")
	}
	status := strings.TrimSpace(opts.Status)
	if status == "" {
		status = "active"
	}
	requestURL := c.pullRequestsURL(opts.RepositoryID)
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("parsing pull request list URL: %w", err)
	}
	query := parsed.Query()
	query.Set("searchCriteria.status", status)
	if opts.SourceRefName != "" {
		query.Set("searchCriteria.sourceRefName", opts.SourceRefName)
	}
	if opts.TargetRefName != "" {
		query.Set("searchCriteria.targetRefName", opts.TargetRefName)
	}
	parsed.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating pull request list request: %w", err)
	}
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing Azure DevOps pull requests: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, responseError("listing Azure DevOps pull requests", 0, resp)
	}

	var prs PullRequestsResponse
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, fmt.Errorf("decoding Azure DevOps pull requests: %w", err)
	}
	if prs.Value == nil {
		return []PullRequest{}, nil
	}
	for i, pr := range prs.Value {
		if pr.ID <= 0 {
			return nil, fmt.Errorf("Azure DevOps pull request list response item %d missing pull request ID", i)
		}
	}
	return prs.Value, nil
}

func (c *Client) CreatePullRequest(ctx context.Context, opts PullRequestCreateOptions) (*PullRequest, error) {
	if strings.TrimSpace(opts.RepositoryID) == "" {
		return nil, fmt.Errorf("pull request create request requires repository ID")
	}
	body := map[string]any{
		"sourceRefName": opts.SourceRefName,
		"targetRefName": opts.TargetRefName,
		"title":         opts.Title,
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	var pr PullRequest
	if err := c.doJSON(ctx, http.MethodPost, c.pullRequestsURL(opts.RepositoryID), body, &pr, "creating Azure DevOps pull request", "decoding Azure DevOps pull request create response"); err != nil {
		return nil, err
	}
	if pr.ID <= 0 {
		return nil, fmt.Errorf("Azure DevOps pull request create response missing pull request ID")
	}
	return &pr, nil
}

func (c *Client) UpdatePullRequest(ctx context.Context, opts PullRequestUpdateOptions) (*PullRequest, error) {
	if strings.TrimSpace(opts.RepositoryID) == "" {
		return nil, fmt.Errorf("pull request update request requires repository ID")
	}
	body := map[string]any{}
	if opts.Title != nil {
		body["title"] = *opts.Title
	}
	if opts.Description != nil {
		body["description"] = *opts.Description
	}
	var pr PullRequest
	if err := c.doJSON(ctx, http.MethodPatch, c.pullRequestRepoURL(opts.RepositoryID, opts.PullRequestID), body, &pr, "updating Azure DevOps pull request", "decoding Azure DevOps pull request update response"); err != nil {
		return nil, err
	}
	if pr.ID <= 0 {
		return nil, fmt.Errorf("Azure DevOps pull request update response missing pull request ID")
	}
	if pr.ID != opts.PullRequestID {
		return nil, fmt.Errorf("Azure DevOps pull request update response ID %d does not match requested ID %d", pr.ID, opts.PullRequestID)
	}
	return &pr, nil
}

func (c *Client) ListPullRequestIterations(ctx context.Context, repositoryID string, pullRequestID int) ([]PullRequestIteration, error) {
	if strings.TrimSpace(repositoryID) == "" {
		return nil, fmt.Errorf("pull request iterations request requires repository ID")
	}
	var response PullRequestIterationsResponse
	if err := c.doJSON(ctx, http.MethodGet, c.pullRequestIterationsURL(repositoryID, pullRequestID), nil, &response, "fetching Azure DevOps pull request iterations", "decoding Azure DevOps pull request iterations"); err != nil {
		return nil, err
	}
	return response.Value, nil
}

func (c *Client) ListPullRequestIterationChanges(ctx context.Context, opts PullRequestIterationChangesOptions) ([]PullRequestIterationChange, error) {
	if strings.TrimSpace(opts.RepositoryID) == "" {
		return nil, fmt.Errorf("pull request iteration changes request requires repository ID")
	}
	top := opts.Top
	if top <= 0 {
		top = 2000
	}
	var all []PullRequestIterationChange
	seenSkips := map[int]bool{}
	for skip := 0; ; {
		var response PullRequestIterationChangesResponse
		if err := c.doJSON(ctx, http.MethodGet, c.pullRequestIterationChangesURL(opts.RepositoryID, opts.PullRequestID, opts.IterationID, opts.CompareTo, top, skip), nil, &response, "fetching Azure DevOps pull request iteration changes", "decoding Azure DevOps pull request iteration changes"); err != nil {
			return nil, err
		}
		all = append(all, response.ChangeEntries...)
		if response.NextSkip <= 0 || response.NextTop <= 0 {
			break
		}
		if response.NextSkip <= skip || seenSkips[response.NextSkip] {
			return nil, fmt.Errorf("Azure DevOps pull request iteration changes pagination did not advance from skip %d to %d", skip, response.NextSkip)
		}
		seenSkips[skip] = true
		skip = response.NextSkip
		top = response.NextTop
	}
	return all, nil
}

func (c *Client) CreatePullRequestThread(ctx context.Context, opts PullRequestThreadCreateOptions) (*PullRequestThread, error) {
	if strings.TrimSpace(opts.RepositoryID) == "" {
		return nil, fmt.Errorf("pull request thread create request requires repository ID")
	}
	if (opts.ThreadContext == nil) != (opts.PullRequestThreadContext == nil) {
		return nil, fmt.Errorf("inline pull request thread create request requires both threadContext and pullRequestThreadContext")
	}
	body := map[string]any{
		"comments": []map[string]any{{
			"parentCommentId": 0,
			"content":         opts.Content,
			"commentType":     "text",
		}},
		"status": "active",
	}
	if opts.ThreadContext != nil {
		body["threadContext"] = opts.ThreadContext
	}
	if opts.PullRequestThreadContext != nil {
		body["pullRequestThreadContext"] = opts.PullRequestThreadContext
	}
	var thread PullRequestThread
	if err := c.doJSON(ctx, http.MethodPost, c.pullRequestThreadCollectionURL(opts.RepositoryID, opts.PullRequestID), body, &thread, "creating Azure DevOps pull request thread", "decoding Azure DevOps pull request thread create response"); err != nil {
		return nil, err
	}
	if thread.ID <= 0 {
		return nil, fmt.Errorf("Azure DevOps pull request thread response missing thread ID")
	}
	return &thread, nil
}

func (c *Client) CreatePullRequestThreadComment(ctx context.Context, opts PullRequestThreadCommentCreateOptions) (*PullRequestComment, error) {
	if strings.TrimSpace(opts.RepositoryID) == "" {
		return nil, fmt.Errorf("pull request thread comment request requires repository ID")
	}
	body := map[string]any{
		"content":     opts.Content,
		"commentType": "text",
	}
	var comment PullRequestComment
	if err := c.doJSON(ctx, http.MethodPost, c.pullRequestThreadCommentsURL(opts.RepositoryID, opts.PullRequestID, opts.ThreadID), body, &comment, "creating Azure DevOps pull request thread comment", "decoding Azure DevOps pull request thread comment"); err != nil {
		return nil, err
	}
	if comment.ID <= 0 {
		return nil, fmt.Errorf("Azure DevOps pull request thread comment response missing comment ID")
	}
	return &comment, nil
}

func (c *Client) UpdatePullRequestThread(ctx context.Context, opts PullRequestThreadUpdateOptions) (*PullRequestThread, error) {
	if strings.TrimSpace(opts.RepositoryID) == "" {
		return nil, fmt.Errorf("pull request thread update request requires repository ID")
	}
	body := map[string]any{"status": opts.Status}
	var thread PullRequestThread
	if err := c.doJSON(ctx, http.MethodPatch, c.pullRequestThreadURL(opts.RepositoryID, opts.PullRequestID, opts.ThreadID), body, &thread, "updating Azure DevOps pull request thread", "decoding Azure DevOps pull request thread update"); err != nil {
		return nil, err
	}
	if thread.ID <= 0 {
		return nil, fmt.Errorf("Azure DevOps pull request thread update response missing thread ID")
	}
	if thread.ID != opts.ThreadID {
		return nil, fmt.Errorf("Azure DevOps pull request thread update response ID %d does not match requested ID %d", thread.ID, opts.ThreadID)
	}
	return &thread, nil
}

func (c *Client) pullRequestURL(id int) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "git", "pullrequests", strconv.Itoa(id)}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) pullRequestThreadsURL(repositoryID string, pullRequestID int) string {
	return c.pullRequestThreadCollectionURL(repositoryID, pullRequestID)
}

func (c *Client) pullRequestsURL(repositoryID string) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "git", "repositories", repositoryID, "pullrequests"}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) pullRequestRepoURL(repositoryID string, pullRequestID int) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "git", "repositories", repositoryID, "pullrequests", strconv.Itoa(pullRequestID)}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) pullRequestIterationsURL(repositoryID string, pullRequestID int) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "git", "repositories", repositoryID, "pullrequests", strconv.Itoa(pullRequestID), "iterations"}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) pullRequestIterationChangesURL(repositoryID string, pullRequestID int, iterationID int, compareTo int, top int, skip int) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "git", "repositories", repositoryID, "pullrequests", strconv.Itoa(pullRequestID), "iterations", strconv.Itoa(iterationID), "changes"}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	query.Set("$compareTo", strconv.Itoa(compareTo))
	query.Set("$top", strconv.Itoa(top))
	query.Set("$skip", strconv.Itoa(skip))
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) pullRequestThreadCollectionURL(repositoryID string, pullRequestID int) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "git", "repositories", repositoryID, "pullrequests", strconv.Itoa(pullRequestID), "threads"}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) pullRequestThreadURL(repositoryID string, pullRequestID int, threadID int) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "git", "repositories", repositoryID, "pullrequests", strconv.Itoa(pullRequestID), "threads", strconv.Itoa(threadID)}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) pullRequestThreadCommentsURL(repositoryID string, pullRequestID int, threadID int) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "git", "repositories", repositoryID, "pullrequests", strconv.Itoa(pullRequestID), "threads", strconv.Itoa(threadID), "comments"}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}
