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
