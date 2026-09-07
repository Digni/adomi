package ado

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientListWorkItemCommentsPagesAndPreservesMetadata(t *testing.T) {
	const (
		workItemID = 12345
		pat        = "secret"
	)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := int(requests.Add(1))
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		if r.URL.EscapedPath() != "/tfs/DefaultCollection/My%20Project/_apis/wit/workitems/12345/comments" {
			t.Fatalf("path = %q, want comments endpoint", r.URL.EscapedPath())
		}
		query := r.URL.Query()
		if query.Get("api-version") != workItemCommentsReadVersion || query.Get("$top") != "100" || query.Get("order") != "asc" || query.Get("includeDeleted") != "false" {
			t.Fatalf("query = %v, want endpoint version, top, ascending order, and non-deleted selection", query)
		}
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+pat))
		if r.Header.Get("Authorization") != wantAuth {
			t.Fatalf("Authorization = %q, want %q", r.Header.Get("Authorization"), wantAuth)
		}
		switch request {
		case 1:
			if query.Get("continuationToken") != "" {
				t.Fatalf("first continuation token = %q, want absent", query.Get("continuationToken"))
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"comments":[`+
				`{"id":11,"commentId":11,"workItemId":12345,"text":"first","format":"markdown","createdBy":{"id":"user-1","displayName":"Ada"},"createdDate":"2026-09-01T10:00:00Z","modifiedDate":"2026-09-01T10:05:00Z","version":2,"url":"https://dev.azure.com/org/comment/11"},`+
				`{"id":12,"workItemId":12345,"text":"deleted secret","isDeleted":true}`+
				`],"continuationToken":"opaque token/+?=","nextPage":"https://attacker.invalid/comments"}`)
		case 2:
			if query.Get("continuationToken") != "opaque token/+?=" {
				t.Fatalf("second continuation token = %q, want opaque token", query.Get("continuationToken"))
			}
			fmt.Fprint(w, `{"comments":[{"commentId":13,"workItemId":12345,"text":"second","author":{"id":"user-2","displayName":"Grace"},"version":1}]}`)
		default:
			t.Fatalf("unexpected request %d", request)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL + "/tfs/DefaultCollection", Project: "My Project", APIVersion: "7.0", PAT: pat})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	comments, err := client.ListWorkItemComments(context.Background(), workItemID)
	if err != nil {
		t.Fatalf("ListWorkItemComments returned error: %v", err)
	}
	if len(comments) != 2 || comments[0].ID != 11 || comments[1].ID != 13 {
		t.Fatalf("comments = %#v, want current IDs [11 13]", comments)
	}
	first := comments[0]
	if first.WorkItemID != workItemID || first.Text != "first" || first.Format == nil || *first.Format != "markdown" || first.Version == nil || *first.Version != 2 {
		t.Fatalf("first comment = %#v, want preserved metadata", first)
	}
	if first.Author == nil || first.Author.ID == nil || *first.Author.ID != "user-1" || first.Author.DisplayName == nil || *first.Author.DisplayName != "Ada" {
		t.Fatalf("first author = %#v, want Ada identity", first.Author)
	}
	if comments[1].Author == nil || comments[1].Author.DisplayName == nil || *comments[1].Author.DisplayName != "Grace" {
		t.Fatalf("second author = %#v, want Grace identity", comments[1].Author)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}

	data, err := json.Marshal(WorkItemComments{WorkItemID: workItemID, Comments: comments})
	if err != nil {
		t.Fatalf("marshaling comments wrapper: %v", err)
	}
	if !strings.Contains(string(data), `"workItemId":12345`) || !strings.Contains(string(data), `"comments":[`) {
		t.Fatalf("wrapper JSON = %s, want stable workItemId/comments fields", data)
	}
}

func TestClientListWorkItemCommentsReturnsNonNilEmptyCollection(t *testing.T) {
	client := newWorkItemCommentsTestClient(t, `{"comments":[]}`)
	comments, err := client.ListWorkItemComments(context.Background(), 12345)
	if err != nil {
		t.Fatalf("ListWorkItemComments returned error: %v", err)
	}
	if comments == nil || len(comments) != 0 {
		t.Fatalf("comments = %#v, want non-nil empty slice", comments)
	}
}

func TestClientListWorkItemCommentsRejectsInvalidIdentityAndCollections(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing collection", body: `{"count":0}`, want: "missing comments collection"},
		{name: "null collection", body: `{"comments":null}`, want: "invalid comments collection"},
		{name: "missing comment ID", body: `{"comments":[{"workItemId":12345,"text":"x"}]}`, want: "comment ID is missing"},
		{name: "conflicting IDs", body: `{"comments":[{"id":1,"commentId":2,"workItemId":12345,"text":"x"}]}`, want: "fields conflict"},
		{name: "wrong work item", body: `{"comments":[{"id":1,"workItemId":999,"text":"x"}]}`, want: "does not match requested ID"},
		{name: "missing text", body: `{"comments":[{"id":1,"workItemId":12345}]}`, want: "comment text is missing"},
		{name: "invalid collection", body: `{"comments":{}}`, want: "invalid comments collection"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newWorkItemCommentsTestClient(t, tt.body)
			comments, err := client.ListWorkItemComments(context.Background(), 12345)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("comments/error = %#v/%v, want %q", comments, err, tt.want)
			}
			if comments != nil {
				t.Fatalf("comments = %#v, want nil on error", comments)
			}
		})
	}
}

func TestClientListWorkItemCommentsRejectsInvalidContinuationTokens(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "blank", body: `{"comments":[],"continuationToken":"   "}`, want: "continuation token is blank"},
		{name: "repeated", body: `{"comments":[],"continuationToken":"cycle"}`, want: "continuation token repeated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			client := newWorkItemCommentsTestClientWithTransport(t, func(req *http.Request) (*http.Response, error) {
				request := int(requests.Add(1))
				body := tt.body
				if tt.name == "repeated" && request == 2 {
					body = tt.body
				}
				return workItemCommentsTestResponse(req, body), nil
			})
			comments, err := client.ListWorkItemComments(context.Background(), 12345)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("comments/error = %#v/%v, want %q", comments, err, tt.want)
			}
		})
	}
}

func TestClientListWorkItemCommentsRejectsResponseSizeLimit(t *testing.T) {
	client := newWorkItemCommentsTestClientWithTransport(t, func(req *http.Request) (*http.Response, error) {
		body := strings.Repeat("x", int(maxWorkItemCommentResponseBytes)+1)
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Request: req}, nil
	})
	_, err := client.ListWorkItemComments(context.Background(), 12345)
	if err == nil || !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("error = %v, want response size error", err)
	}
}

func TestClientListWorkItemCommentsRejectsDuplicateDeletedComment(t *testing.T) {
	client := newWorkItemCommentsTestClient(t, `{"comments":[{"id":1,"workItemId":12345,"text":"deleted","isDeleted":true},{"id":1,"workItemId":12345,"text":"current"}]}`)
	_, err := client.ListWorkItemComments(context.Background(), 12345)
	if err == nil || !strings.Contains(err.Error(), "duplicate comment ID 1") {
		t.Fatalf("error = %v, want duplicate ID error including deleted records", err)
	}
}

func TestClientListWorkItemCommentsRejectsCommentCountCeiling(t *testing.T) {
	const perPage = 1001
	var requests atomic.Int32
	client := newWorkItemCommentsTestClientWithTransport(t, func(req *http.Request) (*http.Response, error) {
		page := int(requests.Add(1))
		var body strings.Builder
		body.WriteString(`{"comments":[`)
		for i := 0; i < perPage; i++ {
			if i > 0 {
				body.WriteByte(',')
			}
			id := (page-1)*perPage + i + 1
			fmt.Fprintf(&body, `{"id":%d,"workItemId":12345,"text":"x"}`, id)
		}
		body.WriteString(`]`)
		if page < 101 {
			fmt.Fprintf(&body, `,"continuationToken":"next-%d"`, page)
		}
		body.WriteByte('}')
		return workItemCommentsTestResponse(req, body.String()), nil
	})
	_, err := client.ListWorkItemComments(context.Background(), 12345)
	if err == nil || !strings.Contains(err.Error(), "maximum of 100000 comments") {
		t.Fatalf("error = %v, want comment ceiling error", err)
	}
	if requests.Load() != 100 {
		t.Fatalf("requests = %d, want 100", requests.Load())
	}
}

func TestClientListWorkItemCommentsRejectsPageCeiling(t *testing.T) {
	var requests atomic.Int32
	client := newWorkItemCommentsTestClientWithTransport(t, func(req *http.Request) (*http.Response, error) {
		request := int(requests.Add(1))
		body := fmt.Sprintf(`{"comments":[{"id":%d,"workItemId":12345,"text":"x"}],"continuationToken":"next-%d"}`, request, request)
		return workItemCommentsTestResponse(req, body), nil
	})
	_, err := client.ListWorkItemComments(context.Background(), 12345)
	if err == nil || !strings.Contains(err.Error(), "maximum of 1000 pages") {
		t.Fatalf("error = %v, want page ceiling error", err)
	}
	if requests.Load() != maxWorkItemCommentPages {
		t.Fatalf("requests = %d, want %d", requests.Load(), maxWorkItemCommentPages)
	}
}

func newWorkItemCommentsTestClient(t *testing.T, body string) *Client {
	t.Helper()
	return newWorkItemCommentsTestClientWithTransport(t, func(req *http.Request) (*http.Response, error) {
		return workItemCommentsTestResponse(req, body), nil
	})
}

func newWorkItemCommentsTestClientWithTransport(t *testing.T, transport roundTripperFunc) *Client {
	t.Helper()
	client, err := NewClient(&http.Client{Transport: transport}, ClientConfig{BaseURL: "https://dev.azure.com/org", Project: "Project", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}

func workItemCommentsTestResponse(req *http.Request, body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Request: req}
}
