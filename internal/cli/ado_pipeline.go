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
	var runs []ado.PipelineRun
	if parsed.last > 0 {
		runs, err = client.ListRecentPipelineRuns(context.Background(), parsed.last)
	} else {
		runs, err = client.ListInProgressPipelineRuns(context.Background())
	}
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

const maxRecentRunCount = 200

type pipelineListArgs struct {
	profile string
	global  bool
	last    int
}

type pipelineGetArgs struct {
	runID   int
	profile string
	global  bool
}

func parsePipelineListArgs(args []string) (pipelineListArgs, error) {
	flags, err := parsePipelineScopeFlags(args, true)
	if err != nil {
		return pipelineListArgs{}, err
	}
	return pipelineListArgs{profile: flags.profile, global: flags.global, last: flags.last}, nil
}

func parsePipelineGetArgs(args []string) (pipelineGetArgs, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return pipelineGetArgs{}, fmt.Errorf("usage: adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]")
	}
	runID, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil || runID < 1 {
		return pipelineGetArgs{}, fmt.Errorf("pipeline run ID must be a decimal integer in the range 1..2147483647")
	}
	flags, err := parsePipelineScopeFlags(args[1:], false)
	if err != nil {
		return pipelineGetArgs{}, err
	}
	return pipelineGetArgs{runID: int(runID), profile: flags.profile, global: flags.global}, nil
}

type pipelineScopeFlags struct {
	profile string
	global  bool
	last    int
}

func parsePipelineScopeFlags(args []string, allowLast bool) (pipelineScopeFlags, error) {
	var flags pipelineScopeFlags
	seenProfile := false
	seenGlobal := false
	seenLast := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--last":
			if !allowLast {
				return pipelineScopeFlags{}, fmt.Errorf("unknown argument %q", args[i])
			}
			if seenLast {
				return pipelineScopeFlags{}, fmt.Errorf("--last cannot be repeated")
			}
			seenLast = true
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" || strings.HasPrefix(args[i+1], "-") {
				return pipelineScopeFlags{}, fmt.Errorf("--last requires a value")
			}
			last, err := strconv.ParseInt(args[i+1], 10, 32)
			if err != nil || last < 1 || last > maxRecentRunCount {
				return pipelineScopeFlags{}, fmt.Errorf("--last must be a decimal integer in the range 1..%d", maxRecentRunCount)
			}
			flags.last = int(last)
			i++
		case "--profile":
			if seenProfile {
				return pipelineScopeFlags{}, fmt.Errorf("--profile cannot be repeated")
			}
			seenProfile = true
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" || strings.HasPrefix(args[i+1], "-") {
				return pipelineScopeFlags{}, fmt.Errorf("--profile requires a value")
			}
			flags.profile = args[i+1]
			i++
		case "--global":
			if seenGlobal {
				return pipelineScopeFlags{}, fmt.Errorf("--global cannot be repeated")
			}
			seenGlobal = true
			flags.global = true
		default:
			return pipelineScopeFlags{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return flags, nil
}
