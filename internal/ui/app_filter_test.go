package ui

import (
	"testing"
)

// filterTestApp returns an app on the table screen with two rows loaded.
func filterTestApp() App {
	th := PickTheme("ansi")
	app := App{theme: th, keys: defaultKeys(), width: 80, height: 24, screen: screenTable, focus: focusMain}
	app.table = newTableView(th)
	app.relayout()
	app.table.setData(fakeTable())
	return app
}

func TestTableFilterArrowConfirmsAndMoves(t *testing.T) {
	app := filterTestApp()

	m, _ := app.updateTable(mkKey("/"))
	app = m.(App)
	if !app.table.filtering {
		t.Fatal("/ did not enter filter mode")
	}

	// "d" fuzzy-matches both fake rows, so the list keeps two entries.
	m, _ = app.updateTable(mkKey("d"))
	app = m.(App)
	if got := app.table.filterValue(); got != "d" {
		t.Fatalf("filter value = %q, want %q", got, "d")
	}

	m, _ = app.updateTable(mkKey("down"))
	app = m.(App)
	if app.table.filtering {
		t.Fatal("down arrow should confirm the filter and leave filter mode")
	}
	if got := app.table.filterValue(); got != "d" {
		t.Fatalf("confirming via arrow dropped the filter text, got %q", got)
	}
	if app.table.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 (the arrow press should also move)", app.table.cursor)
	}

	m, _ = app.updateTable(mkKey("up"))
	app = m.(App)
	if app.table.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after up", app.table.cursor)
	}
}

func TestTableFilterKeepsJKAsText(t *testing.T) {
	app := filterTestApp()

	m, _ := app.updateTable(mkKey("/"))
	app = m.(App)
	for _, s := range []string{"j", "k"} {
		m, _ = app.updateTable(mkKey(s))
		app = m.(App)
	}
	if !app.table.filtering {
		t.Fatal("j/k should stay in filter mode")
	}
	if got := app.table.filterValue(); got != "jk" {
		t.Fatalf("filter value = %q, want %q (j/k must type, not move)", got, "jk")
	}
}

func TestPagerFilterArrowConfirms(t *testing.T) {
	app := App{client: nil, theme: PickTheme("ansi"), keys: defaultKeys(), width: 80, height: 24, screen: screenLogs}
	app.logs = newLogView(app.theme)
	app.logs.setSize(70, 20)
	for i := 0; i < 40; i++ {
		app.logs.appendLine("log line " + itoa(i))
	}

	m, _ := app.updateLogs(mkKey("/"))
	app = m.(App)
	if !app.logs.filtering {
		t.Fatal("/ did not enter filter mode")
	}

	m, _ = app.updateLogs(mkKey("1"))
	app = m.(App)
	if got := app.logs.filter.Value(); got != "1" {
		t.Fatalf("filter value = %q, want %q", got, "1")
	}

	m, _ = app.updateLogs(mkKey("down"))
	app = m.(App)
	if app.logs.filtering {
		t.Fatal("down arrow should confirm the filter and leave filter mode")
	}
	if !app.logs.filterActive() {
		t.Fatal("confirming via arrow dropped the applied pattern")
	}
}
