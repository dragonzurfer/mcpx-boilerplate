# Scoring System (Funnel)

## Purpose

The scoring system turns user behavior into a **funnel stage** so promos and analytics can react to intent. It is driven by **events**, **weights**, and **stage thresholds**, all configurable in the admin funnel page.

## Where scoring happens

- **Score + stage** are computed in `services/funnel_service.go` and persisted by the periodic funnel job in `services/funnel_job.go`.
- The job writes to `user_metrics` (score, stage, last_active_at) via `Store.UpsertUserMetrics`.
- Event ingestion (`/api/events/batch`) **only stores events**. It does not update score immediately.

## Core formula

For each user, within the scoring window:

```
score = sum( weight(event_type) * decay_multiplier(event_time) )
```

- If decay is disabled, `decay_multiplier = 1` for all events.
- If decay is enabled:

```
multiplier = daily_decay_factor ^ days_ago
```

Days are calculated as whole days between `event_time` and `now`.

## Event types and how they fire

These are the default events used by the system:

- `post_open`: 1x per post open
- `scroll_depth`: fires multiple times per post (25/50/75/90% milestones)
- `time_on_page`: fires at 15s, 45s, 90s
- `post_complete`: fired after scroll + time milestones are both met
- `promo_click`: when a promo CTA is clicked
- `paywall_hit`: when a locked post is attempted

> Note: `scroll_depth` and `time_on_page` are **multi‑fire** events, so their weights apply per milestone.

## Admin settings and how they affect the score

### 1) Scoring window (days)
- Only events within the last **N days** are counted.
- Example: If set to 14, only events from the last 14 days contribute.

### 2) Decay enabled + daily decay factor
- If enabled, older events contribute **less**.
- Example: `daily_decay_factor = 0.9`
  - Event from 1 day ago: `weight * 0.9`
  - Event from 3 days ago: `weight * 0.9^3`

### 3) Event weights
- Each event type has a numeric weight (positive integer).
- Higher weight = stronger contribution to total score.
- Disabled events contribute `0`.

### 4) Stage thresholds
- Stages are defined as **score ranges**. Example defaults:
  - NEW: 0–5
  - CASUAL: 6–15
  - ENGAGED: 16–35
  - HOT: 36–60
- If `max_score` is empty, the stage is **open‑ended**.

### 5) Dormant threshold
- If the last activity is older than this threshold, the user becomes `DORMANT` regardless of score.

### 6) Entitlements override stages
- If the user has an **active entitlement**, stage is `PAID_ACTIVE`.
- If they have an **expired entitlement**, stage is `PAID_EXPIRED`.

## Example: score calculation

Assume:
- Scoring window = 14 days
- Decay disabled
- Weights: `post_open=1`, `scroll_depth=2`, `time_on_page=2`, `post_complete=5`

User activity on a single post:
- `post_open` (1x) → +1
- `scroll_depth` fired 4 times (25/50/75/90) → +8
- `time_on_page` fired 3 times (15/45/90) → +6
- `post_complete` (1x) → +5

Total score from this post = **20**

If thresholds are 16–35 for ENGAGED, the user becomes **ENGAGED**.

## Example: decay effect

If decay is enabled at 0.9 and the event is 5 days old:

```
multiplier = 0.9^5 = 0.59049
```

A `post_complete` weight of 5 would contribute:

```
5 * 0.59049 = 2.95
```

## Where to look in code

- Score + stage computation: `services/funnel_service.go`
- Scheduled updates: `services/funnel_job.go`
- Admin config endpoints: `routes/admin_funnel.go`
- Persistence: `stores/funnel_store.go`

