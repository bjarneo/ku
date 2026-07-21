package ui

import (
	"strings"
	"testing"

	"github.com/bjarneo/ku/internal/k8s"
)

// fakePodsTable returns a pods-style table whose wide Node column carries the
// scheduled node, like the server pod printer does.
func fakePodsTable(node string) *k8s.Table {
	return &k8s.Table{
		Columns: []k8s.Column{
			{Name: "Name"},
			{Name: "Status"},
			{Name: "Node", Priority: 1},
		},
		Rows: []k8s.Row{
			{Namespace: "default", Name: "api-7d9", Cells: []string{"api-7d9", "Running", node}},
		},
	}
}

func fakeNodesTable() *k8s.Table {
	return &k8s.Table{
		Columns: []k8s.Column{{Name: "Name"}, {Name: "Status"}},
		Rows: []k8s.Row{
			{Name: "node-a", Cells: []string{"node-a", "Ready"}},
			{Name: "node-b", Cells: []string{"node-b", "Ready"}},
		},
	}
}

var (
	podsRes  = k8s.ResourceInfo{Resource: "pods", Kind: "Pod", Namespaced: true}
	nodesRes = k8s.ResourceInfo{Resource: "nodes", Kind: "Node"}
)

func TestGotoPodNodeGuards(t *testing.T) {
	th := PickTheme("ansi")
	tests := []struct {
		name       string
		res        k8s.ResourceInfo
		dev        bool
		tbl        *k8s.Table
		wantStatus string
	}{
		{
			name:       "non-pod resource",
			res:        k8s.ResourceInfo{Group: "apps", Resource: "deployments", Kind: "Deployment", Namespaced: true},
			tbl:        fakeTable(),
			wantStatus: "switch to pods",
		},
		{
			name:       "developer mode",
			res:        podsRes,
			dev:        true,
			tbl:        fakePodsTable("node-a"),
			wantStatus: "developer mode",
		},
		{
			name:       "pod not scheduled",
			res:        podsRes,
			tbl:        fakePodsTable("<none>"),
			wantStatus: "not scheduled",
		},
		{
			name:       "no node column",
			res:        podsRes,
			tbl:        fakeTable(),
			wantStatus: "not scheduled",
		},
		{
			// A client without a loaded registry cannot resolve "nodes".
			name:       "nodes unavailable",
			res:        podsRes,
			tbl:        fakePodsTable("node-a"),
			wantStatus: "nodes view unavailable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := App{client: &k8s.Client{}, res: tt.res, screen: screenTable, dev: tt.dev}
			app.table = newTableView(th)
			app.table.setData(tt.tbl)

			m, cmd := app.gotoPodNode()
			got := m.(App)
			if cmd != nil {
				t.Fatal("guard path returned a command")
			}
			if got.res.Key() != tt.res.Key() {
				t.Fatalf("resource changed to %q", got.res.Key())
			}
			if !got.statusErr || !strings.Contains(got.status, tt.wantStatus) {
				t.Fatalf("status = %q err=%t, want %q", got.status, got.statusErr, tt.wantStatus)
			}
		})
	}
}

func TestGotoPodNodeNoSelectionIsNoop(t *testing.T) {
	app := App{client: &k8s.Client{}, res: podsRes, screen: screenTable}
	app.table = newTableView(PickTheme("ansi"))

	m, cmd := app.gotoPodNode()
	got := m.(App)
	if cmd != nil || got.status != "" {
		t.Fatalf("empty table: cmd=%v status=%q, want no-op", cmd, got.status)
	}
}

func TestJumpToRowSwitchesAndSetsPendingSelect(t *testing.T) {
	th := PickTheme("ansi")
	app := App{client: &k8s.Client{}, res: podsRes, screen: screenTable}
	app.table = newTableView(th)
	app.table.setData(fakePodsTable("node-b"))

	m, cmd := app.jumpToRow(nodesRes, "", "node-b")
	got := m.(App)
	if cmd == nil {
		t.Fatal("jumpToRow returned no load command")
	}
	if !got.res.IsNodes() || got.screen != screenTable {
		t.Fatalf("res = %q screen = %v, want nodes table", got.res.Key(), got.screen)
	}
	if !got.loading {
		t.Fatal("jumpToRow did not start loading")
	}
	if got.pendingSelect != (rowRef{name: "node-b"}) {
		t.Fatalf("pendingSelect = %+v, want node-b", got.pendingSelect)
	}
}

func TestPendingSelectAppliedOnLoad(t *testing.T) {
	client := &k8s.Client{}
	app := App{client: client, res: nodesRes, screen: screenTable, pendingSelect: rowRef{name: "node-b"}}
	app.table = newTableView(PickTheme("ansi"))

	m, _ := app.Update(resourcesLoadedMsg{client: client, res: nodesRes, tbl: fakeNodesTable()})
	got := m.(App)
	if row, ok := got.table.selected(); !ok || row.Name != "node-b" {
		t.Fatalf("selected row = %+v ok=%t, want node-b", row, ok)
	}
	if got.pendingSelect != (rowRef{}) {
		t.Fatalf("pendingSelect = %+v, want cleared", got.pendingSelect)
	}
}

func TestPendingSelectMissingRowIsSafe(t *testing.T) {
	client := &k8s.Client{}
	app := App{client: client, res: nodesRes, screen: screenTable, pendingSelect: rowRef{name: "node-gone"}}
	app.table = newTableView(PickTheme("ansi"))

	m, _ := app.Update(resourcesLoadedMsg{client: client, res: nodesRes, tbl: fakeNodesTable()})
	got := m.(App)
	if row, ok := got.table.selected(); !ok || row.Name != "node-a" {
		t.Fatalf("selected row = %+v ok=%t, want cursor at top", row, ok)
	}
	if got.pendingSelect != (rowRef{}) {
		t.Fatalf("pendingSelect = %+v, want cleared", got.pendingSelect)
	}
}

func TestUseResourceClearsPendingSelect(t *testing.T) {
	app := App{theme: PickTheme("ansi"), pendingSelect: rowRef{name: "node-b"}}
	app.table = newTableView(app.theme)

	app.useResource(podsRes)
	if app.pendingSelect != (rowRef{}) {
		t.Fatalf("pendingSelect = %+v, want cleared", app.pendingSelect)
	}
}

func TestRowCellReadsHiddenWideColumn(t *testing.T) {
	v := newTableView(PickTheme("ansi"))
	v.setData(fakePodsTable("node-a"))
	if v.showWide {
		t.Fatal("wide columns unexpectedly on by default")
	}
	row, ok := v.selected()
	if !ok {
		t.Fatal("no selected row")
	}
	if got := v.rowCell(row, "NODE"); got != "node-a" {
		t.Fatalf("rowCell(NODE) = %q, want node-a", got)
	}
	if got := v.rowCell(row, "Nope"); got != "" {
		t.Fatalf("rowCell(Nope) = %q, want empty", got)
	}
}
