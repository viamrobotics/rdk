.PHONY: module

default: run-module

module:
	go build ./

run-module: module
	go run ../../../../web/cmd/server/main.go -config module.json

run-client:
	cd client && make
