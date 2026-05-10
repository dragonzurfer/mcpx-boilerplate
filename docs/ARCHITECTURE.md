# Explore Architecture

## Overview

Explore is a self-hosted newsletter + courses platform with:

- Markdown content with Vimeo embeds (sanitized HTML)
- Course catalog with structured metadata JSON, module/lesson hierarchy, and per-lesson free/paid access flags
- Practice problem library with admin-managed IO specs, constraints, datasets/testcases, editorials, official reference solutions, and curated problem lists
- Practice submissions with queued judge execution, results storage, and optional AI coaching
- Public / Trial / Paid access levels
- Engagement-based funnel scoring
- Promo decision engine (trial-only)
- Daily post analytics rollup (unique impressions + engagement aggregates) with same-day raw overlays for real-time admin views
- Manual Razorpay renewals (no auto-renew)
- SEO-first rendering (OG, JSON-LD, sitemap, RSS)
- Guided tools with per-user usage limits (Career Copilot) using Gemini for resume analysis, mentor responses, chat replies, and voice transcription, with focus-stage path previews across all focus/subfocus routes
- Tool analytics rollups (daily aggregation of tool events)

## Runtime modes

- `APP_MODE=all` keeps the original single-process behavior: HTTP server plus recurring jobs plus judge workers.
- `APP_MODE=web` starts the HTTP server only.
- `APP_MODE=worker` starts recurring jobs and judge workers only, which is the preferred production mode when the web and worker containers are split on the same host.

## Request flow

```
HTTP -> Logger/Recovery
     -> / (pages): SSR templates + static assets
     -> /api: OptionalAuth -> RateLimiter
        -> /api/auth/login
        -> /api/posts, /api/courses (public + optional auth listing)
        -> /api/problems, /api/problem-lists (public practice APIs)
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
            -> /api/admin/problem-lists (problem-list CRUD + list membership)
            -> /api/admin/problems/:id/publish
            -> /api/admin/problems/:id/datasets, /api/admin/datasets/:id/testcases
            -> /api/admin/problems/:id/solutions
            -> /api/admin/tools (tool gating + tracking settings)
            -> /api/admin/users (query + funnel-stage filtered member list)
            -> /api/admin/users/:id/activity (paginated events + promo interactions)
```

## Core services

- **ContentService**: fetch posts/courses, render sanitized Goldmark markdown with article-oriented extensions → HTML, enforce access gating.
- **EventService**: batch ingest events with anon/user IDs + daily unique post impressions.
- **FunnelService**: compute score + stage, periodic recalculation job.
- **PromoService**: decision engine (trial-only, caps/cooldowns).
- **PaymentService**: Razorpay order + webhook verification, entitlement updates.
- **ToolService**: tool usage gating (free limits, stage completion, event logging).
- **JudgeService**: claims queued submissions, runs tests, stores results, issues signed receipts.
- **DockerRunner**: default sandbox runner; compiles/runs Go/C/C++/Java submissions inside isolated Docker containers (STDIN-only, no network, cgroup memory/cpu limits, read-only root fs, pids cap, compile-memory floor, timeout kill + cleanup).
- **K8sJobRunner**: optional Kubernetes job-based runner that submits one Job per submission and reads structured worker output from pod logs.
- **LocalRunner**: local fallback runner for explicit dev-only usage (`JUDGE_RUNNER=local`).
- **AIAnalysisService**: on-demand coaching feedback with cached responses + receipt verification, using contextual prompts built from problem statement/editorial/reference solutions, user code, and judge failing-case evidence.
- **SEO**: meta builder, OG image generation, sitemap/robots/RSS.
- **Admin analytics UI**: dashboard pulls funnel + promo metrics from `/api/admin/analytics/*` and per-post analytics from `/api/admin/analytics/posts/*` (rollups + same-day raw overlay).

## Data model (high level)

- `users`, `oauth_identities`
- `posts`, `tags`, `post_tags`
- `problems`, `problem_lists`, `problem_list_problems`, `datasets`, `testcases`, `solutions`
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

