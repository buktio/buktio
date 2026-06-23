# Development

There are two ways to run buktio locally. The first one error you hit —
`env file .env not found` — is fixed by generating the `.env` (step below).

> Current state: the backend implements health endpoints + the full Garage
> StorageProvider adapter (M0–M4). The setup wizard and the `/api/v1/*` product
> endpoints land in M5+, so right now "running it" means: infra comes up, the API
> serves `/healthz` + `/readyz`, and the web shell renders.

## 0. Generate secrets (required, once)

```bash
./deploy/docker-compose/gen-env.sh     # or:  make gen-env
```

This writes `deploy/docker-compose/.env` (mode 600) with random Postgres/Garage/KEK
secrets. It will not overwrite an existing `.env`.

## A. Dev mode (recommended) — infra in Docker, app local

Fast iteration: run **Postgres + Garage** in Docker (host-published ports) and run
the **Go API** and **Next.js** locally.

```bash
make dev-up        # Postgres :5433, Garage :3900 (S3) / :3903 (admin)
make api-local     # runs the API at http://localhost:8080 (Ctrl-C to stop)
# in another terminal:
cd apps/web && pnpm install && pnpm dev   # http://localhost:3000
```

> Postgres is published on **5433** (not 5432) to avoid clashing with a local
> Postgres. Override the host ports by adding `BUKTIO_PG_PORT` / `BUKTIO_S3_PORT` /
> `BUKTIO_ADMIN_PORT` to `deploy/docker-compose/.env`.

Check it:

```bash
curl localhost:8080/healthz   # {"status":"ok",...}
curl localhost:8080/readyz    # db: configured, storage engine: ok (once Garage is healthy)
```

Stop infra: `make dev-down`.

## B. Full stack in Docker — `docker compose up --build`

Builds and runs everything (caddy + web + api + postgres + garage). Requires the
`.env` from step 0.

```bash
make up
# equivalently:
cd deploy/docker-compose && docker compose up -d --build
```

- Open the panel at **https://localhost** (Caddy serves an internal-CA cert; your
  browser will warn — accept it for local dev).
- Garage's ports are **not** published to the host in this mode (internal network
  only); only Caddy publishes 80/443.

Stop: `make down` (from repo root) or `docker compose down` in the compose dir.

> **Why the bare `docker compose up -d` failed before:** the compose references
> built `buktio/api` and `buktio/web` images. They don't exist on a registry yet,
> so you must build them with `--build` (the `build:` sections + `build/*.Dockerfile`
> handle this), and you must have generated `.env` first.

## Tests & checks

```bash
make test               # Go unit tests
make test-integration   # against a real Garage container (testcontainers)
make lint               # go vet + gofmt
cd apps/web && pnpm typecheck && pnpm build
```

> If `go` isn't on your PATH, install Go 1.25+ (the module requires it).

## v1.1 features & notes

- **Subdomains (full stack):** presigned URLs use `s3.localhost` and public bucket
  websites use `<bucket>.web.localhost` (Caddy routes both to Garage without a path
  rewrite, so SigV4 stays valid). Browsers resolve `*.localhost` to loopback
  automatically. For CLI tools (curl/python) add to `/etc/hosts`:
  `127.0.0.1 s3.localhost` and `127.0.0.1 <bucket>.web.localhost`, or use
  `curl --resolve s3.localhost:443:127.0.0.1`. For a real domain, point a wildcard
  (`*.web.example.com`) + `s3.example.com` at the host and set
  `BUKTIO_S3_PUBLIC_ENDPOINT` / `BUKTIO_WEB_PUBLIC_DOMAIN` + garage.toml root_domains.
- **Connect existing Garage** (instead of bootstrapping a managed single-node): set
  `BUKTIO_STORAGE_MODE=external` plus `GARAGE_ADMIN_URL`, `GARAGE_S3_URL`,
  `GARAGE_ADMIN_TOKEN` (and optionally `BUKTIO_SYSTEM_S3_ACCESS_KEY` /
  `BUKTIO_SYSTEM_S3_SECRET`). buktio verifies connectivity and never edits the
  cluster's layout/config.
- **API tokens (PATs):** create one in the panel (Settings → Tokens), then call the
  API with `Authorization: Bearer bk_pat_...` (no cookie/CSRF needed).
- **Metrics:** buktio exposes Prometheus metrics at `/metrics` (guard with
  `BUKTIO_METRICS_TOKEN`); the panel's Ops page proxies the engine's metrics via
  `/api/v1/system/garage-metrics`.
- **Trash:** deleting objects moves them to a recoverable trash (auto-purged after
  7 days); restore/purge from the bucket's Trash tab.
