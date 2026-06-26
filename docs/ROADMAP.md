# buktio roadmap

This is a public, best-effort roadmap. It is **not** a set of promises or dates — priorities
shift with feedback, and items can move, merge, or drop. Its purpose is to be transparent about
where buktio is going, and — just as importantly — about **what stays free**.

## The free/paid promise

buktio is open-core. The Apache-2.0 **free core has no artificial limits** — unlimited buckets,
keys, objects, and nodes — and it never will. Paid editions add **new capabilities for teams and
companies** (access control, SSO, compliance, multi-tenancy); they never take away something that
was free. Licensing is offline — no phone-home.

Each item below is tagged with where it lands:

- 🟢 **Free** — Apache-2.0 core, for everyone.
- 🔵 **Pro** — teams (RBAC, SSO, advanced audit, scheduled/off-box backups).
- 🟣 **Enterprise** — regulated/large orgs (hard multi-tenancy, SCIM, ABAC, white-label, operator).

The roadmap below focuses on the **free core**. For the current free/Pro/Enterprise split, see the
[Editions table in the README](../README.md#editions).

---

## Now — shipped

The free core already manages buckets, S3 access keys, multi-node Garage clusters and external S3
backends (AWS S3, Cloudflare R2, Backblaze B2, SeaweedFS, Ceph RGW); a full object browser
(upload/download, copy/move/rename, search, trash); CORS, lifecycle rules, presigned URLs, SSE-C,
public-website hosting; usage and per-key traffic metrics, a Prometheus endpoint, API tokens, and a
CLI. See the [README](../README.md#features).

## Next — in progress

### 🟢 Single binary, every architecture
The web panel is being embedded directly into the Go binary, so buktio runs as **one executable**
with the UI served alongside the API — no separate Node container. The same change makes every
official image **multi-arch (amd64 + arm64)**, so buktio runs cleanly on a Raspberry Pi or any ARM
home server.

### 🟢 Zero-dependency install
- **Optional SQLite backend** — run the free single-node install with a single database *file*
  instead of a PostgreSQL server. PostgreSQL stays the default and remains required for the Pro and
  Enterprise editions; a one-shot converter migrates a SQLite install to PostgreSQL when you upgrade.
- **Native packaging** — `.deb`/`.rpm` packages and a Homebrew tap for the CLI, plus one-click
  templates for **Unraid Community Apps, CasaOS, TrueNAS, and Portainer**.

### 🟢 Everyday UX
- Object browser: **folder uploads**, multi-select bulk actions, in-panel **previews** (images,
  video, text, markdown), and share-links with an expiry.
- A **usage dashboard** with storage-growth and traffic charts, plus a ready-made Grafana dashboard.

## Later — planned

### 🟢 Connect & automate
- **More S3 backends**: Wasabi, Storj, Hetzner Object Storage, Google Cloud Storage, MinIO-as-backend.
- **Event notifications / webhooks**: fire a webhook when objects are created or deleted, so storage
  becomes part of your automation.

### 🟢 Data control
- **Object versioning** in the UI on backends that support it (restore or delete prior versions).
- A **bucket-policy wizard** that brings visibility, quotas, lifecycle, and CORS into one clear flow.
- **Cross-backend replication** (basic): copy or sync a bucket from one backend to another — for
  example, mirror a local Garage bucket to an off-site S3 provider.

### Under consideration (not committed)
Internationalization (community translations), a mobile-responsive / PWA panel, and extension points
for community integrations.

---

## Paid editions

Pro and Enterprise development tracks alongside the free core. New paid capabilities are additive and
are listed in the [Editions table](../README.md#editions). Notable in-flight/▶planned paid work:

- 🔵 **Pro**: deeper RBAC, richer audit export, more scheduled/off-box backup targets.
- 🟣 **Enterprise**: broader compliance controls, more SCIM/IdP coverage, expanded white-label and
  Kubernetes-operator features.

If you want something prioritized, open a
[GitHub Discussion](https://github.com/buktio/buktio/discussions) — feedback genuinely steers this list.
