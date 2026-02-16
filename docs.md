# docs.md

## Purpose

Explore is a self-hosted newsletter + courses platform with manual Razorpay renewals, engagement-driven funnels (with default weights/stages that can be overridden in admin), structured course metadata + module/lesson management, a practice problem library, a tools hub (Career Copilot), an admin analytics dashboard, and SEO-first rendering.

## Key folders

- `routes/`: HTTP handlers and route registration (public, authed, admin), including course lesson-lock APIs, practice problem listing, and admin course/problem CRUD.
- `services/`: business logic (content rendering, funnel scoring, promo decisions, payments).
- `stores/`: GORM models, persistence helpers (including course metadata JSON + module/lesson ordering), MySQL DSN normalization/performance tuning, and DB latency verification tests.
- `payments/`: plan configuration and pricing helpers.
- `middleware/`: auth, admin guard, rate limiting, and course entitlement/lesson access middleware.
- `web/`: HTML templates and static assets (Tailwind browser runtime `@tailwindcss/browser@4` + vanilla JS), including One Tap prompts with a popup fallback for Google sign-in, a mobile hamburger menu for primary navigation, the Career Copilot dossier-style tool UI with an all-path guided-selection map, course catalog cards + sidebar lesson explorer, practice problem catalog, admin course/problem builders, resilient client-side loading states for home/courses/practice/tools/pricing feeds (class + native `hidden` + inline display fallback), tolerant home-feed response parsing, parallel `/api/me` + page-data frontend bootstrap, centered post reading layout, and pricing value/savings positioning.
- `docs/`: architecture and setup documentation.
- `k8s/`, `scripts/`, `infra/`: deployment helpers and infra tooling.

## Docs entrypoints

- `docs/ARCHITECTURE.md`
- `docs/SETUP.md`
- `docs/SCORING.md`
- `docs/PROMOS.md`
- `docs/POST_ANALYTICS.md`
- `docs/TOOLS.md`
