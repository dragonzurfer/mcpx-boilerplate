# web/templates/partials/docs.md

## Purpose

Shared navigation partials for Explore pages and admin pages.

## Files

- `nav.html`: primary site navigation bar (public pages, including `Courses` + `Practice` links) with a Google sign-in button (`#nav-login`) and a mobile hamburger toggle (`#nav-mobile-toggle` + `#nav-mobile-panel`) wired by `web/assets/app.js`.
- `admin_nav.html`: admin navigation bar (admin pages, including `Courses` + `Problems` management).

## Notes

- Partials are included by higher-level templates in `web/templates/`.
