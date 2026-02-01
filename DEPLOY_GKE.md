# Deploy to GKE (multi-subdomain)

This boilerplate is designed to deploy multiple apps to the same GKE cluster, each on its own subdomain.

## Prereqs

- `gcloud` installed + authenticated (`gcloud auth login`)
- A GCP project with billing enabled
- `kubectl`
- `gcloud components install gke-gcloud-auth-plugin`

## 1) Deploy using the existing cluster

From repo root:

```bash
export PROJECT_ID="your-gcp-project-id"
export REGION="us-central1"
export CLUSTER_NAME="mcpx-gke"
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

This will:

- Ensure a global static IP for the ingress
- Create/update the DNS A record in Cloud DNS (when `MANAGE_DNS=true`)
- Build + push a Docker image with Cloud Build
- Create/update the Kubernetes Secret from your local `.env` (if present)
- Deploy the app + ingress into the existing cluster

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
