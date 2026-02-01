# web/docs.md

## Purpose

Server-rendered HTML templates + static assets for Explore.

## Structure

- `templates/`: SSR templates (layout + page content, including tools + account + admin pages and shared navigation, plus post analytics and admin tools controls).
- `templates/layout.html` injects CSS variables for theme (primary color + off-white) from site settings.
- Admin templates include inline helper copy for funnel and settings pages.
- Admin promos template includes field-level guidance and multi-variant support (add/remove variants).
- `assets/`: Tailwind CDN styles + vanilla JS app.
- Static legal pages: `privacy.html`, `terms.html`, etc.
- `tools.html` + `tool.html`: tools catalog and Career Copilot flow shell (progress bar + latest completion summary, recap cards shown after the initial plan, single active stage card for resume upload, mentor response, chat, voice input, plus loading indicators for resume analysis, plan generation, and chat response).
 - `admin_tools.html`: admin controls for tool gating + tracking.
