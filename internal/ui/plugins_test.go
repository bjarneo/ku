package ui

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"

	"github.com/bjarneo/ku/internal/k8s"
)

func TestNormalizePluginKey(t *testing.T) {
	tests := map[string]string{
		"ctrl+o":       "ctrl+o",
		" Ctrl+O ":     "ctrl+o",
		"shift+x":      "X",
		"X":            "X",
		"b":            "b",
		" F2 ":         "f2",
		"ctrl+shift+x": "ctrl+shift+x",
		"shift+ctrl+x": "ctrl+shift+x",
		"alt+enter":    "alt+enter",
		"":             "",
	}
	for in, want := range tests {
		if got := normalizePluginKey(in); got != want {
			t.Errorf("normalizePluginKey(%q) = %q; want %q", in, got, want)
		}
	}
}

// Every binding in keyMap must be reserved, so the plugin loader keeps refusing
// built-in keys even when groups() falls out of sync with the struct.
func TestReservedKeysCoverEveryBinding(t *testing.T) {
	keys := defaultKeys()
	reserved := reservedKeys(keys)
	v := reflect.ValueOf(keys)
	for i := 0; i < v.NumField(); i++ {
		b, ok := v.Field(i).Interface().(key.Binding)
		if !ok {
			continue
		}
		for _, k := range b.Keys() {
			if !reserved[k] {
				t.Errorf("key %q of binding %s is not reserved", k, v.Type().Field(i).Name)
			}
		}
	}
	for _, k := range []string{"h", "left", "right", "enter", "ctrl+c", "ctrl+\\"} {
		if !reserved[k] {
			t.Errorf("literal key %q is not reserved", k)
		}
	}
	if reserved["b"] || reserved["ctrl+o"] {
		t.Fatal("free keys must not be reserved")
	}
}

func TestPluginCatalogBuildsBindings(t *testing.T) {
	cfg := parseConfig(t, `
plugins:
  - key: ctrl+o
    desc: Workflow job
    scopes: [Pods, ephemeralrunners]
    command: /usr/local/bin/open-job
    args: [$NAMESPACE, $NAME, $CONTEXT]
    background: true
  - key: shift+x
    scopes: [all]
    command: /bin/describe-all
    confirm: true
`)
	ps, warns := cfg.pluginCatalog(defaultKeys())
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	if len(ps) != 2 {
		t.Fatalf("got %d plugins, want 2", len(ps))
	}
	p := ps[0]
	if p.key != "ctrl+o" || p.desc != "Workflow job" || !p.background || p.confirm {
		t.Fatalf("plugin 0 = %+v", p)
	}
	if !reflect.DeepEqual(p.scopes, []string{"pods", "ephemeralrunners"}) {
		t.Fatalf("scopes = %v", p.scopes)
	}
	if !reflect.DeepEqual(p.args, []string{"$NAMESPACE", "$NAME", "$CONTEXT"}) {
		t.Fatalf("args = %v", p.args)
	}
	if !reflect.DeepEqual(p.binding.Keys(), []string{"ctrl+o"}) {
		t.Fatalf("binding keys = %v", p.binding.Keys())
	}
	if h := p.binding.Help(); h.Key != "ctrl+o" || h.Desc != "Workflow job" {
		t.Fatalf("help = %+v", h)
	}
	q := ps[1]
	if q.key != "X" || q.desc != "describe-all" || !q.confirm || q.background {
		t.Fatalf("plugin 1 = %+v", q)
	}
}

