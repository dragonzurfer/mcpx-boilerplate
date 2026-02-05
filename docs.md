# docs.md

## Purpose

Explore is a self-hosted newsletter + courses platform with manual Razorpay renewals, engagement-driven funnels (with default weights/stages that can be overridden in admin), a tools hub (Career Copilot), an admin analytics dashboard, and SEO-first rendering.

## Key folders

- `routes/`: HTTP handlers and route registration (public, authed, admin).
- `services/`: business logic (content rendering, funnel scoring, promo decisions, payments).
- `stores/`: GORM models, persistence helpers, MySQL DSN normalization/performance tuning, and DB latency verification tests.
- `payments/`: plan configuration and pricing helpers.
- `middleware/`: auth, admin guard, rate limiting.
- `web/`: HTML templates and static assets (Tailwind browser runtime `@tailwindcss/browser@4` + vanilla JS), including the Career Copilot dossier-style tool UI with an all-path guided-selection map, resilient client-side loading states for home/tools/pricing feeds, tolerant home-feed response parsing, parallel `/api/me` + page-data frontend bootstrap, centered post reading layout, and pricing value/savings positioning.
- `docs/`: architecture and setup documentation.
- `k8s/`, `scripts/`, `infra/`: deployment helpers and infra tooling.

## Docs entrypoints

- `docs/ARCHITECTURE.md`
- `docs/SETUP.md`
- `docs/SCORING.md`
- `docs/PROMOS.md`
- `docs/POST_ANALYTICS.md`
- `docs/TOOLS.md`
