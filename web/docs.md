# web/docs.md

## Purpose

Server-rendered HTML templates + static assets for Explore.

## Structure

- `templates/`: SSR templates (layout + page content, including tools + courses + practice + account + admin pages and shared navigation with a mobile hamburger menu, plus post analytics, admin tools controls, admin course builder, and admin problem manager; post detail uses a centered reading column while course detail uses a module sidebar + lesson viewer).
- `templates/layout.html` injects CSS variables for theme (primary color + off-white) from site settings.
- Admin templates include inline helper copy for funnel and settings pages.
- Admin promos template includes field-level guidance and multi-variant support (add/remove variants).
- `assets/`: Tailwind browser runtime styles (`@tailwindcss/browser@4`) + vanilla JS app, including One Tap prompts triggered by nav/CTA/login overlay buttons with a popup fallback when the inline chooser is suppressed, resilient loader visibility toggles that use class + native `hidden` + inline `display: none !important` fallback for home/courses/practice/tools/pricing, home-feed parsing that accepts common API envelope variants, parallel frontend bootstrap (`/api/me` hydration + page fetches), signed-in course explorer hydration (single expanded module + lesson lock handling), pricing-plan benefit/savings messaging, and a full focus-path explorer in Career Copilot guided selection.
- Static legal pages: `privacy.html`, `terms.html`, etc.
- `tools.html` + `tool.html`: tools catalog and Career Copilot flow shell (hero + tag chips, dossier-style progress header, completion chips, recap cards after the initial plan, single active stage card for resume upload, ATS analysis, focus, subfocus, questions, mentor response, and chat + voice input, plus loading indicators for tool-list fetch, resume analysis, plan generation, chat response, and voice transcription; focus stage includes a hero + inline status chips + selection panel + all-path preview map; chat stage stacks the recap content and mentorship shell in a single column with dossier-specific card styling, live waveform feedback during voice input, and hidden usage-limit text; the mentor response stage is auto-completed and the UI jumps directly to chat once the plan is generated).
 - `admin_tools.html`: admin controls for tool gating + tracking.
