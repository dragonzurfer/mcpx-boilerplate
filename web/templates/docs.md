# web/templates/docs.md

## Purpose

Server-rendered HTML templates for Explore, including shared partials and the Career Copilot tool shell.

## Files

- `layout.html`: base layout, fonts, Tailwind browser runtime (`@tailwindcss/browser@4`) with `@import "tailwindcss"` + `@theme` tokens, Motion runtime (`motion@latest/dist/motion.js`), and page-wide script loading.
- `home.html`: home page shell with newsletters hero, topic chips, feed loader/grid, and a sign-in CTA button (`#cta-login`) wired by `web/assets/app.js`.
- `tool.html`: Career Copilot UI shell (hero, dossier progress header, completion chips, recap container, stage cards for upload/analysis/focus/subfocus/questions/mentor/chat). Analysis stage uses ATS score cards + score ring; focus stage adds a hero + inline status chips + selection panel plus an "All guided paths" map that previews every focus->subfocus route; subfocus stage uses a centered header + two-column selection grid; chat stage uses a centered mentorship header with a card-style chat shell + composer (chat log expands with content), a live voice waveform, and a transcribing loader, stacked under the dossier recap in a single-column layout. The mentor response stage is kept for markup consistency but the flow jumps directly to chat once a plan is generated. IDs + `data-stage` attributes are required by `web/assets/app.js`.
- `post.html`: post detail page shell with a centered reading column (`max-w-3xl mx-auto`) for headline, metadata, and markdown body.
- `tools.html`: tools catalog listing with a dedicated loading state (`tools-loader`) and a hidden grid (`tools-grid`) revealed after data loads.
- `courses.html`: course catalog page with loading state (`courses-loader`) and card grid (`courses-grid`).
- `course.html`: course learning layout with sidebar roadmap (`course-modules-sidebar`) and lesson content viewer (`course-body`).
- `pricing.html`: pricing page with a dedicated loading state (`pricing-loader`), a hidden plans grid (`pricing-cards`) revealed after plans load, and value-focused copy that explains monthly vs yearly use cases.
- `admin_tools.html`: admin controls for tool gating + tracking.
- `admin_courses.html`: admin course builder for structured metadata, modules, and lesson editing/reordering.
- `partials/`: shared navigation and admin nav fragments (see `web/templates/partials/docs.md` for the mobile hamburger menu details).

## Notes

- Tool-specific icons are rendered via the Lucide script in `tool.html`; dynamic icon injection is refreshed in `web/assets/app.js`.
- Keep element IDs stable for client-side state hydration and stage transitions.
