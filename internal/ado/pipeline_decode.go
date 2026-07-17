package ado

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func decodePipelineRunList(body []byte, maxItems int) ([]pipelineRunResponse, error) {
	var response map[string]json.RawMessage
	if err := decodePipelineJSON(body, &response, "listing Azure DevOps pipeline runs"); err != nil {
		return nil, err
	}
	value, ok := response["value"]
	if !ok {
		return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response is missing value")
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return []pipelineRunResponse{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	token, err := decoder.Token()
	opening, ok := token.(json.Delim)
	if err != nil || !ok || opening != '[' {
		return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response contains invalid value")
	}
	capacity := maxItems
	if capacity > 1024 {
		capacity = 1024
	}
	runs := make([]pipelineRunResponse, 0, capacity)
	for decoder.More() {
		if len(runs) >= maxItems {
			return nil, errPipelineRunLimitExceeded
		}
		var run pipelineRunResponse
		if err := decoder.Decode(&run); err != nil {
			return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response contains invalid value")
		}
		runs = append(runs, run)
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim(']') {
		return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response contains invalid value")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("listing Azure DevOps pipeline runs: response contains invalid value")
	}
	return runs, nil
}

func decodePipelineJSON(body []byte, target any, action string) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s: response contains invalid JSON", action)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%s: response must contain exactly one JSON value", action)
	}
	return nil
}

func pipelineContinuationToken(header http.Header) (string, bool, error) {
	values := header.Values("X-MS-ContinuationToken")
	if len(values) == 0 {
		return "", false, nil
	}
	if len(values) != 1 {
		return "", false, fmt.Errorf("listing Azure DevOps pipeline runs: response contains multiple continuation tokens")
	}
	if strings.TrimSpace(values[0]) == "" {
		return "", false, fmt.Errorf("listing Azure DevOps pipeline runs: response contains a blank continuation token")
	}
	return values[0], true, nil
}

func normalizePipelineRun(response pipelineRunResponse) (PipelineRun, error) {
	if response.ID < 1 || response.ID > maxPipelineRunID {
		return PipelineRun{}, fmt.Errorf("run ID must be between 1 and %d", maxPipelineRunID)
	}
	if response.Definition.ID < 1 || response.Definition.ID > maxPipelineRunID {
		return PipelineRun{}, fmt.Errorf("pipeline ID must be between 1 and %d", maxPipelineRunID)
	}
	if strings.TrimSpace(response.Definition.Name) == "" {
		return PipelineRun{}, fmt.Errorf("pipeline name is required")
	}
	if strings.TrimSpace(response.BuildNumber) == "" {
		return PipelineRun{}, fmt.Errorf("run number is required")
	}
	if strings.TrimSpace(response.Status) == "" {
		return PipelineRun{}, fmt.Errorf("status is required")
	}

	result := normalizePipelineOptionalString(response.Result)
	if result != nil && *result == "none" {
		result = nil
	}
	if response.Status == "completed" && result == nil {
		return PipelineRun{}, fmt.Errorf("completed run requires a result")
	}
	if response.Status != "completed" && result != nil {
		return PipelineRun{}, fmt.Errorf("non-completed run must not contain a result")
	}

	queueTime, err := normalizePipelineTime(response.QueueTime)
	if err != nil {
		return PipelineRun{}, fmt.Errorf("queue time is invalid")
	}
	startTime, err := normalizePipelineTime(response.StartTime)
	if err != nil {
		return PipelineRun{}, fmt.Errorf("start time is invalid")
	}
	finishTime, err := normalizePipelineTime(response.FinishTime)
	if err != nil {
		return PipelineRun{}, fmt.Errorf("finish time is invalid")
	}

	return PipelineRun{
		ID:            response.ID,
		PipelineID:    response.Definition.ID,
		PipelineName:  response.Definition.Name,
		RunNumber:     response.BuildNumber,
		Status:        response.Status,
		Result:        result,
		SourceBranch:  normalizePipelineOptionalString(response.SourceBranch),
		SourceVersion: normalizePipelineOptionalString(response.SourceVersion),
		QueueTime:     queueTime,
		StartTime:     startTime,
		FinishTime:    finishTime,
		WebURL:        normalizePipelineOptionalString(response.Links.Web.Href),
	}, nil
}

func normalizePipelineOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizePipelineTime(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}
