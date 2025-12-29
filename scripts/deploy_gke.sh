#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
K8S_DIR="${ROOT_DIR}/k8s"

PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-us-central1}"
CLUSTER_NAME="${CLUSTER_NAME:-mcpx-gke}"
DOMAIN="${DOMAIN:-mcpx.in}"
SUBDOMAIN="${SUBDOMAIN:-app}"
APP_NAME="${APP_NAME:-app}"
NAMESPACE="${NAMESPACE:-${APP_NAME}}"
MANAGE_DNS="${MANAGE_DNS:-true}"
DNS_ZONE="${DNS_ZONE:-${DOMAIN//./-}}"
STATIC_IP_NAME="${STATIC_IP_NAME:-${APP_NAME}-ip}"
ARTIFACT_REPO="${ARTIFACT_REPO:-mcpx-apps}"
IMAGE_REPO="${IMAGE_REPO:-}"
NODEPOOL_NAME="${NODEPOOL_NAME:-}"
CREATE_NODEPOOL="${CREATE_NODEPOOL:-false}"
NODEPOOL_MACHINE_TYPE="${NODEPOOL_MACHINE_TYPE:-e2-standard-4}"
NODEPOOL_MIN_NODES="${NODEPOOL_MIN_NODES:-1}"
NODEPOOL_MAX_NODES="${NODEPOOL_MAX_NODES:-3}"

if [[ -z "${PROJECT_ID}" ]]; then
  echo "PROJECT_ID is required"
  exit 1
fi

FQDN="${SUBDOMAIN}.${DOMAIN}"

if [[ -z "${IMAGE_REPO}" ]]; then
  IMAGE_REPO="${REGION}-docker.pkg.dev/${PROJECT_ID}/${ARTIFACT_REPO}"
fi

if [[ -n "${TAG:-}" ]]; then
  TAG="${TAG}"
else
  GIT_SHA="$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || true)"
  if [[ -z "${GIT_SHA}" ]]; then
    TAG="$(date +%Y%m%d%H%M%S)"
  else
    if [[ -n "$(git -C "${ROOT_DIR}" status --porcelain)" ]]; then
      TAG="${GIT_SHA}-dirty-$(date +%Y%m%d%H%M%S)"
    else
      TAG="${GIT_SHA}"
    fi
  fi
fi

IMAGE="${IMAGE_REPO}/${APP_NAME}:${TAG}"

echo "==> Configure gcloud project"
gcloud config set project "${PROJECT_ID}" >/dev/null

echo "==> Ensure static IP (${STATIC_IP_NAME})"
if ! gcloud compute addresses describe "${STATIC_IP_NAME}" --global >/dev/null 2>&1; then
  gcloud compute addresses create "${STATIC_IP_NAME}" --global >/dev/null
fi
STATIC_IP="$(gcloud compute addresses describe "${STATIC_IP_NAME}" --global --format="value(address)")"

if [[ "${MANAGE_DNS}" == "true" ]]; then
  echo "==> Ensure Cloud DNS zone (${DNS_ZONE})"
  if ! gcloud dns managed-zones describe "${DNS_ZONE}" >/dev/null 2>&1; then
    gcloud dns managed-zones create "${DNS_ZONE}" \
      --dns-name="${DOMAIN}." \
      --description="DNS zone for ${DOMAIN}"
    gcloud dns managed-zones describe "${DNS_ZONE}" --format="value(nameServers)" || true
  fi

  echo "==> Upsert A record for ${FQDN}"
  RECORD_INFO="$(gcloud dns record-sets list \
    --zone "${DNS_ZONE}" \
    --name "${FQDN}." \
    --type A \
    --format=json | python3 - <<'PY'
import json, sys

data = json.load(sys.stdin)
if not data:
    print("")
    raise SystemExit(0)
record = data[0]
ttl = record.get("ttl", 300)
rrdatas = record.get("rrdatas", [])
print(f"{ttl}|{','.join(rrdatas)}")
PY
)"

  if [[ -n "${RECORD_INFO}" ]]; then
    TTL="${RECORD_INFO%%|*}"
    IPS_CSV="${RECORD_INFO#*|}"
    if [[ "${IPS_CSV}" == "${STATIC_IP}" ]]; then
      echo "==> DNS already points to ${STATIC_IP}"
    else
      IFS="," read -r -a IPS <<< "${IPS_CSV}"
      gcloud dns record-sets transaction start --zone "${DNS_ZONE}"
      for ip in "${IPS[@]}"; do
        gcloud dns record-sets transaction remove "${ip}" \
          --name "${FQDN}." \
          --ttl "${TTL}" \
          --type A \
          --zone "${DNS_ZONE}"
      done
      gcloud dns record-sets transaction add "${STATIC_IP}" \
        --name "${FQDN}." \
        --ttl 300 \
        --type A \
        --zone "${DNS_ZONE}"
      gcloud dns record-sets transaction execute --zone "${DNS_ZONE}"
    fi
  else
    gcloud dns record-sets transaction start --zone "${DNS_ZONE}"
    gcloud dns record-sets transaction add "${STATIC_IP}" \
      --name "${FQDN}." \
      --ttl 300 \
      --type A \
      --zone "${DNS_ZONE}"
    gcloud dns record-sets transaction execute --zone "${DNS_ZONE}"
  fi
else
  echo "==> MANAGE_DNS=false. Create an A record in your DNS provider:"
  echo "  Host: ${SUBDOMAIN}"
  echo "  Type: A"
  echo "  Value: ${STATIC_IP}"
fi

