# Promos System

## Purpose

Promos are the conversion layer. They are selected based on funnel stage, caps, and cooldowns. Paid users never see promos.

## Hard rules

- Promos can appear on:
  - `TRIAL` post surfaces (`INLINE`, `BOTTOM_CARD`, `MODAL_ON_COMPLETE`)
  - course detail top slot (`COURSE_TOP`)
  - practice problem top slot (`PRACTICE_TOP`)
- Post promos do not appear on `PUBLIC` posts.
- Promos never appear for `PAID_ACTIVE` users.
- Paywalls are separate from promos (paywall is shown only when content is locked).

## How a promo is chosen (summary)

1. The frontend requests a decision with `/api/promos/decide?slot=...&entity_type=...&entity_id=...`.
   - Legacy post query (`post_id`) is still accepted for backward compatibility.
2. The backend filters promos by:
   - status = ACTIVE
   - slot matches
   - within start/end window
   - user stage in eligible stages
   - caps and cooldown rules
3. Highest priority wins; ties break randomly.
4. A variant is selected (weighted random).
5. Impression/click are logged when the UI renders or clicks.

## Admin promo fields (what they mean)

### Core fields

- **Code** (required, unique):
  - Identifier used in logs and analytics.
  - Example: `ENGAGED_TRIAL_INLINE`

- **Name** (optional but recommended):
  - Internal label for humans.
  - Example: `Engaged trial CTA`

- **Slot** (required):
  - Where the promo appears in the UI.
  - Values:
    - `INLINE` (mid‑article)
    - `BOTTOM_CARD` (end of article)
    - `MODAL_ON_COMPLETE` (modal after completion)
    - `PAYWALL_CARD` (locked content UI)
    - `COURSE_TOP` (course detail, above lesson content)
    - `PRACTICE_TOP` (practice problem detail, above title)

- **Status**:
  - `ACTIVE` → eligible for delivery
  - `PAUSED` → ignored by decision engine

- **Priority** (integer):
  - Higher wins when multiple promos are eligible in the same slot.
  - Example: `100` beats `10`.

### Eligibility and frequency

- **Eligible stages** (comma separated):
  - Funnel stages allowed for this promo.
  - Example: `ENGAGED,HOT`

- **Cooldown hours**:
  - If the user clicked this promo, block it for N hours.

- **Max impressions / day**:
  - Per user limit per day.

- **Max clicks / day**:
  - Per user limit per day.

- **Start date / End date**:
  - Optional active window for timed campaigns.

### Variant fields (multi-variant supported)

> The admin UI supports **multiple variants** per promo. Add/remove variants to run A/B tests via weights.

- **Headline**: top line text shown on the promo.
- **Body**: short value statement.
- **CTA text**: button label.
- **CTA action**:
  - `OPEN_PRICING` → go to pricing page
  - `START_CHECKOUT` → open Razorpay checkout
  - `OPEN_SAMPLE_POST` → take user to sample post
  - `OPEN_ACCOUNT_RENEW` → route to account renewal
- **CTA payload JSON** (optional):
  - Only required for `START_CHECKOUT` to preselect a plan.
  - Example: `{"plan_default":"monthly"}`
- **Variant weight**:
  - Used when multiple variants exist (higher weight = more likely).

## Example user flows

### Flow 1: Engaged trial reader
1. User reads TRIAL posts and reaches ENGAGED stage.
2. On next TRIAL post, UI requests `INLINE` promo.
3. Promo decision returns an ENGAGED inline CTA.
4. User clicks CTA → pricing page or checkout.

### Flow 2: Hot reader completion modal
1. User reaches HOT stage.
2. User completes a TRIAL post (scroll + time).
3. UI requests `MODAL_ON_COMPLETE` promo.
4. Modal appears with yearly CTA.

### Flow 3: Course/practice conversion touchpoint
1. User opens a course page or a practice problem.
2. UI requests `COURSE_TOP` or `PRACTICE_TOP`.
3. If user stage is eligible and caps/cooldowns allow, promo renders above lesson/title.
4. CTA leads to pricing/checkout.

### Flow 4: Paid expired renewal
1. User’s entitlement expires → stage `PAID_EXPIRED`.
2. On TRIAL post, promo decision returns renewal CTA.
3. CTA leads to account renewal or pricing.

## Where to look in code

- Decision engine: `services/promo_service.go`
- Admin APIs: `routes/admin_promos.go`
- Decision endpoints: `routes/promos.go`
- Storage: `stores/promos_store.go`, `stores/promos.go`
