#!/usr/bin/env bash
# Builds and boot-tests armhf viam-server inside the rdk-focal container.
# Invoked via docker run from focal-build.yml; BUILD_CHANNEL comes from the env.
set -euxo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# The build depends on mise to manage upx. Technically it also manages go and
# many other build tools but those are all baked into the image for now, so
# only install upx to save time and bandwidth.
sudo -Hu testbot bash -lc '
  mkdir -p ~/.local/bin
  curl -fsSL https://github.com/jdx/mise/releases/download/v2026.9.9/mise-v2026.9.9-linux-armv7.tar.xz | tar -C /tmp -xJ
  cp /tmp/mise/bin/mise ~/.local/bin/mise
  ~/.local/bin/mise trust -y
  ~/.local/bin/mise install upx
  ~/.local/bin/mise settings set auto_install false
'

# bind-mount is owned by another uid; allow git without chowning (breaks cleanup).
git config --system --add safe.directory '*'
cd "$repo_root"

sudo -Hu testbot bash -lc "make BUILD_CHANNEL=${BUILD_CHANNEL} UNAME_M=armv7l VERSION_SUFFIX=+focal static-release"

sudo -Hu testbot bash -lc '
  set -euo pipefail
  bin=$(find etc/packaging/static/deploy -type f -name "viam-server-*" | head -1)
  port=$((30000 + RANDOM))
  echo "{\"network\":{\"bind_address\":\"localhost:${port}\"}}" > /tmp/smoke.json
  "$bin" -config /tmp/smoke.json &
  srv=$!
  curl --retry 8 --retry-delay 2 --retry-connrefused -s "localhost:${port}" >/dev/null
  echo "boot OK"
  kill $srv 2>/dev/null || true
  wait $srv 2>/dev/null || true
'
