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
- `tools.html` + `tool.html`: tools catalog and Career Copilot flow shell (hero + tag chips, dossier-style progress header, completion chips, recap cards after the initial plan, single active stage card for resume upload, ATS analysis, focus, subfocus, questions, mentor response, and chat + voice input, plus loading indicators for resume analysis, plan generation, chat response, and voice transcription; focus stage includes a hero + inline status chips + selection panel; chat stage stacks the recap content and mentorship shell in a single column with dossier-specific card styling, live waveform feedback during voice input, and hidden usage-limit text; the mentor response stage is auto-completed and the UI jumps directly to chat once the plan is generated).
 - `admin_tools.html`: admin controls for tool gating + tracking.
