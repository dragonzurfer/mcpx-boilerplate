# docs.md

## Purpose

Explore is a self-hosted newsletter + courses platform with manual Razorpay renewals, engagement-driven funnels (with default weights/stages that can be overridden in admin) across post/course/practice events, promo delivery across post/course/practice surfaces, structured course metadata + module/lesson management, a practice problem library with submissions + judge results, a tools hub (Career Copilot), an admin analytics dashboard, and SEO-first rendering.

## Key folders

- `routes/`: HTTP handlers and route registration (public, authed, admin), including practice problems (public detail now includes `official_solutions` for editorial rendering), list-based problem-library APIs with solved stats, submissions/results (including submission detail code payload for history restore and submission guardrails: one in-flight job per user + per-minute limits), admin course/problem + problem-list CRUD, and admin user APIs for stage-filtered search + paginated activity inspection.
- `services/`: business logic (content rendering with sanitized Goldmark Markdown, GFM, typographer, definition-list, and footnote extensions, funnel scoring, promo decisions, payments, practice judge orchestration with Docker-sandboxed code execution by default, explicit stdin attachment for testcase delivery, compile-time memory flooring separate from runtime limits, explicit local fallback, and AI coaching prompts grounded in problem/editorial/code/failing-test context).
- `stores/`: GORM models, persistence helpers (including practice problem lists, datasets/testcases/submissions/results with ordered testcase retrieval, solved-problem derivation, and submission usage-count helpers for free-tier + per-minute throttles), MySQL DSN normalization/performance tuning, and DB latency verification tests.
- `payments/`: plan configuration and pricing helpers.
- `middleware/`: auth, admin guard, rate limiting, and course entitlement/lesson access middleware.
- `cmd/`: standalone binaries for background/runtime helpers (including the Kubernetes judge worker process).
- `web/`: HTML templates and static assets (Tailwind browser runtime `@tailwindcss/browser@4` + vanilla JS), including a Medium-inspired white/black/green visual system, serif article typography, centered post/course reader shells, Marked + DOMPurify client Markdown rendering for browser-rendered markdown, One Tap prompts with a popup fallback for Google sign-in, a mobile hamburger menu for primary navigation, the Career Copilot dossier-style tool UI with an all-path guided-selection map, course catalog cards + three-column lesson reader (left roadmap, wide center article, right heading index/TOC), a list-first practice library with per-list search + solved markers + sticky stats, and a sticky right-side code workspace on practice detail pages (using page-level scrolling and no extra column scrollbars) with a per-user light/dark toggle, markdown rendering on practice statement/editorial/examples/constraints (headings, lists, quotes, links, fenced code language parsing with Highlight.js), official-solution cards in the editorial tab, admin course/problem builders plus practice list builder controls, admin user filters/pagination/activity panels, resilient client-side loading states for home/courses/practice/tools/pricing feeds (class + native `hidden` + inline display fallback), tolerant home-feed response parsing, parallel `/api/me` + page-data frontend bootstrap, centered post reading layout, and pricing value/savings positioning.
- `docs/`: architecture and setup documentation.
- `k8s/`, `scripts/`, `infra/`: deployment helpers and infra tooling (including the legacy GKE path plus the single-Droplet Docker+Nginx deployment assets under `infra/droplet/`).

## Docs entrypoints

- `docs/ARCHITECTURE.md`
- `docs/CODE_RUN.md`
- `docs/SETUP.md`
- `docs/SCORING.md`
- `docs/PROMOS.md`
- `docs/POST_ANALYTICS.md`
- `docs/TOOLS.md`
- `droplet_deployment.md`
