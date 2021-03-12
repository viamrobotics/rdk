
goformat:
	gofmt -s -w .

format: goformat
	clang-format -i --style="{BasedOnStyle: Google, IndentWidth: 4}" `find samples utils arduino -iname "*.cpp" -or -iname "*.h" -or -iname "*.ino"`

setup:
	bash etc/setup.sh

build:
	go build -v ./...

lint: goformat
	go get -u github.com/edaniels/golinters/cmd/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go vet -vettool=`go env GOPATH`/bin/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v

cover:
	go test -cpu=1 -parallel=1 -short -v -coverprofile=coverage.txt -covermode=atomic -coverpkg=./... ./...

test:
	go test -v ./...

dockerlocal:
	docker build -f Dockerfile.fortest -t 'echolabs/robotcoretest:latest' .

docker: dockerlocal
	docker push 'echolabs/robotcoretest:latest'

minirover2: 
	go build -o minirover2 samples/minirover2/control.go samples/minirover2/util.go

python-macos:
	sudo mkdir -p /usr/local/lib/pkgconfig
	sudo cp etc/darwin/python-2.7.pc /usr/local/lib/pkgconfig/
