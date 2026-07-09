package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/bjarneo/ku/internal/k8s"
)

func newTestLogView() logView {
	l := newLogView(PickTheme("ansi"))
	l.setSize(80, 20)
	return l
}

// setFilter mimics typing a pattern into the focused filter box.
func setFilter(l *logView, pattern string) {
	l.startFilter()
	l.filter.SetValue(pattern)
	l.applyFilter()
}

func TestLogFilterNarrowsToMatches(t *testing.T) {
	l := newTestLogView()
	for _, s := range []string{"info: starting", "error: boom", "info: ready", "ERROR: again"} {
		l.appendLine(s)
	}

	setFilter(&l, "error")
	if l.matched != 1 {
		t.Fatalf("expected 1 match for case-sensitive %q, got %d", "error", l.matched)
	}
	if joined := strings.Join(l.filtered, "\n"); strings.Contains(joined, "info:") || strings.Contains(joined, "ERROR:") {
		t.Fatalf("filtered content should only contain the lower-case error line:\n%s", joined)
	}

	// Case-insensitive flag is honored (RE2 syntax).
	setFilter(&l, "(?i)error")
	if l.matched != 2 {
		t.Fatalf("expected 2 matches for %q, got %d", "(?i)error", l.matched)
	}
}

func TestLogFilterEmptyShowsAll(t *testing.T) {
	l := newTestLogView()
	lines := []string{"one", "two", "three"}
	for _, s := range lines {
		l.appendLine(s)
	}

	setFilter(&l, "two")
	if l.matched != 1 {
		t.Fatalf("expected 1 match, got %d", l.matched)
	}

	setFilter(&l, "")
	if l.matched != len(lines) {
		t.Fatalf("clearing the filter should show all %d lines, got %d", len(lines), l.matched)
	}
	if joined := strings.Join(l.filtered, "\n"); joined != strings.Join(lines, "\n") {
		t.Fatalf("expected full content restored, got:\n%s", joined)
	}
}

func TestLogFilterInvalidRegexIsForgiving(t *testing.T) {
	l := newTestLogView()
	for _, s := range []string{"a", "b", "c"} {
		l.appendLine(s)
	}

	setFilter(&l, "[") // not a valid regex
	if l.re != nil {
		t.Fatalf("invalid pattern should not produce a compiled regex")
	}
	if !l.filterActive() {
		t.Fatalf("an invalid (non-empty) pattern should still register as active")
	}
	if l.matched != 3 {
		t.Fatalf("invalid pattern should keep showing all lines, got %d", l.matched)
	}
}

func TestLogEventDrainsBufferedLinesInOneUpdate(t *testing.T) {
	app := App{client: &k8s.Client{}, theme: PickTheme("ansi"), width: 80, height: 24, screen: screenLogs}
	app.logs = newLogView(app.theme)
	app.logs.setSize(70, 20)
	app.logSession, app.logs.session, app.logs.streams = 7, 7, 1
	ch := make(chan logEvent, 16)
	app.logs.ch = ch
	for i := 0; i < 5; i++ {
		ch <- logEvent{session: 7, line: "buffered " + itoa(i)}
	}

	// One delivered event should pull the 5 already buffered in the same update.
	m, cmd := app.Update(logEvent{session: 7, line: "first"})
	na := m.(App)
	if len(na.logs.lines) != 6 {
		t.Fatalf("expected 6 lines after draining the buffer, got %d", len(na.logs.lines))
	}
	if cmd == nil {
		t.Fatal("expected to keep waiting while a stream is live")
	}
}

func TestLogSelectionCopiesMarkedRange(t *testing.T) {
	l := newTestLogView()
	for i := 0; i < 5; i++ {
		l.appendLine("line-" + itoa(i))
	}
	l.startSelect()
	if !l.selecting {
		t.Fatal("startSelect should enter selection mode")
	}
	l.setSelCursor(1)
	l.mark() // anchor at line 1
	l.setSelCursor(3)

	if got, want := l.copySelection(), "line-1\nline-2\nline-3"; got != want {
		t.Fatalf("copySelection = %q, want %q", got, want)
	}
	if l.selCount() != 3 {
		t.Fatalf("selCount = %d, want 3", l.selCount())
	}
}

