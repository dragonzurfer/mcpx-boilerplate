#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TF_DIR="${ROOT_DIR}/infra/terraform"
K8S_DIR="${ROOT_DIR}/k8s"

PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-us-central1}"
DOMAIN="${DOMAIN:-}"
SUBDOMAIN="${SUBDOMAIN:-explore}"
APP_NAME="${APP_NAME:-explore}"
NAMESPACE="${NAMESPACE:-${APP_NAME}}"
CLUSTER_NAME="${CLUSTER_NAME:-mcpx-gke}"
CLUSTER_LOCATION="${CLUSTER_LOCATION:-${REGION}}"
ARTIFACT_REPO_NAME="${ARTIFACT_REPO_NAME:-mcpx-apps}"
MANAGE_DNS="${MANAGE_DNS:-true}"

if [[ -z "${PROJECT_ID}" ]]; then
  echo "PROJECT_ID is required"
  exit 1
fi

if [[ -z "${DOMAIN}" ]]; then
  echo "DOMAIN is required (e.g., mcpx.in)"
  exit 1
fi

if [[ ! -d "${TF_DIR}" ]]; then
  echo "Missing Terraform directory: ${TF_DIR}"
  exit 1
fi

if [[ ! -d "${K8S_DIR}" ]]; then
  echo "Missing Kubernetes manifests: ${K8S_DIR}"
  exit 1
fi

if [[ -n "${SUBDOMAIN}" ]]; then
  FQDN="${SUBDOMAIN}.${DOMAIN}"
else
  FQDN="${DOMAIN}"
fi
ZONE_NAME="${DOMAIN//./-}"

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

is_truthy() {
  case "$1" in
    true|TRUE|True|1|yes|YES|Yes) return 0 ;;
    *) return 1 ;;
  esac
}

tf_state_has() {
  local addr="$1"
  terraform -chdir="${TF_DIR}" state list 2>/dev/null | grep -Fxq "${addr}"
}

tf_import() {
  local addr="$1"
  local id="$2"
  TF_VAR_project_id="${PROJECT_ID}" \
    TF_VAR_region="${REGION}" \
    TF_VAR_domain="${DOMAIN}" \
    TF_VAR_subdomain="${SUBDOMAIN}" \
    TF_VAR_app_name="${APP_NAME}" \
    TF_VAR_cluster_name="${CLUSTER_NAME}" \
    TF_VAR_artifact_repo_name="${ARTIFACT_REPO_NAME}" \
    TF_VAR_manage_dns="${MANAGE_DNS}" \
    terraform -chdir="${TF_DIR}" import "${addr}" "${id}"
}

echo "==> Terraform init"
terraform -chdir="${TF_DIR}" init

GCLOUD_AVAILABLE=false
if command -v gcloud >/dev/null 2>&1; then
  GCLOUD_AVAILABLE=true
fi

