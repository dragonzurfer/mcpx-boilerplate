# docs.md

## Purpose

Deployment helper scripts for GKE, plus local investigation utilities.

## Files

- `deploy_gke.sh`: full deploy (static IP, DNS, image build, manifests), ensures the Artifact Registry repo exists, handles empty DNS record lookups, renders Docker-runner sandbox settings, auto-switches Autopilot DinD deployments to remote Docker provisioning by default, and runs a post-rollout Docker judge runtime preflight (`docker info` + pull/run smoke test) so deploy fails fast if judge execution is broken.
- `deploy_gke_terraform.sh`: full deploy using Terraform with imports for existing GCP resources.
- `deploy_gke_code.sh`: fast deploy for image-only updates.
- `investigate_promo_user.py`: CLI to inspect promo decisions/eligibility for a user using DB_DSN from `.env`.

## Key deploy envs

- `ENABLE_JUDGE_DIND` (default `true`)
- `JUDGE_DIND_IMAGE`
- `JUDGE_DIND_CPU_REQUEST`, `JUDGE_DIND_MEMORY_REQUEST`
- `JUDGE_DIND_CPU_LIMIT`, `JUDGE_DIND_MEMORY_LIMIT`
- `JUDGE_DOCKER_BIN`
- `JUDGE_REMOTE_DOCKER_HOST` (optional; if set, deploy renders Docker runner env without DinD sidecar)
- `JUDGE_REMOTE_DOCKER_TLS_VERIFY` (optional; forwarded to pod `DOCKER_TLS_VERIFY`)
- `JUDGE_REMOTE_DOCKER_CERT_PATH` (optional; forwarded to pod `DOCKER_CERT_PATH`)
- `ENABLE_JUDGE_REMOTE_DOCKER` (optional; auto-provision a Compute Engine Docker host when host is not provided)
- `FORCE_AUTOPILOT_DIND` (optional; keep DinD on Autopilot instead of auto-enabling remote Docker)
- `ENABLE_JUDGE_RUNTIME_PREFLIGHT` (optional; defaults to true and validates Docker runner health after rollout)
- `JUDGE_RUNTIME_PREFLIGHT_IMAGES` (optional; comma-separated images to `docker pull` during preflight)
- `JUDGE_RUNTIME_PREFLIGHT_SMOKE_IMAGE` (optional; image used for `docker run --rm ... echo` smoke test)
- `JUDGE_REMOTE_DOCKER_INSTANCE_NAME`, `JUDGE_REMOTE_DOCKER_ZONE`, `JUDGE_REMOTE_DOCKER_MACHINE_TYPE`
- `JUDGE_REMOTE_DOCKER_NETWORK`, `JUDGE_REMOTE_DOCKER_SUBNET`, `JUDGE_REMOTE_DOCKER_PORT`
- `JUDGE_REMOTE_DOCKER_TAG`, `JUDGE_REMOTE_DOCKER_FIREWALL_RULE`, `JUDGE_REMOTE_DOCKER_SOURCE_RANGE`
- `JUDGE_REMOTE_DOCKER_IMAGE_FAMILY`, `JUDGE_REMOTE_DOCKER_IMAGE_PROJECT`
- `JUDGE_REMOTE_DOCKER_PREPULL_IMAGES`
