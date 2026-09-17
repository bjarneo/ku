package k8s

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImpersonationActive(t *testing.T) {
	for _, tc := range []struct {
		name string
		imp  Impersonation
		want bool
	}{
		{"zero", Impersonation{}, false},
		{"user", Impersonation{User: "system:admin"}, true},
		{"groups only", Impersonation{Groups: []string{"system:masters"}}, true},
		{"uid only", Impersonation{UID: "abc"}, true},
	} {
		if got := tc.imp.Active(); got != tc.want {
			t.Errorf("%s: Active() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestImpersonationValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		imp     Impersonation
		wantErr string
	}{
		{"zero", Impersonation{}, ""},
		{"user only", Impersonation{User: "system:admin"}, ""},
		{"user and groups", Impersonation{User: "u", Groups: []string{"g"}}, ""},
		{"user and uid", Impersonation{User: "u", UID: "1"}, ""},
		{"groups without user", Impersonation{Groups: []string{"system:masters"}}, "--as-group requires --as"},
		{"uid without user", Impersonation{UID: "1"}, "--as-uid requires --as"},
	} {
		err := tc.imp.Validate()
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

func TestImpersonationString(t *testing.T) {
	for _, tc := range []struct {
		name string
		imp  Impersonation
		want string
	}{
		{"zero", Impersonation{}, ""},
		{"user", Impersonation{User: "system:admin"}, "system:admin"},
		{"user and groups", Impersonation{User: "u", Groups: []string{"a", "b"}}, "u (groups: a, b)"},
		{"user and uid", Impersonation{User: "u", UID: "42"}, "u (uid 42)"},
	} {
		if got := tc.imp.String(); got != tc.want {
			t.Errorf("%s: String() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// writeKubeconfig writes a minimal kubeconfig that resolves without contacting
// a cluster, so the config plumbing can be tested offline.
func writeKubeconfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	body := `apiVersion: v1
kind: Config
current-context: test
clusters:
- name: test
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: test
  context:
    cluster: test
    user: test
    namespace: demo
users:
- name: test
  user:
    token: abc
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The impersonation must land on the rest.Config, because every client and
// transport in this package is built from it.
func TestLoadClientConfigAppliesImpersonation(t *testing.T) {
	path := writeKubeconfig(t)

	// Compared field by field: clientcmd fills Extra with an empty map, so
	// DeepEqual against a literal would fail on nil versus empty.
	for _, tc := range []struct {
		name       string
		imp        Impersonation
		wantUser   string
		wantUID    string
		wantGroups []string
	}{
		{name: "none"},
		{name: "user", imp: Impersonation{User: "system:admin"}, wantUser: "system:admin"},
		{
			name:       "user groups and uid",
			imp:        Impersonation{User: "system:admin", Groups: []string{"system:masters", "devs"}, UID: "42"},
			wantUser:   "system:admin",
			wantUID:    "42",
			wantGroups: []string{"system:masters", "devs"},
		},
	} {
		_, cfg, err := loadClientConfig("", path, tc.imp)
		if err != nil {
			t.Fatalf("%s: loadClientConfig: %v", tc.name, err)
		}
		if cfg.Impersonate.UserName != tc.wantUser {
			t.Errorf("%s: UserName = %q, want %q", tc.name, cfg.Impersonate.UserName, tc.wantUser)
		}
		if cfg.Impersonate.UID != tc.wantUID {
			t.Errorf("%s: UID = %q, want %q", tc.name, cfg.Impersonate.UID, tc.wantUID)
		}
		if got := strings.Join(cfg.Impersonate.Groups, ","); got != strings.Join(tc.wantGroups, ",") {
			t.Errorf("%s: Groups = %q, want %q", tc.name, got, tc.wantGroups)
		}
	}
}

// A context override and impersonation are independent overrides; setting one
// must not drop the other.
func TestLoadClientConfigImpersonationWithContextOverride(t *testing.T) {
	path := writeKubeconfig(t)

	cc, cfg, err := loadClientConfig("test", path, Impersonation{User: "system:admin"})
	if err != nil {
		t.Fatalf("loadClientConfig: %v", err)
	}
	if cfg.Impersonate.UserName != "system:admin" {
		t.Errorf("Impersonate.UserName = %q, want system:admin", cfg.Impersonate.UserName)
	}
	ns, _, err := cc.Namespace()
	if err != nil {
		t.Fatalf("Namespace: %v", err)
	}
	if ns != "demo" {
		t.Errorf("namespace = %q, want demo (context override lost)", ns)
	}
}

func TestNewClientRejectsBadKubeconfigWithImpersonation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	err := ValidateKubeconfig("", path, Impersonation{User: "system:admin"})
	if err == nil || !strings.Contains(err.Error(), "kubeconfig is empty or missing") {
		t.Fatalf("error = %v, want the friendly kubeconfig message", err)
	}
}
