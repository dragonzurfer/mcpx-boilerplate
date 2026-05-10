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
ENABLE_JUDGE_DIND="${ENABLE_JUDGE_DIND:-true}"
JUDGE_DIND_IMAGE="${JUDGE_DIND_IMAGE:-docker:27-dind}"
JUDGE_DIND_CPU_REQUEST="${JUDGE_DIND_CPU_REQUEST:-300m}"
JUDGE_DIND_MEMORY_REQUEST="${JUDGE_DIND_MEMORY_REQUEST:-768Mi}"
JUDGE_DIND_CPU_LIMIT="${JUDGE_DIND_CPU_LIMIT:-1500m}"
JUDGE_DIND_MEMORY_LIMIT="${JUDGE_DIND_MEMORY_LIMIT:-2Gi}"
JUDGE_DOCKER_BIN="${JUDGE_DOCKER_BIN:-/usr/local/bin/docker}"
JUDGE_REMOTE_DOCKER_HOST="${JUDGE_REMOTE_DOCKER_HOST:-}"
JUDGE_REMOTE_DOCKER_TLS_VERIFY="${JUDGE_REMOTE_DOCKER_TLS_VERIFY:-}"
JUDGE_REMOTE_DOCKER_CERT_PATH="${JUDGE_REMOTE_DOCKER_CERT_PATH:-}"
ENABLE_JUDGE_REMOTE_DOCKER="${ENABLE_JUDGE_REMOTE_DOCKER:-false}"
FORCE_AUTOPILOT_DIND="${FORCE_AUTOPILOT_DIND:-false}"
ENABLE_JUDGE_RUNTIME_PREFLIGHT="${ENABLE_JUDGE_RUNTIME_PREFLIGHT:-true}"
JUDGE_RUNTIME_PREFLIGHT_IMAGES="${JUDGE_RUNTIME_PREFLIGHT_IMAGES:-gcc:14-bookworm}"
JUDGE_RUNTIME_PREFLIGHT_SMOKE_IMAGE="${JUDGE_RUNTIME_PREFLIGHT_SMOKE_IMAGE:-busybox:latest}"
JUDGE_REMOTE_DOCKER_INSTANCE_NAME="${JUDGE_REMOTE_DOCKER_INSTANCE_NAME:-judge-docker-1}"
JUDGE_REMOTE_DOCKER_ZONE="${JUDGE_REMOTE_DOCKER_ZONE:-}"
JUDGE_REMOTE_DOCKER_MACHINE_TYPE="${JUDGE_REMOTE_DOCKER_MACHINE_TYPE:-e2-standard-2}"
JUDGE_REMOTE_DOCKER_NETWORK="${JUDGE_REMOTE_DOCKER_NETWORK:-}"
JUDGE_REMOTE_DOCKER_SUBNET="${JUDGE_REMOTE_DOCKER_SUBNET:-}"
JUDGE_REMOTE_DOCKER_PORT="${JUDGE_REMOTE_DOCKER_PORT:-2375}"
JUDGE_REMOTE_DOCKER_TAG="${JUDGE_REMOTE_DOCKER_TAG:-judge-docker}"
JUDGE_REMOTE_DOCKER_FIREWALL_RULE="${JUDGE_REMOTE_DOCKER_FIREWALL_RULE:-allow-gke-pods-to-judge-docker}"
JUDGE_REMOTE_DOCKER_SOURCE_RANGE="${JUDGE_REMOTE_DOCKER_SOURCE_RANGE:-}"
JUDGE_REMOTE_DOCKER_IMAGE_FAMILY="${JUDGE_REMOTE_DOCKER_IMAGE_FAMILY:-ubuntu-2204-lts}"
JUDGE_REMOTE_DOCKER_IMAGE_PROJECT="${JUDGE_REMOTE_DOCKER_IMAGE_PROJECT:-ubuntu-os-cloud}"
JUDGE_REMOTE_DOCKER_PREPULL_IMAGES="${JUDGE_REMOTE_DOCKER_PREPULL_IMAGES:-golang:1.25-bookworm,gcc:14-bookworm,eclipse-temurin:21-jdk}"

