package ado

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const maxAttachmentBytes int64 = 64 * 1024 * 1024

type ClientConfig struct {
	BaseURL    string
	Project    string
	APIVersion string
	PAT        string
}

type Client struct {
	httpClient *http.Client
	config     ClientConfig
	baseURL    *url.URL
}

func NewHTTPClient(proxyURL string) (*http.Client, error) {
	transport := &http.Transport{}
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("parsing proxy URL: %w", err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("proxy URL must include scheme and host")
		}
		transport.Proxy = http.ProxyURL(parsed)
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}, nil
}

func NewClient(httpClient *http.Client, cfg ClientConfig) (*Client, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("http client is required")
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = "7.1"
	}
	baseURL, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parsing Azure DevOps base URL: %w", err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("Azure DevOps base URL must be absolute")
	}
	return &Client{
		httpClient: httpClient,
		config:     cfg,
		baseURL:    baseURL,
	}, nil
}

func (c *Client) FetchWorkItem(ctx context.Context, id int) (*WorkItem, error) {
	requestURL := c.workItemURL(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating work item request: %w", err)
	}
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching Azure DevOps work item %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, responseError("fetching Azure DevOps work item", id, resp)
	}

	var item WorkItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("decoding Azure DevOps work item %d: %w", id, err)
	}
	if item.ID != id {
		return nil, fmt.Errorf("Azure DevOps work item response ID %d does not match requested ID %d", item.ID, id)
	}
	return &item, nil
}

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

func (c *Client) doJSON(ctx context.Context, method, requestURL string, body any, target any, action, decodeAction string) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return responseError(action, 0, resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("%s: %w", decodeAction, err)
	}
	return nil
}

func (c *Client) Download(ctx context.Context, rawURL string) ([]byte, error) {
	if err := c.validateDownloadURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating attachment request: %w", err)
	}
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading Azure DevOps attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, responseError("downloading Azure DevOps attachment", 0, resp)
	}
	if resp.ContentLength > maxAttachmentBytes {
		return nil, fmt.Errorf("Azure DevOps attachment exceeds maximum size of %d bytes", maxAttachmentBytes)
	}

	data, err := readAttachmentBody(resp.Body, maxAttachmentBytes)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readAttachmentBody(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("reading Azure DevOps attachment: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("Azure DevOps attachment exceeds maximum size of %d bytes", limit)
	}
	return data, nil
}

func (c *Client) workItemURL(id int) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "wit", "workitems", strconv.Itoa(id)}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("$expand", "all")
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
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

func (c *Client) authorize(req *http.Request) {
	req.SetBasicAuth("", c.config.PAT)
}

func (c *Client) validateDownloadURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parsing attachment URL: %w", err)
	}
	if parsed.Scheme != c.baseURL.Scheme || parsed.Host != c.baseURL.Host {
		return fmt.Errorf("attachment URL host %q does not match Azure DevOps host %q", parsed.Host, c.baseURL.Host)
	}
	return nil
}

func responseError(action string, id int, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	suffix := strings.TrimSpace(string(body))
	if id > 0 {
		if suffix != "" {
			return fmt.Errorf("%s %d failed with status %d: %s", action, id, resp.StatusCode, suffix)
		}
		return fmt.Errorf("%s %d failed with status %d", action, id, resp.StatusCode)
	}
	if suffix != "" {
		return fmt.Errorf("%s failed with status %d: %s", action, resp.StatusCode, suffix)
	}
	return fmt.Errorf("%s failed with status %d", action, resp.StatusCode)
}
