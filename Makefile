BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)
ENTRYCMD = --testbot-uid $(shell id -u) --testbot-gid $(shell id -g)

GREPPED = $(shell grep -sao jetson /proc/device-tree/compatible)
ifneq ("$(strip $(GREPPED))", "")
   $(info Nvidia Jetson Detected)
   SERVER_DEB_PLATFORM = jetson
else ifneq ("$(wildcard /etc/rpi-issue)","")
   $(info Raspberry Pi Detected)
   SERVER_DEB_PLATFORM = pi
else
   SERVER_DEB_PLATFORM = generic
endif
SERVER_DEB_VER = 0.5

# Linux always needs custom_wasmer_runtime for portability in packaging
ifeq ("$(shell uname -s)", "Linux")
	CGO_LDFLAGS = -lwasmer
	TAGS = -tags="custom_wasmer_runtime"
endif
ifeq ("$(DOCKER_NESTED)", "")
	DOCKER_WORKSPACE=`pwd`
else
	DOCKER_WORKSPACE=$(shell docker container inspect -f '{{range .Mounts}}{{ if eq .Destination "/__w" }}{{.Source}}{{ end }}{{end}}' $(shell hostname) | tr -d '\n')/rdk/rdk
endif
PATH_WITH_TOOLS="`pwd`/bin:`pwd`/node_modules/.bin:${PATH}"

binsetup:
	mkdir -p ${BIN_OUTPUT_PATH}

setup:
	bash etc/setup.sh

build: build-web build-go

build-go: buf-go
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) ./...

build-web: buf-web
	cd web/frontend/core-components && npm install && npm run build:prod
	cd web/frontend && npm install && npx webpack

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
	PATH=$(PATH_WITH_TOOLS) buf generate --template ./etc/buf.web.gen.yaml buf.build/googleapis/googleapis
	PATH=$(PATH_WITH_TOOLS) buf generate --template ./etc/buf.web.gen.yaml buf.build/erdaniels/gostream

lint: tool-install
	PATH=$(PATH_WITH_TOOLS) buf lint
	export pkgs=`go list -f '{{.Dir}}' ./... | grep -v gen | grep -v proto` && echo "$$pkgs" | xargs go vet -vettool=bin/combined
	export pkgs=`go list -f '{{.Dir}}' ./... | grep -v gen | grep -v proto` && echo "$$pkgs" | xargs bin/golangci-lint run -v --fix --config=./etc/.golangci.yaml

cover:
	./etc/test.sh cover

test:
	./etc/test.sh

testpi:
	sudo CGO_LDFLAGS=$(CGO_LDFLAGS) go test $(TAGS) -coverprofile=coverage.txt go.viam.com/rdk/component/board/pi

dockerlocal:
	docker build -f etc/Dockerfile.fortest --no-cache -t 'ghcr.io/viamrobotics/test:latest' .

docker: dockerlocal
	docker push 'ghcr.io/viamrobotics/test:latest'

server:
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) -o $(BIN_OUTPUT_PATH)/server web/cmd/server/main.go

deb-server: buf-go server
	rm -rf etc/packaging/work/
	mkdir etc/packaging/work/
	cp -r etc/packaging/viam-server-$(SERVER_DEB_VER)/ etc/packaging/work/viam-server-$(SERVER_DEB_PLATFORM)-$(SERVER_DEB_VER)/
	install -D $(BIN_OUTPUT_PATH)/server etc/packaging/work/viam-server-$(SERVER_DEB_PLATFORM)-$(SERVER_DEB_VER)/usr/bin/viam-server
	cd etc/packaging/work/viam-server-$(SERVER_DEB_PLATFORM)-$(SERVER_DEB_VER)/ \
	&& sed -i "s/viam-server/viam-server-$(SERVER_DEB_PLATFORM)/g" debian/control debian/changelog \
	&& sed -i "s/viam-camera-servers/viam-camera-servers-$(SERVER_DEB_PLATFORM)/g" debian/control \
	&& dch --force-distribution -D viam -v $(SERVER_DEB_VER)+`date -u '+%Y%m%d%H%M'` "Auto-build from commit `git log --pretty=format:'%h' -n 1`" \
	&& dpkg-buildpackage -us -uc -b \

deb-install: deb-server
	sudo dpkg -i etc/packaging/work/viam-server-$(SERVER_DEB_PLATFORM)_$(SERVER_DEB_VER)+*.deb

boat: samples/boat1/cmd.go
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) -o $(BIN_OUTPUT_PATH)/boat samples/boat1/cmd.go

boat2: samples/boat2/cmd.go
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) -o $(BIN_OUTPUT_PATH)/boat2 samples/boat2/cmd.go

gpstest: samples/gpsTest/cmd.go
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/gpstest samples/gpsTest/cmd.go

resetbox: samples/resetbox/cmd.go
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) -o $(BIN_OUTPUT_PATH)/resetbox samples/resetbox/cmd.go

gamepad: samples/gamepad/cmd.go
	CGO_LDFLAGS=$(CGO_LDFLAGS) go build $(TAGS) -o $(BIN_OUTPUT_PATH)/gamepad samples/gamepad/cmd.go

clean-all:
	rm -rf etc/packaging/work etc/packaging/appimages/deploy etc/packaging/appimages/appimage-builder-cache etc/packaging/appimages/AppDir

appimage: buf-go server
	cd etc/packaging/appimages && appimage-builder --recipe viam-server-`uname -m`.yml
	mkdir -p etc/packaging/appimages/deploy/
	mv etc/packaging/appimages/*.AppImage* etc/packaging/appimages/deploy/
	chmod 755 etc/packaging/appimages/deploy/*.AppImage

# This sets up multi-arch emulation under linux. Run before using multi-arch targets.
docker-emulation:
	docker run --rm --privileged multiarch/qemu-user-static --reset -p yes

appimage-multiarch: appimage-amd64 appimage-arm64

appimage-amd64:
	docker run --platform linux/amd64 -v$(DOCKER_WORKSPACE):/host --workdir /host --rm ghcr.io/viamrobotics/appimage:latest $(ENTRYCMD) make appimage

appimage-arm64:
	docker run --platform linux/arm64 -v$(DOCKER_WORKSPACE):/host --workdir /host --rm ghcr.io/viamrobotics/appimage:latest $(ENTRYCMD) make appimage

appimage-deploy:
	gsutil -m -h "Cache-Control: no-cache" cp etc/packaging/appimages/deploy/* gs://packages.viam.com/apps/viam-server/
