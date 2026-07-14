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

func TestClientFetchWorkItemBuildsURLWithBasePathAndAuth(t *testing.T) {
	var seenPath string
	var seenExpand string
	var seenAPIVersion string
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		seenExpand = r.URL.Query().Get("$expand")
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"id":12345,"fields":{"System.Title":"Title","System.WorkItemType":"Task"}}`)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(server.Client(), ClientConfig{
		BaseURL:    server.URL + "/tfs/DefaultCollection",
		Project:    "My Project",
		APIVersion: "7.0",
		PAT:        "secret",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	item, err := client.FetchWorkItem(context.Background(), 12345)
	if err != nil {
		t.Fatalf("FetchWorkItem returned error: %v", err)
	}
	if item.ID != 12345 {
		t.Fatalf("item ID = %d, want 12345", item.ID)
	}
	if seenPath != "/tfs/DefaultCollection/My%20Project/_apis/wit/workitems/12345" {
		t.Fatalf("path = %q, want Azure DevOps work item path", seenPath)
	}
	if seenExpand != "all" {
		t.Fatalf("$expand = %q, want all", seenExpand)
	}
	if seenAPIVersion != "7.0" {
		t.Fatalf("api-version = %q, want 7.0", seenAPIVersion)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
}

func TestClientFetchWorkItemReturnsNon2xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 1)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %q, want status code", err.Error())
	}
}

func TestNewHTTPClientDoesNotFollowAuthenticationRedirectForFetchWorkItem(t *testing.T) {
	for _, status := range []int{
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect,
	} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var signInRequests atomic.Int32
			signIn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				signInRequests.Add(1)
				fmt.Fprint(w, "<html>secret sign-in page</html>")
			}))
			t.Cleanup(signIn.Close)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, signIn.URL+"/signin", status)
			}))
			t.Cleanup(server.Close)

			httpClient, err := NewHTTPClient("")
			if err != nil {
				t.Fatalf("NewHTTPClient returned error: %v", err)
			}
			client, err := NewClient(httpClient, ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "invalid"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			_, err = client.FetchWorkItem(context.Background(), 1)
			if err == nil {
				t.Fatal("FetchWorkItem error = nil, want redirect status error")
			}
			if signInRequests.Load() != 0 {
				t.Errorf("sign-in target requests = %d, want 0", signInRequests.Load())
			}
			errorText := err.Error()
			if !strings.Contains(errorText, fmt.Sprint(status)) {
				t.Errorf("error = %q, want redirect status code", errorText)
			}
			for _, leaked := range []string{"decoding", "secret sign-in page", signIn.URL, "/signin"} {
				if strings.Contains(errorText, leaked) {
					t.Errorf("error = %q, want no %q", errorText, leaked)
				}
			}
		})
	}
}

func TestNewHTTPClientReturnsStatusForMalformedRedirectLocation(t *testing.T) {
	const malformedLocation = "http://[::1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", malformedLocation)
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)

	httpClient, err := NewHTTPClient("")
	if err != nil {
		t.Fatalf("NewHTTPClient returned error: %v", err)
	}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "invalid"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 1)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want redirect status error")
	}
	errorText := err.Error()
	if !strings.Contains(errorText, "302") {
		t.Errorf("error = %q, want redirect status code", errorText)
	}
	for _, leaked := range []string{"failed to parse Location", malformedLocation} {
		if strings.Contains(errorText, leaked) {
			t.Errorf("error = %q, want no %q", errorText, leaked)
		}
	}
}

func TestNewHTTPClientReturnsStatusForRedirectWithoutLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)

	httpClient, err := NewHTTPClient("")
	if err != nil {
		t.Fatalf("NewHTTPClient returned error: %v", err)
	}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "invalid"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 1)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want redirect status error")
	}
	errorText := err.Error()
	if !strings.Contains(errorText, "302") {
		t.Errorf("error = %q, want redirect status code", errorText)
	}
	if strings.Contains(errorText, "decoding") {
		t.Errorf("error = %q, want no decode error", errorText)
	}
}

func TestClientFetchWorkItemDoesNotClassifyNotModifiedAsRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(server.Close)

	httpClient, err := NewHTTPClient("")
	if err != nil {
		t.Fatalf("NewHTTPClient returned error: %v", err)
	}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "invalid"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 1)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want status error")
	}
	errorText := err.Error()
	if !strings.Contains(errorText, "304") {
		t.Errorf("error = %q, want status code", errorText)
	}
	for _, misleading := range []string{"redirected", "PAT", "base URL"} {
		if strings.Contains(errorText, misleading) {
			t.Errorf("error = %q, want no %q guidance", errorText, misleading)
		}
	}
}

func TestClientFetchWorkItemReturnsCleanNon2xxErrorWithEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 1)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want error")
	}
	if strings.HasSuffix(err.Error(), ":") {
		t.Fatalf("error = %q, want no dangling colon", err.Error())
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %q, want status code", err.Error())
	}
}

func TestClientFetchWorkItemKeepsDirectHTMLStatusBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "<html><body>legacy work item diagnostics</body></html>")
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 12345)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want status error")
	}
	if !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "legacy work item diagnostics") {
		t.Fatalf("error = %q, want status and existing HTML body context", err.Error())
	}
}

func TestClientFetchWorkItemRejectsMismatchedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":999,"fields":{"System.Title":"Wrong"}}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 12345)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want ID mismatch error")
	}
	if !strings.Contains(err.Error(), "12345") || !strings.Contains(err.Error(), "999") {
		t.Fatalf("error = %q, want requested and response IDs", err.Error())
	}
}

func TestClientFetchWorkItemReturnsMalformedJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 12345)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want decode error")
	}
	if !strings.Contains(err.Error(), "decoding Azure DevOps work item 12345") {
		t.Fatalf("error = %q, want decode context", err.Error())
	}
}

func TestClientFetchWorkItemReturnsEmptyBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.FetchWorkItem(context.Background(), 12345)
	if err == nil {
		t.Fatal("FetchWorkItem error = nil, want decode error")
	}
	if !strings.Contains(err.Error(), "decoding Azure DevOps work item 12345") {
		t.Fatalf("error = %q, want decode context", err.Error())
	}
}

func TestClientCreateWorkItemCommentBuildsURLAuthBodyAndNormalizesCommentID(t *testing.T) {
	var seenMethod string
	var seenPath string
	var seenAPIVersion string
	var seenAuth string
	var seenBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenAPIVersion = r.URL.Query().Get("api-version")
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		fmt.Fprint(w, `{"commentId":55,"workItemId":12345,"url":"https://dev.azure.com/org/My%20Project/_workitems/edit/12345#comment-55"}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{
		BaseURL:    server.URL + "/tfs/DefaultCollection",
		Project:    "My Project",
		APIVersion: "7.1",
		PAT:        "secret",
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	comment, err := client.CreateWorkItemComment(context.Background(), WorkItemCommentCreateOptions{WorkItemID: 12345, Text: "Done"})
	if err != nil {
		t.Fatalf("CreateWorkItemComment returned error: %v", err)
	}
	if comment.CreatedID() != 55 {
		t.Fatalf("created comment ID = %d, want 55", comment.CreatedID())
	}
	if comment.WorkItemID != 12345 {
		t.Fatalf("work item ID = %d, want 12345", comment.WorkItemID)
	}
	if comment.URL == "" {
		t.Fatal("comment URL is empty")
	}
	if seenMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", seenMethod)
	}
	if seenPath != "/tfs/DefaultCollection/My%20Project/_apis/wit/workitems/12345/comments" {
		t.Fatalf("path = %q, want Azure DevOps comments path", seenPath)
	}
	if seenAPIVersion != "7.0-preview.3" {
		t.Fatalf("api-version = %q, want 7.0-preview.3", seenAPIVersion)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
	if seenBody["text"] != "Done" {
		t.Fatalf("request text = %q, want Done", seenBody["text"])
	}
}

