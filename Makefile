BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)

TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)

NDK_ROOT ?= etc/android-ndk-r26
BUILD_CHANNEL ?= local

PATH_WITH_TOOLS="`pwd`/$(TOOL_BIN):`pwd`/node_modules/.bin:${PATH}"

GIT_REVISION = $(shell git rev-parse HEAD | tr -d '\n')
TAG_VERSION?=$(shell git tag --points-at | sort -Vr | head -n1)
LDFLAGS = -ldflags "-s -w -extld="$(shell pwd)/etc/ld_wrapper.sh" -X 'go.viam.com/rdk/config.Version=${TAG_VERSION}' -X 'go.viam.com/rdk/config.GitRevision=${GIT_REVISION}'"
GOFLAGS=$(shell [ `uname` = Linux ] && [ `dpkg --print-architecture` = armhf ] && echo -tags=no_cgo || echo)

default: build lint server

setup:
	bash etc/setup.sh

build: build-web build-go

build-go:
	GOFLAGS=$(GOFLAGS) go build ./...

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
bin/$(GOOS)-$(GOARCH)/viam-cli:
	go build $(LDFLAGS) -tags osusergo,netgo -o $@ ./cli/viam

.PHONY: cli
cli: bin/$(GOOS)-$(GOARCH)/viam-cli

.PHONY: cli-ci
cli-ci: bin/$(GOOS)-$(GOARCH)/viam-cli
	if [ -n "$(CI_RELEASE)" ]; then \
		mkdir -p bin/deploy-ci/; \
		cp $< bin/deploy-ci/viam-cli-$(CI_RELEASE)-$(GOOS)-$(GOARCH); \
	fi

build-web: web/runtime-shared/static/control.js

# only generate static files when source has changed.
web/runtime-shared/static/control.js: web/frontend/src/*/* web/frontend/src/*/*/* web/frontend/src/*.* web/frontend/scripts/* web/frontend/*.*
	rm -rf web/runtime-shared/static
	npm ci --audit=false --prefix web/frontend
	npm run build-prod --prefix web/frontend

tool-install:
	GOBIN=`pwd`/$(TOOL_BIN) go install \
		github.com/edaniels/golinters/cmd/combined \
		github.com/golangci/golangci-lint/cmd/golangci-lint \
		github.com/AlekSi/gocov-xml \
		github.com/axw/gocov/gocov \
		gotest.tools/gotestsum \
		github.com/rhysd/actionlint/cmd/actionlint

lint: lint-go lint-web
	PATH=$(PATH_WITH_TOOLS) actionlint

lint-go: tool-install
	go mod tidy
	export pkgs="`go list -f '{{.Dir}}' ./... | grep -v /proto/`" && echo "$$pkgs" | xargs go vet -vettool=$(TOOL_BIN)/combined
	GOGC=50 $(TOOL_BIN)/golangci-lint run -v --fix --config=./etc/.golangci.yaml

lint-web: check-web
	npm run lint --prefix web/frontend

check-web:
	npm run check --prefix web/frontend

cover: tool-install
	PATH=$(PATH_WITH_TOOLS) ./etc/test.sh cover-with-race

test: test-go test-web

test-no-race: test-go-no-race test-web

test-go: tool-install
	PATH=$(PATH_WITH_TOOLS) ./etc/test.sh race

test-go-no-race: tool-install
	PATH=$(PATH_WITH_TOOLS) ./etc/test.sh

test-web:
	npm run test:unit --prefix web/frontend

# test.short skips tests requiring external hardware (motors/servos)
test-pi:
	go test -c -o $(BIN_OUTPUT_PATH)/test-pi go.viam.com/rdk/components/board/pi/impl
	sudo $(BIN_OUTPUT_PATH)/test-pi -test.short -test.v

test-e2e:
	go build $(LDFLAGS) -o bin/test-e2e/server web/cmd/server/main.go
	./etc/e2e.sh -o 'run' $(E2E_ARGS)

open-cypress-ui:
	go build $(LDFLAGS) -o bin/test-e2e/server web/cmd/server/main.go
	./etc/e2e.sh -o 'open'

server: build-web
	rm -f $(BIN_OUTPUT_PATH)/viam-server
	go build $(LDFLAGS) -o $(BIN_OUTPUT_PATH)/viam-server web/cmd/server/main.go

server-static: build-web
	rm -f $(BIN_OUTPUT_PATH)/viam-server
	VIAM_STATIC_BUILD=1 GOFLAGS=$(GOFLAGS) go build $(LDFLAGS) -o $(BIN_OUTPUT_PATH)/viam-server web/cmd/server/main.go

server-static-compressed: server-static
	upx --best --lzma $(BIN_OUTPUT_PATH)/viam-server

$(NDK_ROOT):
	# download ndk (used by server-android)
	cd etc && wget https://dl.google.com/android/repository/android-ndk-r26-linux.zip
	cd etc && unzip android-ndk-r26-linux.zip

.PHONY: server-android
server-android:
	GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
		CC=$(shell realpath $(NDK_ROOT)/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android30-clang) \
		go build -v \
		-tags no_cgo \
		-o bin/viam-server-$(BUILD_CHANNEL)-android-aarch64 \
		./web/cmd/server

clean-all:
	git clean -fxd

license-check:
	license_finder --npm-options='--prefix web/frontend'

include *.make
