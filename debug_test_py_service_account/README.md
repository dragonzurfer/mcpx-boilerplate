# Play purchase check (service account)

This script verifies a Google Play subscription purchase using the same
service account credentials your backend uses.

## 1) Create a venv

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

## 2) Set env vars

These come from your repo `.env` plus two extra values:

- `GOOGLE_PLAY_PACKAGE_NAME` (already in `.env`)
- `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON` or `GOOGLE_PLAY_SERVICE_ACCOUNT_FILE` (already in `.env`)
- `GOOGLE_PLAY_PRODUCT_ID` (the subscription ID, e.g. `app_pro_monthly`)
- `GOOGLE_PLAY_PURCHASE_TOKEN` (from RTDN or app)

Example:

```bash
export GOOGLE_PLAY_PRODUCT_ID=app_pro_monthly
export GOOGLE_PLAY_PURCHASE_TOKEN=PASTE_PURCHASE_TOKEN
```

## 3) Run

```bash
python3 check_play_purchase.py
```

If you get `permissionDenied`, the Play Console service account access is still missing.