func TestPluginCatalogSkipsInvalid(t *testing.T) {
	cfg := parseConfig(t, `
plugins:
  - { key: d, desc: shadows describe, scopes: [pods], command: x }
  - { desc: no key, scopes: [pods], command: x }
  - { key: b, desc: no command, scopes: [pods] }
  - { key: b, desc: no scopes, command: x }
  - { key: b, desc: first, scopes: [pods], command: x }
  - { key: shift+b, desc: same key as first, scopes: [pods], command: x }
  - { key: b, desc: second, scopes: [pods], command: x }
`)
	ps, warns := cfg.pluginCatalog(defaultKeys())
	if len(ps) != 2 || ps[0].desc != "first" || ps[1].desc != "same key as first" {
		t.Fatalf("plugins = %+v", ps)
	}
	want := []string{
		`shadows describe: key "d" is reserved`,
		`no key: missing key`,
		`no command: missing command`,
		`no scopes: missing scopes`,
		`second: key "b" already used by first`,
	}
	if !reflect.DeepEqual(warns, want) {
		t.Fatalf("warnings = %q\nwant %q", warns, want)
	}
}

func TestPluginCatalogEmpty(t *testing.T) {
	ps, warns := (Config{}).pluginCatalog(defaultKeys())
	if ps != nil || warns != nil {
		t.Fatalf("got %v %v; want nil nil", ps, warns)
	}
}

func TestPluginMatchesScopes(t *testing.T) {
	reg := modeTestRegistry()
	deploy := k8s.ResourceInfo{Resource: "deployments", Group: "apps", Kind: "Deployment", Singular: "deployment", Namespaced: true}
	tests := []struct {
		name   string
		scopes []string
		res    k8s.ResourceInfo
		reg    *k8s.Registry
		want   bool
	}{
		{"plural", []string{"deployments"}, deploy, reg, true},
		{"singular alias", []string{"deployment"}, deploy, reg, true},
		{"kind alias", []string{"Deployment"}, deploy, reg, true},
		{"group key", []string{"deployments.apps"}, deploy, reg, true},
		{"other resource", []string{"pods"}, deploy, reg, false},
		{"all", []string{"all"}, nodesRes, reg, true},
		{"nil registry literal plural", []string{"pods"}, podsRes, nil, true},
		{"nil registry literal kind", []string{"pod"}, podsRes, nil, true},
		{"unknown crd", []string{"ephemeralrunners"}, podsRes, reg, false},
		{"unknown crd literal", []string{"ephemeralrunners"}, k8s.ResourceInfo{Resource: "ephemeralrunners", Group: "actions.github.com", Namespaced: true}, reg, true},
	}
	for _, tt := range tests {
		p := plugin{scopes: tt.scopes}
		for i := range p.scopes {
			p.scopes[i] = strings.ToLower(p.scopes[i])
		}
		if got := p.matches(tt.res, tt.reg); got != tt.want {
			t.Errorf("%s: matches = %v; want %v", tt.name, got, tt.want)
		}
	}
}

func TestExpandPluginArgs(t *testing.T) {
	t.Setenv("KU_TEST_HOME", "/home/x")
	vars := map[string]string{"NAMESPACE": "team", "NAME": "api", "RESOURCE": "deployments.apps"}
	got := expandPluginArgs([]string{"$NAMESPACE/$NAME", "${RESOURCE}", "$KU_TEST_HOME/bin", "$KU_TEST_UNSET", "plain"}, vars)
	want := []string{"team/api", "deployments.apps", "/home/x/bin", "", "plain"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q; want %q", got, want)
	}
}

