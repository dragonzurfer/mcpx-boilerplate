# web/assets/docs.md

## Purpose

Client-side behavior for Explore (landing, post reader, pricing, checkout).

## Files

- `app.js`: routing-aware UI logic, auth (GIS; FedCM disabled on localhost), nav state toggling (login/admin links) using locally cached user/entitlement to avoid UI flicker, and post-payment `/api/me` refresh to update cached entitlements. Includes engagement tracking (incl. `post_complete`), promo rendering (inline/bottom/completion modal), Razorpay checkout, account + admin flows rendering, funnel UI normalization (snake/camel-case API fields), multi-variant promo editor UI, and admin form validation/error display (with auto-scroll to errors). Signed-in users load PUBLIC+TRIAL+PAID listings for newsletters/courses and search; admin post analytics UI renders summary, funnel, trends, and promo tables.
- `styles.css`: typography + markdown styles on top of Tailwind CDN, light theme background + off-white surface styling.
