package ado

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Wiki struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	ProjectID    string                 `json:"projectId,omitempty"`
	RepositoryID string                 `json:"repositoryId,omitempty"`
	MappedPath   string                 `json:"mappedPath,omitempty"`
	Versions     []GitVersionDescriptor `json:"versions,omitempty"`
	URL          string                 `json:"url,omitempty"`
	RemoteURL    string                 `json:"remoteUrl,omitempty"`
	Type         string                 `json:"type,omitempty"`
	IsDisabled   bool                   `json:"isDisabled,omitempty"`
	Properties   map[string]any         `json:"properties,omitempty"`
}

type GitVersionDescriptor struct {
	Version        string `json:"version,omitempty"`
	VersionOptions string `json:"versionOptions,omitempty"`
	VersionType    string `json:"versionType,omitempty"`
}

type WikiPage struct {
	ID              int        `json:"id,omitempty"`
	Path            string     `json:"path"`
	Order           int        `json:"order,omitempty"`
	IsParentPage    bool       `json:"isParentPage,omitempty"`
	IsNonConformant bool       `json:"isNonConformant,omitempty"`
	GitItemPath     string     `json:"gitItemPath,omitempty"`
	Content         string     `json:"content"`
	SubPages        []WikiPage `json:"subPages,omitempty"`
	URL             string     `json:"url,omitempty"`
	RemoteURL       string     `json:"remoteUrl,omitempty"`
}

type WikiPageFetchOptions struct {
	Path           string
	RecursionLevel string
	IncludeContent bool
}

type WikiFetcher interface {
	ResolveWiki(ctx context.Context, identifier string) (*Wiki, error)
	FetchWikiPage(ctx context.Context, wikiIdentifier string, opts WikiPageFetchOptions) (*WikiPage, error)
}

func (c *Client) ResolveWiki(ctx context.Context, identifier string) (*Wiki, error) {
	var wiki Wiki
	if err := c.doJSONWithOptions(ctx, http.MethodGet, c.wikiURL(identifier), nil, &wiki, "resolving Azure DevOps wiki", "decoding Azure DevOps wiki", wikiJSONRequestOptions()); err != nil {
		return nil, err
	}
	if strings.TrimSpace(wiki.ID) == "" {
		return nil, fmt.Errorf("Azure DevOps wiki response missing canonical wiki ID")
	}
	return &wiki, nil
}

func (c *Client) FetchWikiPage(ctx context.Context, wikiIdentifier string, opts WikiPageFetchOptions) (*WikiPage, error) {
	if !strings.HasPrefix(opts.Path, "/") {
		return nil, fmt.Errorf("Azure DevOps wiki page request requires an absolute path")
	}
	var page WikiPage
	if err := c.doJSONWithOptions(ctx, http.MethodGet, c.wikiPageURL(wikiIdentifier, opts), nil, &page, "fetching Azure DevOps wiki page", "decoding Azure DevOps wiki page", wikiJSONRequestOptions()); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(page.Path, "/") {
		return nil, fmt.Errorf("Azure DevOps wiki page response missing absolute path")
	}
	if page.Path != opts.Path {
		return nil, fmt.Errorf("Azure DevOps wiki page response path %q does not match requested path %q", page.Path, opts.Path)
	}
	return &page, nil
}

func wikiJSONRequestOptions() jsonRequestOptions {
	return jsonRequestOptions{
		omitNilBody: true,
		statusError: wikiResponseError,
	}
}

func wikiResponseError(action string, id int, resp *http.Response) error {
	return formatResponseError(action, id, resp, true)
}

func (c *Client) wikiURL(identifier string) string {
	u := *c.baseURL
	setURLPathSegments(&u, c.config.Project, "_apis", "wiki", "wikis", identifier)
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) wikiPageURL(identifier string, opts WikiPageFetchOptions) string {
	u := *c.baseURL
	setURLPathSegments(&u, c.config.Project, "_apis", "wiki", "wikis", identifier, "pages")
	query := u.Query()
	query.Set("path", opts.Path)
	query.Set("includeContent", strconv.FormatBool(opts.IncludeContent))
	if opts.RecursionLevel != "" {
		query.Set("recursionLevel", opts.RecursionLevel)
	}
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func setURLPathSegments(u *url.URL, segments ...string) {
	decodedPath := strings.TrimRight(u.Path, "/")
	escapedPath := strings.TrimRight(u.EscapedPath(), "/")
	for _, segment := range segments {
		decodedPath += "/" + segment
		escapedPath += "/" + url.PathEscape(segment)
	}
	u.Path = decodedPath
	u.RawPath = escapedPath
}
