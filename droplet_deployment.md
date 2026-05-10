# Droplet Deployment

This file records how the DigitalOcean Droplet is structured so additional apps can be added without rediscovering the conventions.

## Host layout

- Shared reverse proxy: `/opt/droplet/proxy`
- Explore source checkout: `/opt/apps/explore/current`
- Explore shared state: `/opt/apps/explore/shared`
- Explore env file: `/opt/apps/explore/shared/explore.env`
- Explore logs: `/opt/apps/explore/shared/logs`

## Reverse proxy

- The Droplet runs a shared Nginx container from `infra/droplet/proxy/compose.yaml`.
- App stacks join the shared Docker network `edge`.
- Each hostname gets its own Nginx server block under `/opt/droplet/proxy/conf.d/`.
- `explore.mcpx.in` proxies to the `explore-web` container on port `8080`.

## Explore stack

- The Explore image is built from the repo `Dockerfile`.
- `web` runs with `APP_MODE=web` and serves HTTP only.
- `worker` runs with `APP_MODE=worker` and owns all recurring jobs plus the judge loop.
- The worker mounts `/var/run/docker.sock` and launches local judge containers with `JUDGE_WORKER_CONCURRENCY=4`.
- The worker pins `JUDGE_DOCKER_CPUS=1` so single active compiles are not capped at half a CPU.
- The worker also pins `JUDGE_DOCKER_COMPILE_MEMORY_MB=512` so compile containers are not capped at low per-problem runtime memory limits.
- The worker also pins `JUDGE_DOCKER_COMPILE_TIMEOUT_MS=60000` to compensate for slower Go compiles on the shared-CPU Droplet.
- The web container does not get Docker socket access.

## Deploy flow

1. Sync the repo to `/opt/apps/explore/current`.
2. Ensure `/opt/apps/explore/shared/explore.env` contains the production secrets and DB settings.
3. Sync proxy configuration without deleting runtime certificate state:
   - keep `/opt/droplet/proxy/www` and `/opt/droplet/proxy/certs`
   - only update config, scripts, and templates under `/opt/droplet/proxy`
4. Start or update the shared proxy:
   - `docker compose -f /opt/droplet/proxy/compose.yaml up -d`
5. Start or update Explore:
   - `cd /opt/apps/explore/current`
   - `docker compose -f infra/droplet/explore/compose.yaml up -d --build`
6. Verify the app:
   - `docker compose -f infra/droplet/explore/compose.yaml ps`
   - `docker compose -f infra/droplet/explore/compose.yaml logs --tail=100 web`
   - `docker compose -f infra/droplet/explore/compose.yaml logs --tail=100 worker`

## DNS and certificates

- `explore.mcpx.in` must point to the Droplet public IP before the hostname works publicly.
- HTTP works as soon as the DNS record exists and the proxy container is up.
- After DNS is live, issue the certificate from `/opt/droplet/proxy`:
  - `./issue_certificate.sh explore.mcpx.in <email>`
- The first certificate issuance also swaps `conf.d/explore.mcpx.in.conf` to the checked-in HTTPS server-block template from `templates/` and reloads Nginx.
- Renew existing certificates:
  - `./renew_certificates.sh`
- Install the host timer after the proxy assets are synced:
  - `cp /opt/droplet/proxy/certbot-renew.service /etc/systemd/system/`
  - `cp /opt/droplet/proxy/certbot-renew.timer /etc/systemd/system/`
  - `systemctl daemon-reload`
  - `systemctl enable --now certbot-renew.timer`

## Adding another app later

1. Create `/opt/apps/<app>/current` and `/opt/apps/<app>/shared`.
2. Add a new Compose stack for that app with its web container joined to `edge`.
3. Add a new Nginx server block in `/opt/droplet/proxy/conf.d/<subdomain>.conf` that proxies to the app container DNS name on `edge`.
4. Start the new app stack.
5. Point the new subdomain at the Droplet IP.
6. Issue a certificate for the new hostname.
