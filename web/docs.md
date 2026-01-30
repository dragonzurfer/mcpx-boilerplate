# web/docs.md

## Purpose

Server-rendered HTML templates + static assets for Explore.

## Structure

- `templates/`: SSR templates (layout + page content, including account + admin pages and shared navigation, plus post analytics).
- `templates/layout.html` injects CSS variables for theme (primary color + off-white) from site settings.
- Admin templates include inline helper copy for funnel and settings pages.
- Admin promos template includes field-level guidance and multi-variant support (add/remove variants).
- `assets/`: Tailwind CDN styles + vanilla JS app.
- Static legal pages: `privacy.html`, `terms.html`, etc.