func TestClientCreateWorkItemCommentAcceptsIDFieldAndAbsentWorkItemID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":56,"url":"https://dev.azure.com/org/project/_workitems/edit/12345#comment-56"}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	comment, err := client.CreateWorkItemComment(context.Background(), WorkItemCommentCreateOptions{WorkItemID: 12345, Text: "Done"})
	if err != nil {
		t.Fatalf("CreateWorkItemComment returned error: %v", err)
	}
	if comment.CreatedID() != 56 {
		t.Fatalf("created comment ID = %d, want 56", comment.CreatedID())
	}
}

func TestClientCreateWorkItemCommentReturnsNon2xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.CreateWorkItemComment(context.Background(), WorkItemCommentCreateOptions{WorkItemID: 12345, Text: "Done"})
	if err == nil {
		t.Fatal("CreateWorkItemComment error = nil, want error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %q, want status code", err.Error())
	}
}

func TestClientCreateWorkItemCommentDoesNotFollowAuthenticationRedirect(t *testing.T) {
	var signInRequests atomic.Int32
	signIn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signInRequests.Add(1)
		fmt.Fprint(w, "<html>secret sign-in page</html>")
	}))
	t.Cleanup(signIn.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, signIn.URL+"/signin", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	httpClient, err := NewHTTPClient("")
	if err != nil {
		t.Fatalf("NewHTTPClient returned error: %v", err)
	}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "invalid"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.CreateWorkItemComment(context.Background(), WorkItemCommentCreateOptions{WorkItemID: 12345, Text: "Done"})
	if err == nil {
		t.Fatal("CreateWorkItemComment error = nil, want redirect status error")
	}
	if signInRequests.Load() != 0 {
		t.Errorf("sign-in target requests = %d, want 0", signInRequests.Load())
	}
	errorText := err.Error()
	if !strings.Contains(errorText, "302") {
		t.Errorf("error = %q, want redirect status code", errorText)
	}
	for _, leaked := range []string{"decoding", "secret sign-in page", signIn.URL, "/signin"} {
		if strings.Contains(errorText, leaked) {
			t.Errorf("error = %q, want no %q", errorText, leaked)
		}
	}
}

