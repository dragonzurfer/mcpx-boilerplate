# docs.md

## Purpose

Infrastructure provisioning assets for the backend and GKE hosting.

## Subfolders

- `terraform/`: Terraform for cluster, registry, and DNS resources.
- `../scripts/deploy_gke_terraform.sh`: deployment script that imports existing GCP resources into Terraform state before apply.
