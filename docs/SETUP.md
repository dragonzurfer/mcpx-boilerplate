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
