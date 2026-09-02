package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bcgov/cert-manager-proxy/internal/certrequest"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestRequireBearerToken(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := requireBearerToken("s3cret", ok)

	cases := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"correct token", "Bearer s3cret", http.StatusOK},
		{"wrong token", "Bearer nope", http.StatusUnauthorized},
		{"missing header", "", http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/certificates/x", nil)
			if c.authHeader != "" {
				req.Header.Set("Authorization", c.authHeader)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, c.wantStatus)
			}
		})
	}
}

func TestRun_RequiresToken(t *testing.T) {
	t.Setenv("CERT_PROXY_TOKEN", "")

	err := run()
	if err == nil || !strings.Contains(err.Error(), "CERT_PROXY_TOKEN") {
		t.Errorf("run() error = %v, want a CERT_PROXY_TOKEN error", err)
	}
}

func TestNewHandler(t *testing.T) {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{testGVR: "CertificateList"}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	h := newHandler(client, testNamespace, "s3cret")

	t.Run("rejects missing auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/certificates/x", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("routes an authenticated create", func(t *testing.T) {
		body, _ := json.Marshal(certrequest.Request{Domain: "app.example.com", Provider: "letsencrypt"})
		req := httptest.NewRequest(http.MethodPost, "/certificates", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer s3cret")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusAccepted, rec.Body)
		}
	})

	t.Run("routes an authenticated get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/certificates/app-example-com", nil)
		req.Header.Set("Authorization", "Bearer s3cret")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body)
		}
	})
}
