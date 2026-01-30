# Explore Platform

A self-hosted newsletter + course platform with engagement-based funnels, promo decisions, and manual Razorpay renewals.

## Highlights

- Markdown posts and courses with Vimeo embeds
- Public / Trial / Paid access levels with strict promo rules
- Admin-configurable funnel scoring and promo system
- Manual renewal payments (Razorpay orders + webhook)
- SEO-first rendering (OG tags, JSON-LD, sitemap, robots, RSS)

## Quick start

```bash
cp sample.env .env
# update .env with DB + Razorpay + Google client IDs

go mod tidy

go run .
```

Visit `http://localhost:8080`.

## Docs

- `docs/ARCHITECTURE.md`
- `docs/SETUP.md`
