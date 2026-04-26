package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMainPrintsErrorsToStderrAndExitsNonZero(t *testing.T) {
	if os.Getenv("ADOMI_TEST_MAIN") == "1" {
		os.Args = []string{os.Args[0]}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainPrintsErrorsToStderrAndExitsNonZero")
	cmd.Env = append(os.Environ(), "ADOMI_TEST_MAIN=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: adomi ado <command>") {
		t.Fatalf("stderr = %q, want usage error", stderr.String())
	}
}
