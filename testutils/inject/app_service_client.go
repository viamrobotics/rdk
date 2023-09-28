package inject

import (
	"context"

	apppb "go.viam.com/api/app/v1"
	"google.golang.org/grpc"
)

// AppServiceClient represents a fake instance of an app service client.
type AppServiceClient struct {
	apppb.AppServiceClient
	ListOrganizationsFunc func(ctx context.Context, in *apppb.ListOrganizationsRequest,
		opts ...grpc.CallOption) (*apppb.ListOrganizationsResponse, error)
	CreateKeyFunc func(ctx context.Context, in *apppb.CreateKeyRequest,
		opts ...grpc.CallOption) (*apppb.CreateKeyResponse, error)
	GetLocationFunc func(ctx context.Context, in *apppb.GetLocationRequest, opts ...grpc.CallOption) (*apppb.GetLocationResponse, error)
	GetRobotFunc    func(ctx context.Context, in *apppb.GetRobotRequest, opts ...grpc.CallOption) (*apppb.GetRobotResponse, error)
}

// ListOrganizations calls the injected ListOrganizationsFunc or the real version.
func (asc *AppServiceClient) ListOrganizations(ctx context.Context, in *apppb.ListOrganizationsRequest,
	opts ...grpc.CallOption,
) (*apppb.ListOrganizationsResponse, error) {
	if asc.ListOrganizationsFunc == nil {
		return asc.AppServiceClient.ListOrganizations(ctx, in, opts...)
	}
	return asc.ListOrganizationsFunc(ctx, in, opts...)
}

// CreateKey calls the injected CreateKeyFunc or the real version.
func (asc *AppServiceClient) CreateKey(ctx context.Context, in *apppb.CreateKeyRequest,
	opts ...grpc.CallOption,
) (*apppb.CreateKeyResponse, error) {
	if asc.CreateKeyFunc == nil {
		return asc.AppServiceClient.CreateKey(ctx, in, opts...)
	}
	return asc.CreateKeyFunc(ctx, in, opts...)
}

// GetLocation calls the injected GetLocationFunc or the real version.
func (asc *AppServiceClient) GetLocation(ctx context.Context, in *apppb.GetLocationRequest,
	opts ...grpc.CallOption,
) (*apppb.GetLocationResponse, error) {
	if asc.GetLocationFunc == nil {
		return asc.AppServiceClient.GetLocation(ctx, in, opts...)
	}
	return asc.GetLocationFunc(ctx, in, opts...)
}

// GetRobot calls the injected GetRobotFunc or the real version.
func (asc *AppServiceClient) GetRobot(ctx context.Context,
	in *apppb.GetRobotRequest, opts ...grpc.CallOption,
) (*apppb.GetRobotResponse, error) {
	if asc.GetRobotFunc == nil {
		return asc.AppServiceClient.GetRobot(ctx, in, opts...)
	}
	return asc.GetRobotFunc(ctx, in, opts...)
}
