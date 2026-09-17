package k8s

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestValidateKubeconfigEmptyConfigError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}

	err := ValidateKubeconfig("", path, Impersonation{})
	if err == nil {
		t.Fatal("ValidateKubeconfig succeeded for empty config")
	}
	if !strings.Contains(err.Error(), "kubeconfig is empty or missing") {
		t.Fatalf("error = %q, want friendly kubeconfig message", err)
	}
	if strings.Contains(err.Error(), "KUBERNETES_MASTER") {
		t.Fatalf("error = %q, should not include client-go KUBERNETES_MASTER hint", err)
	}
}

func TestClusterNameFor(t *testing.T) {
	raw := clientcmdapi.Config{Contexts: map[string]*clientcmdapi.Context{
		"ci":     {Cluster: "ci-eks"},
		"broken": nil,
	}}
	tests := map[string]string{"ci": "ci-eks", "broken": "", "missing": ""}
	for ctx, want := range tests {
		if got := clusterNameFor(raw, ctx); got != want {
			t.Errorf("clusterNameFor(%q) = %q; want %q", ctx, got, want)
		}
	}
}
