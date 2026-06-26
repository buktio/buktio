# App-store templates

Starter templates for installing buktio from self-hosting platforms' app catalogs.
They wrap the **lite** stack (the single-binary `buktio-api` with the panel embedded,
plus its Postgres and the hidden Garage engine) — see
[`deploy/docker-compose/docker-compose.lite.yml`](../docker-compose/docker-compose.lite.yml).

| Platform | File | How it's consumed |
|---|---|---|
| Portainer | [`portainer/buktio.yml`](portainer/buktio.yml) | Stacks → Add stack → Upload / Web editor |
| CasaOS | [`casaos/docker-compose.yml`](casaos/docker-compose.yml) | Import as a custom app (has `x-casaos` metadata) |
| Unraid | [`../docker-compose/docker-compose.lite.yml`](../docker-compose/docker-compose.lite.yml) | Docker **Compose Manager** plugin → add the lite compose |
| TrueNAS SCALE | — | Use the Helm chart in [`deploy/helm`](../helm) |

> **Unraid:** buktio is a multi-container app (api + Postgres + Garage), which the
> single-container Community Apps template format doesn't fit. Install it with the
> **Docker Compose Manager** plugin pointed at `docker-compose.lite.yml` (provide the
> secrets via an `.env` next to it). A native CA template can follow once the optional
> single-file SQLite backend lands (which collapses the stack further).

## Secrets — read first

buktio needs a handful of secrets that must be **coordinated** across containers (the
Garage tokens must match between the `garage` and `api` services, and the
`BUKTIO_MASTER_KEY` encrypts stored credentials). Generate them once and paste them
into the platform's environment fields:

```sh
# 32-byte base64 values
openssl rand -base64 32   # BUKTIO_MASTER_KEY
openssl rand -hex 32      # GARAGE_RPC_SECRET
openssl rand -hex 32      # GARAGE_ADMIN_TOKEN
openssl rand -hex 32      # GARAGE_METRICS_TOKEN
openssl rand -hex 24      # POSTGRES_PASSWORD (keep it URL-safe → hex)
```

Never reuse the examples below — they are placeholders.

## TLS

The lite stack terminates TLS in the api binary (`BUKTIO_TLS`):

- `self` (default in these templates) — self-signed HTTPS on :443 (browser warning; fine on a LAN).
- `auto` — Let's Encrypt for a public `BUKTIO_TLS_DOMAIN`.
- `off` — plaintext HTTP on :80 (put it behind your platform's own reverse proxy).

For presigned-URL S3 subdomains and public bucket websites, use the **full** stack
(`docker-compose.yml`, which keeps Caddy) instead of the lite templates here.

## Submitting to the catalogs (maintainer task)

These files are the templates; getting buktio to show up in each store is a separate,
manual submission per platform:

- **Unraid**: submit the XML to the Community Applications repo / a template repo.
- **CasaOS**: submit to the CasaOS AppStore (or host a custom AppStore source).
- **Portainer**: no catalog submission — users import the stack file directly.

Keep these templates in sync with `docker-compose.lite.yml` when the stack changes.
