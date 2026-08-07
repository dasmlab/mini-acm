#!/usr/bin/env bash
# Publish a per-developer mock-me preview via GitOps (dasmlab-live-cicd).
# Does NOT oc-apply from the runner — Argo CD syncs the previews/ path.
set -euo pipefail

VERSION_TAG="${VERSION_TAG:?}"
ACTOR="${PREVIEW_ACTOR:?}"
FRESH="${PREVIEW_FRESH:-false}"
STORAGE_CLASS="${STORAGE_CLASS:-lvms-vg1}"
CLUSTER_APPS="${CLUSTER_APPS_DOMAIN:-apps.2026-prod-1.ocp.dasmlab.org}"

OWNER="$(echo "${ACTOR}" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/-+/-/g' | cut -c1-20)"
OWNER="${OWNER:-dev}"
NS="mock-me-dev-${OWNER}"
HOST="dev-${OWNER}-mock-me.${CLUSTER_APPS}"
PREVIEW_URL="https://${HOST}"

# Stable PVC by default; bump name on fresh so Argo prune drops the old volume.
PVC_NAME="mock-me-data"
if [[ "${FRESH}" == "true" || "${FRESH}" == "1" ]]; then
  PVC_NAME="mock-me-data-${VERSION_TAG//./-}"
fi

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RENDERED="$(mktemp)"
sed \
  -e "s|__VERSION__|${VERSION_TAG}|g" \
  -e "s|__STORAGE_CLASS__|${STORAGE_CLASS}|g" \
  -e "s|__PREVIEW_NS__|${NS}|g" \
  -e "s|__PREVIEW_HOST__|${HOST}|g" \
  -e "s|__PREVIEW_OWNER__|${OWNER}|g" \
  -e "s|claimName: mock-me-data|claimName: ${PVC_NAME}|g" \
  -e "s|name: mock-me-data$|name: ${PVC_NAME}|g" \
  "${ROOT}/k8s_envelope/mock-me_preview-ocp.yaml" > "${RENDERED}"

echo "Preview owner=${OWNER} ns=${NS} host=${HOST} version=${VERSION_TAG} fresh=${FRESH} pvc=${PVC_NAME}"

# When the runner (or operator host) has oc + login, heal secrets/CA/RBAC after NS
# recreate (common after Argo prune on PR merge).
if [[ "${SKIP_PREVIEW_BOOTSTRAP:-}" == "true" ]]; then
  echo "SKIP_PREVIEW_BOOTSTRAP=true — not copying preview secrets"
elif command -v oc >/dev/null 2>&1; then
  if oc whoami >/dev/null 2>&1; then
    echo "oc available — ensuring preview NS secrets via bootstrap-preview-ns.sh"
    bash "${ROOT}/scripts/ci/bootstrap-preview-ns.sh" "${OWNER}" || {
      echo "WARN: bootstrap-preview-ns.sh failed; pod may stay Pending until secrets exist" >&2
    }
  else
    echo "WARN: oc present but not logged in — skip preview secret bootstrap" >&2
  fi
else
  echo "WARN: oc not on PATH — skip preview secret bootstrap (run scripts/ci/bootstrap-preview-ns.sh ${OWNER} from a logged-in host)" >&2
fi

# Edge TLS on HAProxy (no-op if CERT already present).
if [[ "${SKIP_PREVIEW_CERT:-}" != "true" ]]; then
  bash "${ROOT}/scripts/ci/ensure-preview-cert.sh" "${HOST}"
fi

DEPLOY_TOKEN=""
if [ -f "/home/dasm/gh_token" ]; then
  DEPLOY_TOKEN="$(tr -d '\n\r' < /home/dasm/gh_token)"
fi
if [ -z "${DEPLOY_TOKEN}" ]; then
  DEPLOY_TOKEN="${DASMLAB_GHCR_PAT:-${GH_TOKEN:-}}"
fi
if [ -z "${DEPLOY_TOKEN}" ]; then
  echo "ERROR: deploy token not set (gh_token / DASMLAB_GHCR_PAT / GH_TOKEN)" >&2
  exit 1
fi

WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}" "${RENDERED}"' EXIT
git clone --depth 1 "https://x-access-token:${DEPLOY_TOKEN}@github.com/lmcdasm/dasmlab-live-cicd.git" "${WORK}/live-cicd"
PREVIEW_DIR="${WORK}/live-cicd/clusters/2026-prod-1/mock-me/previews"
mkdir -p "${PREVIEW_DIR}"
cp "${RENDERED}" "${PREVIEW_DIR}/${OWNER}.yaml"

cd "${WORK}/live-cicd"
git config user.name "dasmlab-bot"
git config user.email "ci@dasmlab.org"
git add "clusters/2026-prod-1/mock-me/previews/${OWNER}.yaml"
if git diff --cached --quiet; then
  echo "No GitOps preview changes (manifest identical)"
else
  git commit -m "preview(${OWNER}): mock-me ${VERSION_TAG}"
  git push
fi

if [[ -n "${GITHUB_ENV:-}" ]]; then
  echo "PREVIEW_URL=${PREVIEW_URL}" >> "${GITHUB_ENV}"
  echo "PREVIEW_NS=${NS}" >> "${GITHUB_ENV}"
fi
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "## mock-me preview (GitOps)"
    echo ""
    echo "- **URL:** ${PREVIEW_URL}"
    echo "- **Namespace:** \`${NS}\`"
    echo "- **GitOps file:** \`clusters/2026-prod-1/mock-me/previews/${OWNER}.yaml\`"
    echo "- **Argo app:** \`mock-me-previews\` (auto-sync)"
    echo "- **Image:** \`ghcr.io/dasmlab/mock-me:${VERSION_TAG}\`"
    echo "- **Owner:** \`${OWNER}\` — one file / one NS per GitHub user"
    echo "- **Data:** $([[ "${FRESH}" == "true" || "${FRESH}" == "1" ]] && echo "fresh PVC (${PVC_NAME})" || echo "persisted PVC")"
    echo ""
    echo "Sign in with Keycloak (\`mock-me\` / \`admin\` role)."
  } >> "${GITHUB_STEP_SUMMARY}"
fi

echo "Preview GitOps published: ${PREVIEW_URL}"
