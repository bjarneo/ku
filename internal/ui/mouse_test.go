package ui

import (
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
	app.logs.setSize(paneContentWidth(app.width), paneContentHeight(app.bodyH()))
	for i := 0; i < n; i++ {
		app.logs.appendLine("line-" + itoa(i))
	}
	app.logs.follow = false
	app.logs.vp.GotoTop()
	return app
}

func TestMouseIgnoredOutsideShell(t *testing.T) {
	app := logsTestApp(t, 5)
	app.logs.follow = true

	m, cmd := app.Update(tea.MouseClickMsg{X: 2, Y: 4, Button: tea.MouseLeft})
	app = m.(App)
	if cmd != nil {
		t.Fatal("mouse click outside shell returned command")
	}
	if app.logs.selecting {
		t.Fatal("mouse click outside shell should not start selection")
	}
	if !app.logs.follow {
		t.Fatal("mouse click outside shell should not pause log follow")
	}

	m, cmd = app.Update(tea.MouseWheelMsg{X: 2, Y: 4, Button: tea.MouseWheelUp})
	app = m.(App)
	if cmd != nil {
		t.Fatal("mouse wheel outside shell returned command")
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

func TestTableMouseEventsIgnored(t *testing.T) {
	th := PickTheme("ansi")
	app := App{theme: th, width: 80, height: 24, screen: screenTable, focus: focusMain}
	app.table = newTableView(th)
	app.relayout()
	app.table.setData(fakeTable())

	m, cmd := app.Update(tea.MouseClickMsg{X: 2, Y: 4, Button: tea.MouseLeft})
	app = m.(App)
	if cmd != nil {
		t.Fatal("mouse click returned command")
	}
	if app.table.cursor != 0 {
		t.Fatalf("mouse row click changed cursor to %d", app.table.cursor)
	}

	m, cmd = app.Update(tea.MouseClickMsg{X: 2, Y: 2, Button: tea.MouseLeft})
	app = m.(App)
	if cmd != nil {
		t.Fatal("mouse header click returned command")
	}
	if app.table.sortCol != -1 {
		t.Fatalf("mouse header click changed sortCol to %d", app.table.sortCol)
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
