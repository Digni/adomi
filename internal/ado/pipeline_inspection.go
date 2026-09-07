package ado

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type PipelineInspection struct {
	Run            PipelineRun                  `json:"run"`
	BuildURI       string                       `json:"buildUri"`
	Timelines      []PipelineTimeline           `json:"timelines"`
	Logs           map[int][]byte               `json:"logs"`
	TestRuns       []PipelineTestRun            `json:"testRuns"`
	TestResults    map[int][]PipelineTestResult `json:"testResults"`
	ObservedStart  time.Time                    `json:"observedStart"`
	ObservedFinish time.Time                    `json:"observedFinish"`
}

type PipelineTimeline struct {
	ID      string                   `json:"id"`
	Records []PipelineTimelineRecord `json:"records"`
}

type PipelineTimelineRecord struct {
	ID               string                        `json:"id"`
	ParentID         *string                       `json:"parentId"`
	Type             string                        `json:"type"`
	Name             string                        `json:"name"`
	Order            *int                          `json:"order"`
	State            *string                       `json:"state"`
	Result           *string                       `json:"result"`
	StartTime        *time.Time                    `json:"startTime"`
	FinishTime       *time.Time                    `json:"finishTime"`
	Attempt          *int                          `json:"attempt"`
	Identifier       *string                       `json:"identifier"`
	PreviousAttempts []PipelineTimelineAttempt     `json:"previousAttempts"`
	Issues           []PipelineTimelineIssue       `json:"issues"`
	Task             *PipelineTimelineTask         `json:"task"`
	Log              *PipelineTimelineLogReference `json:"log"`
	DetailTimelineID *string                       `json:"detailTimelineId"`
	LogAvailability  string                        `json:"logAvailability"`
	LogPath          string                        `json:"logPath"`
}

type PipelineTimelineAttempt struct {
	RecordID   string  `json:"recordId"`
	Attempt    *int    `json:"attempt"`
	TimelineID *string `json:"timelineId"`
}

type PipelineTimelineIssue struct {
	Type     string         `json:"type"`
	Message  string         `json:"message"`
	Category string         `json:"category"`
	Data     map[string]any `json:"data"`
}

type PipelineTimelineTask struct {
	ID           *string `json:"id"`
	Name         *string `json:"name"`
	Version      *string `json:"version"`
	InstanceName *string `json:"instanceName"`
}

type PipelineTimelineLogReference struct {
	ID   int     `json:"id"`
	Type *string `json:"type"`
	URL  *string `json:"url"`
}

type pipelineInspectionBudgets struct {
	requests int
	bytes    int64
}

type pipelineInspectionSession struct {
	client     *Client
	httpClient *http.Client
	budgets    pipelineInspectionBudgets
}

func (s *pipelineInspectionSession) get(ctx context.Context, requestURL, action, accept string) ([]byte, error) {
	if s.budgets.requests >= maxInspectionRequests {
		return nil, fmt.Errorf("%s: inspection exceeds maximum of %d HTTP requests", action, maxInspectionRequests)
	}
	remaining := maxInspectionBytes - s.budgets.bytes
	if remaining < 1 {
		return nil, fmt.Errorf("%s: inspection exceeds maximum of %d response bytes", action, maxInspectionBytes)
	}
	maxBody := remaining
	if maxBody > maxPipelineResponseBytes {
		maxBody = maxPipelineResponseBytes
	}
	s.budgets.requests++
	body, _, err := s.client.pipelineGETWithLimit(ctx, s.httpClient, requestURL, action, accept, maxBody)
	if err != nil {
		return nil, err
	}
	s.budgets.bytes += int64(len(body))
	return body, nil
}