func TestPluginVars(t *testing.T) {
	cl := &k8s.Client{ContextName: "ci", ClusterName: "ci-eks", Namespace: "ctx-default"}
	deploy := k8s.ResourceInfo{Resource: "deployments", Group: "apps", Namespaced: true}

	a := App{client: cl, res: deploy, namespace: "view-ns"}
	v := a.pluginVars(k8s.Row{Name: "api", Namespace: "row-ns"})
	if v["NAMESPACE"] != "row-ns" || v["NAME"] != "api" || v["RESOURCE"] != "deployments.apps" ||
		v["CONTEXT"] != "ci" || v["CLUSTER"] != "ci-eks" {
		t.Fatalf("vars = %v", v)
	}
	if _, ok := v["KUBECONFIG"]; ok {
		t.Fatal("KUBECONFIG must be unset without an explicit kubeconfig")
	}

	if v := a.pluginVars(k8s.Row{Name: "api"}); v["NAMESPACE"] != "view-ns" {
		t.Fatalf("view namespace fallback: %v", v)
	}
	a.namespace = ""
	if v := a.pluginVars(k8s.Row{Name: "api"}); v["NAMESPACE"] != "ctx-default" {
		t.Fatalf("context namespace fallback: %v", v)
	}
	a.res = nodesRes
	if v := a.pluginVars(k8s.Row{Name: "node-a", Namespace: "ignored"}); v["NAMESPACE"] != "" {
		t.Fatalf("cluster-scoped namespace must be empty: %v", v)
	}

	bare := App{res: podsRes}
	if v := bare.pluginVars(k8s.Row{Name: "p"}); v["CONTEXT"] != "" || v["NAME"] != "p" {
		t.Fatalf("nil client vars = %v", v)
	}
}

func pluginTestApp(res k8s.ResourceInfo, ps ...plugin) App {
	th := PickTheme("ansi")
	app := App{theme: th, keys: defaultKeys(), width: 100, height: 30, screen: screenTable, focus: focusMain,
		client: &k8s.Client{}, res: res, plugins: ps, readOnly: true}
	app.table = newTableView(th)
	app.sel = newSelector(th)
	app.relayout()
	app.table.setData(fakeTable())
	return app
}

func backgroundPlugin(k string, scopes ...string) plugin {
	return plugin{key: k, binding: key.NewBinding(key.WithKeys(k), key.WithHelp(k, "bg")), desc: "bg",
		scopes: scopes, command: "true", background: true}
}

func TestPluginKeyRunsBackgroundFromTable(t *testing.T) {
	app := pluginTestApp(podsRes, backgroundPlugin("b", "pods"))
	m, cmd := app.updateMainKeys(mkKey("b"))
	got := m.(App)
	if cmd == nil {
		t.Fatal("plugin key returned no command")
	}
	if !strings.Contains(got.status, "bg: running") || got.statusErr {
		t.Fatalf("status = %q (err=%v)", got.status, got.statusErr)
	}
	msg, ok := cmd().(statusMsg)
	if !ok || msg.err || msg.text != "bg: done" {
		t.Fatalf("result = %#v", msg)
	}

	// Out of scope: the key falls through to the table and nothing runs.
	app = pluginTestApp(nodesRes, backgroundPlugin("b", "pods"))
	m, cmd = app.updateMainKeys(mkKey("b"))
	if got := m.(App); cmd != nil || got.status != "" {
		t.Fatalf("out-of-scope plugin ran: cmd=%v status=%q", cmd != nil, got.status)
	}
}

func TestPluginKeyIgnoredWhileFiltering(t *testing.T) {
	app := pluginTestApp(podsRes, backgroundPlugin("b", "pods"))
	m, _ := app.updateTable(mkKey("/"))
	app = m.(App)
	m, cmd := app.updateTable(mkKey("b"))
	app = m.(App)
	if !app.table.filtering || app.status != "" {
		t.Fatalf("plugin fired while filtering: filtering=%v status=%q", app.table.filtering, app.status)
	}
	if cmd != nil {
		if _, isStatus := cmd().(statusMsg); isStatus {
			t.Fatal("plugin command ran while filtering")
		}
	}
}

func TestPluginKeyIgnoredWithSidebarFocus(t *testing.T) {
	app := pluginTestApp(podsRes, backgroundPlugin("b", "pods"))
	app.focus = focusSidebar
	m, _ := app.updateTable(mkKey("b"))
	if got := m.(App); got.status != "" {
		t.Fatalf("plugin fired with sidebar focus: %q", got.status)
	}
}

