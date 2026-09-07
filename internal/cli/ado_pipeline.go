package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

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
	listOptions := ado.PipelineRunListOptions{BranchName: parsed.branch}
	if parsed.last > 0 {
		runs, err = client.ListRecentPipelineRuns(context.Background(), parsed.last, listOptions)
	} else {
		runs, err = client.ListInProgressPipelineRuns(context.Background(), listOptions)
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

type pipelineRunResult = ado.PipelineRun

type pipelineRunListResult struct {
	Runs []pipelineRunResult `json:"runs"`
}

func projectPipelineRun(run ado.PipelineRun) pipelineRunResult {
	return run
}

const maxRecentRunCount = 200

type pipelineListArgs struct {
	profile string
	global  bool
	last    int
	branch  string
}

type pipelineGetArgs struct {
	runID   int
	profile string
	global  bool
}

func parsePipelineListArgs(args []string) (pipelineListArgs, error) {
	flags, err := parsePipelineScopeFlags(args, true, true)
	if err != nil {
		return pipelineListArgs{}, err
	}
	return pipelineListArgs{profile: flags.profile, global: flags.global, last: flags.last, branch: flags.branch}, nil
}

func parsePipelineGetArgs(args []string) (pipelineGetArgs, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return pipelineGetArgs{}, fmt.Errorf("usage: adomi ado pipeline get <run-id> [--profile <profile-name>] [--global]")
	}
	runID, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil || runID < 1 {
		return pipelineGetArgs{}, fmt.Errorf("pipeline run ID must be a decimal integer in the range 1..2147483647")
	}
	flags, err := parsePipelineScopeFlags(args[1:], false, false)
	if err != nil {
		return pipelineGetArgs{}, err
	}
	return pipelineGetArgs{runID: int(runID), profile: flags.profile, global: flags.global}, nil
}

type pipelineScopeFlags struct {
	profile string
	global  bool
	last    int
	branch  string
}

func parsePipelineScopeFlags(args []string, allowLast, allowBranch bool) (pipelineScopeFlags, error) {
	var flags pipelineScopeFlags
	seenProfile := false
	seenGlobal := false
	seenLast := false
	seenBranch := false
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
		case "--branch":
			if !allowBranch {
				return pipelineScopeFlags{}, fmt.Errorf("unknown argument %q", args[i])
			}
			if seenBranch {
				return pipelineScopeFlags{}, fmt.Errorf("--branch cannot be repeated")
			}
			seenBranch = true
			branch, err := parseFlagValue("--branch", args, i)
			if err != nil {
				return pipelineScopeFlags{}, err
			}
			branch = strings.TrimSpace(branch)
			if branch == "" {
				return pipelineScopeFlags{}, fmt.Errorf("--branch requires a non-empty value")
			}
			flags.branch = normalizeBranchRef(branch)
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
