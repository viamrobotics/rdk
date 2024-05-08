.PHONY: module

default: run-module

module:
	go build -o summationmodule/summationmodule ./summationmodule
	go build -o gizmomodule/gizmomodule ./gizmomodule

run-module:
	go run ../../../../web/cmd/server/main.go -config module.json

run-client:
	cd client && make
