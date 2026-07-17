package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Digni/adomi/internal/ado"
)

func (r Runner) runADOPipelineList(args []string, stdout io.Writer) error {
	parsed, err := parsePipelineListArgs(args)
	if err != nil {
		return err
	}
	client, err := r.newADOMaintenanceClient(parsed.profile, parsed.global)
	if err != nil {
		return err
	}
	runs, err := client.ListInProgressPipelineRuns(context.Background())
	if err != nil {
		return err
	}
	projected := make([]pipelineRunResult, len(runs))
	for i := range runs {
		projected[i] = projectPipelineRun(runs[i])
	}
	return writeJSONLine(stdout, pipelineRunListResult{Runs: projected})
}

func (r Runner) runADOPipelineGet(args []string, stdout io.Writer) error {
	parsed, err := parsePipelineGetArgs(args)
	if err != nil {
		return err
	}
	client, err := r.newADOMaintenanceClient(parsed.profile, parsed.global)
	if err != nil {
		return err
	}
	run, err := client.GetPipelineRun(context.Background(), parsed.runID)
	if err != nil {
		return err
	}
	return writeJSONLine(stdout, projectPipelineRun(*run))
}

type pipelineRunResult struct {
	ID            int        `json:"id"`
	PipelineID    int        `json:"pipelineId"`
	PipelineName  string     `json:"pipelineName"`
	RunNumber     string     `json:"runNumber"`
	Status        string     `json:"status"`
	Result        *string    `json:"result"`
	SourceBranch  *string    `json:"sourceBranch"`
	SourceVersion *string    `json:"sourceVersion"`
	QueueTime     *time.Time `json:"queueTime"`
	StartTime     *time.Time `json:"startTime"`
	FinishTime    *time.Time `json:"finishTime"`
	WebURL        *string    `json:"webUrl"`
}

type pipelineRunListResult struct {
	Runs []pipelineRunResult `json:"runs"`
}

func projectPipelineRun(run ado.PipelineRun) pipelineRunResult {
	return pipelineRunResult{
		ID:            run.ID,
		PipelineID:    run.PipelineID,
		PipelineName:  run.PipelineName,
		RunNumber:     run.RunNumber,
		Status:        run.Status,
		Result:        run.Result,
		SourceBranch:  run.SourceBranch,
		SourceVersion: run.SourceVersion,
		QueueTime:     run.QueueTime,
		StartTime:     run.StartTime,
		FinishTime:    run.FinishTime,
		WebURL:        run.WebURL,
	}
}

type pipelineListArgs struct {
	profile string
	global  bool
}

type pipelineGetArgs struct {
	runID   int
	profile string
	global  bool
}

func parsePipelineListArgs(args []string) (pipelineListArgs, error) {
	profile, global, err := parsePipelineScopeArgs(args)
	if err != nil {
		return pipelineListArgs{}, err
	}
	return pipelineListArgs{profile: profile, global: global}, nil
}

func parsePipelineGetArgs(args []string) (pipelineGetArgs, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return pipelineGetArgs{}, fmt.Errorf("usage: adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]")
	}
	runID, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil || runID < 1 {
		return pipelineGetArgs{}, fmt.Errorf("pipeline run ID must be a decimal integer in the range 1..2147483647")
	}
	profile, global, err := parsePipelineScopeArgs(args[1:])
	if err != nil {
		return pipelineGetArgs{}, err
	}
	return pipelineGetArgs{runID: int(runID), profile: profile, global: global}, nil
}

func parsePipelineScopeArgs(args []string) (string, bool, error) {
	var profile string
	var global bool
	seenProfile := false
	seenGlobal := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			if seenProfile {
				return "", false, fmt.Errorf("--profile cannot be repeated")
			}
			seenProfile = true
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" || strings.HasPrefix(args[i+1], "-") {
				return "", false, fmt.Errorf("--profile requires a value")
			}
			profile = args[i+1]
			i++
		case "--global":
			if seenGlobal {
				return "", false, fmt.Errorf("--global cannot be repeated")
			}
			seenGlobal = true
			global = true
		default:
			return "", false, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return profile, global, nil
}
