#!/usr/bin/env bash
set -euo pipefail
if [[ $EUID -ne 0 || $# -ne 1 ]]; then
  echo 'Usage: sudo bash deploy/install.sh <linux-binary>' >&2; exit 64
fi
binary=$(realpath "$1")
scripts=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
if [[ -e /opt/fabricworld || -e /etc/systemd/system/fabricworld.service ]] || id fabricworld >/dev/null 2>&1; then
  echo 'Existing installation or identity found; use the documented upgrade procedure.' >&2; exit 1
fi
if ss -H -ltn 'sport = :18082' | grep -q .; then echo 'Port 18082 is in use.' >&2; exit 1; fi
command -v vips >/dev/null
command -v vipsheader >/dev/null
useradd --system --home-dir /opt/fabricworld --shell /usr/sbin/nologin fabricworld
install -d -m 0755 /opt/fabricworld /opt/fabricworld/bin /opt/fabricworld/config /opt/fabricworld/docs /opt/fabricworld/releases
install -d -m 0700 -o fabricworld -g fabricworld /opt/fabricworld/data /opt/fabricworld/backups
install -m 0755 "$binary" /opt/fabricworld/bin/fabricworld
install -m 0644 "$scripts/nginx-location.conf" "$scripts/fabricworld.service" "$scripts/fabricworld-backup.service" "$scripts/fabricworld-backup.timer" /opt/fabricworld/config/
runuser -u fabricworld -- /opt/fabricworld/bin/fabricworld init --data /opt/fabricworld/data
systemctl link /opt/fabricworld/config/fabricworld.service /opt/fabricworld/config/fabricworld-backup.service /opt/fabricworld/config/fabricworld-backup.timer
systemctl daemon-reload
systemctl enable --now fabricworld.service fabricworld-backup.timer
curl --fail --silent --retry 10 --retry-delay 1 --retry-connrefused http://127.0.0.1:18082/healthz
systemctl start fabricworld-backup.service
echo 'FabricWorld installed. Add its Nginx location after local health verification.'
