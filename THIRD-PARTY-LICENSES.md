# Third-Party Licenses

buktio's own source code is licensed under **Apache-2.0** (see [LICENSE](LICENSE)).
It depends on, and is designed to operate alongside, the third-party components below.

## Garage (storage engine) — AGPLv3

| | |
|---|---|
| Project | Garage |
| Author | Deuxfleurs association |
| License | GNU Affero General Public License v3.0 (AGPLv3) |
| Pinned version | `v2.3.0` (image `dxflrs/garage:v2.3.0`) — see [RELEASE-MANIFEST.md](RELEASE-MANIFEST.md) |
| Source | https://git.deuxfleurs.fr/Deuxfleurs/garage (tag `v2.3.0`) |
| Docs | https://garagehq.deuxfleurs.fr/ |

**How buktio uses Garage (compliance posture):**

- Garage is run **unmodified** as a **separate process / container**.
- buktio communicates with Garage **only over its network APIs** — the S3 API (`:3900`) and
  the Admin API v2 (`:3903`). It never links against, embeds, or patches Garage code.
- buktio's deployment artifacts **pull the official upstream Garage image by tag/digest**;
  buktio does not redistribute (convey) Garage object code as part of its own release.
- Because Garage is unmodified and reached only over the network, the two programs are an
  **aggregate**, not a single combined work — see [docs/LICENSING.md](docs/LICENSING.md) for the
  full analysis and the conditions under which AGPLv3 obligations would change (e.g. if Garage
  were ever forked, patched, or bundled into a buktio-published artifact).

> If a future buktio distribution channel (e.g. a `.deb` or a buktio-published image) **bundles**
> the Garage binary, that channel must ship the full AGPLv3 license text, retain Garage's
> attribution, and provide/mirror the exact corresponding source for the pinned version.

## Go and Node dependencies

Go module and npm dependencies are enumerated in `go.mod`/`go.sum` and
`apps/web/package.json`/`pnpm-lock.yaml` respectively, each under its own license. An SBOM is
generated at release time. Run `go-licenses` / `license-checker` (wired in CI) for the full list.
