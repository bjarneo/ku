package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// logsTestApp builds an App parked on the logs screen with n lines and the view
// scrolled to the top, so viewport row r maps to line r.
func logsTestApp(t *testing.T, n int) App {
	t.Helper()
	th := PickTheme("ansi")
	app := App{theme: th, width: 80, height: 24, screen: screenLogs, focus: focusMain, keys: defaultKeys()}
	app.logs = newLogView(th)
	app.logs.setSize(pagerContentWidth(app.width), pagerContentHeight(app.bodyH()))
	for i := 0; i < n; i++ {
		app.logs.appendLine("line-" + itoa(i))
	}
	app.logs.follow = false
	app.logs.vp.GotoTop()
	return app
}

// logRowY returns the terminal Y for viewport row r: past the header row, the
// top rule, and the single chrome (title) line.
func logRowY(r int) int { return 3 + r }

func TestLogsMouseDragSelectsAndCopies(t *testing.T) {
	app := logsTestApp(t, 10)

	m, _ := app.Update(tea.MouseClickMsg{X: 1, Y: logRowY(0), Button: tea.MouseLeft})
	app = m.(App)
	if !app.logs.selecting {
		t.Fatal("left press should start a selection")
	}

	m, _ = app.Update(tea.MouseMotionMsg{X: 1, Y: logRowY(2), Button: tea.MouseLeft})
	app = m.(App)
	if app.logs.selCount() != 3 {
		t.Fatalf("dragging over 3 rows should select 3 lines, got %d", app.logs.selCount())
	}

	m, cmd := app.Update(tea.MouseReleaseMsg{X: 1, Y: logRowY(2), Button: tea.MouseLeft})
	app = m.(App)
	if !app.logs.selecting {
		t.Fatal("release should keep the selection highlighted, not clear it")
	}
	if app.logs.selCount() != 3 {
		t.Fatalf("selection should stay at 3 lines after copy, got %d", app.logs.selCount())
	}
	if cmd == nil {
		t.Fatal("release after a drag should copy to the clipboard")
	}
	if !strings.Contains(app.status, "chars") || !strings.Contains(app.status, "3 lines") {
		t.Fatalf("expected a char/line copy status, got %q", app.status)
	}

	// esc dismisses the kept selection and returns to the live view.
	m, _ = app.updateLogs(mkKey("esc"))
	if m.(App).logs.selecting {
		t.Fatal("esc should dismiss the kept selection")
	}
}

func TestLogsMouseClickWithoutDragDoesNotCopy(t *testing.T) {
	app := logsTestApp(t, 5)

	m, _ := app.Update(tea.MouseClickMsg{X: 1, Y: logRowY(1), Button: tea.MouseLeft})
	app = m.(App)
	m, cmd := app.Update(tea.MouseReleaseMsg{X: 1, Y: logRowY(1), Button: tea.MouseLeft})
	app = m.(App)

	if app.logs.selecting {
		t.Fatal("a click with no drag should not leave the view stuck in selection")
	}
	if cmd != nil {
		t.Fatal("a plain click should not copy to the clipboard")
	}
	if strings.Contains(app.status, "copied") {
		t.Fatalf("a plain click should not report a copy, got %q", app.status)
	}
}

func TestTableMouseHitTesting(t *testing.T) {
	th := PickTheme("ansi")
	tv := newTableView(th)
	tv.setSize(80, 10)
	tv.setData(fakeTable())

	if ci, ok := tv.colAt(0); !ok || ci != 0 {
		t.Fatalf("colAt(0) = %d, %t; want 0, true", ci, ok)
	}
	if _, ok := tv.rowAt(0); ok {
		t.Fatal("rowAt(0) hit header as row")
	}
	if row, ok := tv.rowAt(1); !ok || row != 0 {
		t.Fatalf("rowAt(1) = %d, %t; want 0, true", row, ok)
	}
	if row, ok := tv.rowAt(2); !ok || row != 1 {
		t.Fatalf("rowAt(2) = %d, %t; want 1, true", row, ok)
	}
}

func TestAppMouseSelectsTableRowsAndSortsHeaders(t *testing.T) {
	th := PickTheme("ansi")
	app := App{theme: th, width: 80, height: 24, screen: screenTable, focus: focusMain}
	app.table = newTableView(th)
	app.relayout()
	app.table.setData(fakeTable())

	m, _ := app.Update(tea.MouseClickMsg{X: 2, Y: 4, Button: tea.MouseLeft})
	app = m.(App)
	if app.table.cursor != 1 {
		t.Fatalf("mouse row click selected cursor %d; want 1", app.table.cursor)
	}

	m, _ = app.Update(tea.MouseClickMsg{X: 2, Y: 2, Button: tea.MouseLeft})
	app = m.(App)
	if app.table.sortCol != 0 {
		t.Fatalf("mouse header click sortCol = %d; want 0", app.table.sortCol)
	}

	m, _ = app.Update(tea.MouseWheelMsg{X: 2, Y: 4, Button: tea.MouseWheelUp})
	app = m.(App)
	if app.table.cursor != 0 {
		t.Fatalf("mouse wheel selected cursor %d; want 0", app.table.cursor)
	}
}

func TestSidebarMouseSelectsEntries(t *testing.T) {
	s := sidebar{
		height:     4,
		selectable: []int{0, 2, 3},
		entries: []navEntry{
			{overview: true, label: "Overview", key: overviewKey},
			{header: true, label: "Workloads"},
			{label: "Pods", key: "pods"},
			{label: "Services", key: "services"},
		},
	}

	if _, ok := s.selectAt(1); ok {
		t.Fatal("selectAt hit a section header")
	}
	e, ok := s.selectAt(2)
	if !ok || e.key != "pods" || s.cursor != 1 {
		t.Fatalf("selectAt(2) = %+v, %t cursor=%d; want pods, true cursor=1", e, ok, s.cursor)
	}
}
