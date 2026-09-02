#!/usr/bin/env bash
# Installs cert-manager-proxy on the current kubectl context via the Helm
# chart in charts/cert-manager-proxy, which also pulls in cert-manager and
# approver-policy as dependencies and templates the ClusterIssuers/
# CertificateRequestPolicies/RBAC from chart values -- one install, no
# separate kubectl apply step.
#
# Requires: helm, kubectl, pointed at the cluster you want to bootstrap.
set -euo pipefail

NAMESPACE="cert-proxy"
CHART_DIR="$(dirname "$0")/../charts/cert-manager-proxy"
VALUES_FILE="${VALUES_FILE:-${CHART_DIR}/values-example.yaml}"

TOKEN="${CERT_PROXY_TOKEN:-$(openssl rand -hex 32)}"

echo "==> Building chart dependencies (cert-manager, approver-policy)"
helm dependency build "${CHART_DIR}"

echo "==> Installing cert-manager-proxy (values: ${VALUES_FILE})"
helm upgrade cert-manager-proxy "${CHART_DIR}" \
  --install \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --values "${VALUES_FILE}" \
  --set auth.token="${TOKEN}" \
  --wait

echo "==> Done."
echo "    Bearer token: ${TOKEN}"
if [ "${VALUES_FILE}" = "${CHART_DIR}/values-example.yaml" ]; then
  echo "    Using values-example.yaml -- it's full of REPLACE_ME_* placeholders."
  echo "    Copy it, fill in real values, and pass VALUES_FILE=... before relying on this for real issuance."
fi
if [ -z "${CERT_PROXY_TOKEN:-}" ]; then
  echo "    (token generated randomly -- pass CERT_PROXY_TOKEN=... to pick your own next time)"
fi
