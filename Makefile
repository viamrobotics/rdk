BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)

PATH_WITH_TOOLS="`pwd`/bin:`pwd`/node_modules/.bin:${PATH}"

VERSION := $(shell git fetch --tags && git tag --sort=-version:refname | head -n 1)
GIT_REVISION := $(shell git rev-parse HEAD | tr -d '\n')
LDFLAGS = -ldflags "-X 'go.viam.com/rdk/config.Version=${VERSION}' -X 'go.viam.com/rdk/config.GitRevision=${GIT_REVISION}'"

default: build lint server

setup:
	bash etc/setup.sh

build: build-web build-go

build-go: buf-go
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) ./...

build-web: buf-web
	cd web/frontend/dls && npm ci && npm run build:prod
	cd web/frontend && npm ci && npx webpack

tool-install:
	GOBIN=`pwd`/bin go install google.golang.org/protobuf/cmd/protoc-gen-go \
		github.com/bufbuild/buf/cmd/buf \
		github.com/bufbuild/buf/cmd/protoc-gen-buf-breaking \
		github.com/bufbuild/buf/cmd/protoc-gen-buf-lint \
		github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc \
		google.golang.org/grpc/cmd/protoc-gen-go-grpc \
		github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
		github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
		github.com/edaniels/golinters/cmd/combined \
		github.com/golangci/golangci-lint/cmd/golangci-lint

buf: buf-go buf-web

buf-go: tool-install
	PATH=$(PATH_WITH_TOOLS) buf lint
	PATH=$(PATH_WITH_TOOLS) buf generate

buf-web: tool-install
	npm install
	PATH=$(PATH_WITH_TOOLS) buf lint
	PATH=$(PATH_WITH_TOOLS) buf generate --template ./etc/buf.web.gen.yaml
	PATH=$(PATH_WITH_TOOLS) buf generate --timeout 5m --template ./etc/buf.web.gen.yaml buf.build/googleapis/googleapis
	PATH=$(PATH_WITH_TOOLS) buf generate --template ./etc/buf.web.gen.yaml buf.build/erdaniels/gostream

lint: lint-go lint-web

lint-go: tool-install
	PATH=$(PATH_WITH_TOOLS) buf lint
	export pkgs="`go list -f '{{.Dir}}' ./... | grep -v gen | grep -v proto`" && echo "$$pkgs" | xargs go vet -vettool=bin/combined
	export pkgs="`go list -f '{{.Dir}}' ./... | grep -v gen | grep -v proto`" && echo "$$pkgs" | xargs bin/golangci-lint run -v --fix --config=./etc/.golangci.yaml

lint-web:
	cd web/frontend/dls && npm run lint
	cd web/frontend && npm run lint

cover:
	./etc/test.sh cover

test: test-go test-web

test-go:
	./etc/test.sh

test-web: build-web
	cd web/frontend/dls && npm run test:unit

testpi:
	sudo CGO_LDFLAGS=$(CGO_LDFLAGS) go test $(TAGS) -coverprofile=coverage.txt go.viam.com/rdk/component/board/pi

server:
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) $(LDFLAGS) -o $(BIN_OUTPUT_PATH)/server web/cmd/server/main.go

clean-all:
	git clean -fxd

include *.make
