# Post Analytics (Daily Rollups)

## Purpose

Provide accurate, low-noise analytics per post without storing unlimited raw events.
The system tracks **unique impressions per day** and aggregates engagement + promo activity
into daily rollup tables. Raw rows older than the retention window are deleted.

## Key definitions

- **Unique impression (per day)**: one viewer per post per UTC day.
  - Viewer = `user_id` when logged in, else `anon_id`.
- **Total views**: count of `post_open` events (raw).
- **Completes**: count of `post_complete` events.
- **Scroll milestones**: counts for `scroll_depth` events at 25/50/75/90.
- **Time milestones**: counts for `time_on_page` events at 15/45/90 seconds.
- **Promo metrics**: impressions/clicks tied to the post (via promo impression/click rows).

## Data model

- `post_impressions`: daily unique impressions (deduped at ingestion).
  - Unique indexes:
    - `(post_id, user_id, day_date)` and `(post_id, anon_id, day_date)`
- `post_daily_metrics`: per-post daily aggregates.
- `post_promo_daily_metrics`: per-post + promo + variant daily aggregates.

## Ingestion flow

1. Frontend sends events via `/api/events/batch`.
2. Backend stores events in `events`.
3. If event is `post_open` for a `POST`, backend **upserts** a row in `post_impressions`
   (dedup by day + viewer).

## Rollup job (daily)

Runs every 24 hours:

1. Compute daily aggregates for the previous day:
   - `events` → `post_daily_metrics` (views, scroll, time, completes)
   - `post_impressions` → unique impressions
   - `promo_impressions` / `promo_clicks` → post promo metrics
2. Upsert rollups (idempotent).
3. Cleanup raw rows older than retention window:
   - `events`, `post_impressions`, `promo_impressions`, `promo_clicks`

## Real-time overlay (today)

Admin post analytics endpoints read from rollups **plus** same-day raw rows when the
requested range includes the current UTC day:

- `events` (post_open, post_complete, scroll_depth, time_on_page)
- `post_impressions`
- `promo_impressions`, `promo_clicks`

If a rollup row already exists for today, the raw overlay is skipped to avoid double counting.
Caching is bypassed for ranges that include today.

## Retention

Retention is derived from the **funnel scoring window**:

```
retention_days = max(scoring_window_days, 14) + 7
```

This keeps enough raw data to compute scores while limiting storage growth.
All analytics are served from rollup tables once raw rows are deleted.

## Admin analytics APIs

Per-post endpoints:

- `GET /api/admin/analytics/posts/:id/summary?from=&to=`
- `GET /api/admin/analytics/posts/:id/timeseries?from=&to=`
- `GET /api/admin/analytics/posts/:id/funnel?from=&to=`
- `GET /api/admin/analytics/posts/:id/promos?from=&to=`

Date format: `YYYY-MM-DD` (UTC, inclusive range).

## UI representation

Admin → Posts → **Analytics**:

- KPI cards: unique impressions, total views, completion rate, promo CTR
- Daily trend table (unique + completes)
- Funnel bars (impressions → scroll 50% → scroll 75% → completes → promo clicks)
- Promo breakdown table (promo + variant + CTR)