if [[ -z "${PROJECT_ID}" ]]; then
  echo "PROJECT_ID is required"
  exit 1
fi

is_true() {
  local value
  value="$(printf "%s" "${1:-}" | tr '[:upper:]' '[:lower:]')"
  [[ "${value}" == "true" || "${value}" == "1" || "${value}" == "yes" || "${value}" == "y" ]]
}

run_judge_docker_preflight() {
  local app_ref="deployment/${APP_NAME}"
  local docker_bin="${JUDGE_DOCKER_BIN:-/usr/local/bin/docker}"
  local pull_images="${JUDGE_RUNTIME_PREFLIGHT_IMAGES}"
  local smoke_image="${JUDGE_RUNTIME_PREFLIGHT_SMOKE_IMAGE}"
  local raw_image
  local image
  local old_ifs
  local -a exec_docker_cmd

  exec_docker_cmd=(kubectl -n "${NAMESPACE}" exec "${app_ref}" -c "${APP_NAME}" -- "${docker_bin}")

  echo "==> Judge runtime preflight (Docker runner)"
  if ! "${exec_docker_cmd[@]}" info >/dev/null; then
    echo "==> Docker info check failed from app container via ${docker_bin}"
    echo "==> Verify JUDGE_DOCKER_BIN and DOCKER_HOST env values in deployment."
    return 1
  fi

  old_ifs="${IFS}"
  IFS=','
  for raw_image in ${pull_images}; do
    image="$(printf '%s' "${raw_image}" | tr -d '[:space:]')"
    if [[ -z "${image}" ]]; then
      continue
    fi

    if ! "${exec_docker_cmd[@]}" image inspect "${image}" >/dev/null 2>&1; then
      if ! "${exec_docker_cmd[@]}" pull "${image}" >/dev/null; then
        echo "==> Failed to pull preflight image: ${image}"
        IFS="${old_ifs}"
        return 1
      fi
    fi
  done
  IFS="${old_ifs}"

  if ! "${exec_docker_cmd[@]}" image inspect "${smoke_image}" >/dev/null 2>&1; then
    if ! "${exec_docker_cmd[@]}" pull "${smoke_image}" >/dev/null; then
      echo "==> Failed to pull smoke image: ${smoke_image}"
      return 1
    fi
  fi

  if ! "${exec_docker_cmd[@]}" run --rm --network none "${smoke_image}" echo judge-runtime-ok >/dev/null; then
    echo "==> Failed to run smoke container image: ${smoke_image}"
    return 1
  fi

  echo "==> Judge runtime preflight passed"
  return 0
}

