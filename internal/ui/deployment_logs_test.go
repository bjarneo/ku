package ui

import (
	"testing"

	"github.com/bjarneo/kli/internal/k8s"
)

func TestDeploymentLogsKeyStartsLookup(t *testing.T) {
	th := PickTheme("ansi")
	app := App{
		client:    &k8s.Client{},
		theme:     th,
		keys:      defaultKeys(),
		width:     80,
		height:    20,
		screen:    screenTable,
		res:       k8s.ResourceInfo{Group: "apps", Resource: "deployments", Kind: "Deployment", Namespaced: true},
		namespace: "default",
		focus:     focusMain,
	}
	app.table = newTableView(th)
	app.table.setData(&k8s.Table{
		Columns: []k8s.Column{{Name: "Name"}},
		Rows:    []k8s.Row{{Namespace: "default", Name: "api", Cells: []string{"api"}}},
	})

	model, cmd := app.updateMainKeys(mkKey("L"))
	got := model.(App)
	if cmd == nil {
		t.Fatal("deployment logs key returned nil command")
	}
	if got.logTarget.ns != "default" || got.logTarget.name != "api" || !got.logTarget.res.IsDeployment() {
		t.Fatalf("logTarget = %+v, want default/api deployment", got.logTarget)
	}
	if got.status == "" || got.statusErr {
		t.Fatalf("status = %q err=%t, want non-error loading status", got.status, got.statusErr)
	}
}

func TestDeploymentLogsKeyRequiresDeployment(t *testing.T) {
	app := App{
		keys:   defaultKeys(),
		screen: screenTable,
		res:    k8s.ResourceInfo{Resource: "pods", Kind: "Pod", Namespaced: true},
	}
	app.table = newTableView(PickTheme("ansi"))
	app.table.setData(fakeTable())

	model, cmd := app.updateMainKeys(mkKey("L"))
	got := model.(App)
	if cmd != nil {
		t.Fatal("non-deployment logs key returned command")
	}
	if got.status != "logs: switch to deployments first" || !got.statusErr {
		t.Fatalf("status = %q err=%t, want deployment error", got.status, got.statusErr)
	}
}

func TestDeploymentLogDoneWaitsForRemainingStreams(t *testing.T) {
	app := App{
		screen:     screenLogs,
		logSession: 7,
		logs: logView{
			streams: 2,
			ch:      make(chan logEvent, 1),
		},
	}

	model, cmd := app.Update(logEvent{session: 7, done: true})
	got := model.(App)
	if got.logs.streams != 1 {
		t.Fatalf("streams after first done = %d, want 1", got.logs.streams)
	}
	if cmd == nil {
		t.Fatal("first done returned nil command")
	}

	model, cmd = got.Update(logEvent{session: 7, done: true})
	got = model.(App)
	if got.logs.streams != 0 {
		t.Fatalf("streams after second done = %d, want 0", got.logs.streams)
	}
	if cmd != nil {
		t.Fatal("final done returned command")
	}
}
