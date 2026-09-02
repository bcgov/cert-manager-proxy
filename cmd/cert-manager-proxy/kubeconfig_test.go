package main

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
)

const testKubeconfig = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.invalid:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`

func writeTestKubeconfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(testKubeconfig), 0o600); err != nil {
		t.Fatalf("writing test kubeconfig: %v", err)
	}
	return path
}

func TestLoadKubeConfig_FromEnvVar(t *testing.T) {
	path := writeTestKubeconfig(t)
	t.Setenv("KUBECONFIG", path)

	cfg, err := loadKubeConfig()
	if err != nil {
		t.Fatalf("loadKubeConfig() error = %v", err)
	}
	if cfg.Host != "https://example.invalid:6443" {
		t.Errorf("Host = %q, want %q", cfg.Host, "https://example.invalid:6443")
	}
}

func TestLoadKubeConfig_FallsBackToHomeFile(t *testing.T) {
	path := writeTestKubeconfig(t)
	t.Setenv("KUBECONFIG", "")

	old := clientcmd.RecommendedHomeFile
	clientcmd.RecommendedHomeFile = path
	t.Cleanup(func() { clientcmd.RecommendedHomeFile = old })

	cfg, err := loadKubeConfig()
	if err != nil {
		t.Fatalf("loadKubeConfig() error = %v", err)
	}
	if cfg.Host != "https://example.invalid:6443" {
		t.Errorf("Host = %q, want %q", cfg.Host, "https://example.invalid:6443")
	}
}
