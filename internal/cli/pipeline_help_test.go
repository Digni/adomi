package cli

import (
	"strings"
	"testing"
)

func TestADOPipelineNamespaceAppearsInADOHelp(t *testing.T) {
	stderr := pipelineRunHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "--help"})
	for _, want := range []string{"pipeline", "read-only"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
}

func TestADOPipelineNamespaceHelpDescribesContract(t *testing.T) {
	stderr := pipelineRunHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "pipeline", "--help"})
	assertPipelineHelpContract(t, stderr)
	for _, want := range []string{
		"adomi ado pipeline list",
		"adomi ado pipeline get <run-id>",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
}

func TestADOPipelineListHelpDescribesContract(t *testing.T) {
	stderr := pipelineRunHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "pipeline", "list", "--help"})
	assertPipelineHelpContract(t, stderr)
	if !strings.Contains(stderr, "adomi ado pipeline list") {
		t.Fatalf("stderr = %q, want list usage", stderr)
	}
}

func TestADOPipelineGetHelpDescribesContract(t *testing.T) {
	stderr := pipelineRunHelp(t, Runner{deps: Dependencies{}}, []string{"ado", "pipeline", "get", "--help"})
	assertPipelineHelpContract(t, stderr)
	if !strings.Contains(stderr, "adomi ado pipeline get <run-id>") {
		t.Fatalf("stderr = %q, want get usage", stderr)
	}
}

func TestADOPipelineHelpIsSideEffectFree(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "namespace long help", args: []string{"ado", "pipeline", "--help"}},
		{name: "namespace short help", args: []string{"ado", "pipeline", "-h"}},
		{name: "list long help after invalid arguments", args: []string{"ado", "pipeline", "list", "unexpected", "--help"}},
		{name: "list short help", args: []string{"ado", "pipeline", "list", "-h"}},
		{name: "get long help after invalid ID", args: []string{"ado", "pipeline", "get", "not-a-run-id", "--help"}},
		{name: "get short help", args: []string{"ado", "pipeline", "get", "-h"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := pipelineRunHelp(t, pipelineFailOnDependencyRunner(t), tt.args)
			if !strings.Contains(stderr, "pipeline") {
				t.Fatalf("stderr = %q, want pipeline help", stderr)
			}
		})
	}
}
