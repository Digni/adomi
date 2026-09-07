package ado

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	pipelineTestRunPageSize    = 100
	pipelineTestResultPageSize = 1000
)

// PipelineTestRun preserves the reported Test run identity and aggregate
// counters. CalculatedResultCount is populated from the result pages read for
// this run and remains separate from the service-reported totals.
type PipelineTestRun struct {
	ID                      int            `json:"id"`
	Name                    string         `json:"name,omitempty"`
	State                   string         `json:"state,omitempty"`
	URL                     *string        `json:"url,omitempty"`
	ReportedTotalTests      *int           `json:"reportedTotalTests,omitempty"`
	ReportedPassedTests     *int           `json:"reportedPassedTests,omitempty"`
	ReportedCompletedTests  *int           `json:"reportedCompletedTests,omitempty"`
	ReportedIncompleteTests *int           `json:"reportedIncompleteTests,omitempty"`
	ReportedErrorTests      *int           `json:"reportedErrorTests,omitempty"`
	ReportedNotApplicable   *int           `json:"reportedNotApplicableTests,omitempty"`
	CalculatedResultCount   int            `json:"calculatedResultCount"`
	CalculatedOutcomeCounts map[string]int `json:"calculatedOutcomeCounts"`
}

// PipelineTestResult preserves the fields needed to identify and interpret a
// published Test result without expanding extra result details.
type PipelineTestResult struct {
	ID                int      `json:"id"`
	TestRunID         int      `json:"testRunId"`
	TestCaseTitle     string   `json:"testCaseTitle,omitempty"`
	AutomatedTestName string   `json:"automatedTestName,omitempty"`
	State             string   `json:"state,omitempty"`
	Outcome           string   `json:"outcome,omitempty"`
	DurationInMs      *float64 `json:"durationInMs,omitempty"`
	ErrorMessage      *string  `json:"errorMessage,omitempty"`
	StackTrace        *string  `json:"stackTrace,omitempty"`
}

type pipelineTestRunWire struct {
	ID                 int                   `json:"id"`
	Name               string                `json:"name"`
	State              string                `json:"state"`
	URL                *string               `json:"url"`
	Build              *pipelineBuildRefWire `json:"build"`
	BuildConfiguration *pipelineBuildRefWire `json:"buildConfiguration"`
	TotalTests         *int                  `json:"totalTests"`
	PassedTests        *int                  `json:"passedTests"`
	CompletedTests     *int                  `json:"completedTests"`
	IncompleteTests    *int                  `json:"incompleteTests"`
	ErrorTests         *int                  `json:"errorTests"`
	NotApplicableTests *int                  `json:"notApplicableTests"`
}

type pipelineBuildRefWire struct {
	ID  json.RawMessage `json:"id"`
	URI *string         `json:"uri"`
}

type pipelineTestResultWire struct {
	ID                int                    `json:"id"`
	TestRun           *pipelineTestRunIDWire `json:"testRun"`
	TestCaseTitle     string                 `json:"testCaseTitle"`
	AutomatedTestName string                 `json:"automatedTestName"`
	State             string                 `json:"state"`
	Outcome           string                 `json:"outcome"`
	DurationInMs      *float64               `json:"durationInMs"`
	ErrorMessage      *string                `json:"errorMessage"`
	StackTrace        *string                `json:"stackTrace"`
}

type pipelineTestRunIDWire struct {
	ID json.RawMessage `json:"id"`
}

func (s *pipelineInspectionSession) readTests(ctx context.Context, buildID int, buildURI string) ([]PipelineTestRun, map[int][]PipelineTestResult, error) {
	if s == nil || s.client == nil {
		return nil, nil, fmt.Errorf("pipeline inspection session is required")
	}
	if buildID <= 0 {
		return nil, nil, fmt.Errorf("pipeline test inspection requires a positive build ID")
	}
	buildURI = strings.TrimSpace(buildURI)
	if buildURI == "" {
		return nil, nil, fmt.Errorf("pipeline test inspection requires a build URI")
	}

	runs := make([]PipelineTestRun, 0)
	seenRunIDs := make(map[int]struct{})
	for skip := 0; ; {
		requestURL := s.testRunsURL(buildURI, pipelineTestRunPageSize, skip)
		body, err := s.get(ctx, requestURL, "listing Azure DevOps test runs", "application/json")
		if err != nil {
			return nil, nil, err
		}
		remaining := inspectionCollectionLimit - len(runs)
		page, err := decodeInspectionCollection[pipelineTestRunWire](body, "value", remaining, "listing Azure DevOps test runs")
		if err != nil {
			return nil, nil, err
		}
		if len(page) == 0 {
			break
		}
		for i, wire := range page {
			run, err := normalizePipelineTestRun(wire, buildID, buildURI)
			if err != nil {
				return nil, nil, fmt.Errorf("listing Azure DevOps test runs: invalid run at index %d: %w", i, err)
			}
			if _, exists := seenRunIDs[run.ID]; exists {
				return nil, nil, fmt.Errorf("listing Azure DevOps test runs: duplicate test run ID %d", run.ID)
			}
			seenRunIDs[run.ID] = struct{}{}
			runs = append(runs, run)
		}
		skip += len(page)
	}

	results := make(map[int][]PipelineTestResult, len(runs))
	totalResults := 0
	for i := range runs {
		runID := runs[i].ID
		runResults := make([]PipelineTestResult, 0)
		seenResultIDs := make(map[int]struct{})
		for skip := 0; ; {
			requestURL := s.testResultsURL(runID, pipelineTestResultPageSize, skip)
			body, err := s.get(ctx, requestURL, "listing Azure DevOps test results", "application/json")
			if err != nil {
				return nil, nil, err
			}
			remaining := inspectionCollectionLimit - totalResults
			page, err := decodeInspectionCollection[pipelineTestResultWire](body, "value", remaining, "listing Azure DevOps test results")
			if err != nil {
				return nil, nil, fmt.Errorf("listing Azure DevOps test results for test run %d: %w", runID, err)
			}
			if len(page) == 0 {
				break
			}
			for j, wire := range page {
				result, err := normalizePipelineTestResult(wire, runID)
				if err != nil {
					return nil, nil, fmt.Errorf("listing Azure DevOps test results for test run %d: invalid result at index %d: %w", runID, j, err)
				}
				if _, exists := seenResultIDs[result.ID]; exists {
					return nil, nil, fmt.Errorf("listing Azure DevOps test results: duplicate test result ID %d", result.ID)
				}
				seenResultIDs[result.ID] = struct{}{}
				runResults = append(runResults, result)
			}
			totalResults += len(page)
			skip += len(page)
		}
		runs[i].CalculatedResultCount = len(runResults)
		runs[i].CalculatedOutcomeCounts = make(map[string]int)
		for _, result := range runResults {
			runs[i].CalculatedOutcomeCounts[result.Outcome]++
		}
		results[runID] = runResults
	}
	return runs, results, nil
}

