#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

DOMAIN="${1:?usage: issue_certificate.sh <domain> <email>}"
EMAIL="${2:?usage: issue_certificate.sh <domain> <email>}"

docker compose -f "${SCRIPT_DIR}/compose.yaml" run --rm certbot \
  certonly \
  --webroot \
  --webroot-path /var/www/certbot \
  --domain "${DOMAIN}" \
  --email "${EMAIL}" \
  --agree-tos \
  --no-eff-email

TLS_CONFIG="${SCRIPT_DIR}/templates/${DOMAIN}.https.conf"
ACTIVE_CONFIG="${SCRIPT_DIR}/conf.d/${DOMAIN}.conf"

if [ -f "${TLS_CONFIG}" ]; then
  cp "${TLS_CONFIG}" "${ACTIVE_CONFIG}"
fi

docker compose -f "${SCRIPT_DIR}/compose.yaml" exec -T nginx nginx -s reload
