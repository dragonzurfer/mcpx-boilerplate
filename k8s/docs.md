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

## Health checks

`deployment.yaml` configures startup, readiness, and liveness probes against `/healthz`
on port 8080 with 10-second probe intervals.

## Judge sandbox runtime

`deployment.yaml` includes placeholders rendered by `scripts/deploy_gke.sh` for:

- Docker runner env (`JUDGE_RUNNER`, `JUDGE_DOCKER_BIN`, `DOCKER_HOST`)
- optional remote Docker daemon env forwarding (`DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`)
- optional Docker daemon sidecar (`judge-docker-daemon`)
- optional sidecar storage volume (`emptyDir` for `/var/lib/docker`)
- optional Autopilot gVisor runtime/annotation placeholders

For Autopilot DinD mode, `scripts/deploy_gke.sh` renders `runtimeClassName: gvisor`
and a tuned daemon profile (`dockerd` command, `vfs` driver, bridge/iptables disabled)
to run the sidecar in a restricted sandbox profile.
