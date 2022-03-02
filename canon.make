DOCKER_CMD = docker run -v$(HOME)/.ssh:/home/testbot/.ssh:ro -v$(shell pwd):/host --workdir /host --rm -ti $(DOCKER_PLATFORM) ghcr.io/viamrobotics/canon:$(DOCKER_TAG) --testbot-uid $(shell id -u) --testbot-gid $(shell id -g)

ifeq ("aarch64", "$(shell uname -m)")
	DOCKER_NATIVE_PLATFORM = --platform linux/arm64
	DOCKER_NATIVE_TAG = arm64
	DOCKER_NATIVE_TAG_CACHE = arm64-cache
else ifeq ("x86_64", "$(shell uname -m)")
	DOCKER_NATIVE_PLATFORM = --platform linux/amd64
	DOCKER_NATIVE_TAG = amd64
	DOCKER_NATIVE_TAG_CACHE = amd64-cache
else
	DOCKER_NATIVE_TAG = latest
endif

DOCKER_PLATFORM = $(DOCKER_NATIVE_PLATFORM)
DOCKER_TAG = $(DOCKER_NATIVE_TAG)

# This sets up multi-arch emulation under linux. Run before using multi-arch targets.
canon-emulation:
	docker run --rm --privileged multiarch/qemu-user-static --reset -c yes -p yes

# Canon versions of targets run in the canonical viam docker image
canon-build: DOCKER_TAG = $(DOCKER_NATIVE_TAG_CACHE)
canon-build:
	$(DOCKER_CMD) make build lint

canon-test: DOCKER_TAG = $(DOCKER_NATIVE_TAG_CACHE)
canon-test:
	$(DOCKER_CMD) make build lint test

# Canon shells use the raw (non-cached) canon docker image
canon-shell:
	$(DOCKER_CMD) bash

canon-shell-amd64: DOCKER_PLATFORM = --platform linux/amd64
canon-shell-amd64: DOCKER_TAG = amd64
canon-shell-amd64:
	$(DOCKER_CMD) bash

canon-shell-arm64: DOCKER_PLATFORM = --platform linux/arm64
canon-shell-arm64: DOCKER_TAG = arm64
canon-shell-arm64:
	$(DOCKER_CMD) bash


# Docker targets that pre-cache go module downloads (intended to be rebuilt weekly/nightly)
canon-cache: canon-cache-build canon-cache-upload

canon-cache-build:
	docker buildx build --load --no-cache --platform linux/amd64 -f etc/Dockerfile.amd64-cache -t 'ghcr.io/viamrobotics/canon:amd64-cache' .
	docker buildx build --load --no-cache --platform linux/arm64 -f etc/Dockerfile.arm64-cache -t 'ghcr.io/viamrobotics/canon:arm64-cache' .

canon-cache-upload:
	docker push 'ghcr.io/viamrobotics/canon:amd64-cache'
	docker push 'ghcr.io/viamrobotics/canon:arm64-cache'
