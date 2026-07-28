#!/usr/bin/env bash
# Builds and boot-tests armhf viam-server inside the rdk-focal container.
# Invoked via docker run from focal-build.yml; BUILD_CHANNEL comes from the env.
set -euxo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# static-release compresses with upx; the image ships none, install the armv7 build.
curl -fsSL https://github.com/upx/upx/releases/download/v5.2.0/upx-5.2.0-arm_linux.tar.xz | tar -C /tmp -xJ
cp /tmp/upx-5.2.0-arm_linux/upx /usr/local/bin/upx

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
