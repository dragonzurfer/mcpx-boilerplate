# services/docs.md

## Purpose

Business logic for Explore: content rendering, funnel scoring, promo decisions, payments, SEO utilities, and practice-judge orchestration (including submissions, results, and AI coaching).

## Files

- `content_service.go`: post/course access rules, teaser rendering, HTML caching.
- `content_service_test.go`: unit tests for gate decisions and teaser extraction.
- `markdown_renderer.go`: GFM Markdown → sanitized HTML with Vimeo embeds.
- `markdown_renderer_test.go`: embed, GFM, and sanitization tests.
- `seo_builder.go`: deterministic meta builder (OG/Twitter/JSON-LD), with course description fallback support for both `description` and legacy `excerpt`.
- `seo_builder_test.go`: meta rules tests.
- `funnel_service.go`: scoring + stage derivation (canonicalizes legacy post event names and includes post/course/practice activity events).
- `funnel_service_test.go`: scoring tests.
- `funnel_job.go`: periodic recalculation of user metrics.
- `promo_service.go`: promo decision engine (caps/cooldowns) with per-surface eligibility via `PromosAllowed` (post/course/practice entry points).
- `promo_service_test.go`: decision tests.
- `payment_service.go`: entitlement window rules.
- `payment_service_test.go`: entitlement logic tests.
- `tool_service.go`: tool usage gating (free limits, stage completion) + event logging.
- `gemini_service.go`: Gemini client for resume analysis, mentor response, markdown chat replies, and audio transcription, with resilient JSON extraction across multi-part/candidate model responses.
- `gemini_practice.go`: Gemini prompt execution + JSON parsing for practice analysis.
- `ai_analysis_service.go`: AI analysis orchestration with caching and receipts verification, now building rich coaching context from problem statement, constraints, editorial, official/reference solutions, user code (plus line-numbered/code-signal views), and first failing testcase evidence before prompting Gemini.
- `analysis_id.go`: analysis ID generation helpers.
- `hash.go`: SHA-256 hashing helper used for receipts + analysis caching.
- `judge_types.go`: core judge/result/runner structs shared across judge pipeline.
- `judge_service.go`: judge orchestrator (claims queue, runs tests, stores results, issues receipts).
- `judge_worker_payload.go`: shared payload/result envelope for orchestrator ↔ judge worker communication.
- `runner_k8s.go`: optional Kubernetes Job runner that launches per-submission worker pods (`/app/judge-worker`) via `kubectl`, then reads structured pod logs back into canonical runner output.
- `runner_k8s_test.go`: unit tests for Kubernetes runner timeout/memory helpers and worker-result parsing.
- `runner_docker.go`: sandboxed Docker runner for Go/C/C++/Java using STDIN only (containers run with `docker run -i` so testcase input reaches the process), with per-run container isolation (no network, read-only filesystem, cgroup memory/cpu limits, pids cap, tmpfs `/tmp`, timeout kill + forced container cleanup), daemon-managed per-submission Docker volumes (so compile/test works with local DinD and remote `DOCKER_HOST`), compiler cache/temp files redirected onto the writable `/workspace` volume to avoid tmpfs space exhaustion, language-specific runtime image selection, a separate compile-memory floor (`512 MB` by default, configurable via `JUDGE_DOCKER_COMPILE_MEMORY_MB`), and a default compile-time budget of 4x runtime limit (10s..120s).
- `runner_docker_test.go`: unit tests for Docker runner argument/resource resolution helpers.
- `runner_local.go`: local compiler/runner fallback (unsandboxed) for development-only use when explicitly selected.
- `limits.go`: resolves execution limits from problem constraints + per-language overrides.
- `validator.go`: output validators (exact/JSON/unordered/float-tolerance).
- `receipt.go`: signed receipt generation + verification.

## Notes

- All backend services follow TDD and small-function readability rules.
- Promo decisions are **trial-only** and never shown to paid users.
- Funnel jobs fall back to default weights/stages (sourced from store defaults) when no admin overrides exist, and normalize weight keys through canonical event naming.
- Post analytics rollups run daily (see `docs/POST_ANALYTICS.md`).
- Practice judge executes STDIN-mode problems only. Default runner is Docker sandbox (`JUDGE_RUNNER=docker`), Kubernetes Job runner remains opt-in via `JUDGE_RUNNER=k8s`, and local runner is opt-in via `JUDGE_RUNNER=local`. Hidden test details are hashed before persistence.
- Production Docker deployments are expected to run judge work from a dedicated `APP_MODE=worker` process, with queue parallelism controlled by `JUDGE_WORKER_CONCURRENCY`.
