import json
import os
import sys

try:
    from dotenv import load_dotenv
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    load_dotenv(os.path.join(root, ".env"))
except Exception:
    pass

from google.oauth2 import service_account
from googleapiclient.discovery import build


def must(name: str) -> str:
    value = os.getenv(name, "").strip()
    if not value:
        print(f"Missing {name}")
        sys.exit(1)
    return value


def main() -> int:
    package_name = must("GOOGLE_PLAY_PACKAGE_NAME")
    product_id = must("GOOGLE_PLAY_PRODUCT_ID")
    purchase_token = must("GOOGLE_PLAY_PURCHASE_TOKEN")

    sa_json = os.getenv("GOOGLE_PLAY_SERVICE_ACCOUNT_JSON", "").strip()
    sa_file = os.getenv("GOOGLE_PLAY_SERVICE_ACCOUNT_FILE", "").strip()

    scopes = ["https://www.googleapis.com/auth/androidpublisher"]
    if sa_json:
        try:
            info = json.loads(sa_json)
        except json.JSONDecodeError as exc:
            print(f"Invalid GOOGLE_PLAY_SERVICE_ACCOUNT_JSON: {exc}")
            return 1
        creds = service_account.Credentials.from_service_account_info(info, scopes=scopes)
    elif sa_file:
        if not os.path.exists(sa_file):
            print(f"Service account file not found: {sa_file}")
            return 1
        creds = service_account.Credentials.from_service_account_file(sa_file, scopes=scopes)
    else:
        print("Missing GOOGLE_PLAY_SERVICE_ACCOUNT_JSON or GOOGLE_PLAY_SERVICE_ACCOUNT_FILE")
        return 1

    print("Using package:", package_name)
    print("Using product:", product_id)
    print("Purchase token length:", len(purchase_token))

    service = build("androidpublisher", "v3", credentials=creds, cache_discovery=False)
    purchase = service.purchases().subscriptions().get(
        packageName=package_name,
        subscriptionId=product_id,
        token=purchase_token,
    ).execute()

    print(json.dumps(purchase, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
