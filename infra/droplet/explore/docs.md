# docs.md

## Purpose

Container deployment assets for Explore on the shared DigitalOcean Droplet.

## Files

- `compose.yaml`: launches the Explore web container and the dedicated worker container from the same app image.

## Notes

- The web container runs with `APP_MODE=web` and is the only Explore container attached to the shared `edge` network.
- The worker container runs with `APP_MODE=worker`, mounts `/var/run/docker.sock`, and starts `JUDGE_WORKER_CONCURRENCY=4` queue consumers for local judge execution.
- The worker stack pins `JUDGE_DOCKER_CPUS=1` so a single active compile is not artificially throttled to half a CPU on the shared host.
- The worker stack pins `JUDGE_DOCKER_COMPILE_MEMORY_MB=512` so Go compiles are not killed by low per-problem runtime memory caps.
- The worker stack also overrides `JUDGE_DOCKER_COMPILE_TIMEOUT_MS=60000` so Go compiles can finish even when the shared-CPU Droplet is contended.
- Shared app state lives at `/opt/apps/explore/shared`, including `explore.env` and log files.
