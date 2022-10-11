# CustomComponents

This example demonstrates several ways viam-server can be extended with custom components. It contains several elements.

## Elements

### gizmoapi
Custom (component) api/protocol called "Gizmo" (acme:component:gizmo).

### mygizmo
A specific model (acme:demo:mygizmo) that implements the Gizmo API.

### mybase
Custom component (acme:demo:mybase) that implements Viam's built-in Base API (rdk:builtin:base) and in turn depends on two secondary "real" motors from the parent robot (only works in the module method below.)

### client
Test client that uses/tests the Gizmo api

### server
Standalone robot server (for use as a "remote") which serves serves a Gizmo and Motor.

### module
The lighter weight (and preferred method) to integrate custom components.

## Running

### Remote Server
* Run the server implementing a Gizmo `go run server/server.go -config server/config.json`.
* Run a standard server connecting to the previous server as a remote `cd ../../ && go run web/cmd/server/main.go -config examples/customcomponents/remote.json`.
* Run the client that knows about a Gizmo but talks to it via server with remote `go run client/client.go`.

### Modular Resource
* Build the module `go build -o module/module ./module/module.go`
* Run a standard server, but with the module configuration `cd ../../ && go run web/cmd/server/main.go -config examples/customcomponents/module.json`.
* Run the client `go run client/client.go`.
