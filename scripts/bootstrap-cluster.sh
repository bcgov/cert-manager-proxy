#!/usr/bin/env bash
# Installs cert-manager + approver-policy on the current kubectl context,
# then applies this repo's ClusterIssuers/CertificateRequestPolicies/RBAC.
#
# Requires: helm, kubectl, pointed at the cluster you want to bootstrap.
set -euo pipefail

CERT_MANAGER_VERSION="v1.21.1"
APPROVER_POLICY_VERSION="v0.27.0"
NAMESPACE="cert-manager"

echo "==> Installing cert-manager ${CERT_MANAGER_VERSION}"
helm upgrade cert-manager oci://quay.io/jetstack/charts/cert-manager \
  --install \
  --version "${CERT_MANAGER_VERSION}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set crds.enabled=true \
  --set disableAutoApproval=true \
  --wait

echo "==> Installing cert-manager-approver-policy ${APPROVER_POLICY_VERSION}"
helm upgrade cert-manager-approver-policy oci://quay.io/jetstack/charts/cert-manager-approver-policy \
  --install \
  --version "${APPROVER_POLICY_VERSION}" \
  --namespace "${NAMESPACE}" \
  --wait

echo "==> Applying ClusterIssuers, CertificateRequestPolicies, and RBAC"
kubectl apply -f "$(dirname "$0")/../manifests/"

echo "==> Done. Edit the REPLACE_ME_* values in manifests/cluster-issuers.yaml before relying on this for real issuance."
