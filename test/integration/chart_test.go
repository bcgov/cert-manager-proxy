//go:build integration

// Package integration runs `helmfile sync` (the repo's real, documented
// single-command install path) against a real, ephemeral k3s cluster (via
// testcontainers-go) -- the only test in this repo that talks to an actual
// Kubernetes API server; everything else uses fakes. It can't test real
// ACME issuance (no reachable DNS/domain from inside the test container),
// only that cert-manager, approver-policy, and this chart's own resources
// come up and validate correctly against a real API server -- catching
// cross-release naming collisions, CRD/RBAC/schema mistakes, and
// dependency-ordering bugs that `helm template`/`helm lint`/`helmfile
// template` can't, because those never actually apply anything.
//
// Requires a Docker-API-compatible container runtime (Docker or Podman;
// set DOCKER_HOST for Podman), and `helm`/`helmfile` on PATH. Run with:
//
//	go test -tags integration ./test/integration/...
package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/k3s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var certificateRequestPolicyGVR = schema.GroupVersionResource{
	Group: "policy.cert-manager.io", Version: "v1alpha1", Resource: "certificaterequestpolicies",
}

const namespace = "cert-proxy"

func TestHelmfileSync(t *testing.T) {
	ctx := context.Background()

	k3sContainer, err := k3s.Run(ctx, "docker.io/rancher/k3s:v1.31.2-k3s1")
	if err != nil {
		t.Fatalf("starting k3s: %v", err)
	}
	t.Cleanup(func() {
		if err := k3sContainer.Terminate(context.Background()); err != nil {
			t.Logf("terminating k3s container: %v", err)
		}
	})

	kubeconfigBytes, err := k3sContainer.GetKubeConfig(ctx)
	if err != nil {
		t.Fatalf("getting kubeconfig: %v", err)
	}
	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(kubeconfigPath, kubeconfigBytes, 0o600); err != nil {
		t.Fatalf("writing kubeconfig: %v", err)
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		t.Fatalf("building rest config: %v", err)
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("building k8s client: %v", err)
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("building dynamic client: %v", err)
	}

	repoRoot := "../.."
	cmd := exec.CommandContext(ctx, "helmfile", "sync")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath, "CERT_PROXY_TOKEN=integration-test-token")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helmfile sync: %v\n%s", err, out)
	}

	// cert-manager and approver-policy's own controllers should come up
	// healthy regardless of whether our proxy pod can run (it can't here:
	// no image is published yet, tracked in README's "Not done yet"). Names
	// here aren't guesses -- cert-manager's chart uses the common Helm "if
	// contains .Chart.Name .Release.Name" fullname pattern, which
	// collapses to bare .Release.Name; the "cert-manager" release name is
	// an exact match for its own chart name.
	assertDeploymentReady(t, ctx, client, namespace, "cert-manager")
	assertDeploymentReady(t, ctx, client, namespace, "cert-manager-webhook")
	assertDeploymentReady(t, ctx, client, namespace, "cert-manager-cainjector")
	assertDeploymentReady(t, ctx, client, namespace, "cert-manager-approver-policy")

	// This chart's own resources should exist and be well-formed -- name
	// collisions or invalid RBAC would surface here even if no pod runs.
	assertExists(t, ctx, "Service", func() error {
		_, err := client.CoreV1().Services(namespace).Get(ctx, "cert-manager-proxy-intake", metav1.GetOptions{})
		return err
	})
	assertExists(t, ctx, "ServiceAccount", func() error {
		_, err := client.CoreV1().ServiceAccounts(namespace).Get(ctx, "cert-manager-proxy-intake", metav1.GetOptions{})
		return err
	})
	assertExists(t, ctx, "auth Secret", func() error {
		_, err := client.CoreV1().Secrets(namespace).Get(ctx, "cert-manager-proxy-intake-auth", metav1.GetOptions{})
		return err
	})
	assertExists(t, ctx, "Role", func() error {
		_, err := client.RbacV1().Roles(namespace).Get(ctx, "cert-manager-proxy-intake", metav1.GetOptions{})
		return err
	})
	assertExists(t, ctx, "ClusterRole", func() error {
		_, err := client.RbacV1().ClusterRoles().Get(ctx, "cert-manager-proxy-intake-approver", metav1.GetOptions{})
		return err
	})

	// values-example.yaml (referenced from helmfile.yaml) configures
	// these -- if the CertificateRequestPolicy CRD/schema or the
	// dependency ordering were wrong, this is where it would surface
	// (cluster-scoped, hence no namespace on the Get).
	assertExists(t, ctx, "pre-approved-domains CertificateRequestPolicy", func() error {
		_, err := dynamicClient.Resource(certificateRequestPolicyGVR).Get(ctx, "pre-approved-domains", metav1.GetOptions{})
		return err
	})
	assertExists(t, ctx, "manual-review CertificateRequestPolicy", func() error {
		_, err := dynamicClient.Resource(certificateRequestPolicyGVR).Get(ctx, "manual-review", metav1.GetOptions{})
		return err
	})
}

func assertDeploymentReady(t *testing.T, ctx context.Context, client *kubernetes.Clientset, namespace, name string) {
	t.Helper()
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		d, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // not created yet, keep polling
		}
		return d.Status.ReadyReplicas >= 1, nil
	})
	if err != nil {
		t.Errorf("deployment %s/%s did not become ready: %v", namespace, name, err)
	}
}

func assertExists(t *testing.T, ctx context.Context, what string, get func() error) {
	t.Helper()
	if err := get(); err != nil {
		t.Errorf("%s: %v", what, err)
	}
}
