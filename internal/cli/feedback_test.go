package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestFeedbackWriterEscapesControlCharacters(t *testing.T) {
	var buf bytes.Buffer
	fw := &feedbackWriter{w: &buf}

	// Wiki page path containing newline should yield a single escaped line
	fw.writeLine("Fetching wiki page %q", "/docs/guide\nsetup")

	output := buf.String()
	if strings.Count(output, "\n") != 1 {
		t.Fatalf("output has %d newlines, want exactly 1: %q", strings.Count(output, "\n"), output)
	}
	if !strings.Contains(output, "\\n") {
		t.Fatalf("output = %q, want escaped newline", output)
	}
	if strings.Contains(output, "\nsetup") {
		t.Fatalf("output = %q, raw newline still present", output)
	}
}

func TestFeedbackWriterEscapesCarriageReturn(t *testing.T) {
	var buf bytes.Buffer
	fw := &feedbackWriter{w: &buf}

	fw.writeLine("Exported %d items to %s", 12, "/path/with\r\ncontrol")

	output := buf.String()
	if output != "Exported 12 items to /path/with\\r\\ncontrol\n" {
		t.Fatalf("output = %q, want numeric formatting preserved and controls escaped", output)
	}
	if strings.Count(output, "\n") != 1 {
		t.Fatalf("output has %d newlines, want exactly 1: %q", strings.Count(output, "\n"), output)
	}
	if strings.Contains(output, "\r") && !strings.Contains(output, "\\r") {
		t.Fatalf("output = %q, raw carriage return still present", output)
	}
}

func TestFeedbackWriterPreservesPrintableQuotesAndBackslashes(t *testing.T) {
	var buf bytes.Buffer
	fw := &feedbackWriter{w: &buf}

	fw.writeLine("Fetching wiki page %q from %s", `/Guide`, `C:\context`)

	if got, want := buf.String(), "Fetching wiki page \"/Guide\" from C:\\context\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFeedbackWriterSingleLinePerEvent(t *testing.T) {
	var buf bytes.Buffer
	fw := &feedbackWriter{w: &buf}

	fw.writeLine("Line 1")
	fw.writeLine("Line 2 with %s", "embedded\nnewline")
	fw.writeLine("Line 3")

	output := buf.String()
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %q", len(lines), output)
	}
}

func TestFeedbackWriterPlainText(t *testing.T) {
	var buf bytes.Buffer
	fw := &feedbackWriter{w: &buf}

	fw.writeLine("Stored Azure DevOps PAT for credential ref %q in the keyring", "ado:default")

	output := buf.String()
	if !strings.Contains(output, "ado:default") {
		t.Fatalf("output = %q, want ado:default", output)
	}
	if strings.Count(output, "\n") != 1 {
		t.Fatalf("output has %d newlines, want exactly 1", strings.Count(output, "\n"))
	}
}

func TestWriteMessageTreatsPercentAsData(t *testing.T) {
	var buf bytes.Buffer
	fw := &feedbackWriter{w: &buf}

	// Callback messages containing % must not be interpreted as format verbs.
	fw.writeMessage("Downloaded 100% done")

	output := buf.String()
	if !strings.Contains(output, "100%") {
		t.Fatalf("output = %q, want 100%% preserved", output)
	}
	if strings.Contains(output, "%!") || strings.Contains(output, "MISSING") {
		t.Fatalf("output = %q, must not contain format-error markers", output)
	}
	if strings.Count(output, "\n") != 1 {
		t.Fatalf("output has %d newlines, want exactly 1", strings.Count(output, "\n"))
	}
}
