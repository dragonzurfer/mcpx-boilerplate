# docs.md

## Purpose

Deployment helper scripts for GKE, plus local investigation utilities.

## Files

- `deploy_gke.sh`: full deploy (static IP, DNS, image build, manifests).
- `deploy_gke_terraform.sh`: full deploy using Terraform with imports for existing GCP resources.
- `deploy_gke_code.sh`: fast deploy for image-only updates.
- `investigate_promo_user.py`: CLI to inspect promo decisions/eligibility for a user using DB_DSN from `.env`.