func TestLogSelectionWithoutMarkCopiesCursorLine(t *testing.T) {
	l := newTestLogView()
	for i := 0; i < 5; i++ {
		l.appendLine("line-" + itoa(i))
	}
	l.startSelect()
	l.setSelCursor(2) // moving without marking does not extend a range
	if l.marking {
		t.Fatal("entering selection should not start marking")
	}
	if got := l.copySelection(); got != "line-2" {
		t.Fatalf("copySelection = %q, want %q", got, "line-2")
	}
	if l.selCount() != 1 {
		t.Fatalf("selCount = %d, want 1", l.selCount())
	}
}

func TestLogSelectionFlowCopiesAndExits(t *testing.T) {
	app := App{client: &k8s.Client{}, theme: PickTheme("ansi"), keys: defaultKeys(), screen: screenLogs}
	app.logs = newLogView(app.theme)
	app.logs.setSize(70, 10)
	for i := 0; i < 5; i++ {
		app.logs.appendLine("l" + itoa(i))
	}

	m, _ := app.updateLogs(mkKey("v"))
	a := m.(App)
	if !a.logs.selecting {
		t.Fatal("v should start selection")
	}

	m, cmd := a.updateLogs(mkKey("y"))
	a = m.(App)
	if a.logs.selecting {
		t.Fatal("y should end selection")
	}
	if cmd == nil {
		t.Fatal("y should produce a clipboard command")
	}
	if !strings.Contains(a.status, "copied") {
		t.Fatalf("expected a copy status, got %q", a.status)
	}
}

func TestLogSelectionMarkThenExtend(t *testing.T) {
	app := App{client: &k8s.Client{}, theme: PickTheme("ansi"), keys: defaultKeys(), screen: screenLogs}
	app.logs = newLogView(app.theme)
	app.logs.setSize(70, 10)
	for i := 0; i < 5; i++ {
		app.logs.appendLine("l" + itoa(i))
	}

	m, _ := app.updateLogs(mkKey("v"))
	a := m.(App)
	if a.logs.marking {
		t.Fatal("v should not start marking")
	}
	m, _ = a.updateLogs(mkKey("m"))
	a = m.(App)
	if !a.logs.marking {
		t.Fatal("m should start marking")
	}
	m, cmd := a.updateLogs(mkKey("y"))
	a = m.(App)
	if a.logs.selecting {
		t.Fatal("y should end selection")
	}
	if cmd == nil || !strings.Contains(a.status, "copied") {
		t.Fatalf("y should copy: cmd=%v status=%q", cmd != nil, a.status)
	}
}

func TestLogSelectionEscCancels(t *testing.T) {
	app := App{client: &k8s.Client{}, theme: PickTheme("ansi"), keys: defaultKeys(), screen: screenLogs}
	app.logs = newLogView(app.theme)
	app.logs.setSize(70, 10)
	app.logs.appendLine("only line")

	m, _ := app.updateLogs(mkKey("v"))
	a := m.(App)
	m, _ = a.updateLogs(mkKey("esc"))
	a = m.(App)
	if a.logs.selecting {
		t.Fatal("esc should cancel selection")
	}
}

func TestStartingSelectionDoesNotMoveViewport(t *testing.T) {
	for _, wrap := range []bool{true, false} {
		name := "wrap"
		if !wrap {
			name = "nowrap"
		}
		t.Run(name, func(t *testing.T) {
			l := newTestLogView() // 80x20
			for i := 0; i < 200; i++ {
				l.appendLine("line-" + itoa(i))
			}
			l.vp.SoftWrap = wrap
			l.follow = false
			l.vp.SetYOffset(50)
			before := l.vp.YOffset()

			l.startSelect() // keyboard entry
			if got := l.vp.YOffset(); got != before {
				t.Fatalf("startSelect moved the viewport: YOffset %d -> %d", before, got)
			}
			l.stopSelect()

			l.vp.SetYOffset(before)
		})
	}
}

