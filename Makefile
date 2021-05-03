
BIN_OUTPUT_PATH = bin/$(shell uname -s)-$(shell uname -m)

TAGS = $(shell sh etc/gotags.sh)

binsetup:
	mkdir -p ${BIN_OUTPUT_PATH}

goformat:
	gofmt -s -w .

format: goformat
	clang-format -i --style="{BasedOnStyle: Google, IndentWidth: 4}" `find samples utils -iname "*.cpp" -or -iname "*.h" -or -iname "*.ino"`

setup:
	bash etc/setup.sh

build: buf build-web build-go

build-go:
	go build $(TAGS) ./...

build-web:
	cd robot/web/frontend && npm install && npx webpack

buf:
	buf lint
	buf generate
	buf generate --template buf.web.gen.yaml buf.build/beta/googleapis

lint: goformat
	go install google.golang.org/protobuf/cmd/protoc-gen-go \
      google.golang.org/grpc/cmd/protoc-gen-go-grpc \
      github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway
	buf lint
	go install github.com/edaniels/golinters/cmd/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go vet -vettool=`go env GOPATH`/bin/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v

cover:
	go test -race -coverprofile=coverage.txt `go list ./... | grep -Ev "go.viam.com/robotcore/(vision|rimage)"`
	go test -coverprofile=coverage2.txt go.viam.com/robotcore/vision/... go.viam.com/robotcore/rimage/...
	sed '1d' coverage2.txt >> coverage.txt

test:
	go test -race `go list ./... | grep -Ev "go.viam.com/robotcore/(vision|rimage)"`
	go test go.viam.com/robotcore/vision/... go.viam.com/robotcore/rimage/...

testpi:
	sudo go test $(TAGS) -race -coverprofile=coverage.txt go.viam.com/robotcore/board/pi

dockerlocal:
	docker build -f Dockerfile.fortest -t 'echolabs/robotcoretest:latest' .

docker: dockerlocal
	docker push 'echolabs/robotcoretest:latest'

python-macos:
	sudo mkdir -p /usr/local/lib/pkgconfig
	sudo cp etc/darwin/python-2.7.pc /usr/local/lib/pkgconfig/

server:
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/server robot/cmd/server/main.go

deb-server: server
	rm -rf packaging/work/
	mkdir packaging/work/
	cp -r packaging/viam-server-0.1/ packaging/work/
	install -D $(BIN_OUTPUT_PATH)/server packaging/work/viam-server-0.1/usr/bin/viam-server
	install -m 644 -D robot/web/runtime-shared/templates/* --target-directory=packaging/work/viam-server-0.1/usr/share/viam/templates/
	install -m 644 -D robot/web/runtime-shared/static/* --target-directory=packaging/work/viam-server-0.1/usr/share/viam/static/
	cd packaging/work/viam-server-0.1/ \
	&& dch -v 0.1+`date -u '+%Y%m%d%H%M'` "Auto-build from commit `git log --pretty=format:'%h' -n 1`" \
	&& dch -r viam \
	&& dpkg-buildpackage -us -uc -b \

deb-install: deb-server
	sudo dpkg -i packaging/work/viam-server_0.1+*.deb

boat: samples/boat1/cmd.go
	go build $(TAGS) -o $(BIN_OUTPUT_PATH)/boat samples/boat1/cmd.go