ensure_remote_judge_docker_vm() {
  local instance_name="${JUDGE_REMOTE_DOCKER_INSTANCE_NAME}"
  local instance_zone="${JUDGE_REMOTE_DOCKER_ZONE}"
  local -a create_instance_args
  local judge_remote_script
  local ssh_attempt
  local ssh_ready
  local vm_private_ip

  if [[ -z "${instance_zone}" ]]; then
    echo "JUDGE_REMOTE_DOCKER_ZONE must be set when enabling remote judge Docker provisioning."
    exit 1
  fi

  if [[ -z "${JUDGE_REMOTE_DOCKER_SOURCE_RANGE}" ]]; then
    echo "JUDGE_REMOTE_DOCKER_SOURCE_RANGE is empty; set it to the GKE pod CIDR (for example 10.91.0.0/17)."
    exit 1
  fi

  echo "==> Ensure remote judge Docker VM (${instance_name})"
  if ! gcloud compute instances describe "${instance_name}" --zone "${instance_zone}" >/dev/null 2>&1; then
    create_instance_args=(
      gcloud compute instances create "${instance_name}"
      --zone "${instance_zone}"
      --machine-type "${JUDGE_REMOTE_DOCKER_MACHINE_TYPE}"
      --image-family "${JUDGE_REMOTE_DOCKER_IMAGE_FAMILY}"
      --image-project "${JUDGE_REMOTE_DOCKER_IMAGE_PROJECT}"
      --tags "${JUDGE_REMOTE_DOCKER_TAG}"
      --network "${JUDGE_REMOTE_DOCKER_NETWORK}"
    )
    if [[ -n "${JUDGE_REMOTE_DOCKER_SUBNET}" ]]; then
      create_instance_args+=(--subnet "${JUDGE_REMOTE_DOCKER_SUBNET}")
    fi
    "${create_instance_args[@]}"
  else
    echo "==> Remote judge Docker VM already exists"
  fi

  echo "==> Ensure firewall rule (${JUDGE_REMOTE_DOCKER_FIREWALL_RULE})"
  if ! gcloud compute firewall-rules describe "${JUDGE_REMOTE_DOCKER_FIREWALL_RULE}" >/dev/null 2>&1; then
    gcloud compute firewall-rules create "${JUDGE_REMOTE_DOCKER_FIREWALL_RULE}" \
      --network "${JUDGE_REMOTE_DOCKER_NETWORK}" \
      --direction INGRESS \
      --action ALLOW \
      --rules "tcp:${JUDGE_REMOTE_DOCKER_PORT}" \
      --source-ranges "${JUDGE_REMOTE_DOCKER_SOURCE_RANGE}" \
      --target-tags "${JUDGE_REMOTE_DOCKER_TAG}"
  else
    echo "==> Firewall rule already exists"
  fi

  echo "==> Configure remote Docker daemon on VM"
  ssh_ready="false"
  for ssh_attempt in {1..24}; do
    if gcloud compute ssh "${instance_name}" --zone "${instance_zone}" --quiet --command "echo remote-ssh-ready" >/dev/null 2>&1; then
      ssh_ready="true"
      break
    fi
    sleep 5
  done

  if [[ "${ssh_ready}" != "true" ]]; then
    echo "remote VM ${instance_name} is not reachable over SSH yet"
    exit 1
  fi

  judge_remote_script="$(mktemp)"
  cat > "${judge_remote_script}" <<EOF
#!/usr/bin/env bash
set -euo pipefail

if ! command -v docker >/dev/null 2>&1; then
  sudo apt-get update -y
  sudo apt-get install -y docker.io
fi

sudo systemctl enable docker
sudo mkdir -p /etc/docker
cat <<'JSON' | sudo tee /etc/docker/daemon.json >/dev/null
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" }
}
JSON

sudo mkdir -p /etc/systemd/system/docker.service.d
cat <<SERVICE | sudo tee /etc/systemd/system/docker.service.d/override.conf >/dev/null
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd --host=unix:///var/run/docker.sock --host=tcp://0.0.0.0:${JUDGE_REMOTE_DOCKER_PORT} --containerd=/run/containerd/containerd.sock
SERVICE

sudo systemctl daemon-reload
sudo systemctl restart docker
sudo docker info >/dev/null

PREPULL_IMAGES="${JUDGE_REMOTE_DOCKER_PREPULL_IMAGES}"
if [[ -n "\${PREPULL_IMAGES}" ]]; then
  IFS=',' read -r -a judge_images <<< "\${PREPULL_IMAGES}"
  for judge_image in "\${judge_images[@]}"; do
    trimmed_image="\$(echo "\${judge_image}" | xargs)"
    if [[ -z "\${trimmed_image}" ]]; then
      continue
    fi
    sudo docker pull "\${trimmed_image}" >/dev/null
  done
fi
EOF

  gcloud compute ssh "${instance_name}" --zone "${instance_zone}" --quiet --command "bash -s" < "${judge_remote_script}"
  rm -f "${judge_remote_script}"

  vm_private_ip="$(gcloud compute instances describe "${instance_name}" \
    --zone "${instance_zone}" \
    --format="value(networkInterfaces[0].networkIP)")"
  if [[ -z "${vm_private_ip}" ]]; then
    echo "failed to resolve private IP for ${instance_name}"
    exit 1
  fi

  JUDGE_REMOTE_DOCKER_HOST="tcp://${vm_private_ip}:${JUDGE_REMOTE_DOCKER_PORT}"
  ENABLE_JUDGE_DIND="false"
  echo "==> Remote judge Docker host resolved: ${JUDGE_REMOTE_DOCKER_HOST}"
}

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

