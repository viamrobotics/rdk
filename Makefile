
BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)

ifneq ("$(wildcard /etc/rpi-issue)","")
   $(info Raspberry Pi Detected)
   export CGO_LDFLAGS = -lwasmer
   TAGS = -tags="pi custom_wasmer_runtime"
endif

SERVER_DEB_VER = 0.3

binsetup:
	mkdir -p ${BIN_OUTPUT_PATH}

goformat:
	go install golang.org/x/tools/cmd/goimports
	gofmt -s -w .
	`go env GOPATH`/bin/goimports -w -local=go.viam.com/core `go list -f '{{.Dir}}' ./... | grep -Ev "proto"`

setup:
	bash etc/setup.sh

build: buf build-web build-go

build-go:
	go build $(TAGS) ./...

build-web:
	cd web/frontend && npm install && npx webpack

buf:
	buf lint
	buf generate
	buf generate --template ./etc/buf.web.gen.yaml buf.build/beta/googleapis:1c473ad9220a49bca9320f4cc690eba5

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
	sudo go test $(TAGS) -race -coverprofile=coverage.txt go.viam.com/core/board/pi

dockerlocal:
	docker build -f etc/Dockerfile.fortest --no-cache -t 'echolabs/robotcoretest:latest' .

docker: dockerlocal
	docker push 'echolabs/robotcoretest:latest'

server:
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/server web/cmd/server/main.go

cameras:
	cd etc/camera_servers && make royaleserver
	cd etc/camera_servers && make intelrealserver

deb-server: server cameras
	rm -rf etc/packaging/work/
	mkdir etc/packaging/work/
	cp -r etc/packaging/viam-server-$(SERVER_DEB_VER)/ etc/packaging/work/
	install -D $(BIN_OUTPUT_PATH)/server etc/packaging/work/viam-server-$(SERVER_DEB_VER)/usr/bin/viam-server
	install -D etc/camera_servers/intelrealserver etc/packaging/work/viam-server-$(SERVER_DEB_VER)/usr/bin/intelrealserver
	install -D etc/camera_servers/royaleserver etc/packaging/work/viam-server-$(SERVER_DEB_VER)/usr/bin/royaleserver
	install -m 644 -D web/runtime-shared/templates/* --target-directory=etc/packaging/work/viam-server-$(SERVER_DEB_VER)/usr/share/viam/templates/
	install -m 644 -D web/runtime-shared/static/control.js etc/packaging/work/viam-server-$(SERVER_DEB_VER)/usr/share/viam/static/control.js
	install -m 644 -D web/runtime-shared/static/third-party/vue.js etc/packaging/work/viam-server-$(SERVER_DEB_VER)/usr/share/viam/static/third-party/vue.js
	install -m 644 -D web/runtime-shared/static/third-party/ace/snippets/* --target-directory=etc/packaging/work/viam-server-$(SERVER_DEB_VER)/usr/share/viam/static/third-party/ace/snippets
	install -m 644 -D web/runtime-shared/static/third-party/ace/*.js --target-directory=etc/packaging/work/viam-server-$(SERVER_DEB_VER)/usr/share/viam/static/third-party/ace
	cd etc/packaging/work/viam-server-$(SERVER_DEB_VER)/ \
	&& dch -v $(SERVER_DEB_VER)+`date -u '+%Y%m%d%H%M'` "Auto-build from commit `git log --pretty=format:'%h' -n 1`" \
	&& dch -r viam \
	&& dpkg-buildpackage -us -uc -b \

deb-install: deb-server
	sudo dpkg -i etc/packaging/work/viam-server_$(SERVER_DEB_VER)+*.deb

boat: samples/boat1/cmd.go
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/boat samples/boat1/cmd.go

boat2: samples/boat2/cmd.go
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/boat2 samples/boat2/cmd.go

resetbox: samples/resetbox/cmd.go
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/resetbox samples/resetbox/cmd.go
