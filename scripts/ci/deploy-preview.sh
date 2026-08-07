#!/usr/bin/env bash
# Deploy (or refresh) a per-developer mock-me preview on OpenShift.
# Pattern: interview-me scripts/ci/deploy-preview.sh (oc apply + HAProxy CERT).
set -euo pipefail

VERSION_TAG="${VERSION_TAG:?}"
ACTOR="${PREVIEW_ACTOR:?}"
FRESH="${PREVIEW_FRESH:-false}"
STORAGE_CLASS="${STORAGE_CLASS:-lvms-vg1}"
CLUSTER_APPS="${CLUSTER_APPS_DOMAIN:-apps.2026-prod-1.ocp.dasmlab.org}"
PROD_NS="${PROD_NS:-mock-me-system}"

# DNS-safe owner slug from GitHub login (max ~20 for host length headroom).
OWNER="$(echo "${ACTOR}" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/-+/-/g' | cut -c1-20)"
OWNER="${OWNER:-dev}"
NS="mock-me-dev-${OWNER}"
HOST="dev-${OWNER}-mock-me.${CLUSTER_APPS}"

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp)"
sed \
  -e "s|__VERSION__|${VERSION_TAG}|g" \
  -e "s|__STORAGE_CLASS__|${STORAGE_CLASS}|g" \
  -e "s|__PREVIEW_NS__|${NS}|g" \
  -e "s|__PREVIEW_HOST__|${HOST}|g" \
  -e "s|__PREVIEW_OWNER__|${OWNER}|g" \
  "${ROOT}/k8s_envelope/mock-me_preview-ocp.yaml" > "${TMP}"

echo "Preview owner=${OWNER} ns=${NS} host=${HOST} version=${VERSION_TAG} fresh=${FRESH}"

# Ensure LE edge cert on HAProxy (no-op if already present).
if [[ "${SKIP_PREVIEW_CERT:-}" != "true" ]]; then
  bash "${ROOT}/scripts/ci/ensure-preview-cert.sh" "${HOST}"
fi

# Namespace bootstrap: copy pull + OIDC secrets (+ optional CA/ssh/pull) from prod.
if ! oc get ns "${NS}" >/dev/null 2>&1; then
  oc create namespace "${NS}"
fi

copy_secret() {
  local name="$1"
  local optional="${2:-false}"
  if oc -n "${NS}" get secret "${name}" >/dev/null 2>&1; then
    return 0
  fi
  if ! oc -n "${PROD_NS}" get secret "${name}" >/dev/null 2>&1; then
    if [[ "${optional}" == "true" ]]; then
      echo "WARN: optional secret ${PROD_NS}/${name} missing — skip"
      return 0
    fi
    echo "ERROR: required secret ${PROD_NS}/${name} missing" >&2
    return 1
  fi
  oc -n "${PROD_NS}" get secret "${name}" -o json \
    | python3 -c '
import json,sys
o=json.load(sys.stdin)
o["metadata"]={"name":o["metadata"]["name"],"namespace":"'"${NS}"'"}
o.pop("resourceVersion",None); o.pop("uid",None); o.pop("creationTimestamp",None)
print(json.dumps(o))
' | oc apply -f -
}

copy_secret dasmlab-ghcr-pull
copy_secret mock-me-oidc
copy_secret mock-me-ssh true
copy_secret mock-me-ocp-pull true

if ! oc -n "${NS}" get configmap mock-me-oidc-ca >/dev/null 2>&1; then
  if oc -n "${PROD_NS}" get configmap mock-me-oidc-ca >/dev/null 2>&1; then
    oc -n "${PROD_NS}" get configmap mock-me-oidc-ca -o json \
      | python3 -c '
import json,sys
o=json.load(sys.stdin)
o["metadata"]={"name":o["metadata"]["name"],"namespace":"'"${NS}"'"}
o.pop("resourceVersion",None); o.pop("uid",None); o.pop("creationTimestamp",None)
print(json.dumps(o))
' | oc apply -f -
  else
    echo "WARN: ${PROD_NS}/mock-me-oidc-ca missing — OIDC CA optional volume stays empty"
  fi
fi

if [[ "${FRESH}" == "true" || "${FRESH}" == "1" ]]; then
  echo "Fresh preview requested — deleting PVC (data wipe)"
  oc -n "${NS}" delete pvc mock-me-data --ignore-not-found=true
  oc -n "${NS}" delete pod -l app=mock-me --ignore-not-found=true || true
fi

oc apply -f "${TMP}"
oc -n "${NS}" rollout status deploy/mock-me --timeout=180s

PREVIEW_URL="https://${HOST}"
if [[ -n "${GITHUB_ENV:-}" ]]; then
  echo "PREVIEW_URL=${PREVIEW_URL}" >> "${GITHUB_ENV}"
  echo "PREVIEW_NS=${NS}" >> "${GITHUB_ENV}"
fi
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "## mock-me preview"
    echo ""
    echo "- **URL:** ${PREVIEW_URL}"
    echo "- **Namespace:** \`${NS}\`"
    echo "- **Image:** \`ghcr.io/dasmlab/mock-me:${VERSION_TAG}\`"
    echo "- **Owner:** \`${OWNER}\` (one preview slot per GitHub user)"
    echo "- **Data:** $([[ "${FRESH}" == "true" || "${FRESH}" == "1" ]] && echo wiped/fresh || echo persisted PVC)"
    echo ""
    echo "Sign in with Keycloak (\`mock-me\` / \`admin\` role)."
  } >> "${GITHUB_STEP_SUMMARY}"
fi

echo "Preview ready: ${PREVIEW_URL}"
rm -f "${TMP}"