func (c *Client) InspectPipelineRun(ctx context.Context, id int) (*PipelineInspection, error) {
	if id < 1 || id > maxPipelineRunID {
		return nil, fmt.Errorf("pipeline run ID must be between 1 and %d", maxPipelineRunID)
	}
	if err := c.validatePipelineTransport(); err != nil {
		return nil, err
	}
	httpClient, ownsTransport := c.pipelineHTTPClient()
	if ownsTransport {
		defer httpClient.CloseIdleConnections()
	}
	session := &pipelineInspectionSession{client: c, httpClient: httpClient}
	observedStart := time.Now().UTC()

	buildBody, err := session.get(ctx, c.pipelineRunURL(id), "inspecting Azure DevOps pipeline run", "application/json")
	if err != nil {
		return nil, err
	}
	var buildResponse pipelineRunResponse
	if err := decodePipelineJSON(buildBody, &buildResponse, "inspecting Azure DevOps pipeline run"); err != nil {
		return nil, err
	}
	run, err := normalizePipelineRun(buildResponse)
	if err != nil {
		return nil, fmt.Errorf("inspecting Azure DevOps pipeline run: invalid response: %w", err)
	}
	if run.ID != id {
		return nil, fmt.Errorf("inspecting Azure DevOps pipeline run: response ID does not match requested ID")
	}
	buildURI := normalizePipelineOptionalString(buildResponse.URI)
	if buildURI == nil {
		return nil, fmt.Errorf("inspecting Azure DevOps pipeline run: response is missing build URI")
	}

	timelines, logs, err := c.inspectPipelineTimelines(ctx, session, id)
	if err != nil {
		return nil, err
	}
	testRuns, testResults, err := session.readTests(ctx, id, *buildURI)
	if err != nil {
		return nil, err
	}

	return &PipelineInspection{
		Run:            run,
		BuildURI:       *buildURI,
		Timelines:      timelines,
		Logs:           logs,
		TestRuns:       testRuns,
		TestResults:    testResults,
		ObservedStart:  observedStart,
		ObservedFinish: time.Now().UTC(),
	}, nil
}

func (c *Client) inspectPipelineTimelines(ctx context.Context, session *pipelineInspectionSession, buildID int) ([]PipelineTimeline, map[int][]byte, error) {
	root, err := c.readPipelineTimeline(ctx, session, buildID, "", inspectionCollectionLimit)
	if err != nil {
		return nil, nil, err
	}
	timelines := []PipelineTimeline{root.timeline}
	seenTimelines := map[string]struct{}{root.timeline.ID: {}}
	queue := append([]string(nil), root.detailIDs...)
	recordCount := len(root.timeline.Records)
	for len(queue) > 0 {
		timelineID := queue[0]
		queue = queue[1:]
		if _, seen := seenTimelines[timelineID]; seen {
			continue
		}
		detail, err := c.readPipelineTimeline(ctx, session, buildID, timelineID, inspectionCollectionLimit-recordCount)
		if err != nil {
			return nil, nil, err
		}
		seenTimelines[timelineID] = struct{}{}
		timelines = append(timelines, detail.timeline)
		recordCount += len(detail.timeline.Records)
		queue = append(queue, detail.detailIDs...)
	}

	logs := make(map[int][]byte)
	seenLogs := make(map[int]struct{})
	for timelineIndex := range timelines {
		for recordIndex := range timelines[timelineIndex].Records {
			record := &timelines[timelineIndex].Records[recordIndex]
			if !strings.EqualFold(record.Type, "task") {
				continue
			}
			if !strings.EqualFold(stringValue(record.Result), "failed") {
				record.LogAvailability = "notRequested"
				continue
			}
			if record.Log == nil || record.Log.ID < 1 || record.Log.ID > maxPipelineRunID {
				record.LogAvailability = "notPublished"
				continue
			}
			record.LogAvailability = "downloaded"
			record.LogPath = fmt.Sprintf("logs/%d.txt", record.Log.ID)
			if _, seen := seenLogs[record.Log.ID]; seen {
				continue
			}
			seenLogs[record.Log.ID] = struct{}{}
			logBody, err := session.get(ctx, c.pipelineLogURL(buildID, record.Log.ID), "getting Azure DevOps pipeline task log", "text/plain")
			if err != nil {
				return nil, nil, err
			}
			logs[record.Log.ID] = logBody
		}
	}
	return timelines, logs, nil
}

type pipelineTimelineRead struct {
	timeline  PipelineTimeline
	detailIDs []string
}

type pipelineTimelineResponse struct {
	ID string `json:"id"`
}

type pipelineTimelineRecordResponse struct {
	ID               string                            `json:"id"`
	ParentID         *string                           `json:"parentId"`
	Type             string                            `json:"type"`
	Name             string                            `json:"name"`
	Order            *int                              `json:"order"`
	State            *string                           `json:"state"`
	Result           *string                           `json:"result"`
	StartTime        *string                           `json:"startTime"`
	FinishTime       *string                           `json:"finishTime"`
	Attempt          *int                              `json:"attempt"`
	Identifier       *string                           `json:"identifier"`
	PreviousAttempts []pipelineTimelineAttemptResponse `json:"previousAttempts"`
	Issues           []PipelineTimelineIssue           `json:"issues"`
	Task             *PipelineTimelineTask             `json:"task"`
	Log              *PipelineTimelineLogReference     `json:"log"`
	Details          *pipelineTimelineDetailResponse   `json:"details"`
}

