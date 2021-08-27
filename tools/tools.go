//go:build tools
// +build tools

// Package tools defines helper build time tooling needed by the codebase.
package tools

import (
	_ "github.com/edaniels/golinters/cmd/combined"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/polyfloyd/go-errorlint"
	_ "golang.org/x/tools/cmd/goimports"

	// gRPC
	_ "github.com/fullstorydev/grpcurl/cmd/grpcurl"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
)
