# Force updates if images are older than this. Should be updated for breaking changes to images.
# Obtain from the OLDEST of either amd64 or arm64 (usually amd64) with the following:
# docker inspect -f '{{ .Created }}' ghcr.io/viamrobotics/canon:amd64
DOCKER_MIN_DATE=2022-06-08T00:35:24.791434424Z

DOCKER_CMD = docker run $(DOCKER_SSH_AGENT) $(DOCKER_NETRC_RUN) -v$(HOME)/.ssh:/home/testbot/.ssh:ro -v$(shell pwd):/host --workdir /host --rm -ti $(DOCKER_PLATFORM) ghcr.io/viamrobotics/canon:$(DOCKER_TAG) --testbot-uid $(shell id -u) --testbot-gid $(shell id -g)

ifeq ("Darwin", "$(shell uname -s)")
	# Docker has magic paths for OSX
	DOCKER_SSH_AGENT = -v /run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock -e SSH_AUTH_SOCK="/run/host-services/ssh-auth.sock"
else ifneq ("$(SSH_AUTH_SOCK)x", "x")
	DOCKER_SSH_AGENT = -v "$(SSH_AUTH_SOCK):$(SSH_AUTH_SOCK)" -e SSH_AUTH_SOCK="$(SSH_AUTH_SOCK)"
endif

ifeq ("aarch64", "$(shell uname -m)")
	DOCKER_NATIVE_PLATFORM = --platform linux/arm64
	DOCKER_NATIVE_TAG = arm64
	DOCKER_NATIVE_TAG_CACHE = arm64-cache
else ifeq ("arm64", "$(shell uname -m)")
	DOCKER_NATIVE_PLATFORM = --platform linux/arm64
	DOCKER_NATIVE_TAG = arm64_test
	DOCKER_NATIVE_TAG_CACHE = arm64-cache
else ifeq ("x86_64", "$(shell uname -m)")
	DOCKER_NATIVE_PLATFORM = --platform linux/amd64
	DOCKER_NATIVE_TAG = amd64_test
	DOCKER_NATIVE_TAG_CACHE = amd64-cache
else
	DOCKER_NATIVE_TAG = latest
	DOCKER_NATIVE_TAG_CACHE = latest
endif

DOCKER_PLATFORM = $(DOCKER_NATIVE_PLATFORM)
DOCKER_TAG = $(DOCKER_NATIVE_TAG)

# If there's a netrc file, use it.
ifeq ($(shell grep -qs github.com ~/.netrc && echo -n yes), yes)
	DOCKER_NETRC_BUILD = --secret id=netrc,src=$(HOME)/.netrc
	DOCKER_NETRC_RUN = -v$(HOME)/.netrc:/home/testbot/.netrc:ro
endif

canon-update:
	etc/canon-update.sh $(DOCKER_MIN_DATE)

# This sets up multi-arch emulation under linux. Run before using multi-arch targets.
canon-emulation:
	docker run --rm --privileged multiarch/qemu-user-static --reset -c yes -p yes

# Canon versions of targets run in the canonical viam docker image
canon-build: DOCKER_TAG = $(DOCKER_NATIVE_TAG_CACHE)
canon-build: canon-update
	$(DOCKER_CMD) make build lint

canon-test: DOCKER_TAG = $(DOCKER_NATIVE_TAG_CACHE)
canon-test: canon-update
	$(DOCKER_CMD) make build lint test

# Canon shells use the raw (non-cached) canon docker image
canon-shell: canon-update
	$(DOCKER_CMD) bash

canon-shell-amd64: DOCKER_PLATFORM = --platform linux/amd64
canon-shell-amd64: DOCKER_TAG = amd64_test
canon-shell-amd64: canon-update
	$(DOCKER_CMD) bash

canon-shell-arm64: DOCKER_PLATFORM = --platform linux/arm64
canon-shell-arm64: DOCKER_TAG = arm64_test
canon-shell-arm64: canon-update
	$(DOCKER_CMD) bash


# Docker targets that pre-cache go module downloads (intended to be rebuilt weekly/nightly)
canon-cache: canon-update canon-cache-build canon-cache-upload

canon-cache-build:
	docker buildx build $(DOCKER_NETRC_BUILD) --load --no-cache --platform linux/amd64 -f etc/Dockerfile.amd64-cache -t 'ghcr.io/viamrobotics/canon:amd64-cache' .
	docker buildx build $(DOCKER_NETRC_BUILD) --load --no-cache --platform linux/arm64 -f etc/Dockerfile.arm64-cache -t 'ghcr.io/viamrobotics/canon:arm64-cache' .

canon-cache-upload:
	docker push 'ghcr.io/viamrobotics/canon:amd64-cache'
	docker push 'ghcr.io/viamrobotics/canon:arm64-cache'
