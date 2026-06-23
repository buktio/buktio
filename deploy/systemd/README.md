# systemd install (secondary path)

This directory will hold the `install.sh` (`curl | bash`) flow and the hardened
systemd units (`buktio-garage`, `buktio-api`, `buktio-web`) for a bare-metal
Ubuntu/Debian single-node install. It is built in **M10**.

Design (see the development plan §13):
- FHS layout: binaries in `/opt/buktio/bin`, config in `/etc/buktio` (`750`),
  data in `/var/lib/{buktio,garage}`.
- Non-login `buktio` system user; Garage runs as `buktio`, never root.
- Units hardened (`NoNewPrivileges`, `ProtectSystem=strict`), secrets via
  `EnvironmentFile`; Garage relabeled "buktio storage engine (internal)".
- Garage admin (`:3903`) + RPC (`:3901`) bound to `127.0.0.1`.
- All Garage orchestration reuses `internal/garagemanager` (the same code path as
  compose / Helm / a future operator).
