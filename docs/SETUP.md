# Environment Setup Guide

This guide covers the installation of necessary tools and dependencies for the project on macOS, Windows, and Linux.

## 1. Package Managers

### macOS (Homebrew)
Homebrew is the recommended package manager for macOS.
```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

### Windows (Chocolatey)
Chocolatey is a popular package manager for Windows. Run the following in PowerShell as Administrator:
```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
```
*Alternatively, you can use [Scoop](https://scoop.sh/).*

### Linux
Most Linux distributions come with a package manager (`apt`, `yum`, `dnf`, `pacman`). You can also install Homebrew on Linux:
```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

---

## 2. Go (Golang)

### macOS
```bash
brew install go
```

### Windows
```powershell
choco install golang
```
*Or download the installer from [go.dev/dl](https://go.dev/dl/).*

### Linux
```bash
# Ubuntu/Debian
sudo apt update && sudo apt install golang-go

# Using Homebrew
brew install go
```

---

## 2.1 golangci-lint (Go linting)

Install the linter used by the backend pre-commit hook.

### macOS
```bash
brew install golangci-lint
```

### Windows
```powershell
choco install golangci-lint
```

### Linux
```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.0
```

---

## 3. Google Cloud SDK (gcloud)

### macOS
```bash
brew install --cask google-cloud-sdk
```

### Windows
Download and run the installer: [Google Cloud SDK Installer](https://cloud.google.com/sdk/docs/install#windows)
Or using Chocolatey:
```powershell
choco install gcloudsdk
```

### Linux
```bash
curl https://sdk.cloud.google.com | bash
exec -l $SHELL
gcloud init
```

---

## 4. Kubectl

**Note:** If you installed `gcloud`, you can install `kubectl` via `gcloud` components:
```bash
gcloud components install kubectl
```

### Standalone Installation

**macOS:**
```bash
brew install kubectl
```

**Windows:**
```powershell
choco install kubernetes-cli
```

**Linux:**
```bash
# Ubuntu/Debian
sudo apt-get install -y kubectl
# Or using Homebrew
brew install kubectl
```

---

## 5. Node.js (Required for Codex & Gemini CLI)

### macOS
```bash
brew install node
```

### Windows
```powershell
choco install nodejs
```

### Linux
```bash
# Ubuntu/Debian (using NodeSource)
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs

# Or using Homebrew
brew install node
```

---

## 6. Codex CLI & Gemini CLI

These tools are installed via `npm` (Node.js Package Manager).

### Install Codex CLI
```bash
npm install -g @openai/codex
```

### Install Gemini CLI
```bash
npm install -g @google/gemini-cli
```

---

## 7. Docker (Required for sandboxed practice judge)

The judge now runs untrusted submissions in Docker by default.

### macOS
```bash
brew install --cask docker
open -a Docker
```

### Windows
Install Docker Desktop: [Docker Desktop](https://www.docker.com/products/docker-desktop/)

### Linux
Follow your distro instructions from Docker docs: [Install Docker Engine](https://docs.docker.com/engine/install/)

---

## 8. Judge runtime images

The primary runtime is the Docker runner (`JUDGE_RUNNER=docker`), which uses language images:

- Go: `golang:1.25-bookworm`
- C/C++: `gcc:14-bookworm`
- Java: `eclipse-temurin:21-jdk`

On GKE deploys, `scripts/deploy_gke.sh` can inject a Docker daemon sidecar so the app pod can run sandboxed compile/tests with `docker run`.
If you cannot run DinD in-cluster, set `JUDGE_REMOTE_DOCKER_HOST` during deploy to keep `JUDGE_RUNNER=docker` while targeting a remote Docker daemon.
You can also set `ENABLE_JUDGE_REMOTE_DOCKER=true` to let the deploy script provision/manage that remote Docker VM automatically.

For Autopilot, the deploy script renders gVisor-compatible DinD settings (`runtimeClassName: gvisor`, `dockerd` command, `vfs` storage driver, bridge/iptables disabled).
Make sure the cluster exposes the `gvisor` runtime class.

---

## 9. Judge runtime environment variables

- `APP_MODE` (default: `all`)
  - `all`: HTTP server + recurring jobs + judge workers
  - `web`: HTTP server only
  - `worker`: recurring jobs + judge workers only
- `JUDGE_RUNNER` (default: `docker`)  
  - `docker`: Docker sandbox
  - `k8s`: Kubernetes Job runner (optional fallback)
  - `local`: unsandboxed fallback (dev only)
- `JUDGE_WORKDIR` (optional host temp workspace root for local execution)
- `JUDGE_WORKER_CONCURRENCY` (default: `1`)

### Docker runner (`JUDGE_RUNNER=docker`)

- `JUDGE_DOCKER_IMAGE` (optional global image override)
- `JUDGE_DOCKER_IMAGE_GO` (optional Go image override)
- `JUDGE_DOCKER_IMAGE_C` (optional C image override)
- `JUDGE_DOCKER_IMAGE_CPP` (optional C++ image override)
- `JUDGE_DOCKER_IMAGE_JAVA` (optional Java image override)
- `JUDGE_DOCKER_BIN` (default: `docker`)
- `JUDGE_DOCKER_CPUS` (default: `1`)
- `JUDGE_DOCKER_TMPFS_MB` (default: `64`)
- `JUDGE_DOCKER_PIDS_LIMIT` (default: `128`)
- `JUDGE_DOCKER_COMPILE_MEMORY_MB` (default: `512`; compile-only memory floor so compilers do not inherit very low runtime memory caps)
- `JUDGE_DOCKER_COMPILE_TIMEOUT_MS` (optional override; default dynamic timeout is 4x problem time-limit, clamped to 10s..120s)
- `DOCKER_HOST` (optional remote daemon, e.g. `tcp://10.0.0.25:2375`)
- `DOCKER_TLS_VERIFY` (optional remote daemon TLS toggle)
- `DOCKER_CERT_PATH` (optional remote daemon TLS cert path)
- `ENABLE_JUDGE_REMOTE_DOCKER` (deploy helper: auto-provision remote Docker VM if host is not set)
- `JUDGE_REMOTE_DOCKER_INSTANCE_NAME`, `JUDGE_REMOTE_DOCKER_ZONE`, `JUDGE_REMOTE_DOCKER_MACHINE_TYPE` (optional VM overrides)

### Kubernetes Job runner (`JUDGE_RUNNER=k8s`, optional)

- `JUDGE_K8S_IMAGE` (required image containing `/app/judge-worker`)
- `JUDGE_K8S_NAMESPACE` (optional; falls back to `POD_NAMESPACE` or `default`)
- `JUDGE_K8S_SERVICE_ACCOUNT` (optional; service account used by spawned job pods)
- `JUDGE_K8S_CPU` (default: `1`)
- `JUDGE_K8S_POLL_INTERVAL_MS` (default: `350`)
- `JUDGE_K8S_JOB_TTL_SECONDS` (default: `120`)
- `JUDGE_K8S_KUBECTL_BIN` (default: `kubectl`)
- `JUDGE_K8S_KUBECONFIG` (optional local override; in-cluster uses service account config)

If you use `JUDGE_RUNNER=k8s`, build and push the worker image from `Dockerfile.judge`, then set `JUDGE_K8S_IMAGE` to that image.

## 10. Shared Droplet deployment

The current DigitalOcean deployment is documented in `droplet_deployment.md` and backed by the checked-in Compose assets under `infra/droplet/`.

- `infra/droplet/proxy/`: shared Nginx reverse proxy for subdomain routing
- `infra/droplet/explore/`: Explore web/worker Compose stack

On the shared Droplet, the recommended production split is:

- `web`: `APP_MODE=web`
- `worker`: `APP_MODE=worker`

The worker owns Docker-based judge execution and typically sets `JUDGE_WORKER_CONCURRENCY=4`, `JUDGE_DOCKER_CPUS=1`, `JUDGE_DOCKER_COMPILE_MEMORY_MB=512`, and `JUDGE_DOCKER_COMPILE_TIMEOUT_MS=60000` on the current `2 vCPU / 4 GB RAM` host.
