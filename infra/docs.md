# docs.md

## Purpose

Infrastructure provisioning assets for both the legacy GKE deployment path and the current single-Droplet Docker deployment path.

## Subfolders

- `droplet/`: shared reverse-proxy stack plus per-app Compose assets for the DigitalOcean Droplet layout, including judge worker resource overrides for the Explore stack.
- `terraform/`: Terraform for cluster, registry, and DNS resources.
- `../scripts/deploy_gke_terraform.sh`: deployment script that imports existing GCP resources into Terraform state before apply.
