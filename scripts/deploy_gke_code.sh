#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-us-central1}"
CLUSTER_NAME="${CLUSTER_NAME:-mcpx-gke}"
APP_NAME="${APP_NAME:-app}"
NAMESPACE="${NAMESPACE:-${APP_NAME}}"
ARTIFACT_REPO="${ARTIFACT_REPO:-mcpx-apps}"
IMAGE_REPO="${IMAGE_REPO:-}"
SKIP_BUILD="${SKIP_BUILD:-false}"

if [[ -z "${PROJECT_ID}" ]]; then
  echo "PROJECT_ID is required"
  exit 1
fi

if [[ -n "${IMAGE:-}" ]]; then
  IMAGE="${IMAGE}"
  SKIP_BUILD="true"
else
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
fi

echo "==> Configure gcloud project"
gcloud config set project "${PROJECT_ID}" >/dev/null

echo "==> Fetch GKE credentials"
gcloud container clusters get-credentials "${CLUSTER_NAME}" --region "${REGION}" >/dev/null

if [[ "${SKIP_BUILD}" != "true" ]]; then
  echo "==> Build & push image with Cloud Build: ${IMAGE}"
  gcloud builds submit "${ROOT_DIR}" --tag "${IMAGE}"
else
  echo "==> Skipping build (IMAGE provided or SKIP_BUILD=true): ${IMAGE}"
fi

echo "==> Update deployment image"
kubectl -n "${NAMESPACE}" set image deployment/"${APP_NAME}" "${APP_NAME}"="${IMAGE}"
ROLLOUT_TIMEOUT="${ROLLOUT_TIMEOUT:-10m}"
kubectl -n "${NAMESPACE}" rollout status deployment/"${APP_NAME}" --timeout="${ROLLOUT_TIMEOUT}"

echo "==> Done"
