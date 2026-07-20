package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// feedbackWriter writes line-based progress, summary, and confirmation
// messages to stderr. External text interpolated into messages is escaped
// so that every event remains exactly one physical line.
type feedbackWriter struct {
	w io.Writer
}

// writeLine formats a message, escapes any remaining control characters, and
// writes it as one physical line.
func (f *feedbackWriter) writeLine(format string, a ...any) {
	fmt.Fprintf(f.w, "%s\n", escapeFeedback(fmt.Sprintf(format, a...)))
}

// writeMessage writes a single progress message to the writer. The message
// is treated as data (not a format string) and is control-character-escaped.
func (f *feedbackWriter) writeMessage(msg string) {
	fmt.Fprintf(f.w, "%s\n", escapeFeedback(msg))
}

// escapeFeedback applies strconv.Quote-style escaping to external text
// so that control characters (newlines, carriage returns, etc.) don't
// break the one-line-per-event contract.
func escapeFeedback(s string) string {
	var escaped strings.Builder
	for _, r := range s {
		if !unicode.IsControl(r) {
			escaped.WriteRune(r)
			continue
		}
		quoted := strconv.QuoteRune(r)
		escaped.WriteString(quoted[1 : len(quoted)-1])
	}
	return escaped.String()
}
