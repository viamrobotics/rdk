
BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)

GREPPED = $(shell grep -sao jetson /proc/device-tree/compatible)
ifneq ("$(strip $(GREPPED))", "")
   $(info Nvidia Jetson Detected)
   export CGO_LDFLAGS = -lwasmer
   TAGS = -tags="jetson custom_wasmer_runtime"
   SERVER_DEB_PLATFORM = jetson
else ifneq ("$(wildcard /etc/rpi-issue)","")
   $(info Raspberry Pi Detected)
   export CGO_LDFLAGS = -lwasmer
   TAGS = -tags="pi custom_wasmer_runtime"
   SERVER_DEB_PLATFORM = pi
else
   SERVER_DEB_PLATFORM = generic
endif

SERVER_DEB_VER = 0.5

binsetup:
	mkdir -p ${BIN_OUTPUT_PATH}

goformat:
	go install golang.org/x/tools/cmd/goimports
	gofmt -s -w .
	`go env GOPATH`/bin/goimports -w -local=go.viam.com/rdk `go list -f '{{.Dir}}' ./... | grep -Ev "proto"`

setup:
	bash etc/setup.sh

build: buf build-web build-go

build-go:
	go build $(TAGS) ./...

build-web:
	cd web/frontend/core-components && npm install && npm run build:prod
	cd web/frontend && npm install && npx webpack

buf:
	buf lint
	buf generate
	buf generate --template ./etc/buf.web.gen.yaml buf.build/googleapis/googleapis
	buf generate --template ./etc/buf.web.gen.yaml buf.build/erdaniels/gostream
	go install golang.org/x/tools/cmd/goimports
	`go env GOPATH`/bin/goimports -w -local=go.viam.com/rdk proto

lint: goformat
	buf lint
	go install github.com/edaniels/golinters/cmd/combined
	go install github.com/golangci/golangci-lint/cmd/golangci-lint
	go install github.com/polyfloyd/go-errorlint
	go list -f '{{.Dir}}' ./... | grep -v gen | grep -v proto | xargs go vet -vettool=`go env GOPATH`/bin/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | grep -v proto | xargs `go env GOPATH`/bin/go-errorlint -errorf
	go list -f '{{.Dir}}' ./... | grep -v gen | grep -v proto | xargs go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v --config=./etc/.golangci.yaml

cover:
	./etc/test.sh cover

test:
	./etc/test.sh

testpi:
	sudo go test $(TAGS) -race -coverprofile=coverage.txt go.viam.com/rdk/board/pi

dockerlocal:
	docker build -f etc/Dockerfile.fortest --no-cache -t 'ghcr.io/viamrobotics/test:latest' .

docker: dockerlocal
	docker push 'ghcr.io/viamrobotics/test:latest'

server:
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/server web/cmd/server/main.go

deb-server: server
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
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/boat samples/boat1/cmd.go

boat2: samples/boat2/cmd.go
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/boat2 samples/boat2/cmd.go

resetbox: samples/resetbox/cmd.go
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/resetbox samples/resetbox/cmd.go

gamepad: samples/gamepad/cmd.go
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/gamepad samples/gamepad/cmd.go
