BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)

TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)

PATH_WITH_TOOLS="`pwd`/$(TOOL_BIN):`pwd`/node_modules/.bin:${PATH}"

VERSION = $(shell git fetch --tags && git tag --sort=-version:refname | head -n 1)
GIT_REVISION = $(shell git rev-parse HEAD | tr -d '\n')
LDFLAGS = -ldflags "-X 'go.viam.com/rdk/config.Version=${VERSION}' -X 'go.viam.com/rdk/config.GitRevision=${GIT_REVISION}'"

default: build lint server

setup:
	bash etc/setup.sh

build: build-web build-go

build-go: buf-go
	go build ./...

build-web: buf-web
	export NODE_OPTIONS=--openssl-legacy-provider && node --version 2>/dev/null || unset NODE_OPTIONS;\
	cd web/frontend && npm ci --audit=false && npm run rollup
	cd web/frontend && npm run build

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
		github.com/bufbuild/buf/cmd/buf

buf: buf-go buf-web

buf-go: tool-install
	PATH=$(PATH_WITH_TOOLS) buf --timeout 5m0s lint
	PATH=$(PATH_WITH_TOOLS) buf --timeout 5m0s format -w
	PATH=$(PATH_WITH_TOOLS) buf --timeout 5m0s generate

buf-web: tool-install
	npm ci --audit=false
	PATH=$(PATH_WITH_TOOLS) buf lint
	PATH=$(PATH_WITH_TOOLS) buf generate --template ./etc/buf.web.gen.yaml
	PATH=$(PATH_WITH_TOOLS) buf generate --timeout 5m --template ./etc/buf.web.gen.yaml buf.build/googleapis/googleapis
	PATH=$(PATH_WITH_TOOLS) buf generate --template ./etc/buf.web.gen.yaml buf.build/erdaniels/gostream
	cd web/frontend && npm ci --audit=false && npm run rollup

lint: lint-go

lint-go: tool-install
	PATH=$(PATH_WITH_TOOLS) buf --timeout 5m0s lint
	PATH=$(PATH_WITH_TOOLS) buf --timeout 5m0s format -w
	export pkgs="`go list -f '{{.Dir}}' ./... | grep -v gen | grep -v proto`" && echo "$$pkgs" | xargs go vet -vettool=$(TOOL_BIN)/combined
	export pkgs="`go list -f '{{.Dir}}' ./... | grep -v gen | grep -v proto`" && echo "$$pkgs" | xargs $(TOOL_BIN)/golangci-lint run -v --fix --config=./etc/.golangci.yaml

lint-web: buf-web
	cd web/frontend && npm ci --audit=false && npm run rollup && npm run lint

cover:
	PATH=$(PATH_WITH_TOOLS) ./etc/test.sh cover

test: test-go test-web

test-go:
	./etc/test.sh

test-web: build-web
	cd web/frontend && npm run test:unit

# test.short skips tests requiring external hardware (motors/servos)
test-pi:
	go test -c -o $(BIN_OUTPUT_PATH)/test-pi go.viam.com/rdk/component/board/pi/impl
	sudo $(BIN_OUTPUT_PATH)/test-pi -test.short -test.v

server:
	go build $(LDFLAGS) -o $(BIN_OUTPUT_PATH)/server web/cmd/server/main.go

clean-all:
	git clean -fxd

include *.make
