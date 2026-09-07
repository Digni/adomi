package ado

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	workItemCommentsReadVersion           = "7.1-preview.4"
	workItemCommentsPageSize              = 100
	maxWorkItemCommentPages               = 1000
	maxWorkItemComments                   = 100000
	maxWorkItemCommentResponseBytes int64 = 8 * 1024 * 1024
)

type WorkItemReadComment struct {
	ID           int                    `json:"id"`
	WorkItemID   int                    `json:"workItemId"`
	Text         string                 `json:"text"`
	Format       *string                `json:"format"`
	Author       *WorkItemCommentAuthor `json:"author"`
	CreatedDate  *string                `json:"createdDate"`
	ModifiedDate *string                `json:"modifiedDate"`
	Version      *int                   `json:"version"`
	URL          *string                `json:"url"`
}

type WorkItemCommentAuthor struct {
	ID          *string `json:"id"`
	DisplayName *string `json:"displayName"`
}

type WorkItemComments struct {
	WorkItemID int                   `json:"workItemId"`
	Comments   []WorkItemReadComment `json:"comments"`
}

type workItemCommentsPage struct {
	Comments          json.RawMessage `json:"comments"`
	ContinuationToken *string         `json:"continuationToken"`
}

type workItemCommentWire struct {
	ID           *int                   `json:"id"`
	CommentID    *int                   `json:"commentId"`
	WorkItemID   *int                   `json:"workItemId"`
	Text         *string                `json:"text"`
	Format       *string                `json:"format"`
	CreatedBy    *WorkItemCommentAuthor `json:"createdBy"`
	Author       *WorkItemCommentAuthor `json:"author"`
	CreatedDate  *string                `json:"createdDate"`
	ModifiedDate *string                `json:"modifiedDate"`
	Version      *int                   `json:"version"`
	URL          *string                `json:"url"`
	IsDeleted    bool                   `json:"isDeleted"`
}

type workItemCommentReadResult struct {
	Comment WorkItemReadComment
	Include bool
}

func (c *Client) ListWorkItemComments(ctx context.Context, workItemID int) ([]WorkItemReadComment, error) {
	if workItemID <= 0 {
		return nil, fmt.Errorf("work item comments request requires positive work item ID")
	}

	comments := make([]WorkItemReadComment, 0)
	seenCommentIDs := make(map[int]struct{})
	seenTokens := make(map[string]struct{})
	continuationToken := ""
	for page := 1; page <= maxWorkItemCommentPages; page++ {
		body, err := c.listWorkItemCommentsPage(ctx, workItemID, continuationToken)
		if err != nil {
			return nil, fmt.Errorf("listing Azure DevOps work item %d comments at page %d: %w", workItemID, page, err)
		}
		pageComments, nextToken, err := decodeWorkItemCommentsPage(body, workItemID)
		if err != nil {
			return nil, fmt.Errorf("decoding Azure DevOps work item %d comments at page %d: %w", workItemID, page, err)
		}
		if len(pageComments) > maxWorkItemComments-len(seenCommentIDs) {
			return nil, fmt.Errorf("listing Azure DevOps work item %d comments: response exceeds maximum of %d comments", workItemID, maxWorkItemComments)
		}
		for _, result := range pageComments {
			if _, exists := seenCommentIDs[result.Comment.ID]; exists {
				return nil, fmt.Errorf("listing Azure DevOps work item %d comments: duplicate comment ID %d", workItemID, result.Comment.ID)
			}
			seenCommentIDs[result.Comment.ID] = struct{}{}
			if result.Include {
				comments = append(comments, result.Comment)
			}
		}

		if nextToken == "" {
			return comments, nil
		}
		if _, exists := seenTokens[nextToken]; exists {
			return nil, fmt.Errorf("listing Azure DevOps work item %d comments: continuation token repeated", workItemID)
		}
		seenTokens[nextToken] = struct{}{}
		if page == maxWorkItemCommentPages {
			return nil, fmt.Errorf("listing Azure DevOps work item %d comments: response exceeds maximum of %d pages", workItemID, maxWorkItemCommentPages)
		}
		continuationToken = nextToken
	}

	return nil, fmt.Errorf("listing Azure DevOps work item %d comments: response exceeds maximum of %d pages", workItemID, maxWorkItemCommentPages)
}