func normalizePipelineTestRun(wire pipelineTestRunWire, buildID int, buildURI string) (PipelineTestRun, error) {
	if wire.ID <= 0 {
		return PipelineTestRun{}, fmt.Errorf("missing test run ID")
	}
	if err := validatePipelineTestBuildIdentity(wire.Build, buildID, buildURI, "build reference"); err != nil {
		return PipelineTestRun{}, err
	}
	if err := validatePipelineTestBuildIdentity(wire.BuildConfiguration, buildID, buildURI, "build configuration"); err != nil {
		return PipelineTestRun{}, err
	}
	return PipelineTestRun{
		ID:                      wire.ID,
		Name:                    wire.Name,
		State:                   wire.State,
		URL:                     wire.URL,
		ReportedTotalTests:      wire.TotalTests,
		ReportedPassedTests:     wire.PassedTests,
		ReportedCompletedTests:  wire.CompletedTests,
		ReportedIncompleteTests: wire.IncompleteTests,
		ReportedErrorTests:      wire.ErrorTests,
		ReportedNotApplicable:   wire.NotApplicableTests,
	}, nil
}

func validatePipelineTestBuildIdentity(identity *pipelineBuildRefWire, buildID int, buildURI, label string) error {
	if identity == nil {
		return nil
	}
	rawID := identity.ID
	if len(rawID) > 0 && string(rawID) != "null" {
		refID, err := pipelineIdentityInt(rawID)
		if err != nil {
			return fmt.Errorf("invalid %s ID", label)
		}
		if refID != buildID {
			return fmt.Errorf("%s ID %d does not match requested build %d", label, refID, buildID)
		}
	}
	if uri := identity.URI; uri != nil && strings.TrimSpace(*uri) != "" && strings.TrimSpace(*uri) != buildURI {
		return fmt.Errorf("%s URI does not match requested build URI", label)
	}
	return nil
}

func normalizePipelineTestResult(wire pipelineTestResultWire, runID int) (PipelineTestResult, error) {
	if wire.ID <= 0 {
		return PipelineTestResult{}, fmt.Errorf("missing test result ID")
	}
	if wire.TestRun != nil && len(wire.TestRun.ID) > 0 && string(wire.TestRun.ID) != "null" {
		testRunID, err := pipelineIdentityInt(wire.TestRun.ID)
		if err != nil {
			return PipelineTestResult{}, fmt.Errorf("invalid test run ID")
		}
		if testRunID != runID {
			return PipelineTestResult{}, fmt.Errorf("test run ID %d does not match requested test run %d", testRunID, runID)
		}
	}
	return PipelineTestResult{
		ID:                wire.ID,
		TestRunID:         runID,
		TestCaseTitle:     wire.TestCaseTitle,
		AutomatedTestName: wire.AutomatedTestName,
		State:             wire.State,
		Outcome:           wire.Outcome,
		DurationInMs:      wire.DurationInMs,
		ErrorMessage:      wire.ErrorMessage,
		StackTrace:        wire.StackTrace,
	}, nil
}

func pipelineIdentityInt(raw json.RawMessage) (int, error) {
	var number int
	if err := json.Unmarshal(raw, &number); err == nil {
		return number, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, fmt.Errorf("identity is not an integer")
	}
	value, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, fmt.Errorf("identity is not an integer")
	}
	return value, nil
}

func (s *pipelineInspectionSession) testRunsURL(buildURI string, top, skip int) string {
	u := *s.client.baseURL
	setURLPathSegments(&u, s.client.config.Project, "_apis", "test", "runs")
	query := u.Query()
	query.Set("buildUri", buildURI)
	query.Set("includeRunDetails", "true")
	query.Set("$top", strconv.Itoa(top))
	query.Set("$skip", strconv.Itoa(skip))
	query.Set("api-version", s.client.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}

func (s *pipelineInspectionSession) testResultsURL(runID, top, skip int) string {
	u := *s.client.baseURL
	setURLPathSegments(&u, s.client.config.Project, "_apis", "test", "runs", strconv.Itoa(runID), "results")
	query := u.Query()
	query.Set("$top", strconv.Itoa(top))
	query.Set("$skip", strconv.Itoa(skip))
	query.Set("detailsToInclude", "none")
	query.Set("api-version", s.client.config.APIVersion)
	u.RawQuery = query.Encode()
	return u.String()
}
