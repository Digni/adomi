package ado

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
)

type connectionDataResponse struct {
	AuthenticatedUser IdentityRef `json:"authenticatedUser"`
}

func (c *Client) FetchAuthenticatedIdentity(ctx context.Context) (*IdentityRef, error) {
	var response connectionDataResponse
	if err := c.doJSONWithOptions(
		ctx,
		http.MethodGet,
		c.connectionDataURL(),
		nil,
		&response,
		"fetching Azure DevOps authenticated identity",
		"decoding Azure DevOps authenticated identity",
		jsonRequestOptions{omitNilBody: true, statusError: responseError},
	); err != nil {
		return nil, err
	}
	if strings.TrimSpace(response.AuthenticatedUser.ID) == "" {
		return nil, fmt.Errorf("Azure DevOps authenticated identity response missing user ID")
	}
	return &response.AuthenticatedUser, nil
}

func (c *Client) connectionDataURL() string {
	u := *c.baseURL
	u.Path = path.Join(strings.TrimRight(u.Path, "/"), "_apis", "connectionData")
	u.RawQuery = ""
	return u.String()
}
