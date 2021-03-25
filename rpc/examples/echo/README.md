# Example gRPC Echo Server

This example server demonstrates how to run gRPC accessible via `grpc`, `grpc-web`, and `grpc-gateway` all on the same port while hosting other HTTP services.

## Build

`make build`

## Run

`go run main.go`

## Using

1. Go to [http://localhost:8080](http://localhost:8080) and look at the developer console.
1. Try `curl -XPOST http://localhost:8080/api/v1/echo\?message\=foo`
1. Try `grpcurl -plaintext -d='{"message": "hey"}' localhost:8080 proto.rpc.examples.echo.v1.EchoService/Echo`
