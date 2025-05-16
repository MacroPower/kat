#!/bin/bash

groupadd -r nixbld

mkdir -m 0755 /nix
chown -R ubuntu:ubuntu /nix

for n in $(seq 1 10); do
  useradd -c "Nix build user $n" -d /var/empty -g nixbld -G nixbld -M -N -r -s "$(command -v nologin)" "nixbld$n"
done

echo "Installing nix"
su ubuntu <<'EOF'
  bash <(curl -L https://nixos.org/nix/install) --no-daemon
  chmod +x ~/.nix-profile/etc/profile.d/nix.sh
  ~/.nix-profile/etc/profile.d/nix.sh
EOF
chown -R ubuntu:ubuntu /nix

export DEVBOX_USE_VERSION="0.13.7"
export DEVBOX_USER="ubuntu"
export PATH="/home/${DEVBOX_USER}/.nix-profile/bin:$PATH"

echo "Installing devbox"
curl -L https://get.jetify.com/devbox | bash -s -- -f
chown -R ubuntu:ubuntu /usr/local/bin/devbox
chmod +x /usr/local/bin/devbox

runuser -u ubuntu renovate
