package ui

import (
	"strings"
	"testing"

	"github.com/bjarneo/ku/internal/k8s"
)

func TestKubectlTableCommandAllNamespaces(t *testing.T) {
	app := App{
		res: k8s.ResourceInfo{Resource: "pods", Namespaced: true},
	}

	got := app.kubectlGetTableCommand()
	want := "kubectl get pods --all-namespaces"
	if got != want {
		t.Fatalf("kubectlGetTableCommand() = %q; want %q", got, want)
	}
}

func TestKubectlObjectCommandUsesContextNamespaceAndGroup(t *testing.T) {
	app := App{
		client:    &k8s.Client{ContextName: "kind-ku-demo"},
		namespace: "default",
	}
	target := target{
		res:  k8s.ResourceInfo{Group: "apps", Resource: "deployments", Namespaced: true},
		name: "frontend",
	}

	got := app.kubectlGetObjectCommand(target)
	want := "kubectl --context kind-ku-demo get deployments.apps frontend -n default -o yaml"
	if got != want {
		t.Fatalf("kubectlGetObjectCommand() = %q; want %q", got, want)
	}
}

func TestKubectlLogsCommand(t *testing.T) {
	app := App{
		client: &k8s.Client{ContextName: "kind-ku-demo"},
		logs: logView{
			ns:   "ku-demo",
			pod:  "frontend-7d9",
			cont: "web",
		},
	}

	got := app.kubectlLogsCommand()
	want := "kubectl --context kind-ku-demo logs -n ku-demo frontend-7d9 -c web --tail 1000 -f"
	if got != want {
		t.Fatalf("kubectlLogsCommand() = %q; want %q", got, want)
	}
}

func TestKubectlPreviousLogsCommand(t *testing.T) {
	app := App{
		client: &k8s.Client{ContextName: "kind-ku-demo"},
		logs: logView{
			ns:   "ku-demo",
			pod:  "frontend-7d9",
			cont: "web",
			mode: k8s.LogPrevious,
		},
	}

	got := app.kubectlLogsCommand()
	want := "kubectl --context kind-ku-demo logs -n ku-demo frontend-7d9 -c web --tail 1000 --previous"
	if got != want {
		t.Fatalf("kubectlLogsCommand() = %q; want %q", got, want)
	}
}

func TestKubectlDeploymentLogsCommand(t *testing.T) {
	app := App{
		client: &k8s.Client{ContextName: "kind-ku-demo"},
		logs: logView{
			ns:     "ku-demo",
			deploy: "frontend",
		},
	}

	got := app.kubectlLogsCommand()
	want := "kubectl --context kind-ku-demo logs -n ku-demo deployment/frontend --all-pods --all-containers --prefix --tail 1000 -f"
	if got != want {
		t.Fatalf("kubectlLogsCommand() = %q; want %q", got, want)
	}
}

func TestShellJoinQuotesUnsafeArgs(t *testing.T) {
	got := shellJoin([]string{"kubectl", "--context", "team cluster", "get", "pods"})
	if !strings.Contains(got, "'team cluster'") {
		t.Fatalf("shellJoin() = %q; want quoted context", got)
	}
}

func TestKubectlBaseArgsIncludesImpersonation(t *testing.T) {
	app := App{
		client: &k8s.Client{ContextName: "prod"},
		impersonate: k8s.Impersonation{
			User:   "system:admin",
			Groups: []string{"system:masters", "platform devs"},
			UID:    "42",
		},
		res: k8s.ResourceInfo{Resource: "pods", Namespaced: true},
	}

	got := app.kubectlGetTableCommand()
	want := "kubectl --context prod --as system:admin --as-group system:masters " +
		"--as-group 'platform devs' --as-uid 42 get pods --all-namespaces"
	if got != want {
		t.Fatalf("kubectlGetTableCommand() = %q; want %q", got, want)
	}
}

// kubectl rejects --as-group and --as-uid without --as ("--as-uid must be used
// with --as"), so a printed command must never carry them alone. Validate keeps
// this out of a real session, but the App takes an Impersonation from callers
// that do not run it.
func TestKubectlBaseArgsOmitsGroupsAndUIDWithoutUser(t *testing.T) {
	for _, tc := range []struct {
		name string
		imp  k8s.Impersonation
	}{
		{"groups only", k8s.Impersonation{Groups: []string{"system:masters"}}},
		{"uid only", k8s.Impersonation{UID: "42"}},
	} {
		app := App{
			client:      &k8s.Client{ContextName: "prod"},
			res:         k8s.ResourceInfo{Resource: "pods", Namespaced: true},
			impersonate: tc.imp,
		}

		got := app.kubectlGetTableCommand()
		if strings.Contains(got, "--as") {
			t.Errorf("%s: kubectlGetTableCommand() = %q; want no impersonation flags", tc.name, got)
		}
	}
}

func TestKubectlBaseArgsOmitsImpersonationWhenUnset(t *testing.T) {
	app := App{
		client: &k8s.Client{ContextName: "prod"},
		res:    k8s.ResourceInfo{Resource: "pods", Namespaced: true},
	}

	got := app.kubectlGetTableCommand()
	if strings.Contains(got, "--as") {
		t.Fatalf("kubectlGetTableCommand() = %q; want no impersonation flags", got)
	}
}
