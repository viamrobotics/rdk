
format:
	gofmt -s -w .
	clang-format -i --style="{BasedOnStyle: Google, IndentWidth: 4}" `find . -iname "*.cpp" -o -iname "*.h" -o -iname "*.ino"`
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v ./...

docker:
	docker build -f Dockerfile.fortest -t 'echolabs/robotcoretest:latest' .
	docker push 'echolabs/robotcoretest:latest'

minirover2: 
	go build -o minirover2 samples/minirover2/control.go samples/minirover2/util.go

