# Deploy Android Billing (Google Play)

This boilerplate includes a React Native (Expo) Android app in `mobile/` and backend support for Google Play subscription verification + RTDN webhook.

## 1) Play Console setup

1. Play Console → **Monetize** → **Products** → **Subscriptions**
2. Create subscription products (example: `app_pro_monthly`)
3. Add test accounts under **Settings → License testing**

## 2) Backend setup (Google Play Developer API)

1. Play Console → **Setup → API access**
2. Link a Google Cloud project
3. Create a service account with Play Console subscription access
4. Provide credentials to the backend using one of:
   - `GOOGLE_PLAY_SERVICE_ACCOUNT_FILE=/path/to/key.json`
   - `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON={...}`

Required backend env vars:

- `GOOGLE_PLAY_PACKAGE_NAME` (Android package name)
- `GOOGLE_PLAY_PUBSUB_TOKEN` (secret for RTDN webhook auth)
- Add your product IDs to `config/plans.json` under `googlePlayProductIds`

## 3) RTDN (Real‑time Developer Notifications)

RTDN keeps subscription status synced on renewals/cancellations/refunds.

1. Create a Pub/Sub topic in the linked GCP project
2. Configure RTDN in Play Console to publish to that topic
3. Create a Pub/Sub **push** subscription pointing at:
   - `https://YOUR_DOMAIN/api/billing/webhook/googleplay?token=YOUR_GOOGLE_PLAY_PUBSUB_TOKEN`

## 4) Build + run on Android

1. Update `mobile/src/config.ts` (API + OAuth client IDs)
2. Install + run:

```bash
cd mobile
npm install
npm run prebuild
npm run android
```

Notes:
- For real billing tests, install via Play **internal testing** (USB debug installs often fail purchases).
- Backend verification uses `POST /api/billing/googleplay/verify`.
