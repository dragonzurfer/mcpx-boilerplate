# routes/docs.md

## Purpose

HTTP handlers for public, authenticated, and admin APIs plus server-rendered pages.

## Files

- `auth.go`: Google ID token exchange → JWT with first-login phone gating. Users without a saved phone receive `phone_required=true` and a short-lived `phone_token`; `/api/auth/complete-phone` validates country + national number and only then issues the app JWT. `/api/auth/phone-countries` exposes supported region/dial-code options for the client selector.
- `config.go`: client config (site + auth + Razorpay key id).
- `health.go`: liveness endpoint (`/healthz`).
- `me.go`: `/api/me` profile + stage + entitlement, now including stored phone metadata (`phone_country_code`, `phone_e164`) for authenticated session hydration.
- `posts.go`: public post listing + detail (gated by access level).
- `courses.go`: public course listing plus signed-in course detail/lesson APIs (module + lesson structure with per-lesson lock checks).
- `problems.go`: public practice APIs for published problems, problem detail, and list-based problem library payloads (including per-user solved flags plus solved/total difficulty stats when authenticated; solved is derived from accepted `SUBMIT` attempts only); problem detail now returns `official_solutions` from `problems.solutions_json` so the editorial tab can render reference implementations with language metadata.
- `submissions.go`: submission create/status/result/history endpoints for practice runs and submits; detail responses include `code_text` so the practice editor can restore historical code on row selection. Submission create now enforces one in-flight run/submit per user (`QUEUED`/`RUNNING`) plus per-minute limits (`RUN + SUBMIT` combined: free `5/min`, paid `20/min`) while keeping the existing free lifetime cap.
- `ai_analysis.go`: AI analysis endpoints (`/ai-analysis`, `/ai-analysis/verify`) for coaching feedback, with free-user usage caps enforced before analysis execution and verified payload checks tied to owned submissions.
- `tools.go`: tools catalog + per-tool action tracking (signed-in required for actions), plus resume upload + mentor/chat + voice transcription endpoints.
- `admin_tools.go`: tool admin settings (limits, gating, tracking).
- `events.go`: event batch ingest.
- `promos.go`: promo decision + impression/click logging across post/course/practice contexts (`entity_type` + `entity_id`, with legacy `post_id` compatibility for post surfaces).
- `payments.go`: Razorpay order creation, confirm, webhook.
- `seo.go`: robots, sitemap, RSS, OG images (`/og/post/:slug`, `/og/course/:slug`).
- `pages.go`: SSR templates for home/post/pricing/tools/courses/practice + practice detail + account and admin pages (including post analytics, admin courses, the dedicated admin lesson editor page at `/admin/courses/lesson-editor`, and admin problems pages).
- `admin_posts.go`: admin CRUD for posts (tag de-duplication to avoid duplicate post_tags).
- `admin_courses.go`: admin CRUD for courses, metadata, modules, lessons, and module/lesson reordering.
- `admin_problems.go`: admin CRUD for practice problems, including IO spec + constraints JSON and editorial content.
- `admin_problem_lists.go`: admin CRUD for practice problem lists and list membership ordering.
- `admin_problem_datasets.go`: admin dataset/testcase CRUD plus publish endpoint.
- `admin_problem_solutions.go`: admin CRUD for official solutions table.
- `admin_funnel.go`: funnel config, weights, stages. Config responses are normalized to snake_case keys (`scoring_window_days`, `decay_enabled`, `daily_decay_factor`, `dormant_days_threshold`) for frontend form hydration.
- `admin_promos.go`: promos + variants with validation, duplicate-code handling, and promo detail fetch (includes variants).
- `admin_analytics.go`: basic funnel/promo/content analytics.
- `admin_post_analytics.go`: per-post analytics (summary, timeseries, funnel, promo breakdown).
- `admin_users.go`: user list + detail for admin UI, including funnel-stage filtering on list queries and paginated user activity APIs (`/users/:id/activity`) that read from existing `events` and promo interaction tables.
- `admin_settings.go`: site settings.
