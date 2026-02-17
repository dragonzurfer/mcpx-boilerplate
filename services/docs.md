# services/docs.md

## Purpose

Business logic for Explore: content rendering, funnel scoring, promo decisions, payments, SEO utilities, and practice-judge orchestration (including submissions, results, and AI coaching).

## Files

- `content_service.go`: post/course access rules, teaser rendering, HTML caching.
- `content_service_test.go`: unit tests for gate decisions and teaser extraction.
- `markdown_renderer.go`: Markdown → HTML with Vimeo embeds + sanitization.
- `markdown_renderer_test.go`: embed + sanitization tests.
- `seo_builder.go`: deterministic meta builder (OG/Twitter/JSON-LD), with course description fallback support for both `description` and legacy `excerpt`.
- `seo_builder_test.go`: meta rules tests.
- `funnel_service.go`: scoring + stage derivation.
- `funnel_service_test.go`: scoring tests.
- `funnel_job.go`: periodic recalculation of user metrics.
- `promo_service.go`: promo decision engine (caps/cooldowns).
- `promo_service_test.go`: decision tests.
- `payment_service.go`: entitlement window rules.
- `payment_service_test.go`: entitlement logic tests.
- `tool_service.go`: tool usage gating (free limits, stage completion) + event logging.
- `gemini_service.go`: Gemini client for resume analysis, mentor response, markdown chat replies, and audio transcription, with resilient JSON extraction across multi-part/candidate model responses.
- `gemini_practice.go`: Gemini prompt execution + JSON parsing for practice analysis.
- `ai_analysis_service.go`: AI analysis orchestration with caching, receipts verification, and fallback summaries.
- `analysis_id.go`: analysis ID generation helpers.
- `hash.go`: SHA-256 hashing helper used for receipts + analysis caching.
- `judge_types.go`: core judge/result/runner structs shared across judge pipeline.
- `judge_service.go`: judge orchestrator (claims queue, runs tests, stores results, issues receipts).
- `runner_local.go`: local compiler/runner for Go/C/C++/Java using STDIN only.
- `limits.go`: resolves execution limits from problem constraints + per-language overrides.
- `validator.go`: output validators (exact/JSON/unordered/float-tolerance).
- `receipt.go`: signed receipt generation + verification.

## Notes

- All backend services follow TDD and small-function readability rules.
- Promo decisions are **trial-only** and never shown to paid users.
- Funnel jobs fall back to default weights/stages (sourced from store defaults) when no admin overrides exist.
- Post analytics rollups run daily (see `docs/POST_ANALYTICS.md`).
- Practice judge currently executes STDIN-mode problems only and runs locally (no sandbox). Hidden test details are hashed before persistence.