func TestClientCreateWorkItemCommentWrapsDecodeErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed", body: `{"id":`},
		{name: "trailing garbage", body: `{"id":55}garbage`},
		{name: "empty", body: ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tt.body)
			}))
			t.Cleanup(server.Close)
			client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
			if err != nil {
				t.Fatalf("NewClient returned error: %v", err)
			}

			_, err = client.CreateWorkItemComment(context.Background(), WorkItemCommentCreateOptions{WorkItemID: 12345, Text: "Done"})
			if err == nil {
				t.Fatal("CreateWorkItemComment error = nil, want error")
			}
			if !strings.Contains(err.Error(), "decoding Azure DevOps work item comment") {
				t.Fatalf("error = %q, want decode context", err.Error())
			}
		})
	}
}

func TestClientCreateWorkItemCommentRejectsMissingIDBeforeMismatchedWorkItemID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"workItemId":999}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.CreateWorkItemComment(context.Background(), WorkItemCommentCreateOptions{WorkItemID: 12345, Text: "Done"})
	if err == nil {
		t.Fatal("CreateWorkItemComment error = nil, want error")
	}
	if !strings.Contains(err.Error(), "missing comment ID") {
		t.Fatalf("error = %q, want missing comment ID", err.Error())
	}
	if strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %q, should report missing ID before mismatch", err.Error())
	}
}

func TestClientCreateWorkItemCommentRejectsMismatchedWorkItemID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":57,"workItemId":999}`)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.CreateWorkItemComment(context.Background(), WorkItemCommentCreateOptions{WorkItemID: 12345, Text: "Done"})
	if err == nil {
		t.Fatal("CreateWorkItemComment error = nil, want error")
	}
	if !strings.Contains(err.Error(), "999") || !strings.Contains(err.Error(), "12345") || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %q, want mismatched work item IDs", err.Error())
	}
}

func TestClientDownloadUsesAuthAndReturnsBytes(t *testing.T) {
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, "attachment bytes")
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	got, err := client.Download(context.Background(), server.URL+"/file")
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	if string(got) != "attachment bytes" {
		t.Fatalf("download = %q, want attachment bytes", got)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":secret"))
	if seenAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", seenAuth, wantAuth)
	}
}

