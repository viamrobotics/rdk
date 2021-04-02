
goformat:
	gofmt -s -w .

format: goformat
	clang-format -i --style="{BasedOnStyle: Google, IndentWidth: 4}" `find samples utils arduino -iname "*.cpp" -or -iname "*.h" -or -iname "*.ino"`

setup:
	bash etc/setup.sh

build: buf build-web
	go build ./...

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
	go test -cpu=1 -parallel=1 -coverprofile=coverage.txt -covermode=atomic -coverpkg=./... ./...

test:
	go test ./...

testpi:
	sudo go test -tags pi -coverprofile=coverage.txt -covermode=atomic -coverpkg=./... go.viam.com/robotcore/board/pi

dockerlocal:
	docker build -f Dockerfile.fortest -t 'echolabs/robotcoretest:latest' .

docker: dockerlocal
	docker push 'echolabs/robotcoretest:latest'

python-macos:
	sudo mkdir -p /usr/local/lib/pkgconfig
	sudo cp etc/darwin/python-2.7.pc /usr/local/lib/pkgconfig/

piserver:
	go build -tags=pi -o server robot/cmd/server/main.go

boat: samples/boat1/cmd.go
	go build -tags=pi -o boat samples/boat1/cmd.go
