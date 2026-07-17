package ado

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

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
