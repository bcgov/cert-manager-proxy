#!/usr/bin/env bash
# Installs cert-manager-proxy on the current kubectl context via the Helm
# chart in charts/cert-manager-proxy (which pulls in cert-manager and
# approver-policy as dependencies), then applies this repo's example
# ClusterIssuers/CertificateRequestPolicies/RBAC.
#
# Requires: helm, kubectl, pointed at the cluster you want to bootstrap.
set -euo pipefail

NAMESPACE="cert-proxy"
CHART_DIR="$(dirname "$0")/../charts/cert-manager-proxy"

TOKEN="${CERT_PROXY_TOKEN:-$(openssl rand -hex 32)}"

echo "==> Building chart dependencies (cert-manager, approver-policy)"
helm dependency build "${CHART_DIR}"

echo "==> Installing cert-manager-proxy"
helm upgrade cert-manager-proxy "${CHART_DIR}" \
  --install \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set auth.token="${TOKEN}" \
  --wait

echo "==> Applying ClusterIssuers, CertificateRequestPolicies, and RBAC"
kubectl apply -f "$(dirname "$0")/../manifests/"

echo "==> Done."
echo "    Bearer token: ${TOKEN}"
echo "    Edit the REPLACE_ME_* values in manifests/cluster-issuers.yaml before relying on this for real issuance."
if [ -z "${CERT_PROXY_TOKEN:-}" ]; then
  echo "    (generated randomly -- pass CERT_PROXY_TOKEN=... to pick your own next time)"
fi
