package ado

import (
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
