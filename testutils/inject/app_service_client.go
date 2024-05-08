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
	ListLocationsFunc func(ctx context.Context, in *apppb.ListLocationsRequest,
		opts ...grpc.CallOption) (*apppb.ListLocationsResponse, error)
	ListRobotsFunc func(ctx context.Context, in *apppb.ListRobotsRequest,
		opts ...grpc.CallOption) (*apppb.ListRobotsResponse, error)
	CreateKeyFunc func(ctx context.Context, in *apppb.CreateKeyRequest,
		opts ...grpc.CallOption) (*apppb.CreateKeyResponse, error)
	GetRobotAPIKeysFunc func(ctx context.Context, in *apppb.GetRobotAPIKeysRequest,
		opts ...grpc.CallOption) (*apppb.GetRobotAPIKeysResponse, error)
	GetRobotPartFunc func(ctx context.Context, in *apppb.GetRobotPartRequest,
		opts ...grpc.CallOption) (*apppb.GetRobotPartResponse, error)
	GetRobotPartsFunc func(ctx context.Context, in *apppb.GetRobotPartsRequest,
		opts ...grpc.CallOption) (*apppb.GetRobotPartsResponse, error)
	GetRobotPartLogsFunc func(ctx context.Context, in *apppb.GetRobotPartLogsRequest,
		opts ...grpc.CallOption) (*apppb.GetRobotPartLogsResponse, error)
	UpdateRobotPartFunc func(ctx context.Context, in *apppb.UpdateRobotPartRequest,
		opts ...grpc.CallOption) (*apppb.UpdateRobotPartResponse, error)
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

// ListLocations calls the injected ListLocationsFunc or the real version.
func (asc *AppServiceClient) ListLocations(ctx context.Context, in *apppb.ListLocationsRequest,
	opts ...grpc.CallOption,
) (*apppb.ListLocationsResponse, error) {
	if asc.ListLocationsFunc == nil {
		return asc.AppServiceClient.ListLocations(ctx, in, opts...)
	}
	return asc.ListLocationsFunc(ctx, in, opts...)
}

// ListRobots calls the injected ListRobotsFunc or the real version.
func (asc *AppServiceClient) ListRobots(ctx context.Context, in *apppb.ListRobotsRequest,
	opts ...grpc.CallOption,
) (*apppb.ListRobotsResponse, error) {
	if asc.ListRobotsFunc == nil {
		return asc.AppServiceClient.ListRobots(ctx, in, opts...)
	}
	return asc.ListRobotsFunc(ctx, in, opts...)
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

// GetRobotAPIKeys wraps GetRobotAPIKeys.
func (asc *AppServiceClient) GetRobotAPIKeys(ctx context.Context, in *apppb.GetRobotAPIKeysRequest,
	opts ...grpc.CallOption,
) (*apppb.GetRobotAPIKeysResponse, error) {
	if asc.GetRobotAPIKeysFunc == nil {
		return asc.AppServiceClient.GetRobotAPIKeys(ctx, in, opts...)
	}
	return asc.GetRobotAPIKeysFunc(ctx, in, opts...)
}

// GetRobotPart wraps GetRobotPart.
func (asc *AppServiceClient) GetRobotPart(ctx context.Context, in *apppb.GetRobotPartRequest,
	opts ...grpc.CallOption,
) (*apppb.GetRobotPartResponse, error) {
	if asc.GetRobotPartFunc == nil {
		return asc.AppServiceClient.GetRobotPart(ctx, in, opts...)
	}
	return asc.GetRobotPartFunc(ctx, in, opts...)
}

// UpdateRobotPart wraps UpdateRobotPart.
func (asc *AppServiceClient) UpdateRobotPart(ctx context.Context, in *apppb.UpdateRobotPartRequest,
	opts ...grpc.CallOption,
) (*apppb.UpdateRobotPartResponse, error) {
	if asc.GetRobotPartFunc == nil {
		return asc.AppServiceClient.UpdateRobotPart(ctx, in, opts...)
	}
	return asc.UpdateRobotPartFunc(ctx, in, opts...)
}

// GetRobotParts calls the injected GetRobotPartsFunc or the real version.
func (asc *AppServiceClient) GetRobotParts(ctx context.Context, in *apppb.GetRobotPartsRequest,
	opts ...grpc.CallOption,
) (*apppb.GetRobotPartsResponse, error) {
	if asc.GetRobotPartsFunc == nil {
		return asc.AppServiceClient.GetRobotParts(ctx, in, opts...)
	}
	return asc.GetRobotPartsFunc(ctx, in, opts...)
}

// GetRobotPartLogs calls the injected GetRobotPartLogsFunc or the real version.
func (asc *AppServiceClient) GetRobotPartLogs(ctx context.Context, in *apppb.GetRobotPartLogsRequest,
	opts ...grpc.CallOption,
) (*apppb.GetRobotPartLogsResponse, error) {
	if asc.GetRobotPartLogsFunc == nil {
		return asc.AppServiceClient.GetRobotPartLogs(ctx, in, opts...)
	}
	return asc.GetRobotPartLogsFunc(ctx, in, opts...)
}
