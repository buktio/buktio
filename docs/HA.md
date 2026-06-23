# buktio — High Availability

The buktio API is **stateless** (all state lives in Postgres + Garage), so it
scales horizontally and fails over trivially. A production HA deployment has three
independent layers:

## 1. API / web tier (stateless → replicas + HPA)
- Run ≥2 replicas of `buktio-api` (or `buktio-api-ee`) and `buktio-web` behind the
  edge (Caddy/Ingress). The Helm chart's `api.replicas` / `web.replicas` default to
  2; add a `HorizontalPodAutoscaler` keyed on CPU/RPS.
- Sessions are server-side (Postgres), so any replica can serve any request — no
  sticky sessions required.
- **RLS note:** with `api.rls=true` the API must connect as the non-superuser
  `buktio_app` role (superusers bypass RLS). Set `postgres.appUser` (bundled PG) or
  point `externalDatabase.url` at that role.

## 2. PostgreSQL (the source of record → managed/replicated)
- Use a managed Postgres (RDS/Cloud SQL) or an HA operator (CloudNativePG, Patroni)
  with a primary + ≥1 streaming replica and automated failover.
- The bundled chart Postgres is single-instance — fine for dev/small installs, not
  for HA. Disable it (`postgres.enabled=false`) and set `externalDatabase.url`.
- Back up the **KEK** (`BUKTIO_MASTER_KEY`) separately from the database; without it
  the envelope-encrypted secrets are unrecoverable.

## 3. Garage (the storage engine → multi-node cluster)
- Single-node Garage (`replication_factor=1`) has **no data redundancy**. For HA,
  run a ≥3-node Garage cluster with `replication_factor=3` across zones, managed by
  buktio's multi-node layout orchestration (v2).
- Garage stays unmodified and is reached only over its S3 (:3900) and Admin (:3903)
  APIs on a private network — never exposed through the Ingress.

## Upgrades
- Roll the API/web tier (new image) with a standard rolling update; migrations are
  applied idempotently on boot before serving.
- Garage major-version bumps are explicit, backup-first admin actions
  (`buktio upgrade`), never automatic.

## Topologies
| Scale | API/web | Postgres | Garage |
|---|---|---|---|
| dev | 1 | bundled single | single-node rf=1 |
| small prod | 2 | managed single + backups | single-node rf=1 + disk backups |
| HA | 2–N + HPA | managed primary + replica + failover | 3-node rf=3 across zones |