echo "==> Ensure Artifact Registry repo (${ARTIFACT_REPO})"
if ! gcloud artifacts repositories describe "${ARTIFACT_REPO}" --location "${REGION}" >/dev/null 2>&1; then
  gcloud artifacts repositories create "${ARTIFACT_REPO}" \
    --repository-format=docker \
    --location "${REGION}" \
    --description="Docker images for ${PROJECT_ID} apps"
fi

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
import json
import sys

raw = sys.stdin.read().strip()
if not raw:
    print("")
    raise SystemExit(0)

data = json.loads(raw)
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
    EXISTING_IPS="$(gcloud dns record-sets list \
      --zone "${DNS_ZONE}" \
      --name "${FQDN}." \
      --type A \
      --format="value(rrdatas)")"
    if [[ -n "${EXISTING_IPS}" ]]; then
      if [[ "${EXISTING_IPS}" == "${STATIC_IP}" ]]; then
        echo "==> DNS already points to ${STATIC_IP}"
      else
        EXISTING_TTL="$(gcloud dns record-sets list \
          --zone "${DNS_ZONE}" \
          --name "${FQDN}." \
          --type A \
          --format="value(ttl)")"
        if [[ -z "${EXISTING_TTL}" ]]; then
          EXISTING_TTL="300"
        fi
        IFS=";" read -r -a IPS <<< "${EXISTING_IPS}"
        gcloud dns record-sets transaction start --zone "${DNS_ZONE}"
        for ip in "${IPS[@]}"; do
          gcloud dns record-sets transaction remove "${ip}" \
            --name "${FQDN}." \
            --ttl "${EXISTING_TTL}" \
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
  fi
else
  echo "==> MANAGE_DNS=false. Create an A record in your DNS provider:"
  echo "  Host: ${SUBDOMAIN}"
  echo "  Type: A"
  echo "  Value: ${STATIC_IP}"
fi

echo "==> Fetch GKE credentials"
gcloud container clusters get-credentials "${CLUSTER_NAME}" --region "${REGION}" >/dev/null

AUTOPILOT_CLUSTER="$(gcloud container clusters describe "${CLUSTER_NAME}" --region "${REGION}" --format="value(autopilot.enabled)" 2>/dev/null || true)"
CLUSTER_NETWORK="$(gcloud container clusters describe "${CLUSTER_NAME}" --region "${REGION}" --format="value(network)" 2>/dev/null || true)"
CLUSTER_SUBNETWORK="$(gcloud container clusters describe "${CLUSTER_NAME}" --region "${REGION}" --format="value(subnetwork)" 2>/dev/null || true)"
CLUSTER_POD_CIDR="$(gcloud container clusters describe "${CLUSTER_NAME}" --region "${REGION}" --format="value(clusterIpv4Cidr)" 2>/dev/null || true)"
CLUSTER_DEFAULT_ZONE="$(gcloud container clusters describe "${CLUSTER_NAME}" --region "${REGION}" --format="value(locations[0])" 2>/dev/null || true)"

if [[ -n "${CLUSTER_NETWORK}" ]]; then
  CLUSTER_NETWORK="${CLUSTER_NETWORK##*/}"
fi
if [[ -n "${CLUSTER_SUBNETWORK}" ]]; then
  CLUSTER_SUBNETWORK="${CLUSTER_SUBNETWORK##*/}"
fi
if [[ -z "${CLUSTER_DEFAULT_ZONE}" ]]; then
  CLUSTER_DEFAULT_ZONE="${REGION}-c"
fi

if [[ -z "${JUDGE_REMOTE_DOCKER_NETWORK}" ]]; then
  if [[ -n "${CLUSTER_NETWORK}" ]]; then
    JUDGE_REMOTE_DOCKER_NETWORK="${CLUSTER_NETWORK}"
  else
    JUDGE_REMOTE_DOCKER_NETWORK="default"
  fi
