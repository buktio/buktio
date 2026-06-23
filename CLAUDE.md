# buktio — project rules for agents

buktio is a free, open-source (Apache-2.0), self-hosted control plane for S3-compatible
object storage (web UI + REST API + CLI), with Garage as the default hidden engine.

> ⚠️ **This repository is public.** Never commit secrets (API tokens, passwords, keys).
> Secrets live only in git-ignored files (`.claude/settings.local.json`, `*.env`) or the
> user's global vault `~/.claude/secrets.md`.

## Infrastructure / Deployment

**Domain:** `buktio.org` — registered on **Cloudflare** (account *SPARSIM*,
zone id `681242d4927a35afc5f19602cb22ab38`, Free plan).

**Cloudflare API token** — DNS-only scope (`dns_records:edit`, `dns_records:read`, `zone:read`
on buktio.org). Exposed as env var **`CLOUDFLARE_API_TOKEN_BUKTIO`** via git-ignored
`.claude/settings.local.json`, and recorded in `~/.claude/secrets.md`. Example:

```bash
curl -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN_BUKTIO" \
  https://api.cloudflare.com/client/v4/zones/681242d4927a35afc5f19602cb22ab38/dns_records
```

It **cannot** change SSL/TLS mode, redirect rules, or Email Routing — those are dashboard-only.

**GitHub API token** — for creating/managing repos under the **`buktio`** org. Exposed as env var
**`GITHUB_TOKEN_BUKTIO`** via git-ignored `.claude/settings.local.json`, and recorded in
`~/.claude/secrets.md`. The value is **never** stored here or anywhere git-tracked. Example:

```bash
curl -H "Authorization: Bearer $GITHUB_TOKEN_BUKTIO" https://api.github.com/user
```

**Hosting = PROD Kubernetes cluster** (`kubectl` context **`PROD`**):
- ingress-nginx LoadBalancer **`185.179.212.66`** (the A-record target), IngressClass `nginx`.
- TLS via cert-manager **ClusterIssuer `letsencrypt-buktio-dns`** (Let's Encrypt DNS-01,
  Cloudflare solver scoped to `buktio.org`).
- Landing page runs in namespace **`buktio`**; manifests + deploy script in
  [`deploy/landing/`](deploy/landing/) (serves `site/index.html` from a ConfigMap behind nginx).
- DNS is managed by [`deploy/dns/cloudflare-records.sh`](deploy/dns/cloudflare-records.sh).

**DNS map:**

| Host | Type | Target | Proxy | Purpose |
|---|---|---|---|---|
| `buktio.org` | A | 185.179.212.66 | orange | landing (now) → panel (later) |
| `www` | CNAME | buktio.org | orange | 301 → apex |
| `demo` | A | 185.179.212.66 | orange | read-only demo (after app deploy) |
| `s3` | A | 185.179.212.66 | gray (DNS-only) | S3 API (after app deploy) |
| `*.web` | A | 185.179.212.66 | gray (DNS-only) | public bucket sites (after app deploy) |
| `_dmarc` | TXT | `v=DMARC1; p=reject; …` | — | email anti-spoof |

`s3` and `*.web` stay **DNS-only (gray)**: Cloudflare's free proxy caps uploads at 100 MB and
breaks SigV4, and a 2-level wildcard isn't covered by Universal SSL.

For the proxied (orange) hosts, the zone's SSL/TLS mode must be **Full (strict)** — the origin
ingress presents a real Let's Encrypt cert and force-redirects to HTTPS.
