# web/templates/docs.md

## Purpose

Server-rendered HTML templates for Explore, including shared partials and the Career Copilot tool shell.

## Files

- `layout.html`: base layout, Medium-inspired Libre Franklin + Source Serif font loading, white/black/green Tailwind theme tokens, Explore `E` favicon links, Motion runtime (`motion@latest/dist/motion.js`), Marked + DOMPurify Markdown libraries, and page-wide script loading.
- `home.html`: home page shell with newsletters hero, topic chips, feed loader/grid, and a sign-in CTA button (`#cta-login`) wired by `web/assets/app.js`.
- `tool.html`: Career Copilot UI shell (hero, dossier progress header, completion chips, recap container, stage cards for upload/analysis/focus/subfocus/questions/mentor/chat). Analysis stage uses ATS score cards + score ring; focus stage adds a hero + inline status chips + selection panel plus an "All guided paths" map that previews every focus->subfocus route; subfocus stage uses a centered header + two-column selection grid; chat stage uses a centered mentorship header with a card-style chat shell + composer (chat log expands with content), a live voice waveform, and a transcribing loader, stacked under the dossier recap in a single-column layout. The mentor response stage is kept for markup consistency but the flow jumps directly to chat once a plan is generated. IDs + `data-stage` attributes are required by `web/assets/app.js`.
- `post.html`: post detail page shell with a centered Medium-style reader column for headline, metadata, and markdown body; the markdown body no longer uses `max-w-none` so global reader width rules stay authoritative.
- `tools.html`: tools catalog listing with a dedicated loading state (`tools-loader`) and a hidden grid (`tools-grid`) revealed after data loads.
- `courses.html`: course catalog page with loading state (`courses-loader`) and card grid (`courses-grid`).
- `course.html`: course learning layout with three reading rails: left roadmap (`course-modules-sidebar`) rendered as a modern accordion module list, wide centered lesson content viewer (`course-body`), and right sticky heading index (`course-toc` inside `course-toc-shell`), plus a top promo slot container (`course-promo-slot`) rendered above lesson metadata; the page-level reader shell runs near full viewport width so side whitespace is minimized while keeping centered prose inside the lesson card.
- `practice.html`: practice library page that renders list cards (`problem-lists`) instead of a flat grid, with a Figma-aligned global search field (`practice-list-search`) and a sticky right-side stats panel (`practice-stats`) for solved totals by difficulty and tag.
- `practice_problem.html`: practice problem detail view with description/editorial/submissions tabs, a promo slot container above the problem title (`problem-promo-slot`), Figma-aligned split workspace spacing, sticky right coding pane on desktop with page-level scrolling, a light/dark theme toggle for reading + coding, CodeMirror shell that expands when the results panel is hidden, run/submit controls, results summary, and an AI action button beside the console title; the editorial tab includes a dedicated official-solutions container (`problem-editorial-solutions`) and loads Highlight.js assets so fenced code blocks render with language-aware syntax colors, while submission history uses row selection (highlight + inline expansion under the selected row) instead of a separate "View" action and AI actions are shown only for non-accepted verdicts.
- `pricing.html`: pricing page with a compact two-panel layout so value messaging and plan CTAs stay visible in one desktop view, a yearly recommendation callout, a dedicated loading state (`pricing-loader`), and a hidden plans grid (`pricing-cards`) revealed after plans load.
- `admin_tools.html`: admin controls for tool gating + tracking.
- `admin_courses.html`: admin course builder for structured metadata, modules, and lesson editing/reordering.
- `admin_problems.html`: admin problem manager for CRUD on practice problems with form-based IO spec/constraints/examples/solutions, plus dataset/testcase tooling and a list-builder section for creating/editing practice lists with searchable problem assignment.
- `admin_users.html`: admin member overview with search + funnel-stage filter, paginated user list controls, and a detail panel that renders user profile/metrics plus paginated event and promo activity timelines.
- `partials/`: shared navigation and admin nav fragments (see `web/templates/partials/docs.md` for the mobile hamburger menu details).

## Notes

- Tool-specific icons are rendered via the Lucide script in `tool.html`; dynamic icon injection is refreshed in `web/assets/app.js`.
- Keep element IDs stable for client-side state hydration and stage transitions.
