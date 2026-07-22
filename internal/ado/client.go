package ado

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxAttachmentBytes                int64 = 64 * 1024 * 1024
	workItemCommentsPreviewAPIVersion       = "7.0-preview.3"
)

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

type redirectLocationGuard struct {
	transport *http.Transport
}

func (g *redirectLocationGuard) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := g.transport.RoundTrip(req)
	if err != nil || resp == nil || !isFollowedRedirect(resp.StatusCode) {
		return resp, err
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return resp, nil
	}
	if _, err := req.URL.Parse(location); err != nil {
		resp.Header.Del("Location")
	}
	return resp, nil
}

func (g *redirectLocationGuard) CloseIdleConnections() {
	g.transport.CloseIdleConnections()
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
		Transport: &redirectLocationGuard{transport: transport},
		Timeout:   60 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

func isFollowedRedirect(statusCode int) bool {
	switch statusCode {
	case http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
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

func (c *Client) doJSON(ctx context.Context, method, requestURL string, body any, target any, action, decodeAction string) error {
	return c.doJSONWithOptions(ctx, method, requestURL, body, target, action, decodeAction, jsonRequestOptions{
		statusError: responseError,
	})
}

type statusErrorFunc func(action string, id int, resp *http.Response) error

type jsonRequestOptions struct {
	omitNilBody bool
	contentType string
	statusError statusErrorFunc
}

func (c *Client) doJSONWithOptions(ctx context.Context, method, requestURL string, body any, target any, action, decodeAction string, opts jsonRequestOptions) error {
	var requestBody io.Reader
	if body != nil || !opts.omitNilBody {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		requestBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if body != nil || !opts.omitNilBody {
		contentType := opts.contentType
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return opts.statusError(action, 0, resp)
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s: %w", decodeAction, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%s: response body contains multiple JSON values", decodeAction)
		}
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
	return formatResponseError(action, id, resp, false)
}

func formatResponseError(action string, id int, resp *http.Response, suppressHTML bool) error {
	if isFollowedRedirect(resp.StatusCode) {
		if id > 0 {
			return fmt.Errorf("%s %d failed with status %d: Azure DevOps redirected the request; check the configured PAT and base URL", action, id, resp.StatusCode)
		}
		return fmt.Errorf("%s failed with status %d: Azure DevOps redirected the request; check the configured PAT and base URL", action, resp.StatusCode)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	suffix := strings.TrimSpace(string(body))
	if suppressHTML && isHTMLResponse(resp.Header.Get("Content-Type"), body) {
		suffix = ""
	}
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

func isHTMLResponse(contentType string, body []byte) bool {
	mediaType := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	if strings.EqualFold(mediaType, "text/html") {
		return true
	}
	return strings.HasPrefix(http.DetectContentType(body), "text/html")
}