type pipelineTimelineDetailResponse struct {
	ID string `json:"id"`
}

type pipelineTimelineAttemptResponse struct {
	RecordID   string  `json:"recordId"`
	Attempt    *int    `json:"attempt"`
	TimelineID *string `json:"timelineId"`
}

func (c *Client) readPipelineTimeline(ctx context.Context, session *pipelineInspectionSession, buildID int, timelineID string, remaining int) (pipelineTimelineRead, error) {
	if timelineID != "" {
		canonicalID, err := validatePipelineInspectionIdentifier(timelineID, "timeline")
		if err != nil {
			return pipelineTimelineRead{}, err
		}
		timelineID = canonicalID
	}
	requestURL := c.pipelineTimelineURL(buildID, timelineID)
	action := "getting Azure DevOps pipeline timeline"
	body, err := session.get(ctx, requestURL, action, "application/json")
	if err != nil {
		return pipelineTimelineRead{}, err
	}
	var response pipelineTimelineResponse
	if err := decodePipelineJSON(body, &response, action); err != nil {
		return pipelineTimelineRead{}, err
	}
	response.ID = strings.TrimSpace(response.ID)
	canonicalID, err := validatePipelineInspectionIdentifier(response.ID, "timeline")
	if err != nil {
		return pipelineTimelineRead{}, fmt.Errorf("%s: %w", action, err)
	}
	if timelineID != "" && canonicalID != timelineID {
		return pipelineTimelineRead{}, fmt.Errorf("%s: response timeline ID does not match requested ID", action)
	}
	recordResponses, err := decodeInspectionCollection[pipelineTimelineRecordResponse](body, "records", remaining, action)
	if err != nil {
		return pipelineTimelineRead{}, err
	}
	records := make([]PipelineTimelineRecord, 0, len(recordResponses))
	detailIDs := make([]string, 0)
	seenDetails := make(map[string]struct{})
	seenRecords := make(map[string]struct{}, len(recordResponses))
	for index := range recordResponses {
		recordID, err := validatePipelineInspectionIdentifier(recordResponses[index].ID, "record")
		if err != nil {
			return pipelineTimelineRead{}, fmt.Errorf("%s: invalid record at index %d: %w", action, index, err)
		}
		if _, seen := seenRecords[recordID]; seen {
			return pipelineTimelineRead{}, fmt.Errorf("%s: duplicate record ID %q", action, recordID)
		}
		seenRecords[recordID] = struct{}{}
		recordResponses[index].ID = recordID
		record, detailID, err := normalizePipelineTimelineRecord(recordResponses[index])
		if err != nil {
			return pipelineTimelineRead{}, fmt.Errorf("%s: invalid record at index %d: %w", action, index, err)
		}
		records = append(records, record)
		if detailID == "" || detailID == canonicalID {
			continue
		}
		if _, seen := seenDetails[detailID]; seen {
			continue
		}
		seenDetails[detailID] = struct{}{}
		detailIDs = append(detailIDs, detailID)
	}
	return pipelineTimelineRead{timeline: PipelineTimeline{ID: canonicalID, Records: records}, detailIDs: detailIDs}, nil
}