func TestClientDownloadReturnsNon2xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.Download(context.Background(), server.URL+"/file")
	if err == nil {
		t.Fatal("Download error = nil, want error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error = %q, want status code", err.Error())
	}
}

func TestClientDownloadDoesNotFollowAuthenticationRedirect(t *testing.T) {
	var signInRequests atomic.Int32
	signIn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signInRequests.Add(1)
		fmt.Fprint(w, "<html>secret sign-in page</html>")
	}))
	t.Cleanup(signIn.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, signIn.URL+"/signin", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	httpClient, err := NewHTTPClient("")
	if err != nil {
		t.Fatalf("NewHTTPClient returned error: %v", err)
	}
	client, err := NewClient(httpClient, ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "invalid"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	data, err := client.Download(context.Background(), server.URL+"/attachment")
	if err == nil {
		t.Fatal("Download error = nil, want redirect status error")
	}
	if len(data) != 0 {
		t.Errorf("Download returned %d bytes, want 0", len(data))
	}
	if signInRequests.Load() != 0 {
		t.Errorf("sign-in target requests = %d, want 0", signInRequests.Load())
	}
	errorText := err.Error()
	if !strings.Contains(errorText, "302") {
		t.Errorf("error = %q, want redirect status code", errorText)
	}
	for _, leaked := range []string{"decoding", "secret sign-in page", signIn.URL, "/signin"} {
		if strings.Contains(errorText, leaked) {
			t.Errorf("error = %q, want no %q", errorText, leaked)
		}
	}
}

func TestClientDownloadRejectsAttachmentOverMaxSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(maxAttachmentBytes+1))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.Client(), ClientConfig{BaseURL: server.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.Download(context.Background(), server.URL+"/large.bin")
	if err == nil {
		t.Fatal("Download error = nil, want size limit error")
	}
	if !strings.Contains(err.Error(), "attachment exceeds") {
		t.Fatalf("error = %q, want size limit context", err.Error())
	}
}

func TestReadAttachmentBodyRejectsStreamOverMaxSize(t *testing.T) {
	_, err := readAttachmentBody(strings.NewReader("123456789"), 8)
	if err != nil {
		if strings.Contains(err.Error(), "attachment exceeds") {
			return
		}
		t.Fatalf("error = %q, want size limit context", err.Error())
	}
	t.Fatal("readAttachmentBody error = nil, want size limit error")
}

func TestReadAttachmentBodyWrapsReadError(t *testing.T) {
	_, err := readAttachmentBody(errorReader{}, 8)
	if err == nil {
		t.Fatal("readAttachmentBody error = nil, want read error")
	}
	if !strings.Contains(err.Error(), "reading Azure DevOps attachment") {
		t.Fatalf("error = %q, want read context", err.Error())
	}
}

func TestClientDownloadRejectsExternalHostBeforeAuth(t *testing.T) {
	var externalRequests atomic.Int32
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		externalRequests.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("external request received Authorization header %q", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, "external")
	}))
	t.Cleanup(external.Close)
	base := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "base")
	}))
	t.Cleanup(base.Close)
	client, err := NewClient(base.Client(), ClientConfig{BaseURL: base.URL, Project: "Project", APIVersion: "7.1", PAT: "secret"})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.Download(context.Background(), external.URL+"/file")
	if err == nil {
		t.Fatal("Download error = nil, want same-origin error")
	}
	if externalRequests.Load() != 0 {
		t.Fatalf("external requests = %d, want 0", externalRequests.Load())
	}
}

func TestNewHTTPClientRejectsInvalidProxy(t *testing.T) {
	_, err := NewHTTPClient("://bad proxy")
	if err == nil {
		t.Fatal("NewHTTPClient error = nil, want error")
	}
}

func TestNewHTTPClientRejectsProxyWithoutScheme(t *testing.T) {
	_, err := NewHTTPClient("proxy.company.local:8080")
	if err == nil {
		t.Fatal("NewHTTPClient error = nil, want error")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
