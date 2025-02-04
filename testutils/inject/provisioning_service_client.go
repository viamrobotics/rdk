package inject

import (
	"context"

	provisioningPb "go.viam.com/api/provisioning/v1"
	"google.golang.org/grpc"
)

// ProvisioningServiceClient represents a fake instance of a provisioning client.
type ProvisioningServiceClient struct {
	provisioningPb.ProvisioningServiceClient
	GetSmartMachineStatusFunc func(ctx context.Context, in *provisioningPb.GetSmartMachineStatusRequest,
		opts ...grpc.CallOption) (*provisioningPb.GetSmartMachineStatusResponse, error)
	SetNetworkCredentialsFunc func(ctx context.Context, in *provisioningPb.SetNetworkCredentialsRequest,
		opts ...grpc.CallOption) (*provisioningPb.SetNetworkCredentialsResponse, error)
	SetSmartMachineCredentialsFunc func(ctx context.Context, in *provisioningPb.SetSmartMachineCredentialsRequest,
		opts ...grpc.CallOption) (*provisioningPb.SetSmartMachineCredentialsResponse, error)
	GetNetworkListFunc func(ctx context.Context, in *provisioningPb.GetNetworkListRequest,
		opts ...grpc.CallOption) (*provisioningPb.GetNetworkListResponse, error)
}

// GetSmartMachineStatus calls the injected GetSmartMachineStatusFunc or the real version.
func (psc *ProvisioningServiceClient) GetSmartMachineStatus(ctx context.Context, in *provisioningPb.GetSmartMachineStatusRequest,
	opts ...grpc.CallOption,
) (*provisioningPb.GetSmartMachineStatusResponse, error) {
	if psc.GetSmartMachineStatusFunc == nil {
		return psc.ProvisioningServiceClient.GetSmartMachineStatus(ctx, in, opts...)
	}
	return psc.GetSmartMachineStatusFunc(ctx, in, opts...)
}

// SetNetworkCredentials calls the injected SetNetworkCredentialsFunc or the real version.
func (psc *ProvisioningServiceClient) SetNetworkCredentials(ctx context.Context, in *provisioningPb.SetNetworkCredentialsRequest,
	opts ...grpc.CallOption,
) (*provisioningPb.SetNetworkCredentialsResponse, error) {
	if psc.SetNetworkCredentialsFunc == nil {
		return psc.ProvisioningServiceClient.SetNetworkCredentials(ctx, in, opts...)
	}
	return psc.SetNetworkCredentialsFunc(ctx, in, opts...)
}

// SetSmartMachineCredentials calls the injected SetSmartMachineCredentialsFunc or the real version.
func (psc *ProvisioningServiceClient) SetSmartMachineCredentials(ctx context.Context, in *provisioningPb.SetSmartMachineCredentialsRequest,
	opts ...grpc.CallOption,
) (*provisioningPb.SetSmartMachineCredentialsResponse, error) {
	if psc.SetSmartMachineCredentialsFunc == nil {
		return psc.ProvisioningServiceClient.SetSmartMachineCredentials(ctx, in, opts...)
	}
	return psc.SetSmartMachineCredentialsFunc(ctx, in, opts...)
}

// GetNetworkList calls the injected GetNetworkListFunc or the real version.
func (psc *ProvisioningServiceClient) GetNetworkList(ctx context.Context, in *provisioningPb.GetNetworkListRequest,
	opts ...grpc.CallOption,
) (*provisioningPb.GetNetworkListResponse, error) {
	if psc.GetNetworkListFunc == nil {
		return psc.ProvisioningServiceClient.GetNetworkList(ctx, in, opts...)
	}
	return psc.GetNetworkListFunc(ctx, in, opts...)
}
