# cert-manager-proxy

[![Issues](https://img.shields.io/github/issues/bcgov/cert-manager-proxy.svg)](/../../issues)
[![Apache 2.0 License](https://img.shields.io/github/license/bcgov/cert-manager-proxy.svg)](/LICENSE)
[![Lifecycle](https://img.shields.io/badge/Lifecycle-Experimental-339999)](https://github.com/bcgov/repomountie/blob/master/doc/lifecycle-badges.md)
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

`ClusterIssuer`s and `CertificateRequestPolicy`s are templated from Helm
values — see `charts/cert-manager-proxy/values-example.yaml` for a worked
example, and the "Deploying to a cluster" section below.

## Build & test

```sh
go build ./...
go test ./...
```

## Deploying to a cluster

`charts/cert-manager-proxy` is a Helm chart for the whole stack: the proxy
itself, plus cert-manager and `approver-policy` as chart dependencies (both
pulled from `oci://quay.io/jetstack/charts` — nothing vendored), plus your
`ClusterIssuer`s and `CertificateRequestPolicy`s templated from values. One
`helm install`, no separate `kubectl apply` step.

```sh
helm dependency build charts/cert-manager-proxy
helm install cert-manager-proxy charts/cert-manager-proxy \
  --namespace cert-proxy --create-namespace \
  --values charts/cert-manager-proxy/values-example.yaml \
  --set auth.token=s3cret   # or auth.existingSecretName for anything real
```

The chart sets `disableAutoApproval=true` on the cert-manager dependency —
without that, cert-manager's built-in approver auto-approves every
`CertificateRequest` and the whole pre-approval gate this repo exists for
does nothing. See `charts/cert-manager-proxy/values.yaml` for the rest of
the knobs (image, replica count, resources, existing-secret auth, disabling
either dependency if you already run it cluster-wide).

`issuers[].dns01` in values is passed straight through to cert-manager's
DNS-01 solver, so any provider it supports works — a built-in one
(Route53, Cloudflare, Azure DNS, Google CloudDNS) or, for an internal/
custom DNS system, cert-manager's
[webhook solver](https://cert-manager.io/docs/configuration/acme/dns01/webhook/)
([scaffold](https://github.com/cert-manager/webhook-example)), which is
what `values-example.yaml` demonstrates — see
[cert-manager's DNS-01 docs](https://cert-manager.io/docs/configuration/acme/dns01/)
for each built-in provider's shape. The webhook itself is a separate
service you write and deploy; cert-manager just calls it.

The real secret material any of this needs (account keys, an EAB HMAC key,
your DNS provider's API credentials) is never put in values — it's
created as a Kubernetes Secret out of band and referenced by name/key,
e.g.:

```sh
kubectl create secret generic internal-dns-credentials \
  --namespace cert-proxy \
  --from-literal=api-key=<your DNS provider's API key>
```

## Run locally

Requires a kubeconfig (or in-cluster config) with access to a cluster that
has cert-manager and `approver-policy` installed (see Deploying above).

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

## Not done yet

- CI to build and push the container image (`Dockerfile` exists but nothing
  publishes it to `ghcr.io/bcgov/cert-manager-proxy` yet, so the chart's
  default `image.repository` won't resolve until that's set up)
