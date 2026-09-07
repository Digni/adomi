package ado

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type PipelineInspectionExportOptions struct {
	RepoRoot string
	Profile  string
	Project  string
	BaseURL  string
}

type PipelineInspectionCounts struct {
	TimelineRecords int `json:"timelineRecords"`
	FailedTaskLogs  int `json:"failedTaskLogs"`
	TestRuns        int `json:"testRuns"`
	TestResults     int `json:"testResults"`
}

func (inspection *PipelineInspection) Counts() PipelineInspectionCounts {
	if inspection == nil {
		return PipelineInspectionCounts{}
	}
	counts := PipelineInspectionCounts{TestRuns: len(inspection.TestRuns), FailedTaskLogs: len(inspection.Logs)}
	for _, timeline := range inspection.Timelines {
		counts.TimelineRecords += len(timeline.Records)
	}
	for _, results := range inspection.TestResults {
		counts.TestResults += len(results)
	}
	return counts
}

type pipelineInspectionExportIndex struct {
	Source          string                              `json:"source"`
	Profile         string                              `json:"profile"`
	Project         string                              `json:"project"`
	BaseURL         string                              `json:"baseUrl"`
	BuildURI        string                              `json:"buildUri"`
	RunID           int                                 `json:"runId"`
	ObservedStart   string                              `json:"observedStart"`
	ObservedFinish  string                              `json:"observedFinish"`
	Scope           string                              `json:"scope"`
	Counts          PipelineInspectionCounts            `json:"counts"`
	LogAvailability []pipelineInspectionLogAvailability `json:"logAvailability"`
	Artifacts       pipelineInspectionArtifactPaths     `json:"artifacts"`
}

type pipelineInspectionLogAvailability struct {
	TimelineID   string `json:"timelineId"`
	RecordID     string `json:"recordId"`
	LogID        *int   `json:"logId"`
	Availability string `json:"availability"`
	Path         string `json:"path"`
}

type pipelineInspectionArtifactPaths struct {
	Run         string         `json:"run"`
	Timeline    string         `json:"timeline"`
	Logs        map[int]string `json:"logs"`
	TestRuns    string         `json:"testRuns"`
	TestResults map[int]string `json:"testResults"`
}

type pipelineInspectionTimelineFile struct {
	Timelines []PipelineTimeline `json:"timelines"`
}

type pipelineInspectionTestRunsFile struct {
	Runs []PipelineTestRun `json:"runs"`
}

type pipelineInspectionTestResultsFile struct {
	Results []PipelineTestResult `json:"results"`
}

var pipelineInspectionWriteFile = os.WriteFile

