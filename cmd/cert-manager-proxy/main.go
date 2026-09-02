// cert-manager-proxy is the intake API in front of cert-manager: it accepts a
// certificate request, picks the right ClusterIssuer for the requested
// provider, and creates a Certificate object. Approval (pre-validation)
// is enforced by cert-manager's approver-policy, not this service — see
// manifests/certificate-request-policies.yaml.
package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/bcgov/cert-manager-proxy/internal/certrequest"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run does the real work; split out from main so it can return an error
// instead of calling log.Fatal, which a test can't observe.
func run() error {
	namespace := envOr("CERT_PROXY_NAMESPACE", "cert-proxy")
	addr := envOr("CERT_PROXY_ADDR", ":8080")
	token := os.Getenv("CERT_PROXY_TOKEN")
	if token == "" {
		return errors.New("CERT_PROXY_TOKEN must be set (refusing to start without auth)")
	}

	config, err := loadKubeConfig()
	if err != nil {
		return fmt.Errorf("loading kube config: %w", err)
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("building dynamic client: %w", err)
	}

	log.Printf("cert-proxy listening on %s (namespace=%s)", addr, namespace)
	return http.ListenAndServe(addr, newHandler(client, namespace, token))
}

// newHandler wires the mux and auth middleware together. Split out from run
// so it can be tested end-to-end (routing + middleware) against a fake
// dynamic client, without a real cluster or a blocking ListenAndServe.
func newHandler(client dynamic.Interface, namespace, token string) http.Handler {
	gvr := schema.GroupVersionResource{Group: certrequest.Group, Version: certrequest.Version, Resource: certrequest.Resource}
	s := &server{client: client, gvr: gvr, namespace: namespace}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /certificates", s.create)
	mux.HandleFunc("GET /certificates/{name}", s.get)

	return requireBearerToken(token, mux)
}

// requireBearerToken is a single shared secret compared in constant time.
// ponytail: fine for one trusted caller (the pre-approval workflow); once
// there are multiple callers needing distinct identities, replace with
// Kubernetes TokenReview/SubjectAccessReview (or a kube-rbac-proxy sidecar)
// so RBAC — already the approval mechanism — also drives authn/z here.
func requireBearerToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type server struct {
	client    dynamic.Interface
	gvr       schema.GroupVersionResource
	namespace string
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	var req certrequest.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	obj := certrequest.BuildCertificate(s.namespace, req)
	created, err := s.client.Resource(s.gvr).Namespace(s.namespace).Create(r.Context(), obj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			http.Error(w, "certificate already requested for this domain", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"name":   created.GetName(),
		"status": "pending-approval",
	})
}

func (s *server) get(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	obj, err := s.client.Resource(s.gvr).Namespace(s.namespace).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":       obj.GetName(),
		"conditions": conditionsOf(obj.Object),
	})
}

func loadKubeConfig() (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var errNoStatus = errors.New("object has no status.conditions")

func conditionsOf(obj map[string]interface{}) interface{} {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return errNoStatus.Error()
	}
	return status["conditions"]
}
