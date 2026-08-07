#!/usr/bin/env bash
# Remove a developer's mock-me preview namespace (oc apply path — not GitOps).
set -euo pipefail

ACTOR="${PREVIEW_ACTOR:?PREVIEW_ACTOR is required (GitHub username)}"
CLUSTER_APPS="${CLUSTER_APPS_DOMAIN:-apps.2026-prod-1.ocp.dasmlab.org}"

OWNER="$(echo "${ACTOR}" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/-+/-/g' | cut -c1-20)"
OWNER="${OWNER:-dev}"
NS="mock-me-dev-${OWNER}"
HOST="dev-${OWNER}-mock-me.${CLUSTER_APPS}"

if ! oc get ns "${NS}" >/dev/null 2>&1; then
  echo "No preview namespace ${NS} — nothing to clean"
  exit 0
fi

oc delete ns "${NS}" --wait=false
echo "Preview namespace ${NS} deleted (was https://${HOST})"
