.PHONY: module buf

buf:
	cd apis/proto && make

module:
	go build -o module/module ./module/module.go

run-remote:
	go run server/server.go -config server/remote.json

run-parent:
	cd ../../ && go run web/cmd/server/main.go -config examples/customresources/server/parent.json

run-module: module
	cd ../../ && go run web/cmd/server/main.go -config examples/customresources/module/module.json

run-client:
	go run client/client.go
