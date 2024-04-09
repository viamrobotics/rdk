.PHONY: module

default: run-module

module:
	go build -o summation ./summationmodule
	go build -o gizmo ./gizmomodule

run-module: module
	go run ../../../../web/cmd/server/main.go -config module.json

run-client:
	cd client && make
