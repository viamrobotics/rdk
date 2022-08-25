# MyComponent

This sample demonstrates a user defining a new resource component subtype called MyComponent.

## Running

* Run the server implementing a MyComponent `go run server/server.go`.
* Run a standard server connecting to the previous server as a remote `cd ../../ && go run web/cmd/server/main.go samples/mycomponent/remote.json`.
* Run the client that knows about a MyComponent but talks to it via server with remote `go run client/client.go`.
