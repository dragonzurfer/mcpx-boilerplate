# docs.md

## Purpose

Kubernetes manifests for the service, ingress, and TLS setup.

## Files

- `namespace.yaml`: namespace definition for the app.
- `deployment.yaml`: deployment spec for the Go service.
- `service.yaml`: ClusterIP service exposing the app.
- `ingress.yaml`: GKE ingress with static IP and host routing.
- `managedcertificate.yaml`: GKE managed TLS certificate.
- `frontendconfig.yaml`: HTTPS redirect configuration.
- `kustomization.yaml`: kustomize entrypoint.
