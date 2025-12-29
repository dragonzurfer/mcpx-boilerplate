# docs.md

## Purpose

Reusable Gin middleware for auth, rate limiting, IP filtering, and usage metering.

## Files

- `admin.go`: admin token guard for privileged routes.
- `apikey.go`: API key parsing and context injection.
- `auth.go`: JWT auth and current user loader.
- `ipfilter.go`: IP allow/deny rules with CIDR support.
- `metering.go`: usage quota enforcement and counters.
- `metering_test.go`: unit tests for metering behavior.
- `ratelimit.go`: rate limiter for API routes.
