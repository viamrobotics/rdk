.PHONY: protobuf

default: protobuf

bin/buf bin/protoc-gen-go bin/protoc-gen-grpc-gateway bin/protoc-gen-go-grpc:
	GOBIN=$(shell pwd)/bin go install \
		github.com/bufbuild/buf/cmd/buf \
		google.golang.org/protobuf/cmd/protoc-gen-go \
		github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
		google.golang.org/grpc/cmd/protoc-gen-go-grpc

protobuf: api/component/gizmo/v1/gizmo.proto api/service/summation/v1/summation.proto bin/buf bin/protoc-gen-go bin/protoc-gen-grpc-gateway bin/protoc-gen-go-grpc
	PATH="$(shell pwd)/bin" buf generate
