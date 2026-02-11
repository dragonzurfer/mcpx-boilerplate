# middleware/docs.md

## Purpose

Shared HTTP middleware for authentication, admin gating, and rate limiting.

## Files

- `auth.go`: JWT verification + optional auth for public routes.
- `admin.go`: role-based admin guard.
- `courses.go`: signed-in guard, entitlement lookup, and per-lesson access checks for course APIs (paid lessons return a locked payload when entitlement is missing).
- `ratelimit.go`: global and per-group rate limits.
- `apikey.go`: legacy API key helper (unused in Explore routes).