func TestLogSelectionCopiesFullLineWhenNoWrap(t *testing.T) {
	l := newTestLogView() // 80 columns wide
	long := strings.Repeat("x", 300)
	l.appendLine(long)
	l.appendLine("short")

	l.toggleWrap() // no-wrap truncates the long line on screen
	if l.vp.SoftWrap {
		t.Fatal("expected no-wrap mode")
	}
	l.startSelect()
	l.setSelCursor(0)

	if got := l.copySelection(); got != long {
		t.Fatalf("no-wrap copy must return the full untruncated line: got %d chars, want %d", len(got), len(long))
	}
}

func TestLogSelectionNoWrapHighlightHonorsHorizontalScroll(t *testing.T) {
	l := newLogView(PickTheme("ansi"))
	l.setSize(20, 8)
	l.appendLine(strings.Repeat("a", 25) + "TARGET" + strings.Repeat("b", 25))
	l.toggleWrap() // no-wrap mode uses horizontal scrolling
	l.vp.ScrollRight(25)

	l.startSelect()

	if view := ansi.Strip(l.vp.View()); !strings.Contains(view, "TARGET") {
		t.Fatalf("selection highlight ignored horizontal scroll, view:\n%s", view)
	}
}

func TestLogSelectionSnapshotSurvivesStreaming(t *testing.T) {
	l := newTestLogView()
	for i := 0; i < 3; i++ {
		l.appendLine("line-" + itoa(i))
	}
	l.startSelect()
	l.setSelCursor(0)
	l.mark()
	l.setSelCursor(2)

	// Lines keep streaming in while the selection is frozen. The snapshot must
	// not shift or get overwritten by the new lines.
	for i := 0; i < 5; i++ {
		l.appendLine("streamed-" + itoa(i))
	}
	if got, want := l.copySelection(), "line-0\nline-1\nline-2"; got != want {
		t.Fatalf("selection snapshot corrupted by streaming: got %q, want %q", got, want)
	}
}

func TestLogExpandsTabsToPreventOverflow(t *testing.T) {
	l := newTestLogView()
	l.appendLine("a\tb\tc")

	if strings.Contains(l.lines[0], "\t") {
		t.Fatalf("tabs should be expanded in stored log lines, got %q", l.lines[0])
	}
	// a at col 0, tabs advance to the next 8-column stop.
	if l.lines[0] != "a       b       c" {
		t.Fatalf("unexpected tab expansion: %q", l.lines[0])
	}
}

func TestToggleWrapKeepsContentVisibleWhenScrolled(t *testing.T) {
	l := newLogView(PickTheme("ansi"))
	l.setSize(24, 8) // narrow, so long lines wrap into several rows each
	for i := 0; i < 60; i++ {
		l.appendLine(fmt.Sprintf("line-%02d %s", i, strings.Repeat("x", 40)))
	}
	// Following put us at the bottom, where the wrapped-row YOffset is large.
	// Pause there: the stale offset is what used to blank the view on toggle.
	l.follow = false

	if !strings.Contains(l.vp.View(), "line-") {
		t.Fatalf("precondition: wrapped view should show content:\n%q", l.vp.View())
	}
	l.toggleWrap() // wrap -> no-wrap
	if !strings.Contains(l.vp.View(), "line-") {
		t.Fatalf("toggling to no-wrap blanked the log view:\n%q", l.vp.View())
	}
	l.toggleWrap() // no-wrap -> wrap
	if !strings.Contains(l.vp.View(), "line-") {
		t.Fatalf("toggling back to wrap blanked the log view:\n%q", l.vp.View())
	}
}

func TestLogWrapDefaultsOnAndToggles(t *testing.T) {
	l := newTestLogView()
	if !l.vp.SoftWrap {
		t.Fatalf("log view should wrap long lines by default")
	}
	l.toggleWrap()
	if l.vp.SoftWrap {
		t.Fatalf("w should switch to truncate (no wrap)")
	}
	l.toggleWrap()
	if !l.vp.SoftWrap {
		t.Fatalf("w should switch back to wrap")
	}
}

func TestLogFilterAppliesToNewLines(t *testing.T) {
	l := newTestLogView()
	l.appendLine("keep me")
	l.appendLine("drop me")

	setFilter(&l, "keep")
	if l.matched != 1 {
		t.Fatalf("expected 1 match before streaming, got %d", l.matched)
	}

	// New lines arriving while the filter is active are matched too.
	l.appendLine("drop this one")
	l.appendLine("keep this one")
	if l.matched != 2 {
		t.Fatalf("expected 2 matches after streaming, got %d", l.matched)
	}
	if joined := strings.Join(l.filtered, "\n"); strings.Contains(joined, "drop") {
		t.Fatalf("streamed non-matching lines must stay hidden:\n%s", joined)
	}
}

