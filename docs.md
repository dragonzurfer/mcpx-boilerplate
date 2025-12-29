# docs.md

## Purpose

This repo is a Gin + Go boilerplate for payments, usage metering, auth, and GKE deployment. See `docs/ARCHITECTURE.md` for the end-to-end flow and middleware layering.

## Key folders

- `routes/`: HTTP handlers and route registration.
- `services/`: business logic (billing, usage enforcement).
- `stores/`: DB access and persistence models.
- `payments/`: plan configuration and pricing helpers.
- `middleware/`: auth, rate limit, IP rules, metering.
- `web/`: static demo UI for the API and billing flows.
- `k8s/` + `scripts/`: Kubernetes manifests and deploy helpers.
- `config/`: plan definitions and defaults.
- `docs/`: architecture and operational docs.
- `infra/`: Terraform for provisioning (cluster, registry, DNS).
- `mobile/`: Expo app for Google Play billing flows.

## Docs entrypoints

- `docs/ARCHITECTURE.md`
- `DEPLOY_GKE.md`
- `DEPLOY_PLAY_STORE.md`
