# docs.md

## Purpose

Shared reverse-proxy assets for the DigitalOcean Droplet.

## Files

- `compose.yaml`: runs the shared Nginx container and the ad-hoc Certbot service.
- `conf.d/explore.mcpx.in.conf`: host-based Nginx server block that proxies `explore.mcpx.in` to the Explore web container on the shared `edge` network.
- `templates/explore.mcpx.in.https.conf`: HTTPS-ready server block template that is activated after the first certificate is issued.
- `issue_certificate.sh`: obtains a Let's Encrypt certificate with the webroot challenge and reloads Nginx.
- `renew_certificates.sh`: renews existing certificates and reloads Nginx.
- `certbot-renew.service`: host systemd unit that calls the renewal script.
- `certbot-renew.timer`: host systemd timer that runs certificate renewal automatically.

## Notes

- The reverse proxy is intended to live at `/opt/droplet/proxy` on the Droplet.
- App containers should join the `edge` Docker network and expose their internal HTTP port to that network only.
- The `www/` directory is mounted as the ACME HTTP-01 challenge webroot.
- The initial active config is HTTP-only so Nginx can boot before certificates exist; `issue_certificate.sh` copies the HTTPS template into `conf.d/` after successful issuance.
- `www/` and `certs/` are runtime directories on the Droplet. Do not delete them during config syncs.
