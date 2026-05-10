# Deploy to GKE (multi-subdomain)

This boilerplate is designed to deploy multiple apps to the same GKE cluster, each on its own subdomain.

## Prereqs

- `gcloud` installed + authenticated (`gcloud auth login`)
- A GCP project with billing enabled
- `kubectl`
- `gcloud components install gke-gcloud-auth-plugin`
- If you intentionally force DinD on Autopilot (`FORCE_AUTOPILOT_DIND=true`): GKE Sandbox/gVisor enabled (`runtimeClassName: gvisor` available).

## 1) Deploy using the existing cluster

From repo root:

```bash
export PROJECT_ID="mcpx-27d6c"
export REGION="asia-south1"
export CLUSTER_NAME="mcpx-gke-india"
export DOMAIN="mcpx.in"
export SUBDOMAIN="explore"      
export APP_NAME="explore"       
export NAMESPACE="explore"      
export MANAGE_DNS="true"        
export USE_GKE_GCLOUD_AUTH_PLUGIN=True

./scripts/deploy_gke.sh
```

Optional nodepool targeting (standard clusters only):

```bash
export NODEPOOL_NAME="apps-pool"
export CREATE_NODEPOOL="true"    # create if missing
export NODEPOOL_MACHINE_TYPE="e2-standard-4"
export NODEPOOL_MIN_NODES="1"
export NODEPOOL_MAX_NODES="3"
```

Optional Judge settings

```bash
export JUDGE_REMOTE_DOCKER_INSTANCE_NAME="judge-docker-1"
export JUDGE_REMOTE_DOCKER_ZONE="asia-south1-c"
export JUDGE_REMOTE_DOCKER_MACHINE_TYPE="e2-standard-2"
```

This will:

- Ensure a global static IP for the ingress
- Create/update the DNS A record in Cloud DNS (when `MANAGE_DNS=true`)
- Build + push the app image with Cloud Build
- Create/update the Kubernetes Secret from your local `.env` (if present)
- Deploy the app + ingress into the existing cluster
- Render judge runtime env as `JUDGE_RUNNER=docker`
- Auto-switch Autopilot DinD deployments to remote Docker provisioning (unless `FORCE_AUTOPILOT_DIND=true`)
- If `JUDGE_REMOTE_DOCKER_HOST` is set, skip DinD sidecar and point Docker CLI to the remote daemon
- If `ENABLE_JUDGE_REMOTE_DOCKER=true` and host is not set, auto-provision/update a Compute Engine Docker host and wire `DOCKER_HOST` automatically
- On Autopilot, render `runtimeClassName: gvisor` with a restricted DinD config (`vfs`, no bridge/iptables)
- Run Docker judge preflight after rollout (`docker info`, configured image pulls, and smoke `docker run`) and fail deploy if judge runtime is unhealthy.

### Autopilot recommended judge mode (single command deploy)

For Autopilot clusters, DinD sidecars can fail during image unpack due kernel capability limits.
`deploy_gke.sh` now auto-enables remote Docker provisioning when DinD is enabled and no remote host is set.

If you want to set it explicitly, use:

```bash
export ENABLE_JUDGE_REMOTE_DOCKER="true"
```

If you intentionally want DinD on Autopilot (not recommended), set:

```bash
export FORCE_AUTOPILOT_DIND="true"
```

Remote Docker provisioning will:

- create the VM if missing (`JUDGE_REMOTE_DOCKER_INSTANCE_NAME`)
- configure Docker daemon on TCP port 2375
- create firewall rule allowing cluster pod CIDR to reach that port
- pre-pull Go/C/C++/Java images
- set `DOCKER_HOST` in the app deployment

Judge preflight controls:

```bash
export ENABLE_JUDGE_RUNTIME_PREFLIGHT="true"
export JUDGE_RUNTIME_PREFLIGHT_IMAGES="gcc:14-bookworm"
export JUDGE_RUNTIME_PREFLIGHT_SMOKE_IMAGE="busybox:latest"
```

## 2) Fast deploy (code changes only)

Use this after the ingress + namespace exist and your `.env` has not changed.

```bash
export PROJECT_ID="your-gcp-project-id"
export REGION="us-central1"
export CLUSTER_NAME="mcpx-gke"
export APP_NAME="explore"
export NAMESPACE="explore"
export ARTIFACT_REPO="mcpx-apps"

./scripts/deploy_gke_code.sh
```

If your Artifact Registry repo is named differently (example: `tripit-apps`), set `ARTIFACT_REPO` accordingly.

## DNS notes

If you keep GoDaddy DNS (`MANAGE_DNS=false`), create an A record:

- Type: `A`
- Name/Host: `script`
- Value: the static IP printed by the deploy script
- TTL: `5 min` (or similar)

The ManagedCertificate will stay "Provisioning" until DNS points to the ingress IP.

## Optional: cluster bootstrap (Terraform)

If you need a brand new cluster or Artifact Registry, you can adapt the Terraform in `infra/terraform`.
