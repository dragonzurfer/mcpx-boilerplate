# 1) Create a temporary pull subscription
gcloud pubsub subscriptions create app-debug-sub \
  --topic=projects/YOUR_PROJECT_ID/topics/YOUR_TOPIC \
  --project=YOUR_PROJECT_ID

# 2) Trigger a new event (subscribe/cancel or Play “Test notification”)

# 3) Pull 1 message
gcloud pubsub subscriptions pull app-debug-sub \
  --project=YOUR_PROJECT_ID \
  --limit=1 --auto-ack --format=json > /tmp/rtdn.json

# 4) Decode the RTDN payload (base64)
python3 - <<'PY'
import json,base64
msg=json.load(open("/tmp/rtdn.json"))[0]["message"]["data"]
print(base64.b64decode(msg).decode())
PY

# 5) Clean up
gcloud pubsub subscriptions delete app-debug-sub --project=YOUR_PROJECT_ID
