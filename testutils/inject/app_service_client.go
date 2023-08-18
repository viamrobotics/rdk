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
