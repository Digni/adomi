package ado

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
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

func (c *Client) validatePipelineTransport() error {
	scheme := c.baseURL.Scheme
	if strings.EqualFold(scheme, "https") {
		return nil
	}
	if !strings.EqualFold(scheme, "http") {
		return fmt.Errorf("Azure DevOps pipeline requests require HTTPS or loopback HTTP")
	}

	if isPipelineLoopbackHost(c.baseURL.Hostname()) {
		return nil
	}
	return fmt.Errorf("Azure DevOps pipeline requests require HTTPS or loopback HTTP")
}

func isPipelineLoopbackHost(hostname string) bool {
	if strings.EqualFold(hostname, "localhost") {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}

func (c *Client) pipelineGET(ctx context.Context, httpClient *http.Client, requestURL, action string) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: could not create request", action)
	}
	c.authorize(req)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: request failed; verify Azure DevOps connectivity and configuration", action)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		if isFollowedRedirect(resp.StatusCode) {
			return nil, nil, fmt.Errorf("%s: Azure DevOps returned redirect status %d; verify the base URL and credentials", action, resp.StatusCode)
		}
		return nil, nil, fmt.Errorf("%s: Azure DevOps returned HTTP status %d; verify the project, permissions, and credentials", action, resp.StatusCode)
	}
	if resp.ContentLength > maxPipelineResponseBytes {
		return nil, nil, fmt.Errorf("%s: response exceeds maximum size of %d bytes", action, maxPipelineResponseBytes)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPipelineResponseBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: could not read response", action)
	}
	if int64(len(body)) > maxPipelineResponseBytes {
		return nil, nil, fmt.Errorf("%s: response exceeds maximum size of %d bytes", action, maxPipelineResponseBytes)
	}
	return body, resp.Header.Clone(), nil
}

func (c *Client) pipelineHTTPClient() (*http.Client, bool) {
	httpClient := *c.httpClient
	httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if !strings.EqualFold(c.baseURL.Scheme, "http") || !isPipelineLoopbackHost(c.baseURL.Hostname()) {
		return &httpClient, false
	}

	ownsTransport := false
	switch transport := httpClient.Transport.(type) {
	case nil:
		if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
			clone := defaultTransport.Clone()
			clone.Proxy = nil
			httpClient.Transport = clone
			ownsTransport = true
		}
	case *http.Transport:
		clone := transport.Clone()
		clone.Proxy = nil
		httpClient.Transport = clone
		ownsTransport = true
	case *redirectLocationGuard:
		guard := *transport
		clone := transport.transport.Clone()
		clone.Proxy = nil
		guard.transport = clone
		httpClient.Transport = &guard
		ownsTransport = true
	}
	return &httpClient, ownsTransport
}

func (c *Client) pipelineRunsURL(continuationToken string) string {
	u := *c.baseURL
	setURLPathSegments(&u, c.config.Project, "_apis", "build", "builds")
	query := u.Query()
	query.Set("statusFilter", "inProgress")
	query.Set("queryOrder", "queueTimeDescending")
	if continuationToken != "" {
		query.Set("continuationToken", continuationToken)
	}
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) pipelineRunURL(id int) string {
	u := *c.baseURL
	setURLPathSegments(&u, c.config.Project, "_apis", "build", "builds", strconv.Itoa(id))
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func decodePipelineRunList(body []byte, maxItems int) ([]pipelineRunResponse, error) {
	var response map[string]json.RawMessage
	if err := decodePipelineJSON(body, &response, "listing Azure DevOps pipeline runs"); err != nil {
		return nil, err
	}
	value, ok := response["value"]
	if !ok {
		return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response is missing value")
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return []pipelineRunResponse{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	token, err := decoder.Token()
	opening, ok := token.(json.Delim)
	if err != nil || !ok || opening != '[' {
		return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response contains invalid value")
	}
	capacity := maxItems
	if capacity > 1024 {
		capacity = 1024
	}
	runs := make([]pipelineRunResponse, 0, capacity)
	for decoder.More() {
		if len(runs) >= maxItems {
			return nil, errPipelineRunLimitExceeded
		}
		var run pipelineRunResponse
		if err := decoder.Decode(&run); err != nil {
			return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response contains invalid value")
		}
		runs = append(runs, run)
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim(']') {
		return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response contains invalid value")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response contains invalid value")
	}
	return runs, nil
}

func decodePipelineJSON(body []byte, target any, action string) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s: response contains invalid JSON", action)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%s: response must contain exactly one JSON value", action)
	}
	return nil
}

func pipelineContinuationToken(header http.Header) (string, bool, error) {
	values := header.Values("X-MS-ContinuationToken")
	if len(values) == 0 {
		return "", false, nil
	}
	if len(values) != 1 {
		return "", false, fmt.Errorf("listing Azure DevOps pipeline runs: response contains multiple continuation tokens")
	}
	if strings.TrimSpace(values[0]) == "" {
		return "", false, fmt.Errorf("listing Azure DevOps pipeline runs: response contains a blank continuation token")
	}
	return values[0], true, nil
}

func normalizePipelineRun(response pipelineRunResponse) (PipelineRun, error) {
	if response.ID < 1 || response.ID > maxPipelineRunID {
		return PipelineRun{}, fmt.Errorf("run ID must be between 1 and %d", maxPipelineRunID)
	}
	if response.Definition.ID < 1 || response.Definition.ID > maxPipelineRunID {
		return PipelineRun{}, fmt.Errorf("pipeline ID must be between 1 and %d", maxPipelineRunID)
	}
	if strings.TrimSpace(response.Definition.Name) == "" {
		return PipelineRun{}, fmt.Errorf("pipeline name is required")
	}
	if strings.TrimSpace(response.BuildNumber) == "" {
		return PipelineRun{}, fmt.Errorf("run number is required")
	}
	if strings.TrimSpace(response.Status) == "" {
		return PipelineRun{}, fmt.Errorf("status is required")
	}

	result := normalizePipelineOptionalString(response.Result)
	if result != nil && *result == "none" {
		result = nil
	}
	if response.Status == "completed" && result == nil {
		return PipelineRun{}, fmt.Errorf("completed run requires a result")
	}
	if response.Status != "completed" && result != nil {
		return PipelineRun{}, fmt.Errorf("non-completed run must not contain a result")
	}

	queueTime, err := normalizePipelineTime(response.QueueTime)
	if err != nil {
		return PipelineRun{}, fmt.Errorf("queue time is invalid")
	}
	startTime, err := normalizePipelineTime(response.StartTime)
	if err != nil {
		return PipelineRun{}, fmt.Errorf("start time is invalid")
	}
	finishTime, err := normalizePipelineTime(response.FinishTime)
	if err != nil {
		return PipelineRun{}, fmt.Errorf("finish time is invalid")
	}

	return PipelineRun{
		ID:            response.ID,
		PipelineID:    response.Definition.ID,
		PipelineName:  response.Definition.Name,
		RunNumber:     response.BuildNumber,
		Status:        response.Status,
		Result:        result,
		SourceBranch:  normalizePipelineOptionalString(response.SourceBranch),
		SourceVersion: normalizePipelineOptionalString(response.SourceVersion),
		QueueTime:     queueTime,
		StartTime:     startTime,
		FinishTime:    finishTime,
		WebURL:        normalizePipelineOptionalString(response.Links.Web.Href),
	}, nil
}

func normalizePipelineOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizePipelineTime(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}
