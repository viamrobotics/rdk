
BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)

TAGS = $(shell sh etc/gotags.sh)

binsetup:
	mkdir -p ${BIN_OUTPUT_PATH}

goformat:
	go install golang.org/x/tools/cmd/goimports
	gofmt -s -w .
	goimports -w -local=go.viam.com/core `go list -f '{{.Dir}}' ./... | grep -Ev "proto"`

format: goformat
	clang-format -i --style="{BasedOnStyle: Google, IndentWidth: 4}" `find samples utils -iname "*.cpp" -or -iname "*.h" -or -iname "*.ino"`

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
	buf generate --template ./etc/buf.web.gen.yaml buf.build/beta/googleapis

lint: goformat
	go install google.golang.org/protobuf/cmd/protoc-gen-go \
      google.golang.org/grpc/cmd/protoc-gen-go-grpc \
      github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
      github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc
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
	docker build -f etc/Dockerfile.fortest -t 'echolabs/robotcoretest:latest' .

docker: dockerlocal
	docker push 'echolabs/robotcoretest:latest'

python-macos:
	sudo mkdir -p /usr/local/lib/pkgconfig
	sudo cp etc/darwin/python-2.7.pc /usr/local/lib/pkgconfig/

server:
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/server web/cmd/server/main.go

cameras:
	cd etc/camera_servers && make royaleserver
	cd etc/camera_servers && make intelrealserver

deb-server: server cameras
	rm -rf etc/packaging/work/
	mkdir etc/packaging/work/
	cp -r etc/packaging/viam-server-0.2/ etc/packaging/work/
	install -D $(BIN_OUTPUT_PATH)/server etc/packaging/work/viam-server-0.2/usr/bin/viam-server
	install -D etc/camera_servers/intelrealserver etc/packaging/work/viam-server-0.2/usr/bin/intelrealserver
	install -D etc/camera_servers/royaleserver etc/packaging/work/viam-server-0.2/usr/bin/royaleserver
	install -m 644 -D web/runtime-shared/templates/* --target-directory=etc/packaging/work/viam-server-0.2/usr/share/viam/templates/
	install -m 644 -D web/runtime-shared/static/* --target-directory=etc/packaging/work/viam-server-0.2/usr/share/viam/static/
	install -m 644 -D web/runtime-shared/static/third-party/* --target-directory=etc/packaging/work/viam-server-0.2/usr/share/viam/static/third-party
	cd etc/packaging/work/viam-server-0.2/ \
	&& dch -v 0.2+`date -u '+%Y%m%d%H%M'` "Auto-build from commit `git log --pretty=format:'%h' -n 1`" \
	&& dch -r viam \
	&& dpkg-buildpackage -us -uc -b \

deb-install: deb-server
	sudo dpkg -i etc/packaging/work/viam-server_0.2+*.deb

boat: samples/boat1/cmd.go
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/boat samples/boat1/cmd.go

