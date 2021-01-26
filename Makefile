
format:
	gofmt -s -w .
	clang-format -i --style="{BasedOnStyle: Google, IndentWidth: 4}" `find samples utils arduino -iname "*.cpp" -or -iname "*.h" -or -iname "*.ino"`

lint:
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v
	go get -u github.com/edaniels/golinters/cmd/combined
	go list -f '{{.Dir}}' ./... | grep -v gen | xargs go vet -vettool=${GOPATH}/bin/combined

docker:
	docker build -f Dockerfile.fortest -t 'echolabs/robotcoretest:latest' .
	docker push 'echolabs/robotcoretest:latest'

minirover2: 
	go build -o minirover2 samples/minirover2/control.go samples/minirover2/util.go

python-macos:
	cp etc/darwin/python-2.7.pc /usr/local/lib/pkgconfig/
