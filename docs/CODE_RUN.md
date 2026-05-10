# CODE_RUN.md

## Purpose

This document explains the practice code-execution pipeline end to end:

- how a submission is queued and judged,
- how Docker volumes are created/cleaned,
- how code is sent to the Docker host,
- where runtime configs live,
- and how deployment wires judge execution in both Kubernetes and the shared single-Droplet worker container.

## Runtime architecture

### Main components

- **Submission API** (`routes/submissions.go`): accepts `RUN` / `SUBMIT`, validates limits, and writes `QUEUED` submissions.
- **Judge worker loops** (`main.go` → `startJudgeWorker`): continuously poll the queue from `APP_MODE=all` or `APP_MODE=worker` processes.
- **Judge orchestrator** (`services/judge_service.go`): claims one queued submission, loads problem/dataset/tests, calls runner, persists result.
- **Docker runner** (`services/runner_docker.go`): compile + run inside sandboxed Docker containers.
- **Store layer** (`stores/submissions_store.go`): queue claim with DB lock and status transitions.

### Concurrency model

- Each worker process starts `JUDGE_WORKER_CONCURRENCY` judge loops.
- Each loop processes one submission at a time.
- Queue claim uses `FOR UPDATE SKIP LOCKED`, so multiple pods can safely process different queued rows.

## End-to-end code flow

1. Client calls `POST /api/submissions`.
2. Server validates user, limits, language, dataset selection.
3. Submission row is created with status `QUEUED`.
4. Judge worker calls `ClaimNextQueuedSubmission`:
   - picks oldest queued row,
   - marks it `RUNNING` + sets `started_at`.
5. Judge service builds runner input (source code, limits, tests, stop-early policy).
6. Docker runner executes:
   - create per-submission Docker volume,
   - write source file into volume,
   - compile once,
   - run tests (stop-early policy applied),
   - map raw outputs to canonical verdicts.
7. Judge service stores result JSON and marks submission `COMPLETED` (or `FAILED` on orchestrator error).
8. Client polls `GET /api/submissions/:id/result` and receives status/result payload.

## Docker execution details

### Language build/run templates

Language commands are resolved by shared language config (`services/runner_local.go`):

- Go: `go build -o main main.go`, run `./main`
- C: `gcc ... -o main main.c`, run `./main`
- C++: `g++ ... -o main main.cpp`, run `./main`
- Java: `javac Main.java`, run `java ... Main`

### Volume lifecycle

Implemented in `services/runner_docker.go`:

1. `createDockerVolume(...)` creates `judge-vol-<submission>-<timestamp>`.
2. `writeSourceToVolume(...)` streams source via stdin:
   - launches helper container with volume mounted at `/workspace`,
   - executes `cat > /workspace/<source-file>`.
3. Compile container mounts the same volume and writes binaries/artifacts there.
4. Each test container mounts the same volume and executes compiled program.
   - test containers are launched with `docker run -i` so testcase bytes are delivered to process stdin.
5. `defer removeDockerVolume(...)` guarantees volume cleanup after run path returns.

This volume-based flow is required for remote Docker hosts (`DOCKER_HOST`) because host bind paths from app pod are not valid on remote VM daemons.

### Sandbox flags per container

Each compile/test container uses:

- `-i` (stdin attached for source/testcase piping)
- `--network none`
- `--read-only`
- `--tmpfs /tmp:rw,nosuid,nodev,noexec,size=<...>`
- `--cap-drop ALL`
- `--security-opt no-new-privileges`
- `--pids-limit <...>`
- `--memory` + `--memory-swap`
- `--cpus`

Default runner env also redirects compiler scratch state onto the writable `/workspace` volume:

- `HOME=/workspace`
- `TMPDIR=/workspace`
- `GOCACHE=/workspace/.cache/go-build`

This avoids Go compile/link failures caused by filling the small `/tmp` tmpfs on read-only-rootfs containers.

### Timeouts and cleanup

- Compile containers use a separate memory floor from runtime containers: `512 MB` by default, or `JUDGE_DOCKER_COMPILE_MEMORY_MB` when set.
- Compile timeout defaults to `4x` problem runtime limit, clamped to `10s..120s` (or overridden by `JUDGE_DOCKER_COMPILE_TIMEOUT_MS`).
- Each run is wrapped in `context.WithTimeout(...)`; timed out processes are killed.
- After each run command, runner force-removes container (`docker rm -f`).
- If app process crashes mid-run, orphan containers/volumes can remain; cleanup can be done on Docker host with `docker ps -a` / `docker volume ls`.

