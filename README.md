# cert-manager-proxy

> **Experimental.** Untested against a real cluster, no auth on the intake
> API, credentials in the example manifests are placeholders. Do not deploy
> as-is.

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

```sh
go run ./cmd/cert-manager-proxy
```

```sh
curl -X POST localhost:8080/certificates \
  -d '{"domain":"app.example.com","provider":"letsencrypt"}'

curl localhost:8080/certificates/app-example-com
```

## Not done yet

- Deployment manifests for the proxy itself (Deployment/Service/SA)
- Auth on the intake API
- `approver-policy` Helm install instructions
- Real DNS provider credentials (Route53 keys in `manifests/` are examples)
