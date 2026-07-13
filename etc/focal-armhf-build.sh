#!/usr/bin/env bash
# Builds and boot-tests the armhf viam-server inside the rdk-focal armhf image.
# Run via `docker run` from focal-build.yml (a job-level container can't be used
# on armhf — JS actions can't exec in a 32-bit-arm container on a 64-bit host).
# BUILD_CHANNEL is passed through the environment.
set -euxo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# static-release compresses with upx; the image ships none, install the armv7 build.
curl -fsSL https://github.com/upx/upx/releases/download/v5.2.0/upx-5.2.0-arm_linux.tar.xz | tar -C /tmp -xJ
cp /tmp/upx-5.2.0-arm_linux/upx /usr/local/bin/upx

# The workspace is bind-mounted from the host as another uid; hand it to testbot
# so the build can write and git doesn't flag dubious ownership.
chown -R testbot:testbot "$repo_root"
cd "$repo_root"

sudo -Hu testbot bash -lc "make BUILD_CHANNEL=${BUILD_CHANNEL} UNAME_M=armv7l static-release"

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
