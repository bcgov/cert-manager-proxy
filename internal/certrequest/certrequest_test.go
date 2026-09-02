package certrequest

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		ok   bool
	}{
		{"valid", Request{Domain: "app.example.com", Provider: "letsencrypt"}, true},
		{"valid wildcard", Request{Domain: "*.example.com", Provider: "zerossl"}, true},
		{"empty domain", Request{Domain: "", Provider: "letsencrypt"}, false},
		{"bad domain", Request{Domain: "not a domain", Provider: "letsencrypt"}, false},
		{"unknown provider", Request{Domain: "example.com", Provider: "acme-corp-ca"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.req.Validate()
			if (err == nil) != c.ok {
				t.Fatalf("Validate() error = %v, want ok=%v", err, c.ok)
			}
		})
	}
}

func TestBuildCertificate(t *testing.T) {
	obj := BuildCertificate("cert-proxy", Request{Domain: "*.example.com", Provider: "letsencrypt"})

	if got := obj.GetName(); got != "wildcard-example-com" {
		t.Errorf("name = %q, want %q", got, "wildcard-example-com")
	}
	issuerName, _, _ := unstructured.NestedString(obj.Object, "spec", "issuerRef", "name")
	if issuerName != "letsencrypt-prod" {
		t.Errorf("issuerRef.name = %q, want %q", issuerName, "letsencrypt-prod")
	}
}