echo "==> Fetch GKE credentials"
gcloud container clusters get-credentials "${CLUSTER_NAME}" --region "${REGION}" >/dev/null

if [[ "${CREATE_NODEPOOL}" == "true" && -n "${NODEPOOL_NAME}" ]]; then
  AUTOPILOT="$(gcloud container clusters describe "${CLUSTER_NAME}" --region "${REGION}" --format="value(autopilot.enabled)" 2>/dev/null || true)"
  if [[ "${AUTOPILOT}" == "true" || "${AUTOPILOT}" == "True" ]]; then
    echo "==> Autopilot cluster detected. Skipping nodepool creation."
  else
    if ! gcloud container node-pools describe "${NODEPOOL_NAME}" --cluster "${CLUSTER_NAME}" --region "${REGION}" >/dev/null 2>&1; then
      echo "==> Creating nodepool ${NODEPOOL_NAME}"
      gcloud container node-pools create "${NODEPOOL_NAME}" \
        --cluster "${CLUSTER_NAME}" \
        --region "${REGION}" \
        --machine-type "${NODEPOOL_MACHINE_TYPE}" \
        --enable-autoscaling \
        --min-nodes "${NODEPOOL_MIN_NODES}" \
        --max-nodes "${NODEPOOL_MAX_NODES}"
    fi
  fi
fi

echo "==> Build & push image with Cloud Build: ${IMAGE}"
gcloud builds submit "${ROOT_DIR}" --tag "${IMAGE}"

echo "==> Render Kubernetes manifests"
RENDER_DIR="$(mktemp -d)"
cp -R "${K8S_DIR}/" "${RENDER_DIR}/k8s"
RENDER_DIR="${RENDER_DIR}" IMAGE="${IMAGE}" FQDN="${FQDN}" STATIC_IP_NAME="${STATIC_IP_NAME}" APP_NAME="${APP_NAME}" NAMESPACE="${NAMESPACE}" NODEPOOL_NAME="${NODEPOOL_NAME}" python3 - <<'PY'
import os, pathlib

root = pathlib.Path(os.environ["RENDER_DIR"]) / "k8s"
image = os.environ["IMAGE"]
fqdn = os.environ["FQDN"]
ip_name = os.environ["STATIC_IP_NAME"]
app_name = os.environ["APP_NAME"]
namespace = os.environ["NAMESPACE"]
nodepool = os.environ.get("NODEPOOL_NAME", "").strip()

node_selector = ""
if nodepool:
    node_selector = "nodeSelector:\n        cloud.google.com/gke-nodepool: \"%s\"" % nodepool

for path in root.glob("*.yaml"):
    data = path.read_text(encoding="utf-8")
    data = data.replace("__IMAGE__", image)
    data = data.replace("__FQDN__", fqdn)
    data = data.replace("__IP_NAME__", ip_name)
    data = data.replace("__APP_NAME__", app_name)
    data = data.replace("__NAMESPACE__", namespace)
    data = data.replace("__NODE_SELECTOR__", node_selector)
    path.write_text(data, encoding="utf-8")
PY

echo "==> Ensure namespace exists"
kubectl apply -f "${RENDER_DIR}/k8s/namespace.yaml"

if [[ -f "${ROOT_DIR}/.env" ]]; then
  echo "==> Apply env secret from .env"
  echo "==> Normalize .env (dedupe keys, last wins)"
  CLEAN_ENV="${RENDER_DIR}/clean.env"
  ROOT_DIR="${ROOT_DIR}" CLEAN_ENV="${CLEAN_ENV}" python3 - <<'PY'
import os

src = os.path.join(os.environ["ROOT_DIR"], ".env")
dst = os.environ["CLEAN_ENV"]

order = []
values = {}

with open(src, "r", encoding="utf-8") as f:
    for raw in f.read().splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.lower().startswith("export "):
            line = line[7:].strip()
        if "=" not in line:
            continue
        key, val = line.split("=", 1)
        key = key.strip()
        if not key:
            continue
        if key not in values:
            order.append(key)
        values[key] = val

with open(dst, "w", encoding="utf-8") as out:
    for k in order:
        out.write(f"{k}={values[k]}\n")
PY

  kubectl -n "${NAMESPACE}" create secret generic "${APP_NAME}-env" \
    --from-env-file="${CLEAN_ENV}" \
    --dry-run=client -o yaml | kubectl apply -f -
else
  echo "==> No .env found. Skipping secret creation."
fi

echo "==> Apply Kubernetes manifests"
kubectl apply -k "${RENDER_DIR}/k8s"

echo "==> Set image & rollout"
kubectl -n "${NAMESPACE}" set image deployment/"${APP_NAME}" "${APP_NAME}"="${IMAGE}"
ROLLOUT_TIMEOUT="${ROLLOUT_TIMEOUT:-10m}"
if ! kubectl -n "${NAMESPACE}" rollout status deployment/"${APP_NAME}" --timeout="${ROLLOUT_TIMEOUT}"; then
  echo "==> Rollout did not complete within ${ROLLOUT_TIMEOUT}"
  echo "==> Diagnostics"
  kubectl -n "${NAMESPACE}" get pods -l "app=${APP_NAME}" -o wide || true
  kubectl -n "${NAMESPACE}" describe deployment "${APP_NAME}" || true
  kubectl -n "${NAMESPACE}" get rs -l "app=${APP_NAME}" -o wide || true
  kubectl -n "${NAMESPACE}" get events --sort-by=.metadata.creationTimestamp | tail -n 80 || true
  exit 1
fi

echo "==> Done"
echo "Static IP: ${STATIC_IP}"
echo "Domain: https://${FQDN}"
