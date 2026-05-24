# payments/docs.md

## Purpose

Static plan configuration for manual Razorpay renewals.

## Notes

- Plans are loaded from `config/plans.json` or `PLANS_JSON`.
- Manual renewal only: each plan purchase extends entitlements by `entitlementDays`.
- Supported billing intervals include monthly, quarterly (90 days), and yearly.
