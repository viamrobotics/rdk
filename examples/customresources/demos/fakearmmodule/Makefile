.PHONY: module

default: run-module

module:
	go build ./

run-module: module
	go run ../../../../web/cmd/server -config module.json
