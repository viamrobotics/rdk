#!/usr/bin/env bash
# build for android

set -euo pipefail

# apt-get deps:
# sudo apt install clang libx264-dev pkg-config libnlopt-dev libjpeg-dev

NDK_ROOT=${NDK_ROOT:-~/build/android-ndk-r25c}
GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
	CC=$NDK_ROOT/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android30-clang \
	go build -v \
	-tags no_cgo \
	./web/cmd/server
