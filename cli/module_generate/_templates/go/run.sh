#!/bin/sh
cd `dirname $0`

MODULE=$(basename "$PWD")
export PATH=$PATH:$(go env GOPATH)/bin


echo "Downloading necessary go packages..."
if ! (
    go get go.viam.com/rdk@latest  > /dev/null 2>&1
    go get golang.org/x/tools/cmd/goimports@latest  > /dev/null 2>&1
    gofmt -w -s .
    go mod tidy  > /dev/null 2>&1
); then
    echo "Go packages could not be installed. Quitting..." >&2
    exit 1
fi
# entrypoint is bin/$MODULE as specified in meta.json
go build -o bin/$MODULE main.go

# tar czf module.tar.gz bin/$MODULE
echo "Starting module..."
exec go run main.go $@
