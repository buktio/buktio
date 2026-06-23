# buktio Architecture

buktio is a single self-hosted appliance made of four components. Garage is reached only over
two HTTP planes on a private network and is otherwise invisible to the user.

```
Browser ──TLS──> Caddy (edge, auto-HTTPS) ──> Next.js web ──REST/JSON (cookie)──> Go API ──┐
                                   │                                                        │ SQL
   presigned PUT/GET (direct bytes)│                                               PostgreSQL (state of record)
                                   ▼
                              Garage (UNMODIFIED)  :3900 S3  ·  :3903 Admin v2  ·  :3901 RPC* ·  :3902 web*
                              meta/data volumes                 (* loopback / private only)
```

## Components

| Component | Role |
|---|---|
| **Next.js web** | UI only. Holds a session cookie, never a Garage token. Large transfers go browser ↔ Garage S3 directly via presigned URLs (the API only signs). |
| **Go API** | The brain: auth/RBAC, business logic, the only holder of the Garage `admin_token` and the internal `buktio-system` S3 key, usage scheduler, audit, and the hidden garage-manager (bootstrap/lifecycle). |
| **PostgreSQL** | Source of record: identity, tenancy (org→project→bucket), the buktio-entity ⇄ Garage-UUID mapping, encrypted infra secrets, usage snapshots, audit. Garage has no concept of users/projects. |
| **Garage** | Opaque AGPLv3 engine. S3 (`:3900`) + Admin v2 (`:3903`). Admin/RPC bound to loopback. TLS terminated by Caddy (Garage has no native TLS). |

## The StorageProvider seam

Services depend on the `internal/storage.StorageProvider` interface, never on Garage
specifics. `GarageProvider` composes a typed Admin-v2 client (port 3903, bearer token) + an S3
client (port 3900, SigV4, path-style, using the `buktio-system` owner key). A registry/factory
resolves the configured backend at startup, so SeaweedFS / Ceph RGW / AWS S3 / R2 / B2 can be
added later behind the same interface. This abstraction is the reason buktio is a multi-backend
control plane, not "a Garage UI".

Access control is abstracted to **`private` vs `public-website`** + per-key **`read/write/owner`**
flags — never raw S3 policy/ACL JSON (Garage has none). Per-bucket usage comes from a single
`GetBucketInfo` call; object enumeration is a separate S3 concern.

## Package dependency direction

```
httpapi → service → { storage interface, repository, audit, usage }
storage/garage → storage interface + external SDKs only
nothing → httpapi
```

Wiring happens in `apps/api/cmd/server/main.go`.

## Open-core seams (all enabled/free in OSS)

`internal/edition`, `internal/entitlements` (OSS impl `AlwaysAllow`), `internal/authz` (OSS impl
`PermitAll`), and `internal/metering` (OSS impl `NoOp`) exist from day one. Every sensitive
handler calls `entitlements.Allowed(...)` / `authz.Can(...)`, which always return "allowed" in the
OSS build. The multi-tenant data model (`tenant`/`org` columns) is present even though MVP ships
single-tenant UX. Paid editions later swap in enforcing implementations of these interfaces with
no handler rewrites — see the development plan, section 18.

## Hidden Garage management

`internal/garagemanager` is the only package that knows Garage exists. On first boot it generates
secrets with `crypto/rand`, renders `garage.toml` (secrets injected via `*_FILE` env, never inlined),
starts Garage, waits for `GET /health`, then performs an **idempotent, restart-safe** single-node
layout bootstrap and provisions the `buktio-system` key — **entirely over the Admin HTTP API**. The
`garage` binary is only ever wrapped (hidden) for major-version metadata maintenance
(`meta snapshot`, `repair`, `convert-db`), never for routine operations and never user-facing.
