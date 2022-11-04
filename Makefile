BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)

TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)

PATH_WITH_TOOLS="`pwd`/$(TOOL_BIN):`pwd`/node_modules/.bin:${PATH}"

GIT_REVISION = $(shell git rev-parse HEAD | tr -d '\n')
LDFLAGS = -ldflags "$(shell etc/set_plt.sh) $(shell etc/tag_version.sh) -X 'go.viam.com/rdk/config.GitRevision=${GIT_REVISION}'"
BUILD_TAGS=dynamic
GO_BUILD_TAGS = -tags $(BUILD_TAGS)
LINT_BUILD_TAGS = --build-tags $(BUILD_TAGS)

default: build lint server

setup:
	bash etc/setup.sh

build: build-web build-go

build-go:
	go build $(GO_BUILD_TAGS) ./...

build-web: buf-web
	export NODE_OPTIONS=--openssl-legacy-provider && node --version 2>/dev/null || unset NODE_OPTIONS;\
	npm run build --prefix web/frontend

tool-install:
	GOBIN=`pwd`/$(TOOL_BIN) go install google.golang.org/protobuf/cmd/protoc-gen-go \
		github.com/bufbuild/buf/cmd/protoc-gen-buf-breaking \
		github.com/bufbuild/buf/cmd/protoc-gen-buf-lint \
		github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc \
		google.golang.org/grpc/cmd/protoc-gen-go-grpc \
		github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
		github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
		github.com/edaniels/golinters/cmd/combined \
		github.com/golangci/golangci-lint/cmd/golangci-lint \
		github.com/AlekSi/gocov-xml \
		github.com/axw/gocov/gocov \
		github.com/bufbuild/buf/cmd/buf \
		gotest.tools/gotestsum \
		github.com/rhysd/actionlint/cmd/actionlint

buf: buf-web

buf-web: tool-install
	npm ci --audit=false
	PATH=$(PATH_WITH_TOOLS) buf generate --template ./etc/buf.web.gen.yaml buf.build/erdaniels/gostream
	npm ci --audit=false --prefix web/frontend
	npm run rollup --prefix web/frontend

lint: lint-go lint-web
	PATH=$(PATH_WITH_TOOLS) actionlint

lint-go: tool-install
	export pkgs="`go list $(GO_BUILD_TAGS) -f '{{.Dir}}' ./... | grep -v /proto/`" && echo "$$pkgs" | xargs go vet $(GO_BUILD_TAGS) -vettool=$(TOOL_BIN)/combined
	GOGC=50 $(TOOL_BIN)/golangci-lint run $(LINT_BUILD_TAGS) -v --fix --config=./etc/.golangci.yaml

lint-web: typecheck-web
	npm run lint --prefix web/frontend

typecheck-web:
	npm run typecheck --prefix web/frontend

cover: tool-install
	PATH=$(PATH_WITH_TOOLS) ./etc/test.sh cover

test: test-go test-web

test-go: tool-install
	PATH=$(PATH_WITH_TOOLS) ./etc/test.sh

test-web:
	npm run test:unit --prefix web/frontend

# test.short skips tests requiring external hardware (motors/servos)
test-pi:
	go test -c -o $(BIN_OUTPUT_PATH)/test-pi go.viam.com/rdk/components/board/pi/impl
	sudo --preserve-env=GOOGLE_APPLICATION_CREDENTIALS $(BIN_OUTPUT_PATH)/test-pi -test.short -test.v

test-e2e:
	go build $(LDFLAGS) -o bin/test-e2e/server web/cmd/server/main.go
	./etc/e2e.sh -o 'run'

open-cypress-ui:
	go build $(LDFLAGS) -o bin/test-e2e/server web/cmd/server/main.go
	./etc/e2e.sh -o 'open'

test-integration:
	cd services/slam/builtin && sudo --preserve-env=APPIMAGE_EXTRACT_AND_RUN go test -v -run TestOrbslamIntegration

server:
	go build $(GO_BUILD_TAGS) $(LDFLAGS) -o $(BIN_OUTPUT_PATH)/server web/cmd/server/main.go

clean-all:
	git clean -fxd

license:
	license_finder --npm-options='--prefix web/frontend'

include *.make
