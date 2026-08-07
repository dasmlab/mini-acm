#!/usr/bin/env bash
# Remove a developer's preview GitOps file so Argo prunes the preview instance.
# Intended for PR-merged / branch-done cleanup (does not touch production live/).
set -euo pipefail

ACTOR="${PREVIEW_ACTOR:?PREVIEW_ACTOR is required (GitHub username)}"
CLUSTER_APPS="${CLUSTER_APPS_DOMAIN:-apps.2026-prod-1.ocp.dasmlab.org}"

OWNER="$(echo "${ACTOR}" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/-+/-/g' | cut -c1-20)"
OWNER="${OWNER:-dev}"
NS="mock-me-dev-${OWNER}"
HOST="dev-${OWNER}-mock-me.${CLUSTER_APPS}"
FILE="clusters/2026-prod-1/mock-me/previews/${OWNER}.yaml"

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
trap 'rm -rf "${WORK}"' EXIT
git clone --depth 1 "https://x-access-token:${DEPLOY_TOKEN}@github.com/lmcdasm/dasmlab-live-cicd.git" "${WORK}/live-cicd"
cd "${WORK}/live-cicd"

if [ ! -f "${FILE}" ]; then
  echo "No preview GitOps file for owner=${OWNER} (${FILE}) — nothing to clean"
  {
    echo "## mock-me preview cleanup"
    echo ""
    echo "- **Owner:** \`${OWNER}\`"
    echo "- **Result:** no \`${FILE}\` present (already clean)"
  } >> "${GITHUB_STEP_SUMMARY:-/dev/null}"
  exit 0
fi

git config user.name "dasmlab-bot"
git config user.email "ci@dasmlab.org"
git rm -f "${FILE}"
git commit -m "preview(${OWNER}): remove mock-me after PR merge / branch done"
git push

{
  echo "## mock-me preview cleanup"
  echo ""
  echo "- **Owner:** \`${OWNER}\`"
  echo "- **Removed:** \`${FILE}\`"
  echo "- **Namespace (Argo prune):** \`${NS}\`"
  echo "- **Was URL:** https://${HOST}"
  echo "- **Argo app:** \`mock-me-previews\` (auto-sync + prune)"
} >> "${GITHUB_STEP_SUMMARY:-/dev/null}"

echo "Preview GitOps removed for owner=${OWNER} (Argo will prune ${NS})"
