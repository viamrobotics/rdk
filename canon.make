# Docker targets that pre-cache go module downloads (intended to be rebuilt weekly/nightly)
BUILD_CMD = docker buildx build --pull $(BUILD_PUSH) --force-rm --no-cache --build-arg BASE_TAG=$(BUILD_TAG) --platform linux/$(BUILD_TAG) -f etc/Dockerfile.cache -t 'ghcr.io/viamrobotics/canon:$(BUILD_TAG)-cache' .
BUILD_PUSH = --load

canon-cache: canon-cache-build canon-cache-upload

canon-cache-build: canon-cache-amd64 canon-cache-arm64

canon-cache-amd64: BUILD_TAG = amd64
canon-cache-amd64:
	$(BUILD_CMD)

canon-cache-arm64: BUILD_TAG = arm64
canon-cache-arm64:
	$(BUILD_CMD)

canon-cache-upload:
	docker push 'ghcr.io/viamrobotics/canon:amd64-cache'
	docker push 'ghcr.io/viamrobotics/canon:arm64-cache'

# CI targets that automatically push, avoid for local test-first-then-push workflows
canon-cache-amd64-ci: BUILD_TAG = amd64
canon-cache-amd64-ci: BUILD_PUSH = --push
canon-cache-amd64-ci:	
	$(BUILD_CMD)

canon-cache-arm64-ci: BUILD_TAG = arm64
canon-cache-arm64-ci: BUILD_PUSH = --push
canon-cache-arm64-ci:
	$(BUILD_CMD)
