# TESTBUILD_OUTPUT_PATH should only be defined during testing
ifdef TESTBUILD_OUTPUT_PATH
	BIN_OUTPUT_PATH = $(TESTBUILD_OUTPUT_PATH)
else
	BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)
endif

TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)

BUILD_CHANNEL ?= local

PATH_WITH_TOOLS="`pwd`/$(TOOL_BIN):`pwd`/node_modules/.bin:${PATH}"

GIT_REVISION = $(shell git rev-parse HEAD | tr -d '\n')
TAG_VERSION?=$(shell ./etc/dev-version.sh | sed 's/^v//')
DATE_COMPILED?=$(shell date +'%Y-%m-%d')
COMMON_LDFLAGS = -X 'go.viam.com/rdk/config.Version=${TAG_VERSION}' -X 'go.viam.com/rdk/config.GitRevision=${GIT_REVISION}' -X 'go.viam.com/rdk/config.DateCompiled=${DATE_COMPILED}'
ifdef BUILD_DEBUG
	GCFLAGS = -gcflags "-N -l"
else
	COMMON_LDFLAGS += -s -w
endif
LDFLAGS = -ldflags "-extld=$(shell pwd)/etc/ld_wrapper.sh $(COMMON_LDFLAGS)"

default: build lint server

setup:
	bash etc/setup.sh

build: build-go

build-go:
	go build ./...

GO_FILES=$(shell find . -name "*.go")

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
bin/$(GOOS)-$(GOARCH)/viam-cli: $(GO_FILES) Makefile go.mod go.sum
	# no_cgo necessary here because of motionplan -> nlopt dependency.
	# can be removed if you can run CGO_ENABLED=0 go build ./cli/viam on your local machine.
	# CGO_ENABLED=0 is necessary after bedf954b to prevent go from sneakily doing a cgo build
	CGO_ENABLED=0 go build $(GCFLAGS) $(LDFLAGS) -tags osusergo,netgo,no_cgo -o $@ ./cli/viam

.PHONY: cli
cli: bin/$(GOOS)-$(GOARCH)/viam-cli

.PHONY: cli-ci
cli-ci: bin/$(GOOS)-$(GOARCH)/viam-cli
	if [ -n "$(CI_RELEASE)" ]; then \
		mkdir -p bin/deploy-ci/; \
		cp $< bin/deploy-ci/viam-cli-$(CI_RELEASE)-$(GOOS)-$(GOARCH)$(EXE_SUFFIX); \
	fi

tool-install:
	GOBIN=`pwd`/$(TOOL_BIN) go install \
		github.com/AlekSi/gocov-xml \
		github.com/axw/gocov/gocov \
		gotest.tools/gotestsum \
		github.com/rhysd/actionlint/cmd/actionlint \
		golang.org/x/tools/cmd/stringer

lint: lint-go
	PATH=$(PATH_WITH_TOOLS) actionlint

generate-go: tool-install
	PATH=$(PATH_WITH_TOOLS) go generate ./...

# Yes this regex could be more specific but making it more specific in a way
# that works the same across GNU and BSD grep isn't currently worth the effort.
GOVERSION = $(shell grep '^go .\..' go.mod | head -n1 | cut -d' ' -f2)
lint-go: tool-install
	go mod tidy
	GOTOOLCHAIN=go$(GOVERSION) GOGC=50 go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2 run --config=./etc/.golangci.yaml || true
	GOTOOLCHAIN=go$(GOVERSION) GOGC=50 go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2 run -v --fix --config=./etc/.golangci.yaml
	./etc/lint_register_apis.sh

cover-only: tool-install
	PATH=$(PATH_WITH_TOOLS) ./etc/test.sh cover

cover: test-go cover-only

test-go: tool-install
	PATH=$(PATH_WITH_TOOLS) ./etc/test.sh race

test-go-no-race: tool-install
	PATH=$(PATH_WITH_TOOLS) ./etc/test.sh

server:
	rm -f $(BIN_OUTPUT_PATH)/viam-server
	go build $(GCFLAGS) $(LDFLAGS) -o $(BIN_OUTPUT_PATH)/viam-server ./web/cmd/server

server-static:
	rm -f $(BIN_OUTPUT_PATH)/viam-server
	VIAM_STATIC_BUILD=1 GOFLAGS=$(GOFLAGS) go build $(GCFLAGS) $(LDFLAGS) -o $(BIN_OUTPUT_PATH)/viam-server ./web/cmd/server

full-static:
	mkdir -p bin/static
	go build -tags no_cgo,osusergo,netgo $(GCFLAGS) -ldflags="-extldflags=-static $(COMMON_LDFLAGS)" -o bin/static/viam-server-$(shell go env GOARCH) ./web/cmd/server

# should be kept in sync with the windows build in the BuildViamServer helper in testutils/file_utils.go
windows:
	mkdir -p bin/windows
	GOOS=windows GOARCH=amd64 go build -tags no_cgo $(GCFLAGS) -ldflags="-extldflags=-static $(COMMON_LDFLAGS)" -o bin/windows/viam-server-amd64.exe ./web/cmd/server
	cd bin/windows && zip viam.zip viam-server-amd64.exe

server-static-compressed: server-static
	upx --best --lzma $(BIN_OUTPUT_PATH)/viam-server

clean-all:
	git clean -fxd

license-check:
	license_finder

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
