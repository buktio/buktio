# Security Policy

buktio is a free, open-source, self-hosted control plane for S3-compatible object
storage. Because you run it yourself, the security of a deployment depends both on
buktio's own code and on how you operate it. This document explains how to report
problems, what buktio does to protect your data, and how to harden your install.

> Note: This describes engineering posture, not a legal or compliance guarantee.
> buktio is an early, actively developed project — please read the "Honesty" section
> at the bottom.

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.** Public
issues disclose the problem before a fix exists, which puts other users at risk.

Instead, report privately through either channel:

- **Email:** `security@buktio.org` <!-- TODO: confirm this address is live -->
- **GitHub private security advisories:** open a draft advisory at
  <https://github.com/buktio/buktio/security/advisories/new>

When you report, it helps to include:

- A description of the issue and its impact.
- Steps to reproduce (a proof of concept, affected version/commit, and config if
  relevant).
- Any suggested remediation, if you have one.

### What to expect

buktio is currently a small/new project, so the following are stated as **intent**,
not a contractual SLA:

- We aim to **acknowledge** your report within a **few business days**.
- We will keep you updated as we investigate and work on a fix.
- We follow **coordinated disclosure**: we ask that you give us a reasonable window
  to ship a fix before any public write-up, and we'll credit you (if you want) once
  a fix is available.

If you do not hear back within a reasonable time, please follow up — a message may
have been missed rather than ignored.

## Supported Versions

buktio is early software. Security fixes target the **latest released line**. Older
releases may not receive backported fixes; if you're on an older version, the
recommended remediation is to upgrade.

| Version          | Supported          |
| ---------------- | ------------------ |
| Latest release   | :white_check_mark: |
| Older releases   | :x: (please upgrade) |

There is no long-term-support (LTS) line yet. As the project matures, this table
will be updated.

## Security Model — What buktio Does

The points below describe protections built into buktio's own code. They are
defaults of the design, not a promise that any given deployment is correctly
configured.

### Authentication & sessions

- **Passwords** are hashed with **argon2id**.
- **Web sessions** use HTTP cookies plus a **CSRF double-submit token** to protect
  state-changing requests.
- **Programmatic access** uses **API tokens (PATs)** sent as `Bearer` tokens.

### Secret handling

- Sensitive secrets — the Garage admin token, the internal system S3 key, and
  external-backend credentials — are **encrypted at rest with AES-256-GCM envelope
  encryption**: each record has its own data key, wrapped by a **key-encryption-key
  (KEK)** that you supply via environment variable or file.
- Secret **files are written with `0600` permissions**, and buktio checks those
  permissions on startup.

### Object encryption

- **Optional client-side encryption (SSE-C)** is supported for objects, so the
  storage backend never sees plaintext when you use it.

### Backups

- Backups use `pg_dump` for **metadata and configuration only**.
- Backups **never include the KEK and never include object data**. (Keep this in
  mind for your own disaster-recovery planning — see Hardening below.)

### Network isolation

- The bundled **Garage engine sits on a private network** and is reachable only by
  the buktio API over its network APIs (S3 `:3900`, Admin v2 `:3903`). Operators
  and end users never talk to Garage directly.
- buktio holds the Garage admin token and the internal system S3 key on the
  operator's behalf.

### Enterprise edition

The paid Enterprise edition adds further controls:

- A **tamper-evident, hash-chained audit log** with an `/audit/verify` endpoint and
  SIEM forwarding.
- **PostgreSQL row-level security (RLS)** for hard organization/tenant isolation,
  plus tenant suspend/resume and ABAC policies (IP allowlist / business-hours /
  read-only).

### Free vs. paid

The OSS / free-forever core (Apache-2.0) is fully functional with **no artificial
limits** — unlimited buckets, keys, objects, and nodes. The security-relevant
features in the free core include argon2id passwords, CSRF-protected sessions, API
tokens (PATs), AES-256-GCM envelope encryption of secrets, `0600` secret files with
permission checks, optional client-side encryption (SSE-C), presigned URLs, and
private-network isolation of the bundled Garage engine.

Paid editions (Pro / Enterprise / Hosted) add **new** capabilities for teams and
organizations — for example RBAC and OIDC SSO (Pro), or hard multi-tenancy via
Postgres RLS, ABAC policies, and the tamper-evident audit log (Enterprise). Paid
modules are never the removal of something that was free: the free core does not
gain artificial limits, and licensing is offline (no phone-home).

## Hardening Recommendations for Operators

buktio gives you the building blocks, but a secure deployment is a shared
responsibility. We strongly recommend:

- **Set a strong, unique KEK and protect it.** The KEK unlocks every encrypted
  secret. **If you lose the KEK, you lose access to all encrypted secrets** (and
  backups deliberately do not contain it). Store it in a secret manager or another
  safe, backed-up location separate from the database.
- **Terminate TLS properly.** Local dev ships a **self-signed certificate** for
  `https://localhost`. **Replace it** for any real deployment — use a CA-issued
  certificate (Caddy can obtain one automatically) and serve only over HTTPS.
- **Keep PostgreSQL private.** Do not expose the database to the public internet;
  bind it to the internal network, use strong credentials, and restrict access.
- **Keep the storage engine private.** The bundled Garage engine already runs on a
  private network reachable only by buktio (S3 `:3900`, Admin v2 `:3903`). Keep it
  that way — do not republish those ports publicly or otherwise route around the
  control plane.
- **Rotate credentials.** Periodically rotate S3 access keys, API tokens (PATs),
  and the KEK; revoke tokens and keys that are no longer needed.
- **Keep Garage pinned and updated.** buktio targets a specific Garage version
  (currently v2.3.0). Stay on the pinned/tested version and apply Garage security
  updates as they become available.
- **Keep buktio updated.** Run a supported release (see above) and apply updates
  promptly.
- **Limit who can reach the panel.** Put the web panel behind your normal network
  controls (VPN, firewall, reverse proxy with access rules) where appropriate.

## Scope

This security policy covers **buktio's own code** (the Go REST API, the Next.js web
panel, and the cobra-based CLI — buktio's code is Apache-2.0).

The **bundled Garage storage engine is a separate, unmodified upstream project**
(AGPLv3, by the Deuxfleurs collective) that buktio talks to only over its network
APIs. Because Garage runs as a separate, unmodified process reached only over the
network, our engineering posture is that the two form an "aggregate" and AGPL
copyleft does not reach buktio's own Apache-2.0 code. This is engineering posture,
not legal advice. **Vulnerabilities in Garage itself should be reported upstream**
to the Garage project, not here. If you're unsure whether an issue is in buktio's
code or in Garage, report it to us privately and we'll help triage it.

Similarly, issues in **external S3 backends** you connect (AWS S3, Cloudflare R2,
Backblaze B2, SeaweedFS, Ceph RGW) belong to those providers. buktio's scope is the
control plane and how it integrates with them.

## Honesty — What This Is Not

We want to be plain about the limits of this document:

- buktio **does not currently hold SOC 2, ISO 27001, or any other security or
  compliance certification.** Formal compliance work is **on the roadmap** but not
  done. We will not claim certifications we do not have.
- This policy describes design and intent for an early project. It is **not a
  warranty** that any deployment is secure, nor legal advice.
- Security is ongoing. If something here is unclear, or you think a protection is
  weaker than described, please tell us via the private reporting channels above.

Thank you for helping keep buktio and its users safe.
