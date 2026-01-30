# middleware/docs.md

## Purpose

Shared HTTP middleware for authentication, admin gating, and rate limiting.

## Files

- `auth.go`: JWT verification + optional auth for public routes.
- `admin.go`: role-based admin guard.
- `ratelimit.go`: global and per-group rate limits.
- `apikey.go`: legacy API key helper (unused in Explore routes).