fi

if [[ -z "${JUDGE_REMOTE_DOCKER_SUBNET}" ]]; then
  JUDGE_REMOTE_DOCKER_SUBNET="${CLUSTER_SUBNETWORK}"
fi

if [[ -z "${JUDGE_REMOTE_DOCKER_SOURCE_RANGE}" ]]; then
  JUDGE_REMOTE_DOCKER_SOURCE_RANGE="${CLUSTER_POD_CIDR}"
fi

if [[ -z "${JUDGE_REMOTE_DOCKER_ZONE}" ]]; then
  JUDGE_REMOTE_DOCKER_ZONE="${CLUSTER_DEFAULT_ZONE}"
fi

if [[ "${ENABLE_JUDGE_DIND}" == "true" && ( "${AUTOPILOT_CLUSTER}" == "true" || "${AUTOPILOT_CLUSTER}" == "True" ) && -z "${JUDGE_REMOTE_DOCKER_HOST}" ]]; then
  if is_true "${FORCE_AUTOPILOT_DIND}"; then
    echo "==> FORCE_AUTOPILOT_DIND=true. Keeping DinD on Autopilot."
  elif ! is_true "${ENABLE_JUDGE_REMOTE_DOCKER}"; then
    echo "==> Autopilot + DinD detected. Auto-enabling remote judge Docker provisioning."
    ENABLE_JUDGE_REMOTE_DOCKER="true"
  fi
fi

if [[ -z "${JUDGE_REMOTE_DOCKER_HOST}" ]] && is_true "${ENABLE_JUDGE_REMOTE_DOCKER}"; then
  ensure_remote_judge_docker_vm
fi

if [[ -n "${JUDGE_REMOTE_DOCKER_HOST}" ]]; then
  if [[ "${ENABLE_JUDGE_DIND}" == "true" ]]; then
    echo "==> JUDGE_REMOTE_DOCKER_HOST is set. Disabling DinD sidecar and using remote Docker daemon."
  else
    echo "==> Using remote Docker daemon from JUDGE_REMOTE_DOCKER_HOST."
  fi
  ENABLE_JUDGE_DIND="false"
fi

JUDGE_RUNNER_MODE="local"
if [[ -n "${JUDGE_REMOTE_DOCKER_HOST}" || "${ENABLE_JUDGE_DIND}" == "true" ]]; then
  JUDGE_RUNNER_MODE="docker"
fi

if [[ "${ENABLE_JUDGE_DIND}" == "true" && ( "${AUTOPILOT_CLUSTER}" == "true" || "${AUTOPILOT_CLUSTER}" == "True" ) ]]; then
  echo "==> Autopilot cluster detected. Rendering judge Docker sidecar with gVisor runtime profile."
  if ! kubectl get runtimeclass gvisor >/dev/null 2>&1; then
    echo "runtimeClassName 'gvisor' was not found in the cluster."
    echo "Enable GKE Sandbox on the cluster/node pools before using ENABLE_JUDGE_DIND=true."
    exit 1
  fi
fi

if [[ "${CREATE_NODEPOOL}" == "true" && -n "${NODEPOOL_NAME}" ]]; then
  if [[ "${AUTOPILOT_CLUSTER}" == "true" || "${AUTOPILOT_CLUSTER}" == "True" ]]; then
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
RENDER_DIR="${RENDER_DIR}" IMAGE="${IMAGE}" FQDN="${FQDN}" STATIC_IP_NAME="${STATIC_IP_NAME}" APP_NAME="${APP_NAME}" NAMESPACE="${NAMESPACE}" NODEPOOL_NAME="${NODEPOOL_NAME}" AUTOPILOT_CLUSTER="${AUTOPILOT_CLUSTER}" ENABLE_JUDGE_DIND="${ENABLE_JUDGE_DIND}" JUDGE_DIND_IMAGE="${JUDGE_DIND_IMAGE}" JUDGE_DIND_CPU_REQUEST="${JUDGE_DIND_CPU_REQUEST}" JUDGE_DIND_MEMORY_REQUEST="${JUDGE_DIND_MEMORY_REQUEST}" JUDGE_DIND_CPU_LIMIT="${JUDGE_DIND_CPU_LIMIT}" JUDGE_DIND_MEMORY_LIMIT="${JUDGE_DIND_MEMORY_LIMIT}" JUDGE_DOCKER_BIN="${JUDGE_DOCKER_BIN}" JUDGE_REMOTE_DOCKER_HOST="${JUDGE_REMOTE_DOCKER_HOST}" JUDGE_REMOTE_DOCKER_TLS_VERIFY="${JUDGE_REMOTE_DOCKER_TLS_VERIFY}" JUDGE_REMOTE_DOCKER_CERT_PATH="${JUDGE_REMOTE_DOCKER_CERT_PATH}" python3 - <<'PY'
import os, pathlib

