#!/usr/bin/env bash
set -euo pipefail
if [[ $EUID -ne 0 || $# -ne 1 ]]; then echo 'Usage: sudo bash deploy/setup-ci.sh <public-key.pub>' >&2; exit 64; fi
public_key=$(realpath "$1")
scripts=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
ssh-keygen -l -f "$public_key" >/dev/null
if [[ $(wc -l < "$public_key") -ne 1 ]] || ! grep -q '^ssh-ed25519 ' "$public_key"; then echo 'Expected one Ed25519 public key.' >&2; exit 64; fi
test -x /opt/fabricworld/bin/fabricworld
if id fabricworld-deploy >/dev/null 2>&1 || [[ -e /etc/sudoers.d/fabricworld-deploy ]]; then echo 'CI identity already exists.' >&2; exit 1; fi
useradd --system --home-dir /opt/fabricworld/deploy-user --shell /bin/bash fabricworld-deploy
install -d -m 0755 /opt/fabricworld/deploy-user /opt/fabricworld/deploy-user/.ssh
install -m 0755 "$scripts/deploy-ssh.sh" "$scripts/deploy-release.sh" /opt/fabricworld/bin/
printf 'restrict,command="/opt/fabricworld/bin/deploy-ssh.sh" %s\n' "$(< "$public_key")" > /opt/fabricworld/deploy-user/.ssh/authorized_keys
chmod 0644 /opt/fabricworld/deploy-user/.ssh/authorized_keys
printf 'fabricworld-deploy ALL=(root) NOPASSWD: /opt/fabricworld/bin/deploy-release.sh\n' > /etc/sudoers.d/fabricworld-deploy
chmod 0440 /etc/sudoers.d/fabricworld-deploy
visudo -cf /etc/sudoers.d/fabricworld-deploy