if [[ "${GCLOUD_AVAILABLE}" == "true" ]]; then
  ENABLED_SERVICES="$(gcloud services list --enabled --project "${PROJECT_ID}" --format="value(config.name)" 2>/dev/null || true)"
  PROJECT_NUMBER="$(gcloud projects describe "${PROJECT_ID}" --format="value(projectNumber)" 2>/dev/null || true)"

  SERVICES=(
    "container.googleapis.com"
    "compute.googleapis.com"
    "artifactregistry.googleapis.com"
    "cloudbuild.googleapis.com"
    "dns.googleapis.com"
    "cloudresourcemanager.googleapis.com"
    "iam.googleapis.com"
  )

  for svc in "${SERVICES[@]}"; do
    if [[ -n "${ENABLED_SERVICES}" ]] && echo "${ENABLED_SERVICES}" | grep -qx "${svc}"; then
      if ! tf_state_has "google_project_service.services[\"${svc}\"]"; then
        echo "==> Importing enabled API service: ${svc}"
        tf_import "google_project_service.services[\"${svc}\"]" "${PROJECT_ID}/${svc}"
      fi
    fi
  done

  if ! tf_state_has "google_compute_network.vpc"; then
    if gcloud compute networks describe "${APP_NAME}-vpc" --project "${PROJECT_ID}" >/dev/null 2>&1; then
      echo "==> Importing existing VPC"
      tf_import "google_compute_network.vpc" "projects/${PROJECT_ID}/global/networks/${APP_NAME}-vpc"
    fi
  fi

  if ! tf_state_has "google_compute_subnetwork.subnet"; then
    if gcloud compute networks subnets describe "${APP_NAME}-subnet" --region "${REGION}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
      echo "==> Importing existing subnet"
      tf_import "google_compute_subnetwork.subnet" "projects/${PROJECT_ID}/regions/${REGION}/subnetworks/${APP_NAME}-subnet"
    fi
  fi

  if ! tf_state_has "google_container_cluster.gke"; then
    if gcloud container clusters describe "${CLUSTER_NAME}" --region "${CLUSTER_LOCATION}" --project "${PROJECT_ID}" >/dev/null 2>&1; then
      echo "==> Importing existing GKE cluster"
      tf_import "google_container_cluster.gke" "projects/${PROJECT_ID}/locations/${CLUSTER_LOCATION}/clusters/${CLUSTER_NAME}"
    fi
  fi

  if ! tf_state_has "google_artifact_registry_repository.docker"; then
    if gcloud artifacts repositories describe "${ARTIFACT_REPO_NAME}" \
      --location "${REGION}" \
      --project "${PROJECT_ID}" >/dev/null 2>&1; then
      echo "==> Importing existing Artifact Registry repo"
      tf_import \
        "google_artifact_registry_repository.docker" \
        "projects/${PROJECT_ID}/locations/${REGION}/repositories/${ARTIFACT_REPO_NAME}"
    fi
  fi

  if [[ -n "${PROJECT_NUMBER}" ]] && ! tf_state_has "google_project_iam_member.cloudbuild_artifact_writer"; then
    IAM_MEMBER="serviceAccount:${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com"
    IAM_ROLE="roles/artifactregistry.writer"
    if gcloud projects get-iam-policy "${PROJECT_ID}" \
      --flatten="bindings[].members" \
      --format="value(bindings.role)" \
      --filter="bindings.members:${IAM_MEMBER} AND bindings.role=${IAM_ROLE}" 2>/dev/null | grep -q "${IAM_ROLE}"; then
      echo "==> Importing existing IAM binding for Cloud Build"
      tf_import \
        "google_project_iam_member.cloudbuild_artifact_writer" \
        "projects/${PROJECT_ID}/${IAM_ROLE}/${IAM_MEMBER}"
    fi
  fi

  if ! tf_state_has "google_compute_global_address.lb_ip"; then
    if gcloud compute addresses describe "${APP_NAME}-ip" \
      --global \
      --project "${PROJECT_ID}" >/dev/null 2>&1; then
      echo "==> Importing existing global static IP"
      tf_import \
        "google_compute_global_address.lb_ip" \
        "projects/${PROJECT_ID}/global/addresses/${APP_NAME}-ip"
    fi
  fi

  if is_truthy "${MANAGE_DNS}"; then
    if ! tf_state_has "google_dns_managed_zone.zone[0]"; then
      if gcloud dns managed-zones describe "${ZONE_NAME}" \
        --project "${PROJECT_ID}" >/dev/null 2>&1; then
        echo "==> Importing existing Cloud DNS managed zone"
        tf_import \
          "google_dns_managed_zone.zone[0]" \
          "projects/${PROJECT_ID}/managedZones/${ZONE_NAME}"
      fi
    fi

    if ! tf_state_has "google_dns_record_set.app_a[0]"; then
      if gcloud dns record-sets list \
        --project "${PROJECT_ID}" \
        --zone "${ZONE_NAME}" \
        --name "${FQDN}." \
        --type "A" \
        --format="value(name)" 2>/dev/null | grep -q "${FQDN}."; then
        echo "==> Importing existing DNS A record"
        tf_import \
          "google_dns_record_set.app_a[0]" \
          "projects/${PROJECT_ID}/managedZones/${ZONE_NAME}/rrsets/${FQDN}./A"
      fi
    fi
  fi
