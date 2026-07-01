# syntax=docker/dockerfile:1.7
#
# In-repo CI dev environment for RDK. Replaces the external, Ubuntu-18.04
# ghcr.io/viamrobotics/rdk-devenv base with a lean Ubuntu 20.04 image that
# carries only what `make build-go`, `make lint-go`, and `make test-go` need.
#
# Two published tiers (built via --target in .github/workflows/docker.yml):
#   base  -> ghcr.io/viamrobotics/rdk-devenv:<arch>          (consumed by `lint`)
#   cache -> ghcr.io/viamrobotics/rdk-devenv:<arch>-cache    (consumed by `test_go`,
#            base + a pre-warmed /home/testbot/go module & build cache)

# ---------------------------------------------------------------------------
# base: OS packages, Go toolchain, and the `testbot` user CI runs everything as
# ---------------------------------------------------------------------------
FROM ubuntu:20.04 AS base

# Go version is kept in lockstep with go.mod (`go 1.25.9`).
ARG GO_VERSION=1.25.9
# Provided automatically by buildx; one of amd64 | arm64.
ARG TARGETARCH

ENV DEBIAN_FRONTEND=noninteractive
ENV GOPATH=/home/testbot/go
ENV PATH=/usr/local/go/bin:/home/testbot/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# System + CGO build dependencies. The gostream/media stack needs the x264,
# opus, and X11 dev headers; nlopt is used by motionplan. python3-venv is used
# by the go test harness (etc/test.sh) so we bake it rather than apt-installing
# it at job runtime.
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        git \
        pkg-config \
        sudo \
        python3 \
        python3-venv \
        libnlopt-dev \
        libx264-dev \
        libopus-dev \
        libopusfile-dev \
        libx11-dev \
        libxext-dev \
    && rm -rf /var/lib/apt/lists/*

# Go toolchain from the official tarball (Ubuntu's apt Go is far too old).
# TARGETARCH (amd64/arm64) matches Go's release naming directly.
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" \
        -o /tmp/go.tgz \
    && tar -C /usr/local -xzf /tmp/go.tgz \
    && rm /tmp/go.tgz

# CI does `chown -R testbot:testbot .` then `sudo -Hu testbot bash -lc '...'`,
# so we need a uid/gid-1000 testbot with passwordless sudo.
RUN groupadd --gid 1000 testbot \
    && useradd --uid 1000 --gid 1000 --create-home --shell /bin/bash testbot \
    && echo 'testbot ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/testbot \
    && chmod 0440 /etc/sudoers.d/testbot

# Login-shell env: `sudo -Hu testbot bash -lc` sources /etc/profile.d/*.sh.
RUN printf 'export GOPATH=/home/testbot/go\nexport PATH=/usr/local/go/bin:/home/testbot/go/bin:$PATH\n' \
        > /etc/profile.d/go.sh

USER testbot
WORKDIR /home/testbot

RUN mkdir -p /home/testbot/go \
    # Avoid "dubious ownership" once CI chowns a checkout into this container.
    && git config --global --add safe.directory '*' \
    # RSDK-526: without an application-default-credentials file, some tests leak
    # goroutines and fail. A dummy file is enough to satisfy the gcloud client.
    && mkdir -p /home/testbot/.config/gcloud \
    && printf '{"client_id":"XXXX","client_secret":"XXXX","refresh_token":"XXXX","type":"authorized_user"}' \
        > /home/testbot/.config/gcloud/application_default_credentials.json

# ---------------------------------------------------------------------------
# cache-builder: warm the Go module + build caches against the current tree.
# This layer re-runs on every commit (source changes); the base above does not.
# ---------------------------------------------------------------------------
FROM base AS cache-builder

COPY --chown=1000:1000 ./ /tmp/rdk
WORKDIR /tmp/rdk

# `netrc` secret is optional; supplied by docker.yml for private module fetches.
RUN --mount=type=secret,id=netrc,uid=1000,dst=/home/testbot/.netrc \
    bash -lc 'make build-go lint-go'

# ---------------------------------------------------------------------------
# cache: base + only the warmed GOPATH (drop the /tmp/rdk source tree).
# ---------------------------------------------------------------------------
FROM base AS cache

COPY --from=cache-builder --chown=1000:1000 /home/testbot/go /home/testbot/go
