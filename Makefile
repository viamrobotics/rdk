BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)

TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)

BUILD_CHANNEL ?= local

PATH_WITH_TOOLS="`pwd`/$(TOOL_BIN):`pwd`/node_modules/.bin:${PATH}"

GIT_REVISION = $(shell git rev-parse HEAD | tr -d '\n')
TAG_VERSION?=$(shell git tag --points-at | sort -Vr | head -n1)
DATE_COMPILED?=$(shell date +'%Y-%m-%d')
COMMON_LDFLAGS = -s -w -X 'go.viam.com/rdk/config.Version=${TAG_VERSION}' -X 'go.viam.com/rdk/config.GitRevision=${GIT_REVISION}' -X 'go.viam.com/rdk/config.DateCompiled=${DATE_COMPILED}'
LDFLAGS = -ldflags "-extld=$(shell pwd)/etc/ld_wrapper.sh $(COMMON_LDFLAGS)"
ifeq ($(shell command -v dpkg >/dev/null && dpkg --print-architecture),armhf)
GOFLAGS += -tags=no_tflite
endif

default: build lint server

setup:
	bash etc/setup.sh

build: build-web build-go

build-go:
	go build ./...

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
		cp $< bin/deploy-ci/viam-cli-$(CI_RELEASE)-$(GOOS)-$(GOARCH)$(EXE_SUFFIX); \
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

check-web: build-web
	npm run check --prefix web/frontend

cover-only: tool-install
	PATH=$(PATH_WITH_TOOLS) ./etc/test.sh cover

cover: test-go cover-only

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

full-static: build-web
	mkdir -p bin/static
	go build -tags no_cgo,osusergo,netgo -ldflags="-extldflags=-static $(COMMON_LDFLAGS)" -o bin/static/viam-server-$(shell go env GOARCH) ./web/cmd/server

server-static-compressed: server-static
	upx --best --lzma $(BIN_OUTPUT_PATH)/viam-server

clean-all:
	git clean -fxd

license-check:
	license_finder --npm-options='--prefix web/frontend'

FFMPEG_ROOT ?= etc/FFmpeg
$(FFMPEG_ROOT):
	cd etc && git clone https://github.com/FFmpeg/FFmpeg.git --depth 1 --branch release/6.1

# For ARM64 builds, use the image ghcr.io/viamrobotics/antique:arm64 for backward compatibility
FFMPEG_PREFIX ?= $(shell realpath .)/gostream/ffmpeg/$(shell uname -s)-$(shell uname -m)
# See compilation guide here https://trac.ffmpeg.org/wiki/CompilationGuide
FFMPEG_OPTS = --disable-programs --disable-doc --disable-everything --prefix=$(FFMPEG_PREFIX) --disable-autodetect --disable-x86asm
ifeq ($(shell uname -m),aarch64)
	# We only support hardware encoding on a Raspberry Pi.
	FFMPEG_OPTS += --enable-encoder=h264_v4l2m2m
	FFMPEG_OPTS += --enable-v4l2-m2m
endif
ffmpeg: $(FFMPEG_ROOT)
	cd $(FFMPEG_ROOT) && ($(MAKE) distclean || true)
	cd $(FFMPEG_ROOT) && ./configure $(FFMPEG_OPTS)
	cd $(FFMPEG_ROOT) && $(MAKE)
	cd $(FFMPEG_ROOT) && $(MAKE) install

	# Only keep archive files. Different architectures can share the same source files.
	find $(FFMPEG_PREFIX)/* -type d ! -wholename $(FFMPEG_PREFIX)/lib | xargs rm -rf


include *.make
