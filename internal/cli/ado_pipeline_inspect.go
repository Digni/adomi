package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
	"github.com/spf13/cobra"
)

type pipelineInspectionReader interface {
	InspectPipelineRun(context.Context, int) (*ado.PipelineInspection, error)
}

type pipelineInspectArgs struct {
	runID   int
	profile string
	global  bool
	json    bool
}

type pipelineInspectionJSONResult struct {
	Path            string `json:"path"`
	RunID           int    `json:"runId"`
	TimelineRecords int    `json:"timelineRecords"`
	FailedTaskLogs  int    `json:"failedTaskLogs"`
	TestRuns        int    `json:"testRuns"`
	TestResults     int    `json:"testResults"`
}

func (r Runner) newADOPipelineInspectCommand(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:                "inspect <run-id> [--profile <profile-name>] [--global] [--json]",
		Short:              "Export one Azure DevOps pipeline run inspection",
		Long:               adoPipelineInspectHelp,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isHelpRequest(args) {
				return writeCommandHelp(cmd, adoPipelineInspectHelp)
			}
			return r.runADOPipelineInspect(args, stdout, stderr)
		},
	}
}

func (r Runner) runADOPipelineInspect(args []string, stdout, stderr io.Writer) error {
	parsed, err := parsePipelineInspectArgs(args)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "Inspecting Azure DevOps pipeline run %d...\n", parsed.runID)
	client, repoRoot, profile, err := r.newADOInspectionClient(parsed.profile, parsed.global)
	if err != nil {
		return err
	}
	reader, ok := client.(pipelineInspectionReader)
	if !ok {
		return fmt.Errorf("configured Azure DevOps client does not support pipeline inspection")
	}
	inspection, err := reader.InspectPipelineRun(context.Background(), parsed.runID)
	if err != nil {
		return err
	}
	outputDir, err := ado.ExportPipelineInspection(context.Background(), ado.PipelineInspectionExportOptions{
		RepoRoot: repoRoot,
		Profile:  profile.Name,
		Project:  profile.Project,
		BaseURL:  profile.BaseURL,
	}, inspection)
	if err != nil {
		return err
	}
	counts := inspection.Counts()
	fmt.Fprintf(stderr, "Exported pipeline inspection: %d timeline records, %d failed task logs, %d test runs, %d test results\n", counts.TimelineRecords, counts.FailedTaskLogs, counts.TestRuns, counts.TestResults)
	if parsed.json {
		return writeJSONLine(stdout, pipelineInspectionJSONResult{
			Path:            outputDir,
			RunID:           parsed.runID,
			TimelineRecords: counts.TimelineRecords,
			FailedTaskLogs:  counts.FailedTaskLogs,
			TestRuns:        counts.TestRuns,
			TestResults:     counts.TestResults,
		})
	}
	_, err = fmt.Fprintln(stdout, outputDir)
	return err
}

func (r Runner) newADOInspectionClient(requestedProfile string, global bool) (ADOClient, string, config.Profile, error) {
	loaded, err := r.newADOMaintenanceClientContext(requestedProfile, global)
	if err != nil {
		return nil, "", config.Profile{}, err
	}
	return loaded.client, loaded.repoRoot, loaded.profile, nil
}

func parsePipelineInspectArgs(args []string) (pipelineInspectArgs, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return pipelineInspectArgs{}, fmt.Errorf("usage: adomi ado pipeline inspect <run-id> [--profile <profile-name>] [--global] [--json]")
	}
	runID, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil || runID < 1 {
		return pipelineInspectArgs{}, fmt.Errorf("pipeline run ID must be a decimal integer in the range 1..2147483647")
	}
	var parsed pipelineInspectArgs
	parsed.runID = int(runID)
	seenProfile := false
	seenGlobal := false
	seenJSON := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			if seenProfile {
				return pipelineInspectArgs{}, fmt.Errorf("--profile cannot be repeated")
			}
			seenProfile = true
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" || strings.HasPrefix(args[i+1], "-") {
				return pipelineInspectArgs{}, fmt.Errorf("--profile requires a value")
			}
			parsed.profile = args[i+1]
			i++
		case "--global":
			if seenGlobal {
				return pipelineInspectArgs{}, fmt.Errorf("--global cannot be repeated")
			}
			seenGlobal = true
			parsed.global = true
		case "--json":
			if seenJSON {
				return pipelineInspectArgs{}, fmt.Errorf("--json cannot be repeated")
			}
			seenJSON = true
			parsed.json = true
		default:
			return pipelineInspectArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return parsed, nil
}
