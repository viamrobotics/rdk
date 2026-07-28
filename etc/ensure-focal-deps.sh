#!/usr/bin/env bash
# Installs the Focal toolchain for building RDK's semi-static viam-server.
# Used by etc/Dockerfile.focal.
set -euxo pipefail

# GO_VERSION tracks the `go` directive in go.mod.
GO_VERSION=1.25.9
NLOPT_VERSION=2.11.0
CMAKE_VERSION=4.3.4
# x264 stable branch. Built from source: focal's prebuilt libx264.a references
# __*_finite glibc symbols that fail to resolve when statically linked.
X264_COMMIT=b35605ace3ddf7c1a5d67a2eb553f034aef41d55

deb_arch="$(dpkg --print-architecture)"
case "$deb_arch" in
    amd64) go_arch=amd64 ;;
    arm64) go_arch=arm64 ;;
    armhf) go_arch=armv6l ;;
    *) echo "unsupported arch" >&2; exit 1 ;;
esac

# Build/link deps. nasm builds x264.
apt-get update
apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates curl file git make nasm pkg-config sudo xz-utils \
    libjpeg-turbo8-dev
rm -rf /var/lib/apt/lists/*

# cmake >= 3.18 is required by nlopt >= 2.11; focal apt ships 3.16. Kitware
# ships prebuilt tarballs for amd64/arm64; on armhf use the pip wheel
# (manylinux_2_31 matches focal) since Kitware has no 32-bit arm build.
case "$deb_arch" in
    armhf)
        apt-get update
        apt-get install -y --no-install-recommends python3-pip
        rm -rf /var/lib/apt/lists/*
        # upgrade pip so it finds the manylinux_2_31_armv7l wheel (else source build)
        python3 -m pip install --upgrade pip
        python3 -m pip install "cmake==${CMAKE_VERSION}"
        ;;
    *)
        curl -fsSL "https://github.com/Kitware/CMake/releases/download/v${CMAKE_VERSION}/cmake-${CMAKE_VERSION}-linux-$(uname -m).tar.gz" \
            | tar -xz --strip-components=1 -C /usr/local
        ;;
esac
cmake --version

# nlopt into /usr/local: headers + PIC static lib + pkg-config. NLOPT_CXX=OFF
# skips the C++-only algorithms so the library stays plain C.
curl -fsSL "https://github.com/stevengj/nlopt/archive/refs/tags/v${NLOPT_VERSION}.tar.gz" \
    | tar -C /tmp -xz
cmake -S "/tmp/nlopt-${NLOPT_VERSION}" -B /tmp/nlopt-build \
    -DBUILD_SHARED_LIBS=OFF -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
    -DNLOPT_CXX=OFF \
    -DNLOPT_PYTHON=OFF -DNLOPT_OCTAVE=OFF -DNLOPT_MATLAB=OFF \
    -DNLOPT_GUILE=OFF -DNLOPT_SWIG=OFF -DNLOPT_TESTS=OFF
cmake --build /tmp/nlopt-build -j "$(nproc)"
cmake --install /tmp/nlopt-build
rm -rf "/tmp/nlopt-${NLOPT_VERSION}" /tmp/nlopt-build

# x264 into /usr/local: headers + shared + PIC static + pkg-config.
# armhf targets armv6l (no NEON); drop asm so it doesn't emit NEON and crash there.
case "$deb_arch" in armhf) x264_asm=--disable-asm ;; *) x264_asm= ;; esac
curl -fsSL "https://code.videolan.org/videolan/x264/-/archive/${X264_COMMIT}/x264-${X264_COMMIT}.tar.gz" \
    | tar -C /tmp -xz
(
    cd "/tmp/x264-${X264_COMMIT}"
    ./configure --prefix=/usr/local --enable-shared --enable-static --enable-pic --disable-cli --disable-opencl $x264_asm
    make -j "$(nproc)"
    make install
)
ldconfig
rm -rf "/tmp/x264-${X264_COMMIT}"

# Go toolchain, symlinked into /usr/local/bin so it resolves under sudo's
# secure_path (the sudo -Hu testbot build path).
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" \
    | tar -C /usr/local -xz
ln -s /usr/local/go/bin/go /usr/local/bin/go
go version
