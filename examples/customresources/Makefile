.PHONY: module buf

buf:
	cd ../../ && make tool-install
	PATH="$(PATH):$(shell pwd)/../../bin/gotools/$(shell uname -s)-$(shell uname -m)" buf generate

module:
	go build -o module/module ./module/module.go

run-remote:
	go run server/server.go -config server/config.json

run-parent:
	cd ../../ && go run web/cmd/server/main.go -config examples/customcomponents/remote.json

run-module: module
	cd ../../ && go run web/cmd/server/main.go -config examples/customcomponents/module.json

run-client:
	go run client/client.go
