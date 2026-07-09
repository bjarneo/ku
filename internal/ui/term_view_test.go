package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/vt"
)

func TestTermRendersBorderlessForCleanCopy(t *testing.T) {
	tv := newTermView(PickTheme("ansi"))
	width, bodyH := 60, 16
	cols, rows := termDims(width, bodyH)
	tv.em = vt.NewSafeEmulator(cols, rows)
	tv.cols, tv.rows = cols, rows
	tv.title = "api-7d9 › app"
	tv.em.Write([]byte("$ echo hello\r\nhello world\r\n"))

	out := tv.View(width, bodyH)
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[0], "─") {
		t.Fatalf("expected a top rule for framing, got %q", lines[0])
	}
	// No terminal row may carry a vertical border: native selection must copy
	// clean text, not the pane's │ characters.
	for i, ln := range lines {
		if strings.Contains(ln, "hello world") && strings.Contains(ln, "│") {
			t.Fatalf("terminal row %d carries a vertical border: %q", i, ln)
		}
	}
}
