# services/docs.md

## Purpose

Business logic for Explore: content rendering, funnel scoring, promo decisions, payments, and SEO utilities.

## Files

- `content_service.go`: post/course access rules, teaser rendering, HTML caching.
- `content_service_test.go`: unit tests for gate decisions and teaser extraction.
- `markdown_renderer.go`: Markdown → HTML with Vimeo embeds + sanitization.
- `markdown_renderer_test.go`: embed + sanitization tests.
- `seo_builder.go`: deterministic meta builder (OG/Twitter/JSON-LD).
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

## Notes

- All backend services follow TDD and small-function readability rules.
- Promo decisions are **trial-only** and never shown to paid users.
- Funnel jobs fall back to default weights/stages (sourced from store defaults) when no admin overrides exist.
- Post analytics rollups run daily (see `docs/POST_ANALYTICS.md`).
