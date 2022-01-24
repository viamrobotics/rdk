ifeq ("aarch64", "$(shell uname -m)")
	NATIVE_DOCKER := arm64
else ifeq ("x86_64", "$(shell uname -m)")
	NATIVE_DOCKER := amd64
else
	NATIVE_DOCKER := $(shell uname -m)
endif

ENTRYCMD = --testbot-uid $(shell id -u) --testbot-gid $(shell id -g)

# This sets up multi-arch emulation under linux. Run before using multi-arch targets.
canon-emulation:
	docker run --rm --privileged multiarch/qemu-user-static --reset -c yes -p yes

# Canon versions of targets run in the canonical viam docker image
canon-build:
	docker run -v$(shell pwd):/host --workdir /host --rm -ti ghcr.io/viamrobotics/canon:$(NATIVE_DOCKER)-cache $(ENTRYCMD) make build lint

canon-test:
	docker run -v$(shell pwd):/host --workdir /host --rm -ti ghcr.io/viamrobotics/canon:$(NATIVE_DOCKER)-cache $(ENTRYCMD) make build lint test

# Canon shells use the raw (non-cached) canon docker image
canon-shell:
	docker run -v$(shell pwd):/host --workdir /host --rm -ti ghcr.io/viamrobotics/canon:$(NATIVE_DOCKER) $(ENTRYCMD) bash

canon-shell-amd64:
	docker run --platform linux/amd64 -v$(shell pwd):/host --workdir /host --rm -ti ghcr.io/viamrobotics/canon:amd64 $(ENTRYCMD) bash

canon-shell-arm64:
	docker run --platform linux/arm64 -v$(shell pwd):/host --workdir /host --rm -ti ghcr.io/viamrobotics/canon:arm64 $(ENTRYCMD) bash


# Docker targets that pre-cache go module downloads (intended to be rebuilt weekly/nightly)
canon-cache: canon-cache-build canon-cache-upload

canon-cache-build:
	docker buildx build --load --no-cache --platform linux/amd64 -f etc/Dockerfile.amd64-cache -t 'ghcr.io/viamrobotics/canon:amd64-cache' .
	docker buildx build --load --no-cache --platform linux/arm64 -f etc/Dockerfile.arm64-cache -t 'ghcr.io/viamrobotics/canon:arm64-cache' .

canon-cache-upload:
	docker push 'ghcr.io/viamrobotics/canon:amd64-cache'
	docker push 'ghcr.io/viamrobotics/canon:arm64-cache'