else
  echo "==> gcloud not found; skipping Terraform imports for existing resources."
fi

echo "==> Terraform apply (VPC, GKE, registry, DNS)"
terraform -chdir="${TF_DIR}" apply -auto-approve \
  -var "project_id=${PROJECT_ID}" \
  -var "region=${REGION}" \
  -var "domain=${DOMAIN}" \
  -var "subdomain=${SUBDOMAIN}" \
  -var "app_name=${APP_NAME}" \
  -var "cluster_name=${CLUSTER_NAME}" \
  -var "artifact_repo_name=${ARTIFACT_REPO_NAME}" \
  -var "manage_dns=${MANAGE_DNS}"

REPO="$(terraform -chdir="${TF_DIR}" output -raw artifact_registry_repo)"
STATIC_IP="$(terraform -chdir="${TF_DIR}" output -raw static_ip_address)"
STATIC_IP_NAME="$(terraform -chdir="${TF_DIR}" output -raw static_ip_name)"

if is_truthy "${MANAGE_DNS}"; then
  DNS_NAME_SERVERS_JSON="$(terraform -chdir="${TF_DIR}" output -json dns_nameservers 2>/dev/null || echo "[]")"
  echo "==> If you manage DNS via Cloud DNS, set your domain's nameservers to:"
  DNS_NAME_SERVERS_JSON="${DNS_NAME_SERVERS_JSON}" python3 - <<'PY'
import json
import os

raw = os.environ.get("DNS_NAME_SERVERS_JSON", "[]")
try:
    servers = json.loads(raw)
except json.JSONDecodeError:
    servers = []

if isinstance(servers, list) and servers:
    for ns in servers:
        print(f"  - {ns}")
else:
    print("  (no name servers found; check Terraform outputs)")
PY
fi

export CLOUDSDK_CORE_PROJECT="${PROJECT_ID}"

echo "==> Configure gcloud project"
gcloud config set project "${PROJECT_ID}" >/dev/null

echo "==> Fetch GKE credentials"
gcloud container clusters get-credentials "${CLUSTER_NAME}" --region "${CLUSTER_LOCATION}" >/dev/null

IMAGE="${REPO}/${APP_NAME}:${TAG}"
echo "==> Build & push image with Cloud Build: ${IMAGE}"
gcloud builds submit "${ROOT_DIR}" --tag "${IMAGE}"

echo "==> Render Kubernetes manifests"
RENDER_DIR="$(mktemp -d)"
cp -R "${K8S_DIR}/" "${RENDER_DIR}/k8s"
RENDER_DIR="${RENDER_DIR}" IMAGE="${IMAGE}" FQDN="${FQDN}" STATIC_IP_NAME="${STATIC_IP_NAME}" APP_NAME="${APP_NAME}" NAMESPACE="${NAMESPACE}" python3 - <<'PY'
import os
import pathlib

root = pathlib.Path(os.environ["RENDER_DIR"]) / "k8s"
image = os.environ["IMAGE"]
fqdn = os.environ["FQDN"]
ip_name = os.environ["STATIC_IP_NAME"]
app_name = os.environ["APP_NAME"]
namespace = os.environ["NAMESPACE"]

for path in root.glob("*.yaml"):
    data = path.read_text(encoding="utf-8")
    data = data.replace("__IMAGE__", image)
    data = data.replace("__FQDN__", fqdn)
    data = data.replace("__IP_NAME__", ip_name)
    data = data.replace("__APP_NAME__", app_name)
    data = data.replace("__NAMESPACE__", namespace)
    data = data.replace("__NODE_SELECTOR__", "")
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

def strip_quotes(val: str) -> str:
    if len(val) >= 2 and val[0] == val[-1] and val[0] in ("'", '"'):
        return val[1:-1]
    return val

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
        values[key] = strip_quotes(val)

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
if [[ "${MANAGE_DNS}" != "true" ]]; then
  echo "Create an A record with your DNS provider:"
  echo "  Host: ${SUBDOMAIN:-@}"
  echo "  Type: A"
  echo "  Value: ${STATIC_IP}"
fi
