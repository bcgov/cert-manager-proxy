# cert-manager-proxy

[![Issues](https://img.shields.io/github/issues/bcgov/cert-manager-proxy.svg?style=for-the-badge)](/../../issues)
[![Apache 2.0 License](https://img.shields.io/github/license/bcgov/cert-manager-proxy.svg?style=for-the-badge)](/LICENSE)
[![Lifecycle](https://img.shields.io/badge/Lifecycle-Experimental-339999?style=for-the-badge)](https://github.com/bcgov/repomountie/blob/master/doc/lifecycle-badges.md)
[![codecov](https://codecov.io/gh/bcgov/cert-manager-proxy/branch/main/graph/badge.svg)](https://codecov.io/gh/bcgov/cert-manager-proxy)

> **Experimental.** Untested against a real cluster. Do not deploy as-is.

A thin intake API in front of [cert-manager](https://cert-manager.io/) that
lets you request certificates from multiple ACME providers (DNS-01 only)
behind an out-of-band pre-approval gate.

## How it works

- **Multi-provider** — one `ClusterIssuer` per ACME CA (Let's Encrypt,
  ZeroSSL, ...), each with its own DNS-01 solver. The intake API just picks
  an issuer by name; cert-manager does the ACME/DNS-01 work.
- **Pre-approval** — enforced by cert-manager's
  [`approver-policy`](https://github.com/cert-manager/approver-policy)
  add-on, not by this service. A `pre-approved-domains`
  `CertificateRequestPolicy` auto-approves allowlisted domains for the
  intake API's own ServiceAccount; a permissive `manual-review` policy is
  RBAC-bound to a human-approvers group for everything else. cert-manager
  won't create the ACME order until the `CertificateRequest` is `Approved`.

See `manifests/` for the `ClusterIssuer`, `CertificateRequestPolicy`, and
RBAC definitions.

## Build & test

```sh
go build ./...
go test ./...
```

## Run

Requires a kubeconfig (or in-cluster config) with access to a cluster that
has cert-manager and `approver-policy` installed.

`CERT_PROXY_TOKEN` is required — the server refuses to start without it.
It's a single shared bearer token, fine for one trusted caller; see the
`ponytail:` comment on `requireBearerToken` in `main.go` for the upgrade
path (Kubernetes TokenReview/SubjectAccessReview, or a kube-rbac-proxy
sidecar) once there are multiple callers needing distinct identities.

```sh
CERT_PROXY_TOKEN=s3cret go run ./cmd/cert-manager-proxy
```

```sh
curl -X POST localhost:8080/certificates \
  -H "Authorization: Bearer s3cret" \
  -d '{"domain":"app.example.com","provider":"letsencrypt"}'

curl -H "Authorization: Bearer s3cret" localhost:8080/certificates/app-example-com
```

Every `REPLACE_ME_*` value in `manifests/` is a placeholder — the real
secret material (account keys, EAB HMAC key, Route53 secret access key) is
already kept out of these files via `*SecretRef`s to Kubernetes Secrets. On
EKS, skip static AWS keys entirely and use IRSA instead (see the comment in
`cluster-issuers.yaml`).

## Not done yet

- Deployment manifests for the proxy itself (Deployment/Service/SA)
- `approver-policy` Helm install instructions
