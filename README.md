# Mcpx Boilerplate

A reusable Gin + Go starter with payments, usage metering, auth, and GKE deployment built-in.

## Prerequisites

See [docs/SETUP.md](docs/SETUP.md) for installation instructions for required tools (Go, gcloud, kubectl, Codex CLI, Gemini CLI, etc.).

## Quick start

```bash
cp sample.env .env
# update .env + config/plans.json

go mod tidy

go run .
```

Open `http://localhost:8080` to view the paywall demo UI.

## What’s included

- JWT + API key auth (`routes/auth.go`, `middleware/auth.go`)
- Rate limiting + IP allow/deny (`middleware/ratelimit.go`, `middleware/ipfilter.go`)
- Razorpay + Google Play billing (`routes/billing_handler.go`, `routes/billing_razorpay.go`, `routes/billing_googleplay.go`)
- Usage metering + quota enforcement (`middleware/metering.go`)
- Admin IP rules (`routes/admin.go`)
- GKE deployment scripts + multi-subdomain support (`scripts/`)

## Docs

- `docs/ARCHITECTURE.md`
- `DEPLOY_GKE.md`
- `DEPLOY_PLAY_STORE.md`
