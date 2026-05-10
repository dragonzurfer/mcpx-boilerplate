# docs.md

## Purpose

Single-Droplet deployment assets for containerized app stacks and the shared reverse-proxy layer.

## Subfolders

- `proxy/`: shared Nginx reverse-proxy stack, ACME webroot, and certificate helper scripts for the Droplet.
- `explore/`: Docker Compose stack for the Explore web container and worker container on the shared Droplet.

## Notes

- The Droplet layout is standardized around `/opt/droplet/proxy` for shared ingress and `/opt/apps/<app>` for each deployed application.
- New apps should join the shared `edge` Docker network so Nginx can proxy by container DNS name instead of host ports.