## How code reaches remote Docker host

When `DOCKER_HOST=tcp://<host>:2375` is set:

- Docker CLI runs inside app container.
- CLI sends API requests to remote Docker daemon over TCP.
- Source code bytes are piped from app process stdin into helper container command (`cat > /workspace/file`) on remote host.
- Compile and test containers are created on remote host and access the same named volume.

### Why backend code does not hardcode Docker host

`services/runner_docker.go` executes plain `docker ...` commands via `exec.CommandContext(...)`.
It does not pass `-H` in command args. Docker CLI automatically reads `DOCKER_HOST` from process environment.

In Kubernetes deploys, `scripts/deploy_gke.sh` renders environment variables into the app deployment:

- `JUDGE_RUNNER=docker`
- `JUDGE_DOCKER_BIN=/usr/local/bin/docker`
- `DOCKER_HOST=tcp://<remote-private-ip>:2375`

So the host routing is done by deployment-time env wiring, not by hardcoded Go constants.

### Current host type in this setup

With remote mode enabled, the Docker daemon runs on a separate Compute Engine VM (for example `e2-standard-2`).
Pods call that VM over private VPC IP and Docker remote API TCP port (`2375`).

## Configuration sources

### Application env (`sample.env`)

Relevant keys:

- `APP_MODE` (`all`/`web`/`worker`)
- `JUDGE_RUNNER` (`docker`/`k8s`/`local`)
- `JUDGE_WORKER_CONCURRENCY`
- `JUDGE_DOCKER_BIN`
- `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`
- `JUDGE_DOCKER_IMAGE*` (global/per-language overrides)
- `JUDGE_DOCKER_CPUS`, `JUDGE_DOCKER_TMPFS_MB`, `JUDGE_DOCKER_PIDS_LIMIT`
- `JUDGE_DOCKER_COMPILE_MEMORY_MB`
- `JUDGE_DOCKER_COMPILE_TIMEOUT_MS`

## Shared Droplet deployment flow

On the DigitalOcean Droplet deployment:

1. The `web` container runs with `APP_MODE=web` and serves requests behind shared Nginx.
2. The `worker` container runs with `APP_MODE=worker`.
3. The worker mounts `/var/run/docker.sock` and uses the host Docker daemon for local judge containers.
4. `JUDGE_WORKER_CONCURRENCY=4` starts four queue consumers in the worker process.
5. `JUDGE_DOCKER_COMPILE_MEMORY_MB=512` keeps Go compiles from inheriting low per-problem runtime memory caps on the shared host.
6. `JUDGE_DOCKER_CPUS=1` plus `JUDGE_DOCKER_COMPILE_TIMEOUT_MS=60000` keeps Go compiles from timing out under normal low-traffic use on the shared-CPU host.

This keeps Docker access away from the public web container while preserving local judge latency on the same host.

### Deployment env (`scripts/deploy_gke.sh`)

Remote Docker provisioning/wiring:

- `ENABLE_JUDGE_REMOTE_DOCKER=true` (auto-provision VM path)
- `JUDGE_REMOTE_DOCKER_HOST` (manual host override)
- `JUDGE_REMOTE_DOCKER_INSTANCE_NAME`, `JUDGE_REMOTE_DOCKER_ZONE`, `JUDGE_REMOTE_DOCKER_MACHINE_TYPE`
- `JUDGE_REMOTE_DOCKER_NETWORK`, `JUDGE_REMOTE_DOCKER_SUBNET`
- `JUDGE_REMOTE_DOCKER_SOURCE_RANGE` (firewall allowlist)

## Deployment flow (GKE)

`scripts/deploy_gke.sh` performs:

1. GCP project setup, Artifact Registry, static IP, DNS.
2. Fetch cluster credentials.
3. Optional remote Docker VM provisioning/config:
   - create VM if missing,
   - install/configure Docker daemon + systemd override,
   - open firewall from pod CIDR to Docker port,
   - pre-pull judge images.
4. Build/push app image.
5. Render Kubernetes manifests with judge env wiring.
6. Apply manifests + rollout.

Rendered deployment wiring lives in `k8s/deployment.yaml` placeholders populated by script.

## Quick operational checks

- Check app pods: `kubectl -n <ns> get pods -l app=<app>`
- Check judge logs: `kubectl -n <ns> logs deploy/<app> -c <app-container>`
- Check Docker connectivity from pod:
  - `docker info`
  - `docker pull golang:1.25-bookworm`
