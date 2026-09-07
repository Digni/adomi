package ado

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportPipelineInspectionCreatesUniqueCompleteSnapshot(t *testing.T) {
	root := t.TempDir()
	inspection := testPipelineInspectionForExport()
	opts := PipelineInspectionExportOptions{RepoRoot: root, Profile: "company", Project: "Project", BaseURL: "https://dev.azure.com/org"}

	first, err := ExportPipelineInspection(context.Background(), opts, inspection)
	if err != nil {
		t.Fatalf("first export returned error: %v", err)
	}
	second, err := ExportPipelineInspection(context.Background(), opts, inspection)
	if err != nil {
		t.Fatalf("second export returned error: %v", err)
	}
	if first == second {
		t.Fatalf("snapshot paths are equal: %q", first)
	}
	for _, snapshot := range []string{first, second} {
		for _, relativePath := range []string{
			"index.json", "run.json", "timeline.json", "logs/7.txt", "tests/runs.json", "tests/1/results.json",
		} {
			if _, err := os.Stat(filepath.Join(snapshot, filepath.FromSlash(relativePath))); err != nil {
				t.Fatalf("snapshot %s missing %s: %v", snapshot, relativePath, err)
			}
		}
	}

	var index map[string]any
	indexBytes, err := os.ReadFile(filepath.Join(first, "index.json"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		t.Fatalf("decode index: %v", err)
	}
	if index["source"] != "azure-devops" || index["profile"] != "company" || index["project"] != "Project" || index["baseUrl"] != opts.BaseURL || index["buildUri"] != inspection.BuildURI {
		t.Fatalf("index source fields = %#v", index)
	}
	if index["scope"] == nil || !strings.Contains(index["scope"].(string), "historical attempt reconstruction was not requested") {
		t.Fatalf("index scope = %#v, want explicit one-shot scope", index["scope"])
	}
	counts := index["counts"].(map[string]any)
	if counts["timelineRecords"] != float64(2) || counts["failedTaskLogs"] != float64(1) || counts["testRuns"] != float64(1) || counts["testResults"] != float64(1) {
		t.Fatalf("index counts = %#v", counts)
	}
	if len(index["logAvailability"].([]any)) != 2 {
		t.Fatalf("index log availability = %#v, want both task states", index["logAvailability"])
	}
	if got := inspection.Counts(); got != (PipelineInspectionCounts{TimelineRecords: 2, FailedTaskLogs: 1, TestRuns: 1, TestResults: 1}) {
		t.Fatalf("Counts() = %#v", got)
	}
}

func TestExportPipelineInspectionRemovesOnlyUnfinishedSnapshot(t *testing.T) {
	root := t.TempDir()
	inspection := testPipelineInspectionForExport()
	first, err := ExportPipelineInspection(context.Background(), PipelineInspectionExportOptions{RepoRoot: root}, inspection)
	if err != nil {
		t.Fatalf("first export returned error: %v", err)
	}
	bad := testPipelineInspectionForExport()
	bad.Timelines[0].Records[0].Issues = []PipelineTimelineIssue{{Data: map[string]any{"unsupported": func() {}}}}
	if _, err := ExportPipelineInspection(context.Background(), PipelineInspectionExportOptions{RepoRoot: root}, bad); err == nil {
		t.Fatal("bad export error = nil, want JSON encoding failure")
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("prior snapshot was removed: %v", err)
	}
	runParent := filepath.Join(root, ".adomi", "context", "pipelines", "42")
	entries, err := os.ReadDir(runParent)
	if err != nil {
		t.Fatalf("read snapshot parent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("snapshot parent entries = %d, want only prior snapshot", len(entries))
	}
}

func TestExportPipelineInspectionWritesExplicitEmptyCollections(t *testing.T) {
	root := t.TempDir()
	inspection := &PipelineInspection{
		Run:            PipelineRun{ID: 42, Status: "inProgress"},
		BuildURI:       "build-uri",
		ObservedStart:  time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC),
		ObservedFinish: time.Date(2026, 9, 7, 10, 0, 1, 0, time.UTC),
	}
	snapshot, err := ExportPipelineInspection(context.Background(), PipelineInspectionExportOptions{RepoRoot: root}, inspection)
	if err != nil {
		t.Fatalf("empty export returned error: %v", err)
	}
	for _, relativePath := range []string{"timeline.json", "tests/runs.json"} {
		data, err := os.ReadFile(filepath.Join(snapshot, filepath.FromSlash(relativePath)))
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		if strings.Contains(string(data), "null") {
			t.Fatalf("%s contains null collection: %s", relativePath, data)
		}
	}
}

func TestExportPipelineInspectionRemovesSnapshotAfterInjectedWriteFailure(t *testing.T) {
	root := t.TempDir()
	inspection := testPipelineInspectionForExport()
	prior, err := ExportPipelineInspection(context.Background(), PipelineInspectionExportOptions{RepoRoot: root}, inspection)
	if err != nil {
		t.Fatalf("prior export returned error: %v", err)
	}

	originalWriter := pipelineInspectionWriteFile
	t.Cleanup(func() { pipelineInspectionWriteFile = originalWriter })
	writes := 0
	pipelineInspectionWriteFile = func(path string, data []byte, mode os.FileMode) error {
		writes++
		if writes >= 2 {
			return os.ErrPermission
		}
		return originalWriter(path, data, mode)
	}
	if _, err := ExportPipelineInspection(context.Background(), PipelineInspectionExportOptions{RepoRoot: root}, inspection); err == nil || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("injected write failure = %v, want permission error", err)
	}
	if writes < 2 {
		t.Fatalf("write attempts = %d, want failure after an earlier artifact", writes)
	}
	if _, err := os.Stat(prior); err != nil {
		t.Fatalf("prior snapshot was removed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".adomi", "context", "pipelines", "42"))
	if err != nil {
		t.Fatalf("read snapshot parent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("snapshot parent entries = %d, want only prior snapshot", len(entries))
	}
}

func testPipelineInspectionForExport() *PipelineInspection {
	failed := "failed"
	skipped := "skipped"
	return &PipelineInspection{
		Run:      PipelineRun{ID: 42, PipelineID: 7, PipelineName: "Pipeline", RunNumber: "run-42", Status: "completed", Result: &failed},
		BuildURI: "vstfs:///Build/Build/42",
		Timelines: []PipelineTimeline{{ID: inspectionRootTimelineID, Records: []PipelineTimelineRecord{
			{ID: inspectionRootRecordID, Type: "Task", Result: &failed, Log: &PipelineTimelineLogReference{ID: 7}, LogAvailability: "downloaded", LogPath: "logs/7.txt"},
			{ID: inspectionSecondRecordID, Type: "Task", Result: &skipped, LogAvailability: "notRequested"},
		}}},
		Logs:          map[int][]byte{7: []byte("log body\n")},
		TestRuns:      []PipelineTestRun{{ID: 1, Name: "tests", CalculatedResultCount: 1}},
		TestResults:   map[int][]PipelineTestResult{1: {{ID: 2, TestRunID: 1, Outcome: "Passed"}}},
		ObservedStart: time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC), ObservedFinish: time.Date(2026, 9, 7, 10, 0, 1, 0, time.UTC),
	}
}
