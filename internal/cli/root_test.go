package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootNoArgsPrintsUsageToStderr(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer

	err := runner.Run(nil, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run error = nil, want usage error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Usage:") || !strings.Contains(stderr.String(), "agent") || !strings.Contains(stderr.String(), "config") || !strings.Contains(stderr.String(), "ado") {
		t.Fatalf("stderr = %q, want top-level usage with agent, config, and ado", stderr.String())
	}
}

func TestRootUnknownCommandLeavesStdoutEmpty(t *testing.T) {
	runner := Runner{deps: Dependencies{}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"unknown"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run error = nil, want unknown command error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %q, want unknown command", err.Error())
	}
}
