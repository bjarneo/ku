package ui

import (
	"strings"
	"testing"

	"github.com/bjarneo/ku/internal/k8s"
)

func TestGotoNodePodsGuards(t *testing.T) {
	th := PickTheme("ansi")
	tests := []struct {
		name       string
		res        k8s.ResourceInfo
		tbl        *k8s.Table
		wantStatus string
	}{
		{
			name:       "non-nodes resource",
			res:        podsRes,
			tbl:        fakePodsTable("node-a"),
			wantStatus: "switch to nodes",
		},
		{
			// A client without a loaded registry cannot resolve "pods".
			name:       "pods unavailable",
			res:        nodesRes,
			tbl:        fakeNodesTable(),
			wantStatus: "pods view unavailable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := App{client: &k8s.Client{}, res: tt.res, screen: screenTable}
			app.table = newTableView(th)
			app.table.setData(tt.tbl)

			m, cmd := app.gotoNodePods()
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

func TestGotoNodePodsNoSelectionIsNoop(t *testing.T) {
	app := App{client: &k8s.Client{}, res: nodesRes, screen: screenTable}
	app.table = newTableView(PickTheme("ansi"))

	m, cmd := app.gotoNodePods()
	got := m.(App)
	if cmd != nil || got.status != "" {
		t.Fatalf("empty table: cmd=%v status=%q, want no-op", cmd, got.status)
	}
}

func TestScopeToNodePodsSwitchesAndWidensNamespace(t *testing.T) {
	th := PickTheme("ansi")
	app := App{client: &k8s.Client{}, res: nodesRes, screen: screenTable, namespace: "default"}
	app.table = newTableView(th)
	app.table.setData(fakeNodesTable())

	m, cmd := app.scopeToNodePods(podsRes, "node-a")
	got := m.(App)
	if cmd == nil {
		t.Fatal("scopeToNodePods returned no load command")
	}
	if !got.res.IsPod() || got.screen != screenTable {
		t.Fatalf("res = %q screen = %v, want pods table", got.res.Key(), got.screen)
	}
	if got.namespace != "" || got.lastNS != "default" {
		t.Fatalf("namespace = %q lastNS = %q, want all namespaces with default saved", got.namespace, got.lastNS)
	}
	if got.nodeScope != "node-a" {
		t.Fatalf("nodeScope = %q, want node-a", got.nodeScope)
	}
	if !got.loading {
		t.Fatal("scopeToNodePods did not start loading")
	}
}

func TestScopeSelector(t *testing.T) {
	tests := []struct {
		name string
		app  App
		want string
	}{
		{name: "pods scoped", app: App{res: podsRes, nodeScope: "node-a"}, want: "spec.nodeName=node-a"},
		{name: "no scope", app: App{res: podsRes}, want: ""},
		{name: "scope on non-pods", app: App{res: nodesRes, nodeScope: "node-a"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.app.scopeSelector(); got != tt.want {
				t.Fatalf("scopeSelector() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEscClearsNodeScope(t *testing.T) {
	app := App{client: &k8s.Client{}, theme: PickTheme("ansi"), res: podsRes, screen: screenTable, nodeScope: "node-a"}
	app.keys = defaultKeys()
	app.table = newTableView(app.theme)
	app.table.setData(fakePodsTable("node-a"))

	m, cmd := app.updateTable(mkKey("esc"))
	got := m.(App)
	if got.nodeScope != "" {
		t.Fatalf("nodeScope = %q, want cleared", got.nodeScope)
	}
	if cmd == nil {
		t.Fatal("clearing the scope did not reload")
	}
	if got.status == "" || got.statusErr {
		t.Fatalf("status = %q err=%t, want notice", got.status, got.statusErr)
	}
}

func TestUseResourceClearsNodeScope(t *testing.T) {
	app := App{theme: PickTheme("ansi"), nodeScope: "node-a"}
	app.table = newTableView(app.theme)

	app.useResource(nodesRes)
	if app.nodeScope != "" {
		t.Fatalf("nodeScope = %q, want cleared", app.nodeScope)
	}
}

func TestKubectlCommandIncludesNodeScope(t *testing.T) {
	app := App{client: &k8s.Client{}, res: podsRes, screen: screenTable, nodeScope: "node-a"}
	app.table = newTableView(PickTheme("ansi"))

	cmd := app.kubectlGetTableCommand()
	if !strings.Contains(cmd, "--field-selector spec.nodeName=node-a") {
		t.Fatalf("kubectl command %q missing field selector", cmd)
	}
}