- Promos render on TRIAL post surfaces and on dedicated course/practice top slots.
- PUBLIC post surfaces never show promos.
- PAID users never see promos.
- Stage derivation:
  - Active entitlement → `PAID_ACTIVE`
  - Expired entitlement → `PAID_EXPIRED`
  - Dormant threshold → `DORMANT`
  - Otherwise score thresholds.
- Default weights/stages are seeded automatically if none exist, and admins can override them in the UI.
- Funnel weights now include post/course/practice events (with canonical `post_*` names); legacy post weight keys are migrated automatically.
- Detailed behavior lives in `docs/SCORING.md` and `docs/PROMOS.md`.
- Post analytics rollups and retention live in `docs/POST_ANALYTICS.md`.
- Admin member drilldown now reads directly from existing `events`, `promo_decisions`, `promo_impressions`, and `promo_clicks` tables for paginated activity views.

## SEO

- Server-side templates with OG tags, Twitter cards, canonical, JSON-LD.
- `/robots.txt`, `/sitemap.xml` (PUBLIC posts only), `/rss.xml`.
- OG images served via `/og/post/:slug` and `/og/course/:slug`.

## Practice judge notes

- Judge workers poll the submission queue on a fixed interval.
- Each worker loop processes one submission at a time, and the number of loops per process is controlled by `JUDGE_WORKER_CONCURRENCY`.
- Kubernetes deploys default to `JUDGE_RUNNER=docker` and inject a Docker daemon sidecar (`judge-docker-daemon`) that accepts local-only `DOCKER_HOST=tcp://127.0.0.1:2375`.
- On GKE Autopilot, deploy defaults to remote Docker provisioning for judge runtime when DinD is enabled (unless `FORCE_AUTOPILOT_DIND=true`), because DinD layer unpack can fail under sandbox constraints.
- `deploy_gke.sh` supports remote Docker daemon wiring (`JUDGE_REMOTE_DOCKER_HOST`) and one-command VM provisioning (`ENABLE_JUDGE_REMOTE_DOCKER=true`) to keep Docker runner mode without in-pod DinD.
- Deploy now runs a post-rollout Docker judge preflight (`docker info`, image pull checks, smoke container run) and fails fast if runtime execution is unhealthy.
- Docker runner persists source/build artifacts in daemon-side named volumes per submission and runs containers with stdin attached (`docker run -i`), so compile/run works across local DinD and remote Docker hosts.
- The shared DigitalOcean Droplet deployment runs the worker in its own container (`APP_MODE=worker`) with the host Docker socket mounted only there, so the public web container does not get Docker access.
- For environments where DinD is not desired, `JUDGE_RUNNER=k8s` remains available as a job-based alternative.
- Runner supports STDIN mode only.
- Local unsandboxed runner is still available only when explicitly selected (`JUDGE_RUNNER=local`) for troubleshooting.
- Full judge runtime/deploy flow details: `docs/CODE_RUN.md`.
- Practice history UI uses `/api/users/:id/problems/:problem_id/history` plus `/api/submissions/:id` + `/api/submissions/:id/result` to expand results inline and reload historical code into the editor.
- Submission create additionally enforces one in-flight run/submit per user (`QUEUED`/`RUNNING`) and per-minute request caps (`RUN + SUBMIT`: free `5/min`, paid `20/min`) before queueing.
- Practice solved stats for the library UI are derived from accepted `SUBMIT` attempts only; accepted `RUN` attempts do not mark a problem solved.
- Free users are still usage-limited by persisted totals (`submissions` and `ai_analyses` counts); limit responses return payment-required so the frontend can open the pricing/paywall overlay.

## Deployment notes

- The current low-cost production target is a single DigitalOcean Droplet that runs a shared Nginx reverse proxy plus per-app Docker Compose stacks.
- Explore is deployed as two containers from the same image:
  - `web`: HTTP only
  - `worker`: recurring jobs + judge execution
- The detailed host layout and commands live in `droplet_deployment.md`.
