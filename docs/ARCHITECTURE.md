# Boilerplate Architecture

## Overview

This boilerplate is a Gin + Go backend with:

- JWT + API key auth
- Global and per-route rate limiting
- IP allow/deny filtering with optional admin-managed rules
- Razorpay (web) + Google Play (Android) payment flows
- Usage metering + quota enforcement per plan
- GKE deployment scripts for multi-subdomain hosting

## Request flow + middleware layering

```
HTTP -> Logger/Recovery -> IPFilter
     -> /api group: OptionalAPIKey -> RateLimiter
       -> /api/auth/* (public)
       -> /api/billing/* (public + webhooks)
       -> /api/* (authed): Auth -> RateLimiter
         -> /api/* (metered): Metered -> handlers
       -> /api/admin/* (admin): RequireAdminToken -> admin handlers
```

Key points:

- `middleware.IPFilter` blocks requests based on `IP_ALLOWLIST`/`IP_DENYLIST` and optional DB rules.
- `middleware.RateLimiter` is applied globally at `/api`, with stricter limits on authed routes.
- `middleware.Auth` validates JWTs (HMAC) and loads users from the DB.
- `middleware.OptionalAPIKey` sets API key context for usage metering on service endpoints.
- `middleware.Metered` enforces plan quotas and writes usage counters per request.

## Auth strategy

- JWT: issued via `/api/auth/login` (Google ID token), verified with `JWT_SECRET` and `JWT_ISSUER`.
- API key: `X-API-Key` header. Configure via `API_KEYS` (comma-separated).
- Admin: `X-Admin-Token` header. Configure via `ADMIN_TOKEN`.

## Payments + Paywall

### Plans

Plans live in `config/plans.json` (or `PLANS_JSON`). Each plan defines:

- `code`, `name`, `type` (`free`, `one_time`, `subscription`)
- `interval` (`monthly`/`yearly`)
- `priceInr`
- `quotas` per metric (e.g. `api_calls`)
- `razorpayPlanId` for subscription plans
- `googlePlayProductIds` for Android subscriptions

### Razorpay (web)

- `POST /api/billing/checkout`: creates an order (one-time) or subscription (monthly).
- `POST /api/billing/verify`: verifies Razorpay signature and activates entitlements.
- `POST /api/billing/webhook/razorpay`: updates entitlements on payment/subscription events.

### Google Play (Android)

- `POST /api/billing/googleplay/verify`: verifies purchase token via Play API.
- `POST /api/billing/webhook/googleplay`: RTDN webhook to keep status in sync.

## Usage metering + paywall enforcement

Usage is tracked in `usage_counters` with:

- `project_key` (app slug)
- `subject_type` (`user`, `api_key`, `ip`)
- `subject_id` (user ID, API key hash, or IP)
- `metric` (e.g. `api_calls`)
- `period_start` (monthly)

`middleware.Metered`:

1. Resolves subject (user → API key → IP).
2. Loads the active plan.
3. Checks quota for the metric.
4. Blocks with HTTP 402 if exceeded.
5. Increments usage after successful responses.

For client-side events (e.g. front-end metering), use:

- `POST /api/billing/usage` with `{ metric, delta }`.

## IP allow/deny rules

Two sources are combined:

- Env lists: `IP_ALLOWLIST`, `IP_DENYLIST` (CIDR or IP, comma-separated).
- DB rules: stored in `ip_rules` via admin endpoints:
  - `GET /api/admin/ip-rules`
  - `POST /api/admin/ip-rules`
  - `DELETE /api/admin/ip-rules/:id`

If allowlist is empty, all IPs are allowed unless explicitly denied.

## Routing conventions

- `routes/auth.go`: login + token issuance
- `routes/billing_handler.go`: billing/plans/checkout/verify/usage handlers
- `routes/billing_razorpay.go`: Razorpay checkout + webhook wiring
- `routes/billing_helpers.go`: shared billing helpers
- `routes/billing_googleplay.go`: Play verification + RTDN
- `routes/admin.go`: IP rules admin
- `routes/example.go`: sample metered endpoint

## Deployment + multi-project hosting

Scripts:

- `scripts/deploy_gke.sh`: full deploy to an existing cluster + DNS + static IP
- `scripts/deploy_gke_code.sh`: fast deploy (image only)

Key variables per app:

- `SUBDOMAIN`: `script` → `script.mcpx.in`
- `APP_NAME`: Kubernetes Deployment/Service name
- `NAMESPACE`: per-app namespace (defaults to `APP_NAME`)

Ingress flow:

- Global static IP (`gcloud compute addresses`)
- Cloud DNS A record (optional) → static IP
- GKE Ingress + ManagedCertificate for TLS

Nodepool targeting (standard clusters only):

- `NODEPOOL_NAME` adds a `nodeSelector` to the Deployment
- `CREATE_NODEPOOL=true` will create the nodepool if missing

## Quick bootstrap checklist

1. Copy the repo.
2. Set `APP_NAME`, `APP_KEY`, `SUBDOMAIN`.
3. Update `config/plans.json` (plan codes + quotas + product IDs).
4. Update `.env` (DB, JWT, Razorpay, Play credentials).
5. Deploy with `scripts/deploy_gke.sh`.
