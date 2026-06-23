# Licensing & AGPLv3 Compliance

> This document is engineering guidance, **not legal advice**. Obtain IP-counsel sign-off before
> any commercial launch.

## Summary

- **buktio's own code is Apache-2.0.**
- **Garage is AGPLv3** and is used as an **unmodified, separate process** reached **only over its
  network APIs** (S3 `:3900`, Admin v2 `:3903`).
- Under this posture the two programs form an **aggregate**, not a single combined/derivative
  work, so AGPLv3's copyleft does **not** reach buktio's source.

## Why the AGPL stays "dormant" for buktio

AGPLv3 §13 (the network-interaction clause that distinguishes AGPL from GPL) requires offering
**Corresponding Source of a modified covered work** to remote users. Two facts keep it from firing
on buktio's code:

1. **No modification.** buktio ships and runs Garage **unmodified**. §13's source-offer duty is
   conditioned on you running a **modified** version. Stock Garage triggers no §13 obligation on us.
2. **Separate works, communicating at arm's length.** buktio and Garage are separate programs that
   talk only via documented network protocols (S3 SigV4 + Admin HTTP/JSON). They do not share an
   address space, do not link, and do not exchange complex internal data structures. Per the FSF's
   "mere aggregation" / socket-separation guidance, this is an aggregate — buktio is not a
   derivative work of Garage.

Therefore buktio's panel/API/CLI may be licensed permissively (Apache-2.0), and future closed Pro
modules remain possible, **as long as the boundary below is preserved**.

## Hard rules (do not break)

- **Never modify or fork Garage** for production. A modified Garage offered over a network would
  pull §13 into play for that modified work.
- **Never import Garage crates / link Garage code** into buktio binaries. Communication is HTTP only.
- **Prefer the official image by digest.** `deploy/` pulls `dxflrs/garage:<pinned>` from upstream, so
  buktio **conveys** no Garage object code (lowest-risk path under §6).
- **If you ever bundle the binary** (e.g. a `.deb`, a tarball, or a buktio-published image that embeds
  Garage), you become a conveyor of AGPLv3 object code and must: ship the full AGPLv3 text, keep
  Garage's attribution, and **provide or mirror the exact corresponding source** for the pinned
  version. Keep `THIRD-PARTY-LICENSES.md` and `NOTICE` accurate for that channel.
- **Hosted/SaaS:** keeping Garage unmodified keeps §13 dormant even when operating it as a service.
  Modifying Garage in a hosted offering would obligate source disclosure to users of that service.

## Pinned version & source

See [RELEASE-MANIFEST.md](../RELEASE-MANIFEST.md). Pinned: `dxflrs/garage:v2.3.0`; source tag
`v2.3.0` at https://git.deuxfleurs.fr/Deuxfleurs/garage .
