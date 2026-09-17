package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/bjarneo/ku/internal/k8s"
)

func impersonateTestApp(imp k8s.Impersonation) App {
	return App{
		theme:       PickTheme("ansi"),
		keys:        defaultKeys(),
		client:      &k8s.Client{ContextName: "prod"},
		res:         k8s.ResourceInfo{Resource: "pods", Kind: "Pod", Namespaced: true},
		namespace:   "default",
		width:       160,
		impersonate: imp,
	}
}

func TestImpersonationLabel(t *testing.T) {
	for _, tc := range []struct {
		name string
		imp  k8s.Impersonation
		want string
	}{
		{"user", k8s.Impersonation{User: "system:admin"}, "system:admin"},
		{"one group", k8s.Impersonation{User: "u", Groups: []string{"a"}}, "u +1g"},
		{"two groups", k8s.Impersonation{User: "u", Groups: []string{"a", "b"}}, "u +2g"},
		{"uid is not shown", k8s.Impersonation{User: "u", UID: "42"}, "u"},
		{"no user", k8s.Impersonation{Groups: []string{"a"}}, "(no user) +1g"},
	} {
		if got := impersonationLabel(tc.imp); got != tc.want {
			t.Errorf("%s: impersonationLabel() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// An active impersonation must always be visible: every call in the session
// acts as someone else.
func TestHeaderShowsImpersonationChip(t *testing.T) {
	app := impersonateTestApp(k8s.Impersonation{User: "system:admin", Groups: []string{"system:masters"}})

	got := ansi.Strip(app.headerView())
	if !strings.Contains(got, "as system:admin +1g") {
		t.Fatalf("headerView() = %q, want an \"as\" chip", got)
	}
}

func TestHeaderOmitsImpersonationChipWhenUnset(t *testing.T) {
	app := impersonateTestApp(k8s.Impersonation{})

	got := ansi.Strip(app.headerView())
	if strings.Contains(got, "as ") {
		t.Fatalf("headerView() = %q, want no \"as\" chip", got)
	}
}

// The help overlay is a keybinding reference, so the identity stays out of it.
// The header chip and the `C` command preview are where it shows.
func TestModeNoteOmitsImpersonation(t *testing.T) {
	for _, tc := range []struct {
		name string
		app  App
		want string
	}{
		{
			name: "impersonation only",
			app:  App{impersonate: k8s.Impersonation{User: "system:admin"}},
			want: "",
		},
		{
			name: "impersonation and read-only",
			app: App{
				readOnly:    true,
				impersonate: k8s.Impersonation{User: "system:admin", Groups: []string{"system:masters"}},
			},
			want: "Read-only mode: writes are disabled. Fat-fingers, your cluster is safe.",
		},
		{
			name: "no mode and no impersonation",
			app:  App{},
			want: "",
		},
	} {
		if got := tc.app.modeNote(); got != tc.want {
			t.Errorf("%s: modeNote() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A context switch rebuilds the client, so the identity has to be kept on the
// App and handed to the new client. The command itself needs a cluster, so this
// asserts the state it is built from rather than running it.
func TestContextSwitchKeepsImpersonation(t *testing.T) {
	imp := k8s.Impersonation{User: "system:admin", Groups: []string{"system:masters"}}
	app := impersonateTestApp(imp)
	app.sel = newSelector(app.theme)
	app.sel.open(selContext, "Switch context", "context", []selItem{{title: "staging", id: "staging"}}, false)

	next, cmd := app.applySelection(selResult{accepted: true, id: "staging"})
	if cmd == nil {
		t.Fatal("applySelection returned no command for a context switch")
	}
	na, ok := next.(App)
	if !ok {
		t.Fatalf("applySelection returned %T, want App", next)
	}
	if na.impersonate.User != imp.User || len(na.impersonate.Groups) != 1 {
		t.Errorf("impersonate = %+v, want %+v", na.impersonate, imp)
	}
	if got := ansi.Strip(na.headerView()); !strings.Contains(got, "as system:admin") {
		t.Errorf("headerView() = %q, want the \"as\" chip to survive the switch", got)
	}
}

// The header drops chips that do not fit. A wide "as" chip must not take the
// chips after it with it: the filter chip is what keeps a narrowed list from
// looking like the whole set.
func TestHeaderKeepsFilterChipWhenImpersonationChipOverflows(t *testing.T) {
	app := impersonateTestApp(k8s.Impersonation{
		User:   "system:serviceaccount:kube-system:cluster-admin",
		Groups: []string{"system:masters"},
	})
	app.width = 100
	app.client = &k8s.Client{ContextName: "production-eu-west-1"}
	app.namespace = "kube-system"
	app.table = newTableView(app.theme)
	app.table.filter.SetValue("nginx")

	got := ansi.Strip(app.headerView())
	if !strings.Contains(got, "filter /nginx") {
		t.Fatalf("headerView() = %q, want the filter chip to survive an overflowing \"as\" chip", got)
	}
}
