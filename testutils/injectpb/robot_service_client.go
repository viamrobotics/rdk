// Package injectpb provides dependency injected structures for mocking protobuf
// interfaces.
package injectpb

import (
	"context"

	pb "go.viam.com/api/app/v1"
	"google.golang.org/grpc"
)

// RobotServiceClient is an injected RobotServiceClient.
type RobotServiceClient struct {
	pb.RobotServiceClient
	ConfigFunc       func(ctx context.Context, in *pb.ConfigRequest, opts ...grpc.CallOption) (*pb.ConfigResponse, error)
	CertificateFunc  func(ctx context.Context, in *pb.CertificateRequest, opts ...grpc.CallOption) (*pb.CertificateResponse, error)
	LogFunc          func(ctx context.Context, in *pb.LogRequest, opts ...grpc.CallOption) (*pb.LogResponse, error)
	NeedsRestartFunc func(ctx context.Context, in *pb.NeedsRestartRequest, opts ...grpc.CallOption) (*pb.NeedsRestartResponse, error)
}

// Config calls the injected ConfigFunc or the real version.
func (rsc *RobotServiceClient) Config(ctx context.Context, in *pb.ConfigRequest, opts ...grpc.CallOption) (*pb.ConfigResponse, error) {
	if rsc.ConfigFunc == nil {
		return rsc.Config(ctx, in, opts...)
	}
	return rsc.ConfigFunc(ctx, in, opts...)
}

// Certificate calls the injected CertificateFunc or the real version.
func (rsc *RobotServiceClient) Certificate(
	ctx context.Context,
	in *pb.CertificateRequest,
	opts ...grpc.CallOption,
) (*pb.CertificateResponse, error) {
	if rsc.CertificateFunc == nil {
		return rsc.Certificate(ctx, in, opts...)
	}
	return rsc.CertificateFunc(ctx, in, opts...)
}

// Log calls the injected LogFunc or the real version.
func (rsc *RobotServiceClient) Log(
	ctx context.Context,
	in *pb.LogRequest,
	opts ...grpc.CallOption,
) (*pb.LogResponse, error) {
	if rsc.LogFunc == nil {
		return rsc.Log(ctx, in, opts...)
	}
	return rsc.LogFunc(ctx, in, opts...)
}

// NeedsRestart calls the injected NeedsRestartFunc or the real version.
func (rsc *RobotServiceClient) NeedsRestart(ctx context.Context, in *pb.NeedsRestartRequest, opts ...grpc.CallOption) (*pb.NeedsRestartResponse, error) {
	if rsc.NeedsRestartFunc == nil {
		return rsc.NeedsRestart(ctx, in, opts...)
	}
	return rsc.NeedsRestartFunc(ctx, in, opts...)
}
