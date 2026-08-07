#!/usr/bin/env bash
# One-time (or rare) bootstrap of secrets into a preview namespace.
# Run from a host that already has oc + cluster login (not required on the GHA runner,
# but deploy-preview.sh will call this automatically when oc is available).
set -euo pipefail

OWNER_RAW="${1:?usage: bootstrap-preview-ns.sh <github-username>}"
OWNER="$(echo "${OWNER_RAW}" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/-+/-/g' | cut -c1-20)"
NS="mock-me-dev-${OWNER}"
PROD_NS="${PROD_NS:-mock-me-system}"

if ! command -v oc >/dev/null 2>&1; then
  echo "ERROR: oc not found on this machine" >&2
  exit 1
fi

echo "Bootstrapping secrets + Argo RBAC for ns=${NS} (from ${PROD_NS})"
oc get ns "${NS}" >/dev/null 2>&1 || oc create namespace "${NS}"

# Argo application-controller needs admin in the preview NS (same as prod).
oc apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: openshift-gitops-argocd-application-controller
  namespace: ${NS}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: admin
subjects:
  - kind: ServiceAccount
    name: openshift-gitops-argocd-application-controller
    namespace: openshift-gitops
EOF

copy_secret() {
  local name="$1"
  local optional="${2:-false}"
  if ! oc -n "${PROD_NS}" get secret "${name}" >/dev/null 2>&1; then
    if [[ "${optional}" == "true" ]]; then
      echo "WARN: optional prod secret ${PROD_NS}/${name} missing — skip" >&2
      return 0
    fi
    echo "WARN: prod secret ${PROD_NS}/${name} missing — skip" >&2
    return 0
  fi
  oc -n "${PROD_NS}" get secret "${name}" -o json \
    | python3 -c '
import json,sys
o=json.load(sys.stdin)
o["metadata"]={"name":o["metadata"]["name"],"namespace":"'"${NS}"'"}
for k in ("resourceVersion","uid","creationTimestamp","managedFields","ownerReferences"):
    o.get("metadata",{}).pop(k, None)
o.pop("resourceVersion", None)
print(json.dumps(o))
' | oc apply -f -
}

copy_secret dasmlab-ghcr-pull
copy_secret mock-me-oidc
copy_secret mock-me-ssh true
copy_secret mock-me-ocp-pull true

if oc -n "${PROD_NS}" get configmap mock-me-oidc-ca >/dev/null 2>&1; then
  oc -n "${PROD_NS}" get configmap mock-me-oidc-ca -o json \
    | python3 -c '
import json,sys
o=json.load(sys.stdin)
o["metadata"]={"name":o["metadata"]["name"],"namespace":"'"${NS}"'"}
for k in ("resourceVersion","uid","creationTimestamp","managedFields","ownerReferences"):
    o.get("metadata",{}).pop(k, None)
print(json.dumps(o))
' | oc apply -f -
else
  echo "WARN: prod ConfigMap ${PROD_NS}/mock-me-oidc-ca missing — creating empty stub" >&2
  oc apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: mock-me-oidc-ca
  namespace: ${NS}
data:
  ca.crt: |
EOF
fi

echo "Done. Preview NS ${NS} has pull + OIDC secrets/CA and Argo admin RoleBinding."
echo "GitOps will deploy the app when CI writes clusters/.../previews/${OWNER}.yaml"
