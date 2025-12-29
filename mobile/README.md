# Android App (Expo)

This is a React Native (Expo) wrapper around the existing web UI (`/web`) using a WebView, plus:

- Native Google sign-in (triggered from the web auth modal)
- Google Play subscriptions (triggered from the web paywall modal)

## 0) Prereqs

- Node.js (>= 18)
- Android Studio + Android SDK + an Android device with USB debugging enabled
- A deployed backend URL (or run the Go server on your LAN)

## 1) Configure

1. Edit `mobile/src/config.ts`
2. Backend env:
   - Set `GOOGLE_CLIENT_ID` for the web UI
   - Optional: set `GOOGLE_CLIENT_IDS` if you also want to accept non-web audiences
   - Set Google Play env vars (see `sample.env`)

## 2) Run on your Android phone (dev)

```bash
cd mobile
npm install
npm run prebuild
npm run android
```

If you want the app to point at a local backend on your LAN, set `WEBAPP_URL` and `API_BASE_URL` in `mobile/src/config.ts` to your machine IP (example: `http://192.168.1.20:8080`).

## 3) Play Store subscriptions setup

- Create a subscription product (example: `app_pro_monthly`)
- Map the plan code to the product ID in `mobile/src/config.ts`
- Upload an internal test build (billing works only for Play-installed builds)

More details: `DEPLOY_PLAY_STORE.md`
