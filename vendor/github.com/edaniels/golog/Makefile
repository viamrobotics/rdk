goformat:
	gofmt -s -w .

lint: goformat
	go get -u github.com/edaniels/golinters/cmd/combined
	go vet -vettool=`go env GOPATH`/bin/combined ./...
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v ./...

test:
	go test -race ./...
