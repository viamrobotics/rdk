# Docker targets that pre-cache go module downloads (intended to be rebuilt weekly/nightly)
BUILD_CMD = docker buildx build --pull $(BUILD_PUSH) --force-rm --no-cache --build-arg MAIN_TAG=$(MAIN_TAG) --build-arg BASE_TAG=$(BUILD_TAG) --platform linux/$(BUILD_TAG) -f $(BUILD_FILE) -t '$(MAIN_TAG):$(BUILD_TAG)-cache' .
BUILD_PUSH = --load
BUILD_FILE = etc/Dockerfile.cache
CANON_IMAGE ?= rdk-devenv
X86_TAG ?= amd64-beta-81a6398
ARM_TAG ?= arm64-beta-81a6398
ANTIQUE_X86 ?= amd64-beta-30cf9f0
ANTIQUE_ARM ?= arm64-beta-81a6398


canon-cache: canon-cache-build canon-cache-upload

canon-cache-build: canon-cache-amd64 canon-cache-arm64

canon-cache-amd64: MAIN_TAG = ghcr.io/viamrobotics/$(CANON_IMAGE)
canon-cache-amd64: BUILD_TAG = $(X86_TAG)
canon-cache-amd64:
	$(BUILD_CMD)

canon-cache-arm64: MAIN_TAG = ghcr.io/viamrobotics/$(CANON_IMAGE)
canon-cache-arm64: BUILD_TAG = $(ARM_TAG)
canon-cache-arm64:
	$(BUILD_CMD)

canon-cache-upload:
	docker push 'ghcr.io/viamrobotics/$(CANON_IMAGE):$(X86_TAG)-cache'
	docker push 'ghcr.io/viamrobotics/$(CANON_IMAGE):$(ARM_TAG)-cache'

# CI targets that automatically push, avoid for local test-first-then-push workflows
canon-cache-amd64-ci: MAIN_TAG = ghcr.io/viamrobotics/$(CANON_IMAGE)
canon-cache-amd64-ci: BUILD_TAG = $(X86_TAG)
canon-cache-amd64-ci: BUILD_PUSH = --push
canon-cache-amd64-ci:	
	$(BUILD_CMD)

canon-cache-arm64-ci: MAIN_TAG = ghcr.io/viamrobotics/$(CANON_IMAGE)
canon-cache-arm64-ci: BUILD_TAG = $(ARM_TAG)
canon-cache-arm64-ci: BUILD_PUSH = --push
canon-cache-arm64-ci:
	$(BUILD_CMD)


antique-cache: antique-cache-build antique-cache-upload

antique-cache-build: antique-cache-amd64 antique-cache-arm64

antique-cache-amd64: MAIN_TAG = ghcr.io/viamrobotics/antique
antique-cache-amd64: BUILD_TAG = $(ANTIQUE_X86)
antique-cache-amd64: BUILD_FILE = etc/Dockerfile.antique-cache
antique-cache-amd64:
	$(BUILD_CMD)

antique-cache-arm64: MAIN_TAG = ghcr.io/viamrobotics/antique
antique-cache-arm64: BUILD_TAG = $(ANTIQUE_ARM)
antique-cache-arm64: BUILD_FILE = etc/Dockerfile.antique-cache
antique-cache-arm64:
	$(BUILD_CMD)

antique-cache-upload:
	docker push 'ghcr.io/viamrobotics/antique:$(ANTIQUE_X86)-cache'
	docker push 'ghcr.io/viamrobotics/antique:$(ANTIQUE_ARM)-cache'

# CI targets that automatically push, avoid for local test-first-then-push workflows
antique-cache-amd64-ci: MAIN_TAG = ghcr.io/viamrobotics/antique
antique-cache-amd64-ci: BUILD_TAG = $(ANTIQUE_X86)
antique-cache-amd64-ci: BUILD_PUSH = --push
antique-cache-amd64-ci: BUILD_FILE = etc/Dockerfile.antique-cache
antique-cache-amd64-ci:	
	$(BUILD_CMD)

antique-cache-arm64-ci: MAIN_TAG = ghcr.io/viamrobotics/antique
antique-cache-arm64-ci: BUILD_TAG = $(ANTIQUE_ARM)
antique-cache-arm64-ci: BUILD_PUSH = --push
antique-cache-arm64-ci: BUILD_FILE = etc/Dockerfile.antique-cache
antique-cache-arm64-ci:
	$(BUILD_CMD)

