package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bcgov/cert-manager-proxy/internal/certrequest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const testNamespace = "cert-proxy"

var testGVR = schema.GroupVersionResource{Group: certrequest.Group, Version: certrequest.Version, Resource: certrequest.Resource}

func newTestServer(objects ...runtime.Object) *server {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{testGVR: "CertificateList"}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objects...)
	return &server{client: client, gvr: testGVR, namespace: testNamespace}
}

func TestServerCreate(t *testing.T) {
	s := newTestServer()
	body, _ := json.Marshal(certrequest.Request{Domain: "app.example.com", Provider: "letsencrypt"})

	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.create(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusAccepted, rec.Body)
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp["name"] != "app-example-com" {
		t.Errorf("name = %q, want %q", resp["name"], "app-example-com")
	}
	if resp["status"] != "pending-approval" {
		t.Errorf("status = %q, want %q", resp["status"], "pending-approval")
	}
}

func TestServerCreate_InvalidJSON(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	s.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServerCreate_FailsValidation(t *testing.T) {
	s := newTestServer()
	body, _ := json.Marshal(certrequest.Request{Domain: "not a domain", Provider: "letsencrypt"})
	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServerCreate_AlreadyExists(t *testing.T) {
	existing := certrequest.BuildCertificate(testNamespace, certrequest.Request{Domain: "app.example.com", Provider: "letsencrypt"})
	s := newTestServer(existing)

	body, _ := json.Marshal(certrequest.Request{Domain: "app.example.com", Provider: "letsencrypt"})
	req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.create(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body)
	}
}

func TestServerGet(t *testing.T) {
	existing := certrequest.BuildCertificate(testNamespace, certrequest.Request{Domain: "app.example.com", Provider: "letsencrypt"})
	unstructured.SetNestedField(existing.Object, []interface{}{
		map[string]interface{}{"type": "Ready", "status": "False"},
	}, "status", "conditions")
	s := newTestServer(existing)

	req := httptest.NewRequest(http.MethodGet, "/certificates/app-example-com", nil)
	req.SetPathValue("name", "app-example-com")
	rec := httptest.NewRecorder()
	s.get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp["name"] != "app-example-com" {
		t.Errorf("name = %v, want %q", resp["name"], "app-example-com")
	}
	if resp["conditions"] == nil {
		t.Errorf("conditions = nil, want the seeded condition")
	}
}

func TestServerGet_NotFound(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/certificates/missing", nil)
	req.SetPathValue("name", "missing")
	rec := httptest.NewRecorder()
	s.get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestConditionsOf(t *testing.T) {
	if got := conditionsOf(map[string]interface{}{}); got != errNoStatus.Error() {
		t.Errorf("conditionsOf(no status) = %v, want %q", got, errNoStatus.Error())
	}
	obj := map[string]interface{}{"status": map[string]interface{}{"conditions": "x"}}
	if got := conditionsOf(obj); got != "x" {
		t.Errorf("conditionsOf(with status) = %v, want %q", got, "x")
	}
}

func TestEnvOr(t *testing.T) {
	t.Setenv("CERT_PROXY_TEST_VAR", "")
	if got := envOr("CERT_PROXY_TEST_VAR", "fallback"); got != "fallback" {
		t.Errorf("envOr(unset) = %q, want fallback", got)
	}
	t.Setenv("CERT_PROXY_TEST_VAR", "value")
	if got := envOr("CERT_PROXY_TEST_VAR", "fallback"); got != "value" {
		t.Errorf("envOr(set) = %q, want value", got)
	}
}
