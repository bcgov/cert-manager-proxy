// Package certrequest turns an intake request into a cert-manager
// Certificate object. It does not talk to Kubernetes; callers create the
// returned object with a dynamic client.
package certrequest

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GVR is the cert-manager.io v1 Certificate resource, used with a
// k8s.io/client-go/dynamic.Interface.
const (
	Group    = "cert-manager.io"
	Version  = "v1"
	Resource = "certificates"
)

// issuers maps the provider name a caller passes in to the ClusterIssuer
// that handles it. Add an entry here per provider you onboard.
var issuers = map[string]string{
	"letsencrypt": "letsencrypt-prod",
	"zerossl":     "zerossl-prod",
}

var validDomain = regexp.MustCompile(`^(\*\.)?([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$`)

type Request struct {
	Domain   string `json:"domain"`
	Provider string `json:"provider"`
}

func (r Request) Validate() error {
	if r.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if !validDomain.MatchString(strings.ToLower(r.Domain)) {
		return fmt.Errorf("domain %q is not a valid DNS name", r.Domain)
	}
	if _, ok := issuers[r.Provider]; !ok {
		return fmt.Errorf("unknown provider %q", r.Provider)
	}
	return nil
}

// Name derives the Certificate object's name from the domain, since
// Kubernetes names can't contain '*' or '.'.
func Name(domain string) string {
	n := strings.ToLower(domain)
	n = strings.ReplaceAll(n, "*", "wildcard")
	n = strings.ReplaceAll(n, ".", "-")
	return n
}

// BuildCertificate renders the cert-manager Certificate object for r.
// Validate must be called first.
func BuildCertificate(namespace string, r Request) *unstructured.Unstructured {
	name := Name(r.Domain)
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": Group + "/" + Version,
		"kind":       "Certificate",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"secretName": name + "-tls",
			"dnsNames":   []interface{}{r.Domain},
			"issuerRef": map[string]interface{}{
				"name":  issuers[r.Provider],
				"kind":  "ClusterIssuer",
				"group": Group,
			},
		},
	}}
}