func (c *Client) listWorkItemCommentsPage(ctx context.Context, workItemID int, continuationToken string) ([]byte, error) {
	requestURL := c.workItemCommentsReadURL(workItemID, continuationToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	c.authorize(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, responseError("listing Azure DevOps work item comments", workItemID, resp)
	}
	if resp.ContentLength > maxWorkItemCommentResponseBytes {
		return nil, fmt.Errorf("response exceeds maximum size of %d bytes", maxWorkItemCommentResponseBytes)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxWorkItemCommentResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if int64(len(body)) > maxWorkItemCommentResponseBytes {
		return nil, fmt.Errorf("response exceeds maximum size of %d bytes", maxWorkItemCommentResponseBytes)
	}
	return body, nil
}

func (c *Client) workItemCommentsReadURL(workItemID int, continuationToken string) string {
	u := *c.baseURL
	setURLPathSegments(&u, c.config.Project, "_apis", "wit", "workitems", strconv.Itoa(workItemID), "comments")
	query := u.Query()
	query.Set("$top", strconv.Itoa(workItemCommentsPageSize))
	query.Set("order", "asc")
	query.Set("includeDeleted", "false")
	query.Set("api-version", workItemCommentsReadVersion)
	if continuationToken != "" {
		query.Set("continuationToken", continuationToken)
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func decodeWorkItemCommentsPage(body []byte, workItemID int) ([]workItemCommentReadResult, string, error) {
	var response workItemCommentsPage
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&response); err != nil {
		return nil, "", err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, "", fmt.Errorf("response body contains multiple JSON values")
		}
		return nil, "", err
	}
	if len(response.Comments) == 0 {
		return nil, "", fmt.Errorf("response is missing comments collection")
	}
	if bytes.Equal(bytes.TrimSpace(response.Comments), []byte("null")) {
		return nil, "", fmt.Errorf("response contains invalid comments collection")
	}
	var rawComments []json.RawMessage
	if err := json.Unmarshal(response.Comments, &rawComments); err != nil || rawComments == nil {
		if err != nil {
			return nil, "", fmt.Errorf("response contains invalid comments collection: %w", err)
		}
		return nil, "", fmt.Errorf("response contains invalid comments collection")
	}
	comments := make([]workItemCommentReadResult, 0, len(rawComments))
	for i, rawComment := range rawComments {
		comment, include, err := normalizeWorkItemReadComment(rawComment, workItemID)
		if err != nil {
			return nil, "", fmt.Errorf("comment at index %d: %w", i, err)
		}
		comments = append(comments, workItemCommentReadResult{Comment: comment, Include: include})
	}
	nextToken, err := normalizeWorkItemCommentsToken(response.ContinuationToken)
	if err != nil {
		return nil, "", err
	}
	return comments, nextToken, nil
}

func normalizeWorkItemCommentsToken(token *string) (string, error) {
	if token == nil {
		return "", nil
	}
	if err := validateWorkItemCommentsContinuation(*token); err != nil {
		return "", err
	}
	return *token, nil
}

func normalizeWorkItemReadComment(raw []byte, workItemID int) (WorkItemReadComment, bool, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return WorkItemReadComment{}, false, fmt.Errorf("comment is null")
	}
	var wire workItemCommentWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return WorkItemReadComment{}, false, err
	}
	commentID, err := normalizeWorkItemCommentID(wire.ID, wire.CommentID)
	if err != nil {
		return WorkItemReadComment{}, false, err
	}
	if wire.WorkItemID == nil || *wire.WorkItemID != workItemID {
		return WorkItemReadComment{}, false, fmt.Errorf("work item ID does not match requested ID %d", workItemID)
	}
	if wire.IsDeleted {
		return WorkItemReadComment{ID: commentID, WorkItemID: *wire.WorkItemID}, false, nil
	}
	if wire.Text == nil {
		return WorkItemReadComment{}, false, fmt.Errorf("comment text is missing")
	}
	author := wire.CreatedBy
	if author == nil {
		author = wire.Author
	}
	return WorkItemReadComment{
		ID:           commentID,
		WorkItemID:   *wire.WorkItemID,
		Text:         *wire.Text,
		Format:       wire.Format,
		Author:       author,
		CreatedDate:  wire.CreatedDate,
		ModifiedDate: wire.ModifiedDate,
		Version:      wire.Version,
		URL:          wire.URL,
	}, true, nil
}

func normalizeWorkItemCommentID(id, commentID *int) (int, error) {
	if id == nil && commentID == nil {
		return 0, fmt.Errorf("comment ID is missing")
	}
	if id != nil && commentID != nil && *id != *commentID {
		return 0, fmt.Errorf("comment ID fields conflict: id=%d commentId=%d", *id, *commentID)
	}
	value := id
	if value == nil {
		value = commentID
	}
	if *value <= 0 {
		return 0, fmt.Errorf("comment ID must be positive")
	}
	return *value, nil
}

func validateWorkItemCommentsContinuation(token string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("continuation token is blank")
	}
	return nil
}