func TestPluginConfirmOpensOverlay(t *testing.T) {
	p := backgroundPlugin("b", "pods")
	p.confirm = true
	app := pluginTestApp(podsRes, p)
	m, cmd := app.updateMainKeys(mkKey("b"))
	app = m.(App)
	if cmd != nil || app.overlay != overlayConfirm {
		t.Fatalf("confirm overlay not opened: overlay=%v", app.overlay)
	}
	if !strings.Contains(app.confirm.message, "default/api-7d9") || app.confirm.danger {
		t.Fatalf("confirm = %+v", app.confirm)
	}
	run, ok := app.confirm.action().(pluginRunMsg)
	if !ok || run.plugin.desc != "bg" || run.vars["NAME"] != "api-7d9" {
		t.Fatalf("confirm action = %#v", run)
	}
	m, cmd = app.Update(run)
	if cmd == nil {
		t.Fatal("pluginRunMsg did not start the command")
	}
	if got := m.(App); !strings.Contains(got.status, "running") {
		t.Fatalf("status = %q", got.status)
	}
}

func TestRunPluginBackground(t *testing.T) {
	run := func(args ...string) statusMsg {
		msg, ok := runPluginBackground("p", exec.Command(args[0], args[1:]...))().(statusMsg)
		if !ok {
			t.Fatal("not a statusMsg")
		}
		return msg
	}
	if got := run("sh", "-c", "printf 'hello\\nworld\\n'"); got.err || got.text != "p: hello" {
		t.Fatalf("stdout: %#v", got)
	}
	if got := run("true"); got.err || got.text != "p: done" {
		t.Fatalf("silent success: %#v", got)
	}
	if got := run("sh", "-c", "echo boom >&2; exit 3"); !got.err || got.text != "p: boom" {
		t.Fatalf("stderr: %#v", got)
	}
	if got := run("sh", "-c", "exit 4"); !got.err || !strings.Contains(got.text, "exit status 4") {
		t.Fatalf("silent failure: %#v", got)
	}
	if got := run("/nonexistent/ku-plugin-binary"); !got.err {
		t.Fatalf("missing binary: %#v", got)
	}
}

func TestPluginPaletteAndHints(t *testing.T) {
	app := pluginTestApp(podsRes, backgroundPlugin("b", "pods"), backgroundPlugin("ctrl+o", "nodes"))

	hints := app.hints()
	var found []string
	for _, h := range hints {
		if h.desc == "bg" {
			found = append(found, h.key)
		}
	}
	if !reflect.DeepEqual(found, []string{"b"}) {
		t.Fatalf("hints for pods = %v; want [b]", found)
	}

	m, _ := app.openPalette()
	app = m.(App)
	var ids []string
	for _, it := range app.sel.items {
		if strings.HasPrefix(it.id, "plugin:") {
			ids = append(ids, it.id+"="+it.desc)
		}
	}
	if !reflect.DeepEqual(ids, []string{"plugin:0=b"}) {
		t.Fatalf("palette plugin items = %v", ids)
	}

	m, cmd := app.applyPalette("plugin:0")
	if cmd == nil || !strings.Contains(m.(App).status, "running") {
		t.Fatal("palette did not run the plugin")
	}
	if _, cmd := app.applyPalette("plugin:9"); cmd != nil {
		t.Fatal("out-of-range plugin index ran something")
	}
	if _, cmd := app.applyPalette("plugin:zz"); cmd != nil {
		t.Fatal("malformed plugin id ran something")
	}
}

func TestPluginHelpGroup(t *testing.T) {
	if g := pluginHelpGroups(nil); g != nil {
		t.Fatalf("empty plugins yielded %v", g)
	}
	h := newHelpView(PickTheme("ansi"), defaultKeys())
	h.extra = pluginHelpGroups([]plugin{backgroundPlugin("ctrl+o", "pods")})
	out := h.View(120, 40)
	if !strings.Contains(out, "Plugins") || !strings.Contains(out, "ctrl+o") {
		t.Fatalf("help view lacks the plugin column:\n%s", out)
	}
}
