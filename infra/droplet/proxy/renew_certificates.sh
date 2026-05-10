#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

docker compose -f "${SCRIPT_DIR}/compose.yaml" run --rm certbot \
  renew \
  --webroot \
  --webroot-path /var/www/certbot \
  --quiet

docker compose -f "${SCRIPT_DIR}/compose.yaml" exec -T nginx nginx -s reload
