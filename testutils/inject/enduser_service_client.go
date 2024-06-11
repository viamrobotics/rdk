package inject

import (
	"context"

	apppb "go.viam.com/api/app/v1"
	"google.golang.org/grpc"
)

// EndUserServiceClient represents a fake instance of an end user service client.
type EndUserServiceClient struct {
	apppb.EndUserServiceClient
	RegisterAuthApplicationFunc func(ctx context.Context, in *apppb.RegisterAuthApplicationRequest,
		opts ...grpc.CallOption,
	) (*apppb.RegisterAuthApplicationResponse, error)
	UpdateAuthApplicationFunc func(ctx context.Context, in *apppb.UpdateAuthApplicationRequest,
		opts ...grpc.CallOption,
	) (*apppb.UpdateAuthApplicationResponse, error)
	GetAuthApplicationFunc func(ctx context.Context, in *apppb.GetAuthApplicationRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetAuthApplicationResponse, error)
}

// RegisterAuthApplication calls the injected RegisterAuthApplicationFunc or the real version.
func (eusc *EndUserServiceClient) RegisterAuthApplication(ctx context.Context, in *apppb.RegisterAuthApplicationRequest,
	opts ...grpc.CallOption,
) (*apppb.RegisterAuthApplicationResponse, error) {
	if eusc.RegisterAuthApplicationFunc == nil {
		return eusc.EndUserServiceClient.RegisterAuthApplication(ctx, in, opts...)
	}
	return eusc.RegisterAuthApplicationFunc(ctx, in, opts...)
}

// UpdateAuthApplication calls the injected UpdateAuthApplicationFunc or the real version.
func (eusc *EndUserServiceClient) UpdateAuthApplication(ctx context.Context, in *apppb.UpdateAuthApplicationRequest,
	opts ...grpc.CallOption,
) (*apppb.UpdateAuthApplicationResponse, error) {
	if eusc.UpdateAuthApplicationFunc == nil {
		return eusc.EndUserServiceClient.UpdateAuthApplication(ctx, in, opts...)
	}
	return eusc.UpdateAuthApplicationFunc(ctx, in, opts...)
}

// GetAuthApplication calls the injected GetAuthApplication or the real version.
func (eusc *EndUserServiceClient) GetAuthApplication(ctx context.Context, in *apppb.GetAuthApplicationRequest,
	opts ...grpc.CallOption,
) (*apppb.GetAuthApplicationResponse, error) {
	if eusc.GetAuthApplicationFunc == nil {
		return eusc.EndUserServiceClient.GetAuthApplication(ctx, in, opts...)
	}
	return eusc.GetAuthApplicationFunc(ctx, in, opts...)
}
