package ado

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
)

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

func (c *Client) CreateWorkItemComment(ctx context.Context, opts WorkItemCommentCreateOptions) (*WorkItemComment, error) {
	if opts.WorkItemID <= 0 {
		return nil, fmt.Errorf("work item comment request requires positive work item ID")
	}
	body := map[string]any{"text": opts.Text}
	var comment WorkItemComment
	if err := c.doJSON(ctx, http.MethodPost, c.workItemCommentsURL(opts.WorkItemID), body, &comment, "creating Azure DevOps work item comment", "decoding Azure DevOps work item comment"); err != nil {
		return nil, err
	}
	createdID := comment.CreatedID()
	if createdID <= 0 {
		return nil, fmt.Errorf("Azure DevOps work item comment response missing comment ID")
	}
	if comment.WorkItemID > 0 && comment.WorkItemID != opts.WorkItemID {
		return nil, fmt.Errorf("Azure DevOps work item comment response work item ID %d does not match requested ID %d", comment.WorkItemID, opts.WorkItemID)
	}
	return &comment, nil
}

func (c *Client) LinkWorkItemToPullRequest(ctx context.Context, workItemID, expectedRevision int, artifactURL string) (*WorkItem, error) {
	if workItemID <= 0 {
		return nil, fmt.Errorf("work item link request requires positive work item ID")
	}
	if expectedRevision <= 0 {
		return nil, fmt.Errorf("work item link request requires positive revision")
	}
	if strings.TrimSpace(artifactURL) == "" {
		return nil, fmt.Errorf("work item link request requires pull request artifact URL")
	}

	body := []map[string]any{
		{
			"op":    "test",
			"path":  "/rev",
			"value": expectedRevision,
		},
		{
			"op":   "add",
			"path": "/relations/-",
			"value": map[string]any{
				"rel": artifactLinkRelationType,
				"url": artifactURL,
				"attributes": map[string]any{
					"name": "Pull Request",
				},
			},
		},
	}
	var item WorkItem
	if err := c.doJSONWithOptions(
		ctx,
		http.MethodPatch,
		c.workItemRelationsURL(workItemID),
		body,
		&item,
		fmt.Sprintf("linking Azure DevOps work item %d to pull request", workItemID),
		fmt.Sprintf("decoding Azure DevOps work item %d link response", workItemID),
		jsonRequestOptions{contentType: "application/json-patch+json", statusError: responseError},
	); err != nil {
		return nil, err
	}
	if item.ID != workItemID {
		return nil, fmt.Errorf("Azure DevOps work item link response ID %d does not match requested ID %d", item.ID, workItemID)
	}
	if item.Rev <= 0 {
		return nil, fmt.Errorf("Azure DevOps work item %d link response missing positive revision", workItemID)
	}
	if !item.HasArtifactLink(artifactURL) {
		return nil, fmt.Errorf("Azure DevOps work item %d link response missing pull request ArtifactLink", workItemID)
	}
	return &item, nil
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

func (c *Client) workItemCommentsURL(id int) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "wit", "workitems", strconv.Itoa(id), "comments"}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("api-version", workItemCommentsPreviewAPIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) workItemRelationsURL(id int) string {
	u := *c.baseURL
	segments := []string{strings.TrimRight(u.Path, "/"), c.config.Project, "_apis", "wit", "workitems", strconv.Itoa(id)}
	u.Path = path.Join(segments...)
	query := u.Query()
	query.Set("$expand", "relations")
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}
