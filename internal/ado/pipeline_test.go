package ado

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func newPipelineTestClient(t *testing.T, transport roundTripperFunc) *Client {
	t.Helper()
	return newPipelineTestClientWithPAT(t, "", transport)
}

func newPipelineTestClientWithPAT(t *testing.T, pat string, transport roundTripperFunc) *Client {
	t.Helper()
	client, err := NewClient(&http.Client{Transport: transport}, ClientConfig{
		BaseURL: "https://dev.azure.com/org",
		Project: "Project",
		PAT:     pat,
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}

func pipelineRunJSON(id, pipelineID int, status, resultJSON string) string {
	result := ""
	if resultJSON != "" {
		result = `,"result":` + resultJSON
	}
	return fmt.Sprintf(
		`{"id":%d,"buildNumber":"run-%d","status":%q%s,"definition":{"id":%d,"name":"Pipeline"}}`,
		id,
		id,
		status,
		result,
		pipelineID,
	)
}

func pipelineRunListJSON(ids ...int) string {
	var body strings.Builder
	body.WriteString(`{"value":[`)
	for i, id := range ids {
		if i > 0 {
			body.WriteByte(',')
		}
		body.WriteString(pipelineRunJSON(id, id, "inProgress", "null"))
	}
	body.WriteString(`]}`)
	return body.String()
}

func pipelineRunRangeListJSON(startID, count int) string {
	var body strings.Builder
	body.Grow(count * 100)
	body.WriteString(`{"value":[`)
	for offset := 0; offset < count; offset++ {
		if offset > 0 {
			body.WriteByte(',')
		}
		id := startID + offset
		body.WriteString(pipelineRunJSON(id, 1, "inProgress", "null"))
	}
	body.WriteString(`]}`)
	return body.String()
}

func pipelineRunIDs(runs []PipelineRun) []int {
	ids := make([]int, len(runs))
	for i := range runs {
		ids[i] = runs[i].ID
	}
	return ids
}

func pipelineRunIDsAtEnds(runs []PipelineRun) []int {
	if len(runs) == 0 {
		return nil
	}
	return []int{runs[0].ID, runs[len(runs)-1].ID}
}

func pipelineString(value string) *string {
	return &value
}

func equalPipelineString(got, want *string) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}

func pipelineSizedResponse(req *http.Request, validJSON string, size, declaredLength int64) *http.Response {
	remaining := size - int64(len(validJSON))
	if remaining < 0 {
		remaining = 0
	}
	body := io.MultiReader(
		strings.NewReader(validJSON),
		io.LimitReader(pipelineRepeatingReader(' '), remaining),
	)
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Header:        make(http.Header),
		Body:          io.NopCloser(body),
		ContentLength: declaredLength,
		Request:       req,
	}
}

type pipelineRepeatingReader byte

func (r pipelineRepeatingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

func assertPipelineErrorOmits(t *testing.T, errorText string, markers ...string) {
	t.Helper()
	for _, marker := range markers {
		if strings.Contains(errorText, marker) {
			t.Errorf("error = %q, want marker %q suppressed", errorText, marker)
		}
	}
}

func pipelineTestResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}