root = pathlib.Path(os.environ["RENDER_DIR"]) / "k8s"
image = os.environ["IMAGE"]
fqdn = os.environ["FQDN"]
ip_name = os.environ["STATIC_IP_NAME"]
app_name = os.environ["APP_NAME"]
namespace = os.environ["NAMESPACE"]
nodepool = os.environ.get("NODEPOOL_NAME", "").strip()
autopilot = os.environ.get("AUTOPILOT_CLUSTER", "").strip().lower() == "true"
enable_judge_dind = os.environ.get("ENABLE_JUDGE_DIND", "true").strip().lower() == "true"
judge_dind_image = os.environ.get("JUDGE_DIND_IMAGE", "docker:27-dind").strip() or "docker:27-dind"
judge_dind_cpu_request = os.environ.get("JUDGE_DIND_CPU_REQUEST", "300m").strip() or "300m"
judge_dind_memory_request = os.environ.get("JUDGE_DIND_MEMORY_REQUEST", "768Mi").strip() or "768Mi"
judge_dind_cpu_limit = os.environ.get("JUDGE_DIND_CPU_LIMIT", "1500m").strip() or "1500m"
judge_dind_memory_limit = os.environ.get("JUDGE_DIND_MEMORY_LIMIT", "2Gi").strip() or "2Gi"
judge_docker_bin = os.environ.get("JUDGE_DOCKER_BIN", "/usr/local/bin/docker").strip() or "/usr/local/bin/docker"
judge_remote_docker_host = os.environ.get("JUDGE_REMOTE_DOCKER_HOST", "").strip()
judge_remote_docker_tls_verify = os.environ.get("JUDGE_REMOTE_DOCKER_TLS_VERIFY", "").strip()
judge_remote_docker_cert_path = os.environ.get("JUDGE_REMOTE_DOCKER_CERT_PATH", "").strip()

node_selector = ""
if nodepool:
    node_selector = "nodeSelector:\n        cloud.google.com/gke-nodepool: \"%s\"" % nodepool

judge_runner_env = """
            - name: JUDGE_RUNNER
              value: "local"
""".rstrip()

pod_annotations = ""
runtime_class = ""
judge_dind_sidecar = ""
judge_dind_volumes = ""

if judge_remote_docker_host:
    remote_env_lines = [
        '            - name: JUDGE_RUNNER',
        '              value: "docker"',
        '            - name: JUDGE_DOCKER_BIN',
        f'              value: "{judge_docker_bin}"',
        '            - name: DOCKER_HOST',
        f'              value: "{judge_remote_docker_host}"',
    ]
    if judge_remote_docker_tls_verify:
        remote_env_lines.extend([
            '            - name: DOCKER_TLS_VERIFY',
            f'              value: "{judge_remote_docker_tls_verify}"',
        ])
    if judge_remote_docker_cert_path:
        remote_env_lines.extend([
            '            - name: DOCKER_CERT_PATH',
            f'              value: "{judge_remote_docker_cert_path}"',
        ])
    judge_runner_env = "\n".join(remote_env_lines)
