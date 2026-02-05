# stores/docs.md

## Purpose

GORM models and persistence helpers for Explore.

## Key models

- `users`, `oauth_identities`
- `posts`, `tags`, `post_tags`
- `courses`, `course_modules`, `course_lessons`
- `events`
- `post_impressions`, `post_daily_metrics`, `post_promo_daily_metrics`
- `user_metrics`, `funnel_config`, `funnel_event_weights`, `funnel_stage_thresholds`
- `promos`, `promo_variants`, `promo_decisions`, `promo_impressions`, `promo_clicks`
- `tools`, `tool_usages`, `tool_events`, `tool_daily_metrics`
- `payments`, `entitlements`
- `site_settings`, `admin_audit_logs`

## Store helpers

- MySQL DSN normalization (`normalizeDSN`) now enforces `parseTime=true`, `interpolateParams=true`, and safe TLS defaults for managed DBs before GORM opens the connection.
- MySQL connection pool tuning is applied during `NewStore` (`max open/idle`, idle timeout, lifetime) to reduce connection churn.
- User lookup + upsert with OAuth identity
- Post/course CRUD + HTML cache update
- Event batch ingest, anon merge, and daily post impression de-dup
- Funnel config + metrics upsert (weights/stages auto-seeded with defaults on first read)
- Promo metrics + impression/click logging
- Post analytics rollup (daily aggregation + retention cleanup)
- Tool catalog seeding, per-user usage state, and tool event logging
- Tool daily rollups and tool-event retention cleanup
- Payment/entitlement upsert and expiry
- `NewStoreWithDB` helper for tests.

## Verification tests

- `store_dsn_test.go` validates DSN normalization defaults used by GORM.
- `store_latency_test.go` adds an opt-in real-DB comparison test (`mysql` CLI vs GORM): set `RUN_DB_LATENCY_TEST=1` and `DB_DSN`.
