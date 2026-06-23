# buktio

**A free, open-source, self-hosted control plane (web UI + REST API + CLI) for S3-compatible object storage.**

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![API: Go](https://img.shields.io/badge/API-Go-00ADD8.svg?logo=go&logoColor=white)](go.mod)
[![Web: Next.js](https://img.shields.io/badge/Web-Next.js-000000.svg?logo=next.js&logoColor=white)](apps/web)
[![Storage: Garage](https://img.shields.io/badge/Storage-Garage_v2.3.0-3b5998.svg)](https://garagehq.deuxfleurs.fr/)
[![Deploy: Docker Compose](https://img.shields.io/badge/Deploy-Docker_Compose-2496ED.svg?logo=docker&logoColor=white)](deploy/docker-compose)
[![PRs welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)

<!-- TODO: add a GIF here showing the core loop: create-bucket -> issue-key -> browse/upload a file -->
> **TODO: add a screenshot / GIF** of the panel walking through create-bucket → issue an S3 access key → browse and upload a file.

---

## What is buktio

buktio is a self-hosted web panel, REST API, and CLI for managing S3-compatible object storage. You run one Docker Compose stack, open a web panel, and manage everything from there: create buckets, issue S3 access keys, browse/upload/download objects, set CORS and lifecycle rules, restore from trash, and watch usage and traffic metrics.

The default storage engine is [Garage](https://garagehq.deuxfleurs.fr/) (open-source, AGPLv3), shipped **unmodified and hidden** inside the stack — you never touch Garage's CLI or config files. buktio talks to Garage only over its network APIs (S3 on `:3900`, Admin v2 on `:3903`) on a private network.

**The 3-second version:** one command brings up the stack, you open a panel, Garage stays hidden — and the same panel can manage any other S3 backend too (AWS S3, Cloudflare R2, Backblaze B2, SeaweedFS, Ceph RGW).

## Why buktio

- **A self-hosted S3 panel that actually manages things.** On 2025-02-26 MinIO removed almost all management features from its free web console — bucket/key/policy/lifecycle/replication management plus IAM/SSO — leaving essentially a read-only object browser; that change reached users around 2025-05-24. Around December 2025 its open-source project entered a maintenance/security-fixes-only posture and its main GitHub repository is marked "archived". (MinIO the company is still active and sells a commercial product; what changed is the free open-source experience. Please verify the current repo status yourself.) buktio keeps the full management surface free.
- **Garage is great, but has no UI.** Garage (by the Deuxfleurs collective, in production since ~2020) is managed only via CLI and config files; there is no official web UI. buktio is that UI — and treats Garage as a hidden internal component you never operate by hand.
- **Self-hosting keeps egress at $0.** AWS S3 charges roughly $0.09/GB to download data out — about $900 to pull 10 TB in a month — versus $0 on storage you host yourself. (Cloudflare R2 also has $0 egress; Backblaze B2 is low-cost with cheap/free egress.)
- **One panel over any S3.** Manage your hidden Garage cluster and your external buckets (AWS S3, R2, B2, SeaweedFS, Ceph RGW) through a single interface. On external backends, some control-plane features (key management, quotas, cluster health, website hosting) may be unavailable — buktio reports those capability gaps honestly; object operations work everywhere.

## Quick start

Requires Docker (with Compose).

```bash
cd deploy/docker-compose

# Copy the env template and fill in generated secrets.
# (In a normal install the first-run bootstrap generates these for you; this
#  is the manual-compose fallback. Generate values with a CSPRNG, e.g.
#  `openssl rand -base64 32` and `openssl rand -hex 32`.)
cp .env.example .env

docker compose up -d
```

Then open **https://localhost**. In local dev, Caddy serves a self-signed certificate, so your browser will warn once — that's expected.

On first run, a **setup wizard** creates your admin account. After that: create a bucket, issue an S3 access key, and start browsing/uploading objects.

## Features

**Storage & clusters**
- Create and manage buckets
- Issue and revoke S3 access keys
- Multi-node Garage clusters and multi-cluster management
- Connect to an existing Garage cluster
- Cluster health and usage metrics

**Objects**
- Object browser with search
- Upload/download with progress
- Copy, move, rename
- CORS editor
- Lifecycle rules (object expiry, abort-incomplete-multipart)
- App-level trash (restore + auto-purge)
- Presigned URLs
- Client-side encryption (SSE-C)
- Public-website hosting

**Operations**
- First-run setup wizard
- API tokens (PATs) with Bearer auth
- Usage metrics and per-key traffic metering
- Prometheus `/metrics` endpoint
- Manual metadata backups
- cobra-based CLI: `status`, `logs`, `restart`, `doctor`, `backup`, `restore`, `upgrade`, `cluster`
- Single Docker Compose stack; Caddy for TLS

**Multi-backend (one interface)**
- Garage (default, hidden engine)
- AWS S3
- Cloudflare R2
- Backblaze B2
- SeaweedFS
- Ceph RGW

## Editions

buktio is open-core: the OSS core is fully functional with **no artificial limits**.

| Feature | Free (OSS) | Pro | Enterprise |
| --- | :---: | :---: | :---: |
| Web panel, buckets, S3 access keys | ✅ | ✅ | ✅ |
| Object browser (search, upload/download, copy/move/rename) | ✅ | ✅ | ✅ |
| CORS editor, lifecycle rules, app-level trash | ✅ | ✅ | ✅ |
| Usage metrics, per-key traffic metering, Prometheus `/metrics` | ✅ | ✅ | ✅ |
| API tokens (PATs), manual metadata backups | ✅ | ✅ | ✅ |
| Multi-node Garage clusters, multi-cluster, connect-existing | ✅ | ✅ | ✅ |
| Multi-backend (AWS/R2/B2/SeaweedFS/Ceph) | ✅ | ✅ | ✅ |
| Client-side encryption (SSE-C), presigned URLs, website hosting | ✅ | ✅ | ✅ |
| RBAC, members / invitations / seats | — | ✅ | ✅ |
| OIDC SSO (single sign-on) | — | ✅ | ✅ |
| Advanced audit export | — | ✅ | ✅ |
| Scheduled + off-box (external) backups | — | ✅ | ✅ |
| Hard multi-tenancy (Postgres RLS), tenant suspend/resume | — | — | ✅ |
| Per-org storage quotas, per-org dedicated clusters | — | — | ✅ |
| SCIM 2.0 provisioning, ABAC policies (IP allowlist / hours / read-only) | — | — | ✅ |
| Tamper-evident hash-chained audit + SIEM forward | — | — | ✅ |
| White-label branding + custom domains | — | — | ✅ |
| Helm chart + Kubernetes operator + Terraform provider + systemd | — | — | ✅ |

> **No artificial limits in the free core** — unlimited buckets, keys, objects, and nodes. The free core never gains artificial caps.

Paid modules are **new capabilities** (for teams and companies) — never the removal of something that was free. Licensing is offline (no phone-home). A separately operated **Hosted/SaaS** edition adds self-serve signup with email verification, per-tenant cluster provisioning, usage-based billing (storage GB-month + egress + requests, via Stripe), and resumable S3-to-S3 migration import.

## Architecture

```
                         private network
                       ┌───────────────────────────────────┐
Browser ──TLS──▶ Caddy ──▶ Next.js web ──REST/JSON──▶ Go API ──▶ PostgreSQL
                                                       │  │      (system of record)
                                                       │  └──▶ Garage  :3900 S3
                                                       └─────▶ Garage  :3903 Admin v2
                                                               (UNMODIFIED, hidden)
```

- The **Go API** is the only component holding the Garage admin token and the internal system S3 key.
- **PostgreSQL 16** is the system of record: identity, tenancy, the buktio⇄Garage mapping, encrypted secrets, usage, and audit.
- **Garage** runs unmodified on a private network, reached only over its S3 (`:3900`) and Admin v2 (`:3903`) HTTP APIs.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

## License

buktio's **own code is Apache-2.0** (see [LICENSE](LICENSE)) — permissive and free for any use, including commercial.

Garage is **AGPLv3** and runs as a **separate, unmodified process** reached only over the network, so the two programs form an **aggregate** — AGPL copyleft does **not** reach buktio's code. This is engineering posture, **not legal advice**.

See [docs/LICENSING.md](docs/LICENSING.md) and [THIRD-PARTY-LICENSES.md](THIRD-PARTY-LICENSES.md) for the full reasoning and attribution.

## Contributing

Contributions are welcome. See [DEVELOPMENT.md](DEVELOPMENT.md) to set up a local environment, then open an issue or pull request on the repository. Please keep the Apache-2.0 / Garage boundary intact (never modify, fork, or link Garage — communication is HTTP only; see [docs/LICENSING.md](docs/LICENSING.md)).

## Security

Found a vulnerability? Please report it responsibly — see [SECURITY.md](SECURITY.md) for how to disclose privately.

## Links

- **Repository:** https://github.com/buktio/buktio
- **Website:** https://buktio.org
- **Live read-only demo:** https://demo.buktio.org _(coming soon)_