func ExportPipelineInspection(ctx context.Context, opts PipelineInspectionExportOptions, inspection *PipelineInspection) (outputDir string, err error) {
	if inspection == nil {
		return "", fmt.Errorf("pipeline inspection is required")
	}
	if opts.RepoRoot == "" {
		return "", fmt.Errorf("repository root is required")
	}
	if inspection.Run.ID < 1 || inspection.Run.ID > maxPipelineRunID {
		return "", fmt.Errorf("pipeline run ID must be between 1 and %d", maxPipelineRunID)
	}
	if err := validatePipelineInspectionExport(inspection); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	parentDir := filepath.Join(opts.RepoRoot, ".adomi", "context", "pipelines")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return "", fmt.Errorf("creating pipeline snapshot directory: %w", err)
	}
	runParentDir := filepath.Join(parentDir, strconv.Itoa(inspection.Run.ID))
	if err := os.MkdirAll(runParentDir, 0o755); err != nil {
		return "", fmt.Errorf("creating pipeline run snapshot directory: %w", err)
	}
	observedAt := inspection.ObservedStart.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	prefix := observedAt.Format("20060102T150405.000000000Z") + "-"
	outputDir, err = os.MkdirTemp(runParentDir, prefix)
	if err != nil {
		return "", fmt.Errorf("creating pipeline snapshot directory: %w", err)
	}
	snapshotDir := outputDir
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(snapshotDir)
		}
	}()

	if err := os.MkdirAll(filepath.Join(outputDir, "logs"), 0o755); err != nil {
		return "", fmt.Errorf("creating pipeline log directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "tests"), 0o755); err != nil {
		return "", fmt.Errorf("creating pipeline test directory: %w", err)
	}
	timelines := inspection.Timelines
	if timelines == nil {
		timelines = []PipelineTimeline{}
	}
	testRuns := inspection.TestRuns
	if testRuns == nil {
		testRuns = []PipelineTestRun{}
	}
	artifacts := pipelineInspectionArtifactPaths{
		Run:         "run.json",
		Timeline:    "timeline.json",
		Logs:        make(map[int]string, len(inspection.Logs)),
		TestRuns:    "tests/runs.json",
		TestResults: make(map[int]string, len(inspection.TestResults)),
	}
	if err := writePipelineInspectionJSON(ctx, filepath.Join(outputDir, artifacts.Run), inspection.Run); err != nil {
		return "", err
	}
	if err := writePipelineInspectionJSON(ctx, filepath.Join(outputDir, artifacts.Timeline), pipelineInspectionTimelineFile{Timelines: timelines}); err != nil {
		return "", err
	}
	for logID, body := range inspection.Logs {
		relativePath := filepath.ToSlash(filepath.Join("logs", strconv.Itoa(logID)+".txt"))
		if err := writePipelineInspectionBytes(ctx, filepath.Join(outputDir, filepath.FromSlash(relativePath)), body); err != nil {
			return "", fmt.Errorf("writing pipeline log %d: %w", logID, err)
		}
		artifacts.Logs[logID] = relativePath
	}
	if err := writePipelineInspectionJSON(ctx, filepath.Join(outputDir, artifacts.TestRuns), pipelineInspectionTestRunsFile{Runs: testRuns}); err != nil {
		return "", err
	}
	for _, testRun := range testRuns {
		runID := testRun.ID
		results := inspection.TestResults[runID]
		if results == nil {
			results = []PipelineTestResult{}
		}
		relativePath := filepath.ToSlash(filepath.Join("tests", strconv.Itoa(runID), "results.json"))
		if err := writePipelineInspectionJSON(ctx, filepath.Join(outputDir, filepath.FromSlash(relativePath)), pipelineInspectionTestResultsFile{Results: results}); err != nil {
			return "", err
		}
		artifacts.TestResults[runID] = relativePath
	}

	index := pipelineInspectionExportIndex{
		Source:          "azure-devops",
		Profile:         opts.Profile,
		Project:         opts.Project,
		BaseURL:         opts.BaseURL,
		BuildURI:        inspection.BuildURI,
		RunID:           inspection.Run.ID,
		ObservedStart:   inspection.ObservedStart.UTC().Format(time.RFC3339Nano),
		ObservedFinish:  inspection.ObservedFinish.UTC().Format(time.RFC3339Nano),
		Scope:           "one-shot observation of the current root and referenced detail timelines, failed-task logs, and published tests; previous attempts are references only; full historical attempt reconstruction was not requested; empty published tests do not establish that no tests executed",
		Counts:          inspection.Counts(),
		LogAvailability: pipelineInspectionLogAvailabilityEntries(inspection),
		Artifacts:       artifacts,
	}
	if err := writePipelineInspectionJSON(ctx, filepath.Join(outputDir, "index.json"), index); err != nil {
		return "", err
	}
	published = true
	return outputDir, nil
}

func validatePipelineInspectionExport(inspection *PipelineInspection) error {
	if inspection.Run.ID < 1 || inspection.Run.ID > maxPipelineRunID {
		return fmt.Errorf("pipeline run ID must be between 1 and %d", maxPipelineRunID)
	}
	seenRuns := make(map[int]struct{}, len(inspection.TestRuns))
	for _, run := range inspection.TestRuns {
		if run.ID < 1 || run.ID > maxPipelineRunID {
			return fmt.Errorf("test run ID %d is invalid", run.ID)
		}
		if _, exists := seenRuns[run.ID]; exists {
			return fmt.Errorf("duplicate test run ID %d", run.ID)
		}
		seenRuns[run.ID] = struct{}{}
	}
	for runID, results := range inspection.TestResults {
		if _, exists := seenRuns[runID]; !exists {
			return fmt.Errorf("test results reference unknown test run ID %d", runID)
		}
		for _, result := range results {
			if result.ID < 1 || result.ID > maxPipelineRunID {
				return fmt.Errorf("test result ID %d is invalid", result.ID)
			}
		}
	}
	for logID := range inspection.Logs {
		if logID < 1 || logID > maxPipelineRunID {
			return fmt.Errorf("log ID %d is invalid", logID)
		}
	}
	return nil
}

func pipelineInspectionLogAvailabilityEntries(inspection *PipelineInspection) []pipelineInspectionLogAvailability {
	entries := make([]pipelineInspectionLogAvailability, 0)
	for _, timeline := range inspection.Timelines {
		for _, record := range timeline.Records {
			if record.LogAvailability == "" {
				continue
			}
			entry := pipelineInspectionLogAvailability{TimelineID: timeline.ID, RecordID: record.ID, Availability: record.LogAvailability, Path: record.LogPath}
			if record.Log != nil {
				logID := record.Log.ID
				entry.LogID = &logID
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

func writePipelineInspectionJSON(ctx context.Context, path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON %s: %w", path, err)
	}
	data = append(data, '\n')
	return writePipelineInspectionBytes(ctx, path, data)
}

func writePipelineInspectionBytes(ctx context.Context, path string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", path, err)
	}
	if err := pipelineInspectionWriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