func TestLogCopyAllReturnsWholeBufferIgnoringFilter(t *testing.T) {
	l := newTestLogView()
	l.appendLine("keep me")
	l.appendLine("drop me")

	setFilter(&l, "keep") // only "keep me" is visible on screen
	if l.matched != 1 {
		t.Fatalf("filter should narrow the view to 1 line, got %d", l.matched)
	}
	// copyAll grabs the raw buffer, so the filtered-out line is still copied.
	if got, want := l.copyAll(), "keep me\ndrop me"; got != want {
		t.Fatalf("copyAll = %q, want %q", got, want)
	}
}

func TestLogCopyAllKeyCopiesToClipboard(t *testing.T) {
	app := App{client: &k8s.Client{}, theme: PickTheme("ansi"), keys: defaultKeys(), screen: screenLogs}
	app.logs = newLogView(app.theme)
	app.logs.setSize(70, 10)
	for i := 0; i < 3; i++ {
		app.logs.appendLine("l" + itoa(i))
	}

	// Route through handleKey so the global shortcut layer is exercised too.
	m, cmd := app.handleKey(mkKey("c"))
	a := m.(App)
	if cmd == nil {
		t.Fatal("c should produce a clipboard command")
	}
	if !strings.Contains(a.status, "copied 3 lines") {
		t.Fatalf("expected a copy status, got %q", a.status)
	}
}

func TestLogsPaneKeepsSideBorders(t *testing.T) {
	a := App{theme: PickTheme("ansi")}
	out := a.renderPane(a.theme.PaneActive, "line one\nline two", 40, 10)
	rows := strings.Split(out, "\n")
	if len(rows) < 3 {
		t.Fatalf("expected a framed pane, got %d rows", len(rows))
	}
	// The full frame, including side borders, keeps the TUI look.
	if !strings.Contains(rows[0], "─") || !strings.Contains(rows[len(rows)-1], "─") {
		t.Fatalf("expected top and bottom rules, got:\n%s", out)
	}
	if !strings.Contains(rows[1], "│") {
		t.Fatalf("expected side border on content row, got %q", rows[1])
	}
	if !strings.Contains(rows[1], "line one") {
		t.Fatalf("content row missing text, got %q", rows[1])
	}
}

func TestLogClearKeyEmptiesBufferAndResumesFollow(t *testing.T) {
	app := App{client: &k8s.Client{}, theme: PickTheme("ansi"), keys: defaultKeys(), screen: screenLogs}
	app.logs = newLogView(app.theme)
	app.logs.setSize(70, 10)
	for i := 0; i < 5; i++ {
		app.logs.appendLine("l" + itoa(i))
	}
	app.logs.follow = false // a paused, scrolled-up view

	// ctrl+l clears the buffer; it does not collide with any global shortcut.
	m, _ := app.handleKey(mkKey("ctrl+l"))
	a := m.(App)
	if a.overlay != overlayNone {
		t.Fatalf("ctrl+l in logs should clear, not open an overlay (got overlay %v)", a.overlay)
	}
	if len(a.logs.lines) != 0 || len(a.logs.filtered) != 0 || a.logs.matched != 0 {
		t.Fatalf("clear should empty the buffer: lines=%d filtered=%d matched=%d",
			len(a.logs.lines), len(a.logs.filtered), a.logs.matched)
	}
	if !a.logs.follow {
		t.Fatal("clear should re-enable follow so fresh lines auto-scroll")
	}
	if !strings.Contains(a.status, "cleared") {
		t.Fatalf("expected a clear status, got %q", a.status)
	}

	// The stream keeps running, so new lines flow back in after a clear.
	a.logs.appendLine("fresh")
	if joined := strings.Join(a.logs.filtered, "\n"); joined != "fresh" {
		t.Fatalf("post-clear append = %q, want %q", joined, "fresh")
	}
}
