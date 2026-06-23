# Release Manifest

A buktio release is the tuple **(api, web, CLI, pinned Garage version, migration set)**. These
move together; the Garage tag is pinned because its metadata format and Admin API are
version-coupled (never use `:latest`).

| Component | Version / pin | Notes |
|---|---|---|
| buktio api | `2.0.0-dev` | Go REST API (`apps/api/cmd/server`) — released lines: `v1.0.0`, `v1.1.0` |
| buktio web | `2.0.0-dev` | Next.js panel (`apps/web`) — released lines: `v1.0.0`, `v1.1.0` |
| buktio cli | `2.0.0-dev` | `cmd/buktio` (cobra) — released lines: `v1.0.0`, `v1.1.0` |
| buktio s3proxy | `2.0.0-dev` | `cmd/buktio-s3proxy` — traffic-metering reverse proxy (new in v2) |
| **Garage** | **`v2.3.0`** | image `dxflrs/garage:v2.3.0`; Admin API **v2**; min supported `>= 2.0`, `--single-node` needs `>= 2.3` |
| Admin API | `v2` | buktio pins all admin calls to `/v2/...` |
| PostgreSQL | `16` | |
| DB migrations | up to `0013` | `internal/db/migrations` (v2 adds 0011 generic-S3, 0012 traffic_snapshots, 0013 backup_jobs) |

## v2 components (multi-backend, multi-node, metering, backups)

- **Storage backends**: Garage (full-featured) + generic-S3 (`aws_s3`/`r2`/`b2`/`seaweedfs`/`ceph_rgw`)
  behind the same `StorageProvider`, resolved per cluster by `internal/cluster` registry. The
  generic adapter uses the S3 API only with operator-supplied credentials; control-plane ops
  (keys/quota/cluster-health/website) report `Capabilities` gaps + `ErrUnsupported`.
- **Multi-node Garage**: Admin v2 ConnectClusterNodes / layout staging (assign+remove) / preview /
  revert; a background reconciler projects topology into `storage_nodes`.
- **Traffic metering**: `buktio-s3proxy` fronts the S3 plane (Caddy `s3.<domain>` → s3proxy → garage),
  counting per-(access-key, bucket, method) requests + egress into `traffic_snapshots`.
- **Backups**: api runs `pg_dump` (metadata + config only; never the KEK, never object data) into a
  backups volume; cobra CLI adds `backup`/`restore`/`upgrade`/`cluster`/`doctor`.

## Version guard

On startup (and on "connect existing cluster") buktio reads the Garage version via
`GET /v2/GetClusterStatus` and:

- **fails fast** if Garage `< 2.0` (Admin API v1 is incompatible with the v2 client), and
- **warns** if `< 2.3` when relying on the `--single-node` bootstrap convenience.

`GARAGE_IMAGE=dxflrs/garage:v2.3.0` is the single source of truth shared by compose, the
install script, Helm, and the test harness (testcontainers).
