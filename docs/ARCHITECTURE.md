# Explore Architecture

## Overview

Explore is a self-hosted newsletter + courses platform with:

- Markdown content with Vimeo embeds (sanitized HTML)
- Course catalog with structured metadata JSON, module/lesson hierarchy, and per-lesson free/paid access flags
- Practice problem library with admin-managed IO specs, constraints, datasets/testcases, and editorials
- Practice submissions with queued judge execution, results storage, and optional AI coaching
- Public / Trial / Paid access levels
- Engagement-based funnel scoring
- Promo decision engine (trial-only)
- Daily post analytics rollup (unique impressions + engagement aggregates) with same-day raw overlays for real-time admin views
- Manual Razorpay renewals (no auto-renew)
- SEO-first rendering (OG, JSON-LD, sitemap, RSS)
- Guided tools with per-user usage limits (Career Copilot) using Gemini for resume analysis, mentor responses, chat replies, and voice transcription, with focus-stage path previews across all focus/subfocus routes
- Tool analytics rollups (daily aggregation of tool events)

## Request flow

```
HTTP -> Logger/Recovery
     -> / (pages): SSR templates + static assets
     -> /api: OptionalAuth -> RateLimiter
        -> /api/auth/login
        -> /api/posts, /api/courses (public + optional auth listing)
        -> /api/problems (public practice list)
        -> /api/tools, /api/tools/:slug (public + optional auth)
        -> /api/events/batch, /api/promos/decide
        -> /api/plans (public)
        -> /api/payments/webhook (public, signed)
        -> /api/* (authed): Auth -> RateLimiter
            -> /api/me
            -> /api/courses/:slug, /api/courses/:slug/lessons/:lessonSlug
            -> /api/submissions, /api/submissions/:id, /api/submissions/:id/result
            -> /api/users/:id/problems/:problem_id/history
            -> /api/ai-analysis, /api/ai-analysis/verify
            -> /api/tools/:slug/action
            -> /api/tools/:slug/resume, /mentor, /chat, /transcribe
            -> /api/payments/create-order, /confirm
        -> /api/admin/*: Auth -> RequireAdminRole
            -> /api/admin/courses (course metadata + module/lesson CRUD/reorder)
            -> /api/admin/problems (problem CRUD with JSON specs)
            -> /api/admin/problems/:id/publish
            -> /api/admin/problems/:id/datasets, /api/admin/datasets/:id/testcases
            -> /api/admin/problems/:id/solutions
            -> /api/admin/tools (tool gating + tracking settings)
```

## Core services

- **ContentService**: fetch posts/courses, render markdown → HTML, enforce access gating.
- **EventService**: batch ingest events with anon/user IDs + daily unique post impressions.
- **FunnelService**: compute score + stage, periodic recalculation job.
- **PromoService**: decision engine (trial-only, caps/cooldowns).
- **PaymentService**: Razorpay order + webhook verification, entitlement updates.
- **ToolService**: tool usage gating (free limits, stage completion, event logging).
- **JudgeService**: claims queued submissions, runs tests, stores results, issues signed receipts.
- **LocalRunner**: compiles/runs Go/C/C++/Java submissions locally (STDIN-only, no sandbox).
- **AIAnalysisService**: on-demand coaching feedback with cached responses + receipt verification.
- **SEO**: meta builder, OG image generation, sitemap/robots/RSS.
- **Admin analytics UI**: dashboard pulls funnel + promo metrics from `/api/admin/analytics/*` and per-post analytics from `/api/admin/analytics/posts/*` (rollups + same-day raw overlay).

## Data model (high level)

- `users`, `oauth_identities`
- `posts`, `tags`, `post_tags`
- `problems`, `datasets`, `testcases`, `solutions`
- `submissions`, `submission_results`, `ai_analyses`
- `courses` (includes `metadata_json`, `thumbnail_url`, `description`), `course_modules`, `course_lessons` (`is_free` for lesson gating)
- `events`, `post_impressions`, `post_daily_metrics`, `post_promo_daily_metrics`, `user_metrics`
- `funnel_config`, `funnel_event_weights`, `funnel_stage_thresholds`
- `promos`, `promo_variants`, `promo_decisions`, `promo_impressions`, `promo_clicks`
- `tools`, `tool_usages`, `tool_events`
- `tool_daily_metrics`
- `payments`, `entitlements`
- `site_settings`, `admin_audit_logs`

## Database connectivity

- `DB_DSN` is read from environment and normalized before GORM initializes MySQL.
- DSN normalization enforces `parseTime=true` and `interpolateParams=true`, with managed-MySQL TLS defaults when not explicitly configured.
- Connection pooling is configured at startup (`max open/idle: 25`, idle timeout: 2m, lifetime: 30m) to keep query latency stable under concurrent load.
- A real-DB opt-in comparison test exists at `stores/store_latency_test.go` to compare `mysql` CLI query timing with GORM timing using the same `DB_DSN`.

## Payment flow (manual renewal)

1. Frontend requests `/api/payments/create-order` for plan.
2. Razorpay order is created; payment record saved.
3. Frontend completes checkout; `/api/payments/confirm` verifies signature.
4. Webhook also marks payments as paid (idempotent).
5. Entitlements are created or extended from current end date.

## Funnel + promo rules

- Promos **only** on TRIAL posts.
- PUBLIC posts never show promos.
- PAID users never see promos.
- Stage derivation:
  - Active entitlement → `PAID_ACTIVE`
  - Expired entitlement → `PAID_EXPIRED`
  - Dormant threshold → `DORMANT`
  - Otherwise score thresholds.
- Default weights/stages are seeded automatically if none exist, and admins can override them in the UI.
- Detailed behavior lives in `docs/SCORING.md` and `docs/PROMOS.md`.
- Post analytics rollups and retention live in `docs/POST_ANALYTICS.md`.

## SEO

- Server-side templates with OG tags, Twitter cards, canonical, JSON-LD.
- `/robots.txt`, `/sitemap.xml` (PUBLIC posts only), `/rss.xml`.
- OG images served via `/og/post/:slug` and `/og/course/:slug`.

## Practice judge notes

- Judge worker polls the submission queue on a fixed interval and processes one submission at a time.
- Current runner supports STDIN mode only and executes locally without sandbox isolation; plan for a hardened sandbox runner before untrusted traffic.
