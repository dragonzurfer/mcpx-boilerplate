# docs.md

## Purpose

Explore is a self-hosted newsletter + courses platform with manual Razorpay renewals, engagement-driven funnels (with default weights/stages that can be overridden in admin), a tools hub (Career Copilot), an admin analytics dashboard, and SEO-first rendering.

## Key folders

- `routes/`: HTTP handlers and route registration (public, authed, admin).
- `services/`: business logic (content rendering, funnel scoring, promo decisions, payments).
- `stores/`: GORM models and persistence helpers.
- `payments/`: plan configuration and pricing helpers.
- `middleware/`: auth, admin guard, rate limiting.
- `web/`: HTML templates and static assets (Tailwind CDN + vanilla JS), including the Career Copilot dossier-style tool UI.
- `docs/`: architecture and setup documentation.
- `k8s/`, `scripts/`, `infra/`: deployment helpers and infra tooling.

## Docs entrypoints

- `docs/ARCHITECTURE.md`
- `docs/SETUP.md`
- `docs/SCORING.md`
- `docs/PROMOS.md`
- `docs/POST_ANALYTICS.md`
- `docs/TOOLS.md`