func normalizePipelineTimelineRecord(response pipelineTimelineRecordResponse) (PipelineTimelineRecord, string, error) {
	startTime, err := normalizePipelineTime(response.StartTime)
	if err != nil {
		return PipelineTimelineRecord{}, "", fmt.Errorf("start time is invalid")
	}
	finishTime, err := normalizePipelineTime(response.FinishTime)
	if err != nil {
		return PipelineTimelineRecord{}, "", fmt.Errorf("finish time is invalid")
	}
	detailID := ""
	if response.Details != nil {
		detailID = strings.TrimSpace(response.Details.ID)
	}
	if detailID != "" {
		canonicalID, err := validatePipelineInspectionIdentifier(detailID, "timeline")
		if err != nil {
			return PipelineTimelineRecord{}, "", err
		}
		detailID = canonicalID
	}
	if response.Log != nil && (response.Log.ID < 1 || response.Log.ID > maxPipelineRunID) {
		return PipelineTimelineRecord{}, "", fmt.Errorf("log identifier is invalid")
	}
	previousAttempts := make([]PipelineTimelineAttempt, 0, len(response.PreviousAttempts))
	for _, attempt := range response.PreviousAttempts {
		recordID, err := validatePipelineInspectionIdentifier(attempt.RecordID, "previous attempt record")
		if err != nil {
			return PipelineTimelineRecord{}, "", err
		}
		timelineID := normalizePipelineOptionalString(attempt.TimelineID)
		if timelineID != nil {
			canonicalTimelineID, err := validatePipelineInspectionIdentifier(*timelineID, "timeline")
			if err != nil {
				return PipelineTimelineRecord{}, "", err
			}
			timelineID = &canonicalTimelineID
		}
		previousAttempts = append(previousAttempts, PipelineTimelineAttempt{RecordID: recordID, Attempt: attempt.Attempt, TimelineID: timelineID})
	}
	parentID := normalizePipelineOptionalString(response.ParentID)
	if parentID != nil {
		canonicalParentID, err := validatePipelineInspectionIdentifier(*parentID, "parent record")
		if err != nil {
			return PipelineTimelineRecord{}, "", err
		}
		parentID = &canonicalParentID
	}
	return PipelineTimelineRecord{
		ID:               response.ID,
		ParentID:         parentID,
		Type:             response.Type,
		Name:             response.Name,
		Order:            response.Order,
		State:            normalizePipelineOptionalString(response.State),
		Result:           normalizePipelineOptionalString(response.Result),
		StartTime:        startTime,
		FinishTime:       finishTime,
		Attempt:          response.Attempt,
		Identifier:       normalizePipelineOptionalString(response.Identifier),
		PreviousAttempts: previousAttempts,
		Issues:           response.Issues,
		Task:             response.Task,
		Log:              normalizePipelineLogReference(response.Log),
		DetailTimelineID: normalizePipelineOptionalString(&detailID),
	}, detailID, nil
}

func normalizePipelineLogReference(value *PipelineTimelineLogReference) *PipelineTimelineLogReference {
	if value == nil {
		return nil
	}
	copy := *value
	copy.Type = normalizePipelineOptionalString(copy.Type)
	copy.URL = normalizePipelineOptionalString(copy.URL)
	return &copy
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func validatePipelineInspectionIdentifier(value, kind string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return "", fmt.Errorf("%s identifier is invalid", kind)
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return "", fmt.Errorf("%s identifier is invalid", kind)
		}
	}
	return strings.ToLower(value), nil
}

func (c *Client) pipelineTimelineURL(buildID int, timelineID string) string {
	u := *c.baseURL
	segments := []string{c.config.Project, "_apis", "build", "builds", fmt.Sprintf("%d", buildID), "timeline"}
	if timelineID != "" {
		segments = append(segments, timelineID)
	}
	setURLPathSegments(&u, segments...)
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) pipelineLogURL(buildID, logID int) string {
	u := *c.baseURL
	setURLPathSegments(&u, c.config.Project, "_apis", "build", "builds", fmt.Sprintf("%d", buildID), "logs", fmt.Sprintf("%d", logID))
	query := u.Query()
	query.Set("api-version", c.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func decodeInspectionCollection[T any](body []byte, field string, remaining int, action string) ([]T, error) {
	if remaining < 0 {
		return nil, fmt.Errorf("%s: invalid collection limit", action)
	}
	var response map[string]json.RawMessage
	if err := decodePipelineJSON(body, &response, action); err != nil {
		return nil, err
	}
	value, ok := response[field]
	if !ok {
		return nil, fmt.Errorf("%s: response is missing %s", action, field)
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, fmt.Errorf("%s: response contains invalid %s collection", action, field)
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return nil, fmt.Errorf("%s: response contains invalid %s collection", action, field)
	}
	capacity := remaining
	if capacity > 1024 {
		capacity = 1024
	}
	items := make([]T, 0, capacity)
	for decoder.More() {
		if len(items) >= remaining {
			return nil, fmt.Errorf("%s: %s collection exceeds maximum of %d entries", action, field, remaining)
		}
		var item T
		if err := decoder.Decode(&item); err != nil {
			return nil, fmt.Errorf("%s: response contains invalid %s collection", action, field)
		}
		items = append(items, item)
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim(']') {
		return nil, fmt.Errorf("%s: response contains invalid %s collection", action, field)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("%s: response contains invalid %s collection", action, field)
	}
	return items, nil
}
