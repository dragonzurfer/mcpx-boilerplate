# docs.md

## Purpose

HTTP route handlers and registration for the Gin API.

## Files

- `admin.go`: admin-only endpoints (IP rule CRUD).
- `auth.go`: login + token issuance (JWT) endpoints.
- `billing_handler.go`: billing routes for plans, checkout, verify, usage, and "me".
- `billing_razorpay.go`: Razorpay checkout and webhook integration.
- `billing_googleplay.go`: Google Play verification and RTDN webhook handling.
- `billing_helpers.go`: shared helpers for billing responses and env parsing.
- `config.go`: public config endpoint for client bootstrap.
- `example.go`: sample metered endpoint wiring.
