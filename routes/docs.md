# routes/docs.md

## Purpose

HTTP handlers for public, authenticated, and admin APIs plus server-rendered pages.

## Files

- `auth.go`: Google ID token exchange → JWT.
- `config.go`: client config (site + auth + Razorpay key id).
- `health.go`: liveness endpoint (`/healthz`).
- `me.go`: `/api/me` profile + stage + entitlement.
- `posts.go`: public post listing + detail (gated by access level).
- `courses.go`: public course listing plus signed-in course detail/lesson APIs (module + lesson structure with per-lesson lock checks).
- `tools.go`: tools catalog + per-tool action tracking (signed-in required for actions), plus resume upload + mentor/chat + voice transcription endpoints.
- `admin_tools.go`: tool admin settings (limits, gating, tracking).
- `events.go`: event batch ingest.
- `promos.go`: promo decision + impression/click logging.
- `payments.go`: Razorpay order creation, confirm, webhook.
- `seo.go`: robots, sitemap, RSS, OG images (`/og/post/:slug`, `/og/course/:slug`).
- `pages.go`: SSR templates for home/post/pricing/tools/courses + account and admin pages (including post analytics and admin courses pages).
- `admin_posts.go`: admin CRUD for posts (tag de-duplication to avoid duplicate post_tags).
- `admin_courses.go`: admin CRUD for courses, metadata, modules, lessons, and module/lesson reordering.
- `admin_funnel.go`: funnel config, weights, stages.
- `admin_promos.go`: promos + variants with validation, duplicate-code handling, and promo detail fetch (includes variants).
- `admin_analytics.go`: basic funnel/promo/content analytics.
- `admin_post_analytics.go`: per-post analytics (summary, timeseries, funnel, promo breakdown).
- `admin_users.go`: user list + detail (normalized JSON fields for admin UI).
- `admin_settings.go`: site settings.
