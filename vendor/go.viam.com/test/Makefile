goformat:
	gofmt -s -w .

lint: goformat
	go install github.com/edaniels/golinters/cmd/combined
	go list -f '{{.Dir}}' ./... | grep -Ev '(gen|mmal)' | xargs go vet -vettool=`go env GOPATH`/bin/combined
	go list -f '{{.Dir}}' ./... | grep -Ev '(gen|mmal)' | xargs go run github.com/golangci/golangci-lint/cmd/golangci-lint run -v

test:
	go test ./...
