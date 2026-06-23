#!/usr/bin/env bash
# buktio bare-metal installer (Ubuntu/Debian, systemd). Secondary install path; the
# primary is docker-compose. Idempotent: re-running upgrades binaries/units without
# clobbering data or regenerating existing secrets.
#
#   curl -fsSL https://get.buktio.io/install.sh | sudo bash
#   sudo ./install.sh --edition enterprise --license-file /path/to/buktio.license
#
# It creates a non-login `buktio` user, lays out FHS paths, installs the pinned
# (unmodified) Garage binary + buktio binaries, generates secrets with the buktio
# CLI, renders config, installs the three hardened units, and starts them. Garage
# is relabeled "storage engine" throughout and never runs as root.
set -euo pipefail

GARAGE_VERSION="v2.3.0"            # PINNED. never latest
EDITION="oss"
LICENSE_FILE=""
DRY_RUN=false
PREFIX="/opt/buktio"
ETC="/etc/buktio"
DATA="/var/lib/buktio"
GARAGE_DATA="/var/lib/garage"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!! \033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mxx \033[0m %s\n' "$*" >&2; exit 1; }
run()  { if $DRY_RUN; then echo "+ $*"; else eval "$@"; fi; }

while [ $# -gt 0 ]; do
  case "$1" in
    --edition)       EDITION="$2"; shift 2 ;;
    --license-file)  LICENSE_FILE="$2"; shift 2 ;;
    --dry-run)       DRY_RUN=true; shift ;;
    -h|--help)       grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)               die "unknown flag: $1" ;;
  esac
done

[ "$(id -u)" -eq 0 ] || die "must run as root (sudo)."
command -v systemctl >/dev/null || die "systemd is required."
. /etc/os-release 2>/dev/null || true
case "${ID:-}" in ubuntu|debian) ;; *) warn "untested on ${ID:-unknown}; proceeding." ;; esac

log "Creating the buktio system user + FHS layout"
run "id -u buktio >/dev/null 2>&1 || useradd --system --home $DATA --shell /usr/sbin/nologin buktio"
run "install -d -m 0755 $PREFIX/bin $PREFIX/web"
run "install -d -m 0750 -o buktio -g buktio $ETC $DATA $GARAGE_DATA/meta $GARAGE_DATA/data"

log "Installing the pinned storage engine ($GARAGE_VERSION) — unmodified, with its AGPLv3 NOTICE"
# (real installer downloads + verifies checksum & signature; placeholder here)
if [ ! -x "$PREFIX/bin/garage" ]; then
  warn "fetch + checksum-verify garage $GARAGE_VERSION into $PREFIX/bin/garage (and ship its LICENSE/NOTICE)"
fi

log "Installing buktio binaries"
API_BIN="buktio-api"; [ "$EDITION" != "oss" ] && API_BIN="buktio-api-ee"
warn "place $API_BIN + the web bundle under $PREFIX; symlink:"
run "ln -sf $PREFIX/bin/$API_BIN $PREFIX/bin/buktio-api"

log "Generating secrets (idempotent) + rendering config"
if [ ! -f "$ETC/api.env" ]; then
  MASTER_KEY="$(head -c32 /dev/urandom | base64)"
  run "umask 077; cat > $ETC/api.env <<EOF
DATABASE_URL=postgres://buktio@127.0.0.1:5432/buktio?sslmode=disable
GARAGE_ADMIN_URL=http://127.0.0.1:3903
GARAGE_S3_URL=http://127.0.0.1:3900
BUKTIO_MASTER_KEY=$MASTER_KEY
BUKTIO_HTTP_ADDR=:8080
EOF"
  if [ "$EDITION" != "oss" ] && [ -n "$LICENSE_FILE" ]; then
    run "echo BUKTIO_LICENSE_TOKEN=\"\$(cat '$LICENSE_FILE')\" >> $ETC/api.env"
  fi
  run "chown root:buktio $ETC/api.env && chmod 0640 $ETC/api.env"
else
  log "  $ETC/api.env exists — keeping existing secrets"
fi
[ -f "$ETC/garage.env" ] || warn "render $ETC/garage.toml + $ETC/garage.env (admin/RPC bound to 127.0.0.1)"
[ -f "$ETC/web.env" ]    || run "umask 077; echo 'BUKTIO_API_URL=http://127.0.0.1:8080' > $ETC/web.env"

log "Installing hardened systemd units"
SRC="$(cd "$(dirname "$0")" && pwd)"
for u in buktio-garage buktio-api buktio-web; do
  run "install -m 0644 $SRC/$u.service /etc/systemd/system/$u.service"
done
run "systemctl daemon-reload"
run "systemctl enable --now buktio-garage buktio-api buktio-web"

log "Done. Panel: http://127.0.0.1:8080 (front it with TLS). 'systemctl status buktio-api' for health."
[ "$EDITION" != "oss" ] && [ -z "$LICENSE_FILE" ] && warn "edition=$EDITION but no --license-file: api FAILS CLOSED to OSS."
exit 0
