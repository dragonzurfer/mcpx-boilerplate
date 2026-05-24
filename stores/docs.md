# stores/docs.md

## Purpose

GORM models and persistence helpers for Explore.

## Key models

- `users`, `oauth_identities` (users now persist phone fields: `phone_country_code`, `phone_national_number`, `phone_e164`)
- `posts`, `tags`, `post_tags`
- `problems`
- `problem_lists`, `problem_list_problems`
- `datasets`, `testcases`, `solutions`
- `submissions`, `submission_results`, `ai_analyses`
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
- User phone persistence helper (`UpdateUserPhone`) for post-OAuth profile completion (country code + national number + E.164 normalization).
- Admin user listing supports optional funnel-stage filtering (via `user_metrics.stage`) and paginated list metadata.
- Admin user activity queries expose paginated events plus merged promo decisions/impressions/clicks from existing tables (no new service layer required).
- Post/course CRUD + HTML cache update
- Problem CRUD with JSON statement, IO spec, constraints, tags, and editorial helpers
- Problem list CRUD + list-membership ordering helpers (with slug normalization and assignment validation)
- Dataset/testcase CRUD with execution policy + validator defaults
- Testcase listing orders by `group`, `position`, and `id` with escaped SQL for the reserved `group` column in MySQL.
- Submission queueing (claim next) + result persistence
- Submission count helpers for user-level quota checks, including active in-flight submission counts (`QUEUED`/`RUNNING`) and time-window counts (`queued_at >= since`) for per-minute throttling
- Solved-problem derivation for a user now requires an accepted `SUBMIT` (`mode=SUBMIT` + `AC` verdict in submission results); accepted `RUN` attempts do not mark the problem solved.
- AI analysis caching (fingerprint lookup + response storage)
- AI analysis count helpers for user-level quota checks
- Course metadata JSON helpers (`ParseCourseMetadata` / `SerializeCourseMetadata`) and course/module/lesson CRUD + reorder helpers (metadata module/lesson counts are synced automatically when structure changes)
- Event batch ingest, anon merge, and daily post impression de-dup (supports post/course/practice event families for funnel scoring)
- Funnel config + metrics upsert (weights/stages auto-seeded on read, with legacy post weight keys migrated to canonical `post_*` rows)
- Funnel config/weight/stage updates refresh cache entries immediately so admin reads reflect saved values without waiting for TTL expiry.
- Promo metrics + impression/click logging (now stores optional `entity_type` + `entity_id` for post/course/practice promo surfaces, while keeping `post_id` for post analytics joins)
- Post analytics rollup (daily aggregation + retention cleanup) plus same-day raw overlays for real-time admin views
- Tool catalog seeding, per-user usage state, and tool event logging
- Tool daily rollups and tool-event retention cleanup
- Payment/entitlement upsert and expiry
- `NewStoreWithDB` helper for tests.

## Verification tests

- `store_dsn_test.go` validates DSN normalization defaults used by GORM.
- `store_latency_test.go` adds an opt-in real-DB comparison test (`mysql` CLI vs GORM): set `RUN_DB_LATENCY_TEST=1` and `DB_DSN`.
