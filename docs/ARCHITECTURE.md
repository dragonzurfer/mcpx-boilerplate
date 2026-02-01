# Explore Architecture

## Overview

Explore is a self-hosted newsletter + courses platform with:

- Markdown content with Vimeo embeds (sanitized HTML)
- Public / Trial / Paid access levels
- Engagement-based funnel scoring
- Promo decision engine (trial-only)
- Daily post analytics rollup (unique impressions + engagement aggregates)
- Manual Razorpay renewals (no auto-renew)
- SEO-first rendering (OG, JSON-LD, sitemap, RSS)
- Guided tools with per-user usage limits (Career Copilot) using Gemini for resume analysis, mentor responses, chat replies, and voice transcription
- Tool analytics rollups (daily aggregation of tool events)

## Request flow

```
HTTP -> Logger/Recovery
     -> / (pages): SSR templates + static assets
     -> /api: OptionalAuth -> RateLimiter
        -> /api/auth/login
        -> /api/posts, /api/courses (public + optional auth)
        -> /api/tools, /api/tools/:slug (public + optional auth)
        -> /api/events/batch, /api/promos/decide
        -> /api/plans (public)
        -> /api/payments/webhook (public, signed)
        -> /api/* (authed): Auth -> RateLimiter
            -> /api/me
            -> /api/tools/:slug/action
            -> /api/tools/:slug/resume, /mentor, /chat, /transcribe
            -> /api/payments/create-order, /confirm
        -> /api/admin/*: Auth -> RequireAdminRole
            -> /api/admin/tools (tool gating + tracking settings)
```

## Core services

- **ContentService**: fetch posts/courses, render markdown → HTML, enforce access gating.
- **EventService**: batch ingest events with anon/user IDs + daily unique post impressions.
- **FunnelService**: compute score + stage, periodic recalculation job.
- **PromoService**: decision engine (trial-only, caps/cooldowns).
- **PaymentService**: Razorpay order + webhook verification, entitlement updates.
- **ToolService**: tool usage gating (free limits, stage completion, event logging).
- **SEO**: meta builder, OG image generation, sitemap/robots/RSS.
- **Admin analytics UI**: dashboard pulls funnel + promo metrics from `/api/admin/analytics/*` and per-post analytics from `/api/admin/analytics/posts/*`.

## Data model (high level)

- `users`, `oauth_identities`
- `posts`, `tags`, `post_tags`
- `courses`, `course_modules`, `course_lessons`
- `events`, `post_impressions`, `post_daily_metrics`, `post_promo_daily_metrics`, `user_metrics`
- `funnel_config`, `funnel_event_weights`, `funnel_stage_thresholds`
- `promos`, `promo_variants`, `promo_decisions`, `promo_impressions`, `promo_clicks`
- `tools`, `tool_usages`, `tool_events`
- `tool_daily_metrics`
- `payments`, `entitlements`
- `site_settings`, `admin_audit_logs`

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
