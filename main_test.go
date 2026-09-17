package main

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/bjarneo/ku/internal/k8s"
)

func TestRunUpgradeHelpPrintsUsage(t *testing.T) {
	out := captureStdout(t, func() {
		if err := runUpgrade([]string{"--help"}); err != nil {
			t.Fatalf("runUpgrade(--help): %v", err)
		}
	})
	if !strings.Contains(out, "usage: ku upgrade") {
		t.Fatalf("help output = %q, want usage", out)
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	f()
	if err := w.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	os.Stdout = old
	return string(b)
}

// --as-group is repeatable, like kubectl's, so Set must append.
func TestGroupListFlagRepeats(t *testing.T) {
	fs := flag.NewFlagSet("ku", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var groups groupList
	fs.Var(&groups, "as-group", "group")

	if err := fs.Parse([]string{"--as-group", "system:masters", "--as-group", "devs"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := strings.Join(groups, ","); got != "system:masters,devs" {
		t.Fatalf("groups = %q, want %q", got, "system:masters,devs")
	}
	if got := groups.String(); got != "system:masters,devs" {
		t.Fatalf("String() = %q, want %q", got, "system:masters,devs")
	}
}

func TestGroupListFlagRejectsEmpty(t *testing.T) {
	fs := flag.NewFlagSet("ku", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var groups groupList
	fs.Var(&groups, "as-group", "group")

	if err := fs.Parse([]string{"--as-group", ""}); err == nil {
		t.Fatal("parse accepted an empty --as-group")
	}
}

// Groups or a uid without a user are rejected up front so the error names the
// flag, rather than surfacing later as a kubeconfig load failure.
func TestImpersonationFlagValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"none", nil, ""},
		{"user", []string{"--as", "system:admin"}, ""},
		{"user and group", []string{"--as", "system:admin", "--as-group", "system:masters"}, ""},
		{"group without user", []string{"--as-group", "system:masters"}, "--as-group requires --as"},
		{"uid without user", []string{"--as-uid", "42"}, "--as-uid requires --as"},
	} {
		fs := flag.NewFlagSet("ku", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var (
			as, asUID string
			asGroups  groupList
		)
		fs.StringVar(&as, "as", "", "")
		fs.Var(&asGroups, "as-group", "")
		fs.StringVar(&asUID, "as-uid", "", "")
		if err := fs.Parse(tc.args); err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}

		imp := k8s.Impersonation{User: as, Groups: asGroups, UID: asUID}
		err := imp.Validate()
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: Validate() = %v, want nil", tc.name, err)
		case tc.wantErr != "" && err == nil:
			t.Errorf("%s: Validate() = nil, want %q", tc.name, tc.wantErr)
		case tc.wantErr != "" && err != nil && err.Error() != tc.wantErr:
			t.Errorf("%s: Validate() = %q, want %q", tc.name, err, tc.wantErr)
		}
	}
}
