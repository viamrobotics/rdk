.PHONY: module

default: run-module

module:
	go build -o simplemodule ./ 

run-module: module
	go run ../../../../web/cmd/server/main.go -config module.json

run-client:
	cd client && make