elif enable_judge_dind:
    judge_runner_env = f"""
            - name: JUDGE_RUNNER
              value: "docker"
            - name: JUDGE_DOCKER_BIN
              value: "{judge_docker_bin}"
            - name: DOCKER_HOST
              value: "tcp://127.0.0.1:2375"
""".rstrip()

    sidecar_security_context = """
          securityContext:
            privileged: true
""".rstrip()

    if autopilot:
        runtime_class = """
      runtimeClassName: gvisor
""".rstrip()
        sidecar_security_context = ""

    judge_dind_sidecar = f"""
        - name: judge-docker-daemon
          image: "{judge_dind_image}"
          imagePullPolicy: IfNotPresent
          command:
            - dockerd
          env:
            - name: DOCKER_TLS_CERTDIR
              value: ""
          args:
            - --host=tcp://0.0.0.0:2375
            - --host=unix:///var/run/docker.sock
            - --storage-driver=overlay2
{sidecar_security_context}
          volumeMounts:
            - name: dind-storage
              mountPath: /var/lib/docker
          resources:
            requests:
              cpu: "{judge_dind_cpu_request}"
              memory: "{judge_dind_memory_request}"
            limits:
              cpu: "{judge_dind_cpu_limit}"
              memory: "{judge_dind_memory_limit}"
""".rstrip()

    if autopilot:
        judge_dind_sidecar = f"""
        - name: judge-docker-daemon
          image: "{judge_dind_image}"
          imagePullPolicy: IfNotPresent
          command:
            - dockerd
          env:
            - name: DOCKER_TLS_CERTDIR
              value: ""
          args:
            - --host=tcp://0.0.0.0:2375
            - --host=unix:///var/run/docker.sock
            - --storage-driver=vfs
            - --iptables=false
            - --bridge=none
            - --ip-forward=false
            - --ip-masq=false
{sidecar_security_context}
          volumeMounts:
            - name: dind-storage
              mountPath: /var/lib/docker
          resources:
            requests:
              cpu: "{judge_dind_cpu_request}"
              memory: "{judge_dind_memory_request}"
            limits:
              cpu: "{judge_dind_cpu_limit}"
              memory: "{judge_dind_memory_limit}"
""".rstrip()

    judge_dind_volumes = """
      volumes:
        - name: dind-storage
          emptyDir: {}
""".rstrip()

for path in root.glob("*.yaml"):
    data = path.read_text(encoding="utf-8")
    data = data.replace("__IMAGE__", image)
    data = data.replace("__FQDN__", fqdn)
    data = data.replace("__IP_NAME__", ip_name)
    data = data.replace("__APP_NAME__", app_name)
    data = data.replace("__NAMESPACE__", namespace)
    data = data.replace("__NODE_SELECTOR__", node_selector)
    data = data.replace("__POD_ANNOTATIONS__", pod_annotations)
    data = data.replace("__RUNTIME_CLASS__", runtime_class)
    data = data.replace("__JUDGE_RUNNER_ENV__", judge_runner_env)
    data = data.replace("__JUDGE_DIND_SIDECAR__", judge_dind_sidecar)
    data = data.replace("__JUDGE_DIND_VOLUMES__", judge_dind_volumes)
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

if [[ "${JUDGE_RUNNER_MODE}" == "docker" ]] && is_true "${ENABLE_JUDGE_RUNTIME_PREFLIGHT}"; then
  if ! run_judge_docker_preflight; then
    echo "==> Judge runtime preflight failed"
    echo "==> Recent app logs"
    kubectl -n "${NAMESPACE}" logs deployment/"${APP_NAME}" -c "${APP_NAME}" --tail=120 || true

    if [[ "${ENABLE_JUDGE_DIND}" == "true" ]]; then
      echo "==> Recent DinD sidecar logs"
      kubectl -n "${NAMESPACE}" logs deployment/"${APP_NAME}" -c judge-docker-daemon --tail=120 || true
    fi

    exit 1
  fi
fi

echo "==> Done"
echo "Static IP: ${STATIC_IP}"
echo "Domain: https://${FQDN}"
if [[ -n "${JUDGE_REMOTE_DOCKER_HOST}" ]]; then
  echo "Judge Docker Host: ${JUDGE_REMOTE_DOCKER_HOST}"
fi
