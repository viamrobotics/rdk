package inject

import (
	"context"

	apppb "go.viam.com/api/app/v1"
	"google.golang.org/grpc"
)

// AppServiceClient represents a fake instance of an app service client.
type AppServiceClient struct {
	apppb.AppServiceClient
	GetUserIDByEmailFunc func(ctx context.Context, in *apppb.GetUserIDByEmailRequest,
		opts ...grpc.CallOption) (*apppb.GetUserIDByEmailResponse, error)
	CreateOrganizationFunc func(ctx context.Context, in *apppb.CreateOrganizationRequest,
		opts ...grpc.CallOption) (*apppb.CreateOrganizationResponse, error)
	ListOrganizationsFunc func(ctx context.Context, in *apppb.ListOrganizationsRequest,
		opts ...grpc.CallOption) (*apppb.ListOrganizationsResponse, error)
	GetOrganizationsWithAccessToLocationFunc func(ctx context.Context, in *apppb.GetOrganizationsWithAccessToLocationRequest,
		opts ...grpc.CallOption) (*apppb.GetOrganizationsWithAccessToLocationResponse, error)
	ListOrganizationsByUserFunc func(ctx context.Context, in *apppb.ListOrganizationsByUserRequest,
		opts ...grpc.CallOption) (*apppb.ListOrganizationsByUserResponse, error)
	GetOrganizationFunc func(ctx context.Context, in *apppb.GetOrganizationRequest,
		opts ...grpc.CallOption) (*apppb.GetOrganizationResponse, error)
	GetOrganizationNamespaceAvailabilityFunc func(ctx context.Context, in *apppb.GetOrganizationNamespaceAvailabilityRequest,
		opts ...grpc.CallOption) (*apppb.GetOrganizationNamespaceAvailabilityResponse, error)
	UpdateOrganizationFunc func(ctx context.Context, in *apppb.UpdateOrganizationRequest,
		opts ...grpc.CallOption) (*apppb.UpdateOrganizationResponse, error)
	DeleteOrganizationFunc func(ctx context.Context, in *apppb.DeleteOrganizationRequest,
		opts ...grpc.CallOption) (*apppb.DeleteOrganizationResponse, error)
	ListOrganizationMembersFunc func(ctx context.Context, in *apppb.ListOrganizationMembersRequest,
		opts ...grpc.CallOption) (*apppb.ListOrganizationMembersResponse, error)
	CreateOrganizationInviteFunc func(ctx context.Context, in *apppb.CreateOrganizationInviteRequest,
		opts ...grpc.CallOption) (*apppb.CreateOrganizationInviteResponse, error)
	UpdateOrganizationInviteAuthorizationsFunc func(ctx context.Context, in *apppb.UpdateOrganizationInviteAuthorizationsRequest,
		opts ...grpc.CallOption) (*apppb.UpdateOrganizationInviteAuthorizationsResponse, error)
	DeleteOrganizationMemberFunc func(ctx context.Context, in *apppb.DeleteOrganizationMemberRequest,
		opts ...grpc.CallOption) (*apppb.DeleteOrganizationMemberResponse, error)
	DeleteOrganizationInviteFunc func(ctx context.Context, in *apppb.DeleteOrganizationInviteRequest,
		opts ...grpc.CallOption) (*apppb.DeleteOrganizationInviteResponse, error)
	ResendOrganizationInviteFunc func(ctx context.Context, in *apppb.ResendOrganizationInviteRequest,
		opts ...grpc.CallOption) (*apppb.ResendOrganizationInviteResponse, error)
	EnableBillingServiceFunc func(ctx context.Context, in *apppb.EnableBillingServiceRequest,
		opts ...grpc.CallOption) (*apppb.EnableBillingServiceResponse, error)
	GetBillingServiceConfigFunc func(ctx context.Context, in *apppb.GetBillingServiceConfigRequest,
		opts ...grpc.CallOption) (*apppb.GetBillingServiceConfigResponse, error)
	DisableBillingServiceFunc func(ctx context.Context, in *apppb.DisableBillingServiceRequest,
		opts ...grpc.CallOption) (*apppb.DisableBillingServiceResponse, error)
	UpdateBillingServiceFunc func(ctx context.Context, in *apppb.UpdateBillingServiceRequest,
		opts ...grpc.CallOption) (*apppb.UpdateBillingServiceResponse, error)
	OrganizationSetSupportEmailFunc func(ctx context.Context, in *apppb.OrganizationSetSupportEmailRequest,
		opts ...grpc.CallOption) (*apppb.OrganizationSetSupportEmailResponse, error)
	OrganizationGetSupportEmailFunc func(ctx context.Context, in *apppb.OrganizationGetSupportEmailRequest,
		opts ...grpc.CallOption) (*apppb.OrganizationGetSupportEmailResponse, error)
	OrganizationSetLogoFunc func(ctx context.Context, in *apppb.OrganizationSetLogoRequest,
		opts ...grpc.CallOption) (*apppb.OrganizationSetLogoResponse, error)
	OrganizationGetLogoFunc func(ctx context.Context, in *apppb.OrganizationGetLogoRequest,
		opts ...grpc.CallOption) (*apppb.OrganizationGetLogoResponse, error)
	CreateLocationFunc func(ctx context.Context, in *apppb.CreateLocationRequest,
		opts ...grpc.CallOption) (*apppb.CreateLocationResponse, error)
	GetLocationFunc func(ctx context.Context, in *apppb.GetLocationRequest,
		opts ...grpc.CallOption) (*apppb.GetLocationResponse, error)
	UpdateLocationFunc func(ctx context.Context, in *apppb.UpdateLocationRequest,
		opts ...grpc.CallOption) (*apppb.UpdateLocationResponse, error)
	DeleteLocationFunc func(ctx context.Context, in *apppb.DeleteLocationRequest,
		opts ...grpc.CallOption) (*apppb.DeleteLocationResponse, error)
	ListLocationsFunc func(ctx context.Context, in *apppb.ListLocationsRequest,
		opts ...grpc.CallOption) (*apppb.ListLocationsResponse, error)
	ShareLocationFunc func(ctx context.Context, in *apppb.ShareLocationRequest,
		opts ...grpc.CallOption) (*apppb.ShareLocationResponse, error)
	UnshareLocationFunc func(ctx context.Context, in *apppb.UnshareLocationRequest,
		opts ...grpc.CallOption) (*apppb.UnshareLocationResponse, error)
	LocationAuthFunc func(ctx context.Context, in *apppb.LocationAuthRequest,
		opts ...grpc.CallOption) (*apppb.LocationAuthResponse, error)
	CreateLocationSecretFunc func(ctx context.Context, in *apppb.CreateLocationSecretRequest,
		opts ...grpc.CallOption) (*apppb.CreateLocationSecretResponse, error)
	DeleteLocationSecretFunc func(ctx context.Context, in *apppb.DeleteLocationSecretRequest,
		opts ...grpc.CallOption) (*apppb.DeleteLocationSecretResponse, error)
	GetRobotFunc func(ctx context.Context, in *apppb.GetRobotRequest,
		opts ...grpc.CallOption) (*apppb.GetRobotResponse, error)
	GetRoverRentalRobotsFunc func(ctx context.Context, in *apppb.GetRoverRentalRobotsRequest,
		opts ...grpc.CallOption) (*apppb.GetRoverRentalRobotsResponse, error)
	GetRobotPartsFunc func(ctx context.Context, in *apppb.GetRobotPartsRequest,
		opts ...grpc.CallOption) (*apppb.GetRobotPartsResponse, error)
	GetRobotPartFunc func(ctx context.Context, in *apppb.GetRobotPartRequest,
		opts ...grpc.CallOption) (*apppb.GetRobotPartResponse, error)
	GetRobotPartLogsFunc func(ctx context.Context, in *apppb.GetRobotPartLogsRequest,
		opts ...grpc.CallOption) (*apppb.GetRobotPartLogsResponse, error)
	TailRobotPartLogsFunc func(ctx context.Context, in *apppb.TailRobotPartLogsRequest,
		opts ...grpc.CallOption) (apppb.AppService_TailRobotPartLogsClient, error)
	GetRobotPartHistoryFunc func(ctx context.Context, in *apppb.GetRobotPartHistoryRequest,
		opts ...grpc.CallOption) (*apppb.GetRobotPartHistoryResponse, error)
	UpdateRobotPartFunc func(ctx context.Context, in *apppb.UpdateRobotPartRequest,
		opts ...grpc.CallOption) (*apppb.UpdateRobotPartResponse, error)
	NewRobotPartFunc func(ctx context.Context, in *apppb.NewRobotPartRequest,
		opts ...grpc.CallOption) (*apppb.NewRobotPartResponse, error)
	DeleteRobotPartFunc func(ctx context.Context, in *apppb.DeleteRobotPartRequest,
		opts ...grpc.CallOption) (*apppb.DeleteRobotPartResponse, error)
	GetRobotAPIKeysFunc func(ctx context.Context, in *apppb.GetRobotAPIKeysRequest,
		opts ...grpc.CallOption) (*apppb.GetRobotAPIKeysResponse, error)
	MarkPartAsMainFunc func(ctx context.Context, in *apppb.MarkPartAsMainRequest,
		opts ...grpc.CallOption) (*apppb.MarkPartAsMainResponse, error)
	MarkPartForRestartFunc func(ctx context.Context, in *apppb.MarkPartForRestartRequest,
		opts ...grpc.CallOption) (*apppb.MarkPartForRestartResponse, error)
	CreateRobotPartSecretFunc func(ctx context.Context, in *apppb.CreateRobotPartSecretRequest,
		opts ...grpc.CallOption) (*apppb.CreateRobotPartSecretResponse, error)
	DeleteRobotPartSecretFunc func(ctx context.Context, in *apppb.DeleteRobotPartSecretRequest,
		opts ...grpc.CallOption) (*apppb.DeleteRobotPartSecretResponse, error)
	ListRobotsFunc func(ctx context.Context, in *apppb.ListRobotsRequest,
		opts ...grpc.CallOption) (*apppb.ListRobotsResponse, error)
	NewRobotFunc func(ctx context.Context, in *apppb.NewRobotRequest,
		opts ...grpc.CallOption) (*apppb.NewRobotResponse, error)
	UpdateRobotFunc func(ctx context.Context, in *apppb.UpdateRobotRequest,
		opts ...grpc.CallOption) (*apppb.UpdateRobotResponse, error)
	DeleteRobotFunc func(ctx context.Context, in *apppb.DeleteRobotRequest,
		opts ...grpc.CallOption) (*apppb.DeleteRobotResponse, error)
	ListFragmentsFunc func(ctx context.Context, in *apppb.ListFragmentsRequest,
		opts ...grpc.CallOption) (*apppb.ListFragmentsResponse, error)
	GetFragmentFunc func(ctx context.Context, in *apppb.GetFragmentRequest,
		opts ...grpc.CallOption) (*apppb.GetFragmentResponse, error)
	CreateFragmentFunc func(ctx context.Context, in *apppb.CreateFragmentRequest,
		opts ...grpc.CallOption) (*apppb.CreateFragmentResponse, error)
	UpdateFragmentFunc func(ctx context.Context, in *apppb.UpdateFragmentRequest,
		opts ...grpc.CallOption) (*apppb.UpdateFragmentResponse, error)
	DeleteFragmentFunc func(ctx context.Context, in *apppb.DeleteFragmentRequest,
		opts ...grpc.CallOption) (*apppb.DeleteFragmentResponse, error)
	ListMachineFragmentsFunc func(ctx context.Context, in *apppb.ListMachineFragmentsRequest,
		opts ...grpc.CallOption) (*apppb.ListMachineFragmentsResponse, error)
	GetFragmentHistoryFunc func(ctx context.Context, in *apppb.GetFragmentHistoryRequest,
		opts ...grpc.CallOption) (*apppb.GetFragmentHistoryResponse, error)
	AddRoleFunc func(ctx context.Context, in *apppb.AddRoleRequest,
		opts ...grpc.CallOption) (*apppb.AddRoleResponse, error)
	RemoveRoleFunc func(ctx context.Context, in *apppb.RemoveRoleRequest,
		opts ...grpc.CallOption) (*apppb.RemoveRoleResponse, error)
	ChangeRoleFunc func(ctx context.Context, in *apppb.ChangeRoleRequest,
		opts ...grpc.CallOption) (*apppb.ChangeRoleResponse, error)
	ListAuthorizationsFunc func(ctx context.Context, in *apppb.ListAuthorizationsRequest,
		opts ...grpc.CallOption) (*apppb.ListAuthorizationsResponse, error)
	CheckPermissionsFunc func(ctx context.Context, in *apppb.CheckPermissionsRequest,
		opts ...grpc.CallOption) (*apppb.CheckPermissionsResponse, error)
	GetRegistryItemFunc func(ctx context.Context, in *apppb.GetRegistryItemRequest,
		opts ...grpc.CallOption) (*apppb.GetRegistryItemResponse, error)
	CreateRegistryItemFunc func(ctx context.Context, in *apppb.CreateRegistryItemRequest,
		opts ...grpc.CallOption) (*apppb.CreateRegistryItemResponse, error)
	UpdateRegistryItemFunc func(ctx context.Context, in *apppb.UpdateRegistryItemRequest,
		opts ...grpc.CallOption) (*apppb.UpdateRegistryItemResponse, error)
	ListRegistryItemsFunc func(ctx context.Context, in *apppb.ListRegistryItemsRequest,
		opts ...grpc.CallOption) (*apppb.ListRegistryItemsResponse, error)
	DeleteRegistryItemFunc func(ctx context.Context, in *apppb.DeleteRegistryItemRequest,
		opts ...grpc.CallOption) (*apppb.DeleteRegistryItemResponse, error)
	TransferRegistryItemFunc func(ctx context.Context, in *apppb.TransferRegistryItemRequest,
		opts ...grpc.CallOption) (*apppb.TransferRegistryItemResponse, error)
	CreateModuleFunc func(ctx context.Context, in *apppb.CreateModuleRequest,
		opts ...grpc.CallOption) (*apppb.CreateModuleResponse, error)
	UpdateModuleFunc func(ctx context.Context, in *apppb.UpdateModuleRequest,
		opts ...grpc.CallOption) (*apppb.UpdateModuleResponse, error)
	UploadModuleFileFunc func(ctx context.Context, opts ...grpc.CallOption) (apppb.AppService_UploadModuleFileClient, error)
	GetModuleFunc        func(ctx context.Context, in *apppb.GetModuleRequest,
		opts ...grpc.CallOption) (*apppb.GetModuleResponse, error)
	ListModulesFunc func(ctx context.Context, in *apppb.ListModulesRequest,
		opts ...grpc.CallOption) (*apppb.ListModulesResponse, error)
	CreateKeyFunc func(ctx context.Context, in *apppb.CreateKeyRequest,
		opts ...grpc.CallOption) (*apppb.CreateKeyResponse, error)
	DeleteKeyFunc func(ctx context.Context, in *apppb.DeleteKeyRequest,
		opts ...grpc.CallOption) (*apppb.DeleteKeyResponse, error)
	ListKeysFunc func(ctx context.Context, in *apppb.ListKeysRequest,
		opts ...grpc.CallOption) (*apppb.ListKeysResponse, error)
	RenameKeyFunc func(ctx context.Context, in *apppb.RenameKeyRequest,
		opts ...grpc.CallOption) (*apppb.RenameKeyResponse, error)
	RotateKeyFunc func(ctx context.Context, in *apppb.RotateKeyRequest,
		opts ...grpc.CallOption) (*apppb.RotateKeyResponse, error)
	CreateKeyFromExistingKeyAuthorizationsFunc func(ctx context.Context, in *apppb.CreateKeyFromExistingKeyAuthorizationsRequest,
		opts ...grpc.CallOption) (*apppb.CreateKeyFromExistingKeyAuthorizationsResponse, error)
}

// GetUserIDByEmail calls the injected GetUserIDByEmailFunc or the real version.
func (asc *AppServiceClient) GetUserIDByEmail(
	ctx context.Context, in *apppb.GetUserIDByEmailRequest, opts ...grpc.CallOption,
) (*apppb.GetUserIDByEmailResponse, error) {
	if asc.GetUserIDByEmailFunc == nil {
		return asc.AppServiceClient.GetUserIDByEmail(ctx, in, opts...)
	}
	return asc.GetUserIDByEmailFunc(ctx, in, opts...)
}

// CreateOrganization calls the injected CreateOrganizationFunc or the real version.
func (asc *AppServiceClient) CreateOrganization(
	ctx context.Context, in *apppb.CreateOrganizationRequest, opts ...grpc.CallOption,
) (*apppb.CreateOrganizationResponse, error) {
	if asc.CreateOrganizationFunc == nil {
		return asc.AppServiceClient.CreateOrganization(ctx, in, opts...)
	}
	return asc.CreateOrganizationFunc(ctx, in, opts...)
}

// ListOrganizations calls the injected ListOrganizationsFunc or the real version.
func (asc *AppServiceClient) ListOrganizations(
	ctx context.Context, in *apppb.ListOrganizationsRequest, opts ...grpc.CallOption,
) (*apppb.ListOrganizationsResponse, error) {
	if asc.ListOrganizationsFunc == nil {
		return asc.AppServiceClient.ListOrganizations(ctx, in, opts...)
	}
	return asc.ListOrganizationsFunc(ctx, in, opts...)
}

// GetOrganizationsWithAccessToLocation calls the injected GetOrganizationsWithAccessToLocationFunc or the real version.
func (asc *AppServiceClient) GetOrganizationsWithAccessToLocation(
	ctx context.Context, in *apppb.GetOrganizationsWithAccessToLocationRequest, opts ...grpc.CallOption,
) (*apppb.GetOrganizationsWithAccessToLocationResponse, error) {
	if asc.GetOrganizationsWithAccessToLocationFunc == nil {
		return asc.AppServiceClient.GetOrganizationsWithAccessToLocation(ctx, in, opts...)
	}
	return asc.GetOrganizationsWithAccessToLocationFunc(ctx, in, opts...)
}

// ListOrganizationsByUser calls the injected ListOrganizationsByUserFunc or the real version.
func (asc *AppServiceClient) ListOrganizationsByUser(
	ctx context.Context, in *apppb.ListOrganizationsByUserRequest, opts ...grpc.CallOption,
) (*apppb.ListOrganizationsByUserResponse, error) {
	if asc.ListOrganizationsByUserFunc == nil {
		return asc.AppServiceClient.ListOrganizationsByUser(ctx, in, opts...)
	}
	return asc.ListOrganizationsByUserFunc(ctx, in, opts...)
}

// GetOrganization calls the injected GetOrganizationFunc or the real version.
func (asc *AppServiceClient) GetOrganization(
	ctx context.Context, in *apppb.GetOrganizationRequest, opts ...grpc.CallOption,
) (*apppb.GetOrganizationResponse, error) {
	if asc.GetOrganizationFunc == nil {
		return asc.AppServiceClient.GetOrganization(ctx, in, opts...)
	}
	return asc.GetOrganizationFunc(ctx, in, opts...)
}

// GetOrganizationNamespaceAvailability calls the injected GetOrganizationNamespaceAvailabilityFunc or the real version.
func (asc *AppServiceClient) GetOrganizationNamespaceAvailability(
	ctx context.Context, in *apppb.GetOrganizationNamespaceAvailabilityRequest, opts ...grpc.CallOption,
) (*apppb.GetOrganizationNamespaceAvailabilityResponse, error) {
	if asc.GetOrganizationNamespaceAvailabilityFunc == nil {
		return asc.AppServiceClient.GetOrganizationNamespaceAvailability(ctx, in, opts...)
	}
	return asc.GetOrganizationNamespaceAvailabilityFunc(ctx, in, opts...)
}

// UpdateOrganization calls the injected UpdateOrganizationFunc or the real version.
func (asc *AppServiceClient) UpdateOrganization(
	ctx context.Context, in *apppb.UpdateOrganizationRequest, opts ...grpc.CallOption,
) (*apppb.UpdateOrganizationResponse, error) {
	if asc.UpdateOrganizationFunc == nil {
		return asc.AppServiceClient.UpdateOrganization(ctx, in, opts...)
	}
	return asc.UpdateOrganizationFunc(ctx, in, opts...)
}

// DeleteOrganization calls the injected DeleteOrganizationFunc or the real version.
func (asc *AppServiceClient) DeleteOrganization(
	ctx context.Context, in *apppb.DeleteOrganizationRequest, opts ...grpc.CallOption,
) (*apppb.DeleteOrganizationResponse, error) {
	if asc.DeleteOrganizationFunc == nil {
		return asc.AppServiceClient.DeleteOrganization(ctx, in, opts...)
	}
	return asc.DeleteOrganizationFunc(ctx, in, opts...)
}

// ListOrganizationMembers calls the injected ListOrganizationMembersFunc or the real version.
func (asc *AppServiceClient) ListOrganizationMembers(
	ctx context.Context, in *apppb.ListOrganizationMembersRequest, opts ...grpc.CallOption,
) (*apppb.ListOrganizationMembersResponse, error) {
	if asc.ListOrganizationMembersFunc == nil {
		return asc.AppServiceClient.ListOrganizationMembers(ctx, in, opts...)
	}
	return asc.ListOrganizationMembersFunc(ctx, in, opts...)
}

// CreateOrganizationInvite calls the injected CreateOrganizationInviteFunc or the real version.
func (asc *AppServiceClient) CreateOrganizationInvite(
	ctx context.Context, in *apppb.CreateOrganizationInviteRequest, opts ...grpc.CallOption,
) (*apppb.CreateOrganizationInviteResponse, error) {
	if asc.CreateOrganizationInviteFunc == nil {
		return asc.AppServiceClient.CreateOrganizationInvite(ctx, in, opts...)
	}
	return asc.CreateOrganizationInviteFunc(ctx, in, opts...)
}

// UpdateOrganizationInviteAuthorizations calls the injected UpdateOrganizationInviteAuthorizationsFunc or the real version.
func (asc *AppServiceClient) UpdateOrganizationInviteAuthorizations(
	ctx context.Context, in *apppb.UpdateOrganizationInviteAuthorizationsRequest, opts ...grpc.CallOption,
) (*apppb.UpdateOrganizationInviteAuthorizationsResponse, error) {
	if asc.UpdateOrganizationInviteAuthorizationsFunc == nil {
		return asc.AppServiceClient.UpdateOrganizationInviteAuthorizations(ctx, in, opts...)
	}
	return asc.UpdateOrganizationInviteAuthorizationsFunc(ctx, in, opts...)
}

// DeleteOrganizationMember calls the injected DeleteOrganizationMemberFunc or the real version.
func (asc *AppServiceClient) DeleteOrganizationMember(
	ctx context.Context, in *apppb.DeleteOrganizationMemberRequest, opts ...grpc.CallOption,
) (*apppb.DeleteOrganizationMemberResponse, error) {
	if asc.DeleteOrganizationMemberFunc == nil {
		return asc.AppServiceClient.DeleteOrganizationMember(ctx, in, opts...)
	}
	return asc.DeleteOrganizationMemberFunc(ctx, in, opts...)
}

// DeleteOrganizationInvite calls the injected DeleteOrganizationInviteFunc or the real version.
func (asc *AppServiceClient) DeleteOrganizationInvite(
	ctx context.Context, in *apppb.DeleteOrganizationInviteRequest, opts ...grpc.CallOption,
) (*apppb.DeleteOrganizationInviteResponse, error) {
	if asc.DeleteOrganizationInviteFunc == nil {
		return asc.AppServiceClient.DeleteOrganizationInvite(ctx, in, opts...)
	}
	return asc.DeleteOrganizationInviteFunc(ctx, in, opts...)
}

// ResendOrganizationInvite calls the injected ResendOrganizationInviteFunc or the real version.
func (asc *AppServiceClient) ResendOrganizationInvite(
	ctx context.Context, in *apppb.ResendOrganizationInviteRequest, opts ...grpc.CallOption,
) (*apppb.ResendOrganizationInviteResponse, error) {
	if asc.ResendOrganizationInviteFunc == nil {
		return asc.AppServiceClient.ResendOrganizationInvite(ctx, in, opts...)
	}
	return asc.ResendOrganizationInviteFunc(ctx, in, opts...)
}

// EnableBillingService calls the injected EnableBillingServiceFunc or the real version.
func (asc *AppServiceClient) EnableBillingService(
	ctx context.Context, in *apppb.EnableBillingServiceRequest, opts ...grpc.CallOption,
) (*apppb.EnableBillingServiceResponse, error) {
	if asc.EnableBillingServiceFunc == nil {
		return asc.AppServiceClient.EnableBillingService(ctx, in, opts...)
	}
	return asc.EnableBillingServiceFunc(ctx, in, opts...)
}

// GetBillingServiceConfig calls the injected GetBillingServiceConfigFunc or the real version.
func (asc *AppServiceClient) GetBillingServiceConfig(
	ctx context.Context, in *apppb.GetBillingServiceConfigRequest, opts ...grpc.CallOption,
) (*apppb.GetBillingServiceConfigResponse, error) {
	if asc.GetBillingServiceConfigFunc == nil {
		return asc.AppServiceClient.GetBillingServiceConfig(ctx, in, opts...)
	}
	return asc.GetBillingServiceConfigFunc(ctx, in, opts...)
}

// DisableBillingService calls the injected DisableBillingServiceFunc or the real version.
func (asc *AppServiceClient) DisableBillingService(
	ctx context.Context, in *apppb.DisableBillingServiceRequest, opts ...grpc.CallOption,
) (*apppb.DisableBillingServiceResponse, error) {
	if asc.DisableBillingServiceFunc == nil {
		return asc.AppServiceClient.DisableBillingService(ctx, in, opts...)
	}
	return asc.DisableBillingServiceFunc(ctx, in, opts...)
}

// UpdateBillingService calls the injected UpdateBillingServiceFunc or the real version.
func (asc *AppServiceClient) UpdateBillingService(
	ctx context.Context, in *apppb.UpdateBillingServiceRequest, opts ...grpc.CallOption,
) (*apppb.UpdateBillingServiceResponse, error) {
	if asc.UpdateBillingServiceFunc == nil {
		return asc.AppServiceClient.UpdateBillingService(ctx, in, opts...)
	}
	return asc.UpdateBillingServiceFunc(ctx, in, opts...)
}

// OrganizationSetSupportEmail calls the injected OrganizationSetSupportEmailFunc or the real version.
func (asc *AppServiceClient) OrganizationSetSupportEmail(
	ctx context.Context, in *apppb.OrganizationSetSupportEmailRequest, opts ...grpc.CallOption,
) (*apppb.OrganizationSetSupportEmailResponse, error) {
	if asc.OrganizationSetSupportEmailFunc == nil {
		return asc.AppServiceClient.OrganizationSetSupportEmail(ctx, in, opts...)
	}
	return asc.OrganizationSetSupportEmailFunc(ctx, in, opts...)
}

// OrganizationGetSupportEmail calls the injected OrganizationGetSupportEmailFunc or the real version.
func (asc *AppServiceClient) OrganizationGetSupportEmail(
	ctx context.Context, in *apppb.OrganizationGetSupportEmailRequest, opts ...grpc.CallOption,
) (*apppb.OrganizationGetSupportEmailResponse, error) {
	if asc.OrganizationGetSupportEmailFunc == nil {
		return asc.AppServiceClient.OrganizationGetSupportEmail(ctx, in, opts...)
	}
	return asc.OrganizationGetSupportEmailFunc(ctx, in, opts...)
}

// OrganizationSetLogo calls the injected OrganizationSetLogoFunc or the real version.
func (asc *AppServiceClient) OrganizationSetLogo(
	ctx context.Context, in *apppb.OrganizationSetLogoRequest, opts ...grpc.CallOption,
) (*apppb.OrganizationSetLogoResponse, error) {
	if asc.OrganizationSetLogoFunc == nil {
		return asc.AppServiceClient.OrganizationSetLogo(ctx, in, opts...)
	}
	return asc.OrganizationSetLogoFunc(ctx, in, opts...)
}

// OrganizationGetLogo calls the injected OrganizationGetLogoFunc or the real version.
func (asc *AppServiceClient) OrganizationGetLogo(
	ctx context.Context, in *apppb.OrganizationGetLogoRequest, opts ...grpc.CallOption,
) (*apppb.OrganizationGetLogoResponse, error) {
	if asc.OrganizationGetLogoFunc == nil {
		return asc.AppServiceClient.OrganizationGetLogo(ctx, in, opts...)
	}
	return asc.OrganizationGetLogoFunc(ctx, in, opts...)
}

// CreateLocation calls the injected CreateLocationFunc or the real version.
func (asc *AppServiceClient) CreateLocation(
	ctx context.Context, in *apppb.CreateLocationRequest, opts ...grpc.CallOption,
) (*apppb.CreateLocationResponse, error) {
	if asc.CreateLocationFunc == nil {
		return asc.AppServiceClient.CreateLocation(ctx, in, opts...)
	}
	return asc.CreateLocationFunc(ctx, in, opts...)
}

// GetLocation calls the injected GetLocationFunc or the real version.
func (asc *AppServiceClient) GetLocation(
	ctx context.Context, in *apppb.GetLocationRequest, opts ...grpc.CallOption,
) (*apppb.GetLocationResponse, error) {
	if asc.GetLocationFunc == nil {
		return asc.AppServiceClient.GetLocation(ctx, in, opts...)
	}
	return asc.GetLocationFunc(ctx, in, opts...)
}

// UpdateLocation calls the injected UpdateLocationFunc or the real version.
func (asc *AppServiceClient) UpdateLocation(
	ctx context.Context, in *apppb.UpdateLocationRequest, opts ...grpc.CallOption,
) (*apppb.UpdateLocationResponse, error) {
	if asc.UpdateLocationFunc == nil {
		return asc.AppServiceClient.UpdateLocation(ctx, in, opts...)
	}
	return asc.UpdateLocationFunc(ctx, in, opts...)
}

// DeleteLocation calls the injected DeleteLocationFunc or the real version.
func (asc *AppServiceClient) DeleteLocation(
	ctx context.Context, in *apppb.DeleteLocationRequest, opts ...grpc.CallOption,
) (*apppb.DeleteLocationResponse, error) {
	if asc.DeleteLocationFunc == nil {
		return asc.AppServiceClient.DeleteLocation(ctx, in, opts...)
	}
	return asc.DeleteLocationFunc(ctx, in, opts...)
}

// ListLocations calls the injected ListLocationsFunc or the real version.
func (asc *AppServiceClient) ListLocations(
	ctx context.Context, in *apppb.ListLocationsRequest, opts ...grpc.CallOption,
) (*apppb.ListLocationsResponse, error) {
	if asc.ListLocationsFunc == nil {
		return asc.AppServiceClient.ListLocations(ctx, in, opts...)
	}
	return asc.ListLocationsFunc(ctx, in, opts...)
}

// ShareLocation calls the injected ShareLocationFunc or the real version.
func (asc *AppServiceClient) ShareLocation(
	ctx context.Context, in *apppb.ShareLocationRequest, opts ...grpc.CallOption,
) (*apppb.ShareLocationResponse, error) {
	if asc.ShareLocationFunc == nil {
		return asc.AppServiceClient.ShareLocation(ctx, in, opts...)
	}
	return asc.ShareLocationFunc(ctx, in, opts...)
}

// UnshareLocation calls the injected UnshareLocationFunc or the real version.
func (asc *AppServiceClient) UnshareLocation(
	ctx context.Context, in *apppb.UnshareLocationRequest, opts ...grpc.CallOption,
) (*apppb.UnshareLocationResponse, error) {
	if asc.UnshareLocationFunc == nil {
		return asc.AppServiceClient.UnshareLocation(ctx, in, opts...)
	}
	return asc.UnshareLocationFunc(ctx, in, opts...)
}

// LocationAuth calls the injected LocationAuthFunc or the real version.
func (asc *AppServiceClient) LocationAuth(
	ctx context.Context, in *apppb.LocationAuthRequest, opts ...grpc.CallOption,
) (*apppb.LocationAuthResponse, error) {
	if asc.LocationAuthFunc == nil {
		return asc.AppServiceClient.LocationAuth(ctx, in, opts...)
	}
	return asc.LocationAuthFunc(ctx, in, opts...)
}

// CreateLocationSecret calls the injected CreateLocationSecretFunc or the real version.
func (asc *AppServiceClient) CreateLocationSecret(
	ctx context.Context, in *apppb.CreateLocationSecretRequest, opts ...grpc.CallOption,
) (*apppb.CreateLocationSecretResponse, error) {
	if asc.CreateLocationSecretFunc == nil {
		return asc.AppServiceClient.CreateLocationSecret(ctx, in, opts...)
	}
	return asc.CreateLocationSecretFunc(ctx, in, opts...)
}

// DeleteLocationSecret calls the injected DeleteLocationSecretFunc or the real version.
func (asc *AppServiceClient) DeleteLocationSecret(
	ctx context.Context, in *apppb.DeleteLocationSecretRequest, opts ...grpc.CallOption,
) (*apppb.DeleteLocationSecretResponse, error) {
	if asc.DeleteLocationSecretFunc == nil {
		return asc.AppServiceClient.DeleteLocationSecret(ctx, in, opts...)
	}
	return asc.DeleteLocationSecretFunc(ctx, in, opts...)
}

// GetRobot calls the injected GetRobotFunc or the real version.
func (asc *AppServiceClient) GetRobot(
	ctx context.Context, in *apppb.GetRobotRequest, opts ...grpc.CallOption,
) (*apppb.GetRobotResponse, error) {
	if asc.GetRobotFunc == nil {
		return asc.AppServiceClient.GetRobot(ctx, in, opts...)
	}
	return asc.GetRobotFunc(ctx, in, opts...)
}

// GetRoverRentalRobots calls the injected GetRoverRentalRobotsFunc or the real version.
func (asc *AppServiceClient) GetRoverRentalRobots(
	ctx context.Context, in *apppb.GetRoverRentalRobotsRequest, opts ...grpc.CallOption,
) (*apppb.GetRoverRentalRobotsResponse, error) {
	if asc.GetRoverRentalRobotsFunc == nil {
		return asc.AppServiceClient.GetRoverRentalRobots(ctx, in, opts...)
	}
	return asc.GetRoverRentalRobotsFunc(ctx, in, opts...)
}

// GetRobotParts calls the injected GetRobotPartsFunc or the real version.
func (asc *AppServiceClient) GetRobotParts(
	ctx context.Context, in *apppb.GetRobotPartsRequest, opts ...grpc.CallOption,
) (*apppb.GetRobotPartsResponse, error) {
	if asc.GetRobotPartsFunc == nil {
		return asc.AppServiceClient.GetRobotParts(ctx, in, opts...)
	}
	return asc.GetRobotPartsFunc(ctx, in, opts...)
}

// GetRobotPart calls the injected GetRobotPartFunc or the real version.
func (asc *AppServiceClient) GetRobotPart(
	ctx context.Context, in *apppb.GetRobotPartRequest, opts ...grpc.CallOption,
) (*apppb.GetRobotPartResponse, error) {
	if asc.GetRobotPartFunc == nil {
		return asc.AppServiceClient.GetRobotPart(ctx, in, opts...)
	}
	return asc.GetRobotPartFunc(ctx, in, opts...)
}

// GetRobotPartLogs calls the injected GetRobotPartLogsFunc or the real version.
func (asc *AppServiceClient) GetRobotPartLogs(
	ctx context.Context, in *apppb.GetRobotPartLogsRequest, opts ...grpc.CallOption,
) (*apppb.GetRobotPartLogsResponse, error) {
	if asc.GetRobotPartLogsFunc == nil {
		return asc.AppServiceClient.GetRobotPartLogs(ctx, in, opts...)
	}
	return asc.GetRobotPartLogsFunc(ctx, in, opts...)
}

// TailRobotPartLogs calls the injected TailRobotPartLogsFunc or the real version.
func (asc *AppServiceClient) TailRobotPartLogs(
	ctx context.Context, in *apppb.TailRobotPartLogsRequest, opts ...grpc.CallOption,
) (apppb.AppService_TailRobotPartLogsClient, error) {
	if asc.TailRobotPartLogsFunc == nil {
		return asc.AppServiceClient.TailRobotPartLogs(ctx, in, opts...)
	}
	return asc.TailRobotPartLogsFunc(ctx, in, opts...)
}

// AppServiceTailRobotPartLogsClient represents a fake instance of a proto AppService_TailRobotPartLogsClient.
type AppServiceTailRobotPartLogsClient struct {
	apppb.AppService_TailRobotPartLogsClient
	RecvFunc func() (*apppb.TailRobotPartLogsResponse, error)
}

// Recv calls the injected RecvFunc or the real version.
func (c *AppServiceTailRobotPartLogsClient) Recv() (*apppb.TailRobotPartLogsResponse, error) {
	if c.RecvFunc == nil {
		return c.AppService_TailRobotPartLogsClient.Recv()
	}
	return c.RecvFunc()
}

// GetRobotPartHistory calls the injected GetRobotPartHistoryFunc or the real version.
func (asc *AppServiceClient) GetRobotPartHistory(
	ctx context.Context, in *apppb.GetRobotPartHistoryRequest, opts ...grpc.CallOption,
) (*apppb.GetRobotPartHistoryResponse, error) {
	if asc.GetRobotPartHistoryFunc == nil {
		return asc.AppServiceClient.GetRobotPartHistory(ctx, in, opts...)
	}
	return asc.GetRobotPartHistoryFunc(ctx, in, opts...)
}

// UpdateRobotPart calls the injected UpdateRobotPartFunc or the real version.
func (asc *AppServiceClient) UpdateRobotPart(
	ctx context.Context, in *apppb.UpdateRobotPartRequest, opts ...grpc.CallOption,
) (*apppb.UpdateRobotPartResponse, error) {
	if asc.UpdateRobotPartFunc == nil {
		return asc.AppServiceClient.UpdateRobotPart(ctx, in, opts...)
	}
	return asc.UpdateRobotPartFunc(ctx, in, opts...)
}

// NewRobotPart calls the injected NewRobotPartFunc or the real version.
func (asc *AppServiceClient) NewRobotPart(
	ctx context.Context, in *apppb.NewRobotPartRequest, opts ...grpc.CallOption,
) (*apppb.NewRobotPartResponse, error) {
	if asc.NewRobotPartFunc == nil {
		return asc.AppServiceClient.NewRobotPart(ctx, in, opts...)
	}
	return asc.NewRobotPartFunc(ctx, in, opts...)
}

// DeleteRobotPart calls the injected DeleteRobotPartFunc or the real version.
func (asc *AppServiceClient) DeleteRobotPart(
	ctx context.Context, in *apppb.DeleteRobotPartRequest, opts ...grpc.CallOption,
) (*apppb.DeleteRobotPartResponse, error) {
	if asc.DeleteRobotPartFunc == nil {
		return asc.AppServiceClient.DeleteRobotPart(ctx, in, opts...)
	}
	return asc.DeleteRobotPartFunc(ctx, in, opts...)
}

// GetRobotAPIKeys calls the injected GetRobotAPIKeysFunc or the real version.
func (asc *AppServiceClient) GetRobotAPIKeys(
	ctx context.Context, in *apppb.GetRobotAPIKeysRequest, opts ...grpc.CallOption,
) (*apppb.GetRobotAPIKeysResponse, error) {
	if asc.GetRobotAPIKeysFunc == nil {
		return asc.AppServiceClient.GetRobotAPIKeys(ctx, in, opts...)
	}
	return asc.GetRobotAPIKeysFunc(ctx, in, opts...)
}

// MarkPartAsMain calls the injected MarkPartAsMainFunc or the real version.
func (asc *AppServiceClient) MarkPartAsMain(
	ctx context.Context, in *apppb.MarkPartAsMainRequest, opts ...grpc.CallOption,
) (*apppb.MarkPartAsMainResponse, error) {
	if asc.MarkPartAsMainFunc == nil {
		return asc.AppServiceClient.MarkPartAsMain(ctx, in, opts...)
	}
	return asc.MarkPartAsMainFunc(ctx, in, opts...)
}

// MarkPartForRestart calls the injected MarkPartForRestartFunc or the real version.
func (asc *AppServiceClient) MarkPartForRestart(
	ctx context.Context, in *apppb.MarkPartForRestartRequest, opts ...grpc.CallOption,
) (*apppb.MarkPartForRestartResponse, error) {
	if asc.MarkPartForRestartFunc == nil {
		return asc.AppServiceClient.MarkPartForRestart(ctx, in, opts...)
	}
	return asc.MarkPartForRestartFunc(ctx, in, opts...)
}

// CreateRobotPartSecret calls the injected CreateRobotPartSecretFunc or the real version.
func (asc *AppServiceClient) CreateRobotPartSecret(
	ctx context.Context, in *apppb.CreateRobotPartSecretRequest, opts ...grpc.CallOption,
) (*apppb.CreateRobotPartSecretResponse, error) {
	if asc.CreateRobotPartSecretFunc == nil {
		return asc.AppServiceClient.CreateRobotPartSecret(ctx, in, opts...)
	}
	return asc.CreateRobotPartSecretFunc(ctx, in, opts...)
}

// DeleteRobotPartSecret calls the injected DeleteRobotPartSecretFunc or the real version.
func (asc *AppServiceClient) DeleteRobotPartSecret(
	ctx context.Context, in *apppb.DeleteRobotPartSecretRequest, opts ...grpc.CallOption,
) (*apppb.DeleteRobotPartSecretResponse, error) {
	if asc.DeleteRobotPartSecretFunc == nil {
		return asc.AppServiceClient.DeleteRobotPartSecret(ctx, in, opts...)
	}
	return asc.DeleteRobotPartSecretFunc(ctx, in, opts...)
}

// ListRobots calls the injected ListRobotsFunc or the real version.
func (asc *AppServiceClient) ListRobots(
	ctx context.Context, in *apppb.ListRobotsRequest, opts ...grpc.CallOption,
) (*apppb.ListRobotsResponse, error) {
	if asc.ListRobotsFunc == nil {
		return asc.AppServiceClient.ListRobots(ctx, in, opts...)
	}
	return asc.ListRobotsFunc(ctx, in, opts...)
}

// NewRobot calls the injected NewRobotFunc or the real version.
func (asc *AppServiceClient) NewRobot(
	ctx context.Context, in *apppb.NewRobotRequest, opts ...grpc.CallOption,
) (*apppb.NewRobotResponse, error) {
	if asc.NewRobotFunc == nil {
		return asc.AppServiceClient.NewRobot(ctx, in, opts...)
	}
	return asc.NewRobotFunc(ctx, in, opts...)
}

// UpdateRobot calls the injected UpdateRobotFunc or the real version.
func (asc *AppServiceClient) UpdateRobot(
	ctx context.Context, in *apppb.UpdateRobotRequest, opts ...grpc.CallOption,
) (*apppb.UpdateRobotResponse, error) {
	if asc.UpdateRobotFunc == nil {
		return asc.AppServiceClient.UpdateRobot(ctx, in, opts...)
	}
	return asc.UpdateRobotFunc(ctx, in, opts...)
}

// DeleteRobot calls the injected DeleteRobotFunc or the real version.
func (asc *AppServiceClient) DeleteRobot(
	ctx context.Context, in *apppb.DeleteRobotRequest, opts ...grpc.CallOption,
) (*apppb.DeleteRobotResponse, error) {
	if asc.DeleteRobotFunc == nil {
		return asc.AppServiceClient.DeleteRobot(ctx, in, opts...)
	}
	return asc.DeleteRobotFunc(ctx, in, opts...)
}

// ListFragments calls the injected ListFragmentsFunc or the real version.
func (asc *AppServiceClient) ListFragments(
	ctx context.Context, in *apppb.ListFragmentsRequest, opts ...grpc.CallOption,
) (*apppb.ListFragmentsResponse, error) {
	if asc.ListFragmentsFunc == nil {
		return asc.AppServiceClient.ListFragments(ctx, in, opts...)
	}
	return asc.ListFragmentsFunc(ctx, in, opts...)
}

// GetFragment calls the injected GetFragmentFunc or the real version.
func (asc *AppServiceClient) GetFragment(
	ctx context.Context, in *apppb.GetFragmentRequest, opts ...grpc.CallOption,
) (*apppb.GetFragmentResponse, error) {
	if asc.GetFragmentFunc == nil {
		return asc.AppServiceClient.GetFragment(ctx, in, opts...)
	}
	return asc.GetFragmentFunc(ctx, in, opts...)
}

// CreateFragment calls the injected CreateFragmentFunc or the real version.
func (asc *AppServiceClient) CreateFragment(
	ctx context.Context, in *apppb.CreateFragmentRequest, opts ...grpc.CallOption,
) (*apppb.CreateFragmentResponse, error) {
	if asc.CreateFragmentFunc == nil {
		return asc.AppServiceClient.CreateFragment(ctx, in, opts...)
	}
	return asc.CreateFragmentFunc(ctx, in, opts...)
}

// UpdateFragment calls the injected UpdateFragmentFunc or the real version.
func (asc *AppServiceClient) UpdateFragment(
	ctx context.Context, in *apppb.UpdateFragmentRequest, opts ...grpc.CallOption,
) (*apppb.UpdateFragmentResponse, error) {
	if asc.UpdateFragmentFunc == nil {
		return asc.AppServiceClient.UpdateFragment(ctx, in, opts...)
	}
	return asc.UpdateFragmentFunc(ctx, in, opts...)
}

// DeleteFragment calls the injected DeleteFragmentFunc or the real version.
func (asc *AppServiceClient) DeleteFragment(
	ctx context.Context, in *apppb.DeleteFragmentRequest, opts ...grpc.CallOption,
) (*apppb.DeleteFragmentResponse, error) {
	if asc.DeleteFragmentFunc == nil {
		return asc.AppServiceClient.DeleteFragment(ctx, in, opts...)
	}
	return asc.DeleteFragmentFunc(ctx, in, opts...)
}

// ListMachineFragments calls the injected ListMachineFragmentsFunc or the real version.
func (asc *AppServiceClient) ListMachineFragments(
	ctx context.Context, in *apppb.ListMachineFragmentsRequest, opts ...grpc.CallOption,
) (*apppb.ListMachineFragmentsResponse, error) {
	if asc.ListMachineFragmentsFunc == nil {
		return asc.AppServiceClient.ListMachineFragments(ctx, in, opts...)
	}
	return asc.ListMachineFragmentsFunc(ctx, in, opts...)
}

// GetFragmentHistory calls the injected GetFragmentHistoryFunc or the real version.
func (asc *AppServiceClient) GetFragmentHistory(
	ctx context.Context, in *apppb.GetFragmentHistoryRequest, opts ...grpc.CallOption,
) (*apppb.GetFragmentHistoryResponse, error) {
	if asc.GetFragmentHistoryFunc == nil {
		return asc.AppServiceClient.GetFragmentHistory(ctx, in, opts...)
	}
	return asc.GetFragmentHistoryFunc(ctx, in, opts...)
}

// AddRole calls the injected AddRoleFunc or the real version.
func (asc *AppServiceClient) AddRole(
	ctx context.Context, in *apppb.AddRoleRequest, opts ...grpc.CallOption,
) (*apppb.AddRoleResponse, error) {
	if asc.AddRoleFunc == nil {
		return asc.AppServiceClient.AddRole(ctx, in, opts...)
	}
	return asc.AddRoleFunc(ctx, in, opts...)
}

// RemoveRole calls the injected RemoveRoleFunc or the real version.
func (asc *AppServiceClient) RemoveRole(
	ctx context.Context, in *apppb.RemoveRoleRequest, opts ...grpc.CallOption,
) (*apppb.RemoveRoleResponse, error) {
	if asc.RemoveRoleFunc == nil {
		return asc.AppServiceClient.RemoveRole(ctx, in, opts...)
	}
	return asc.RemoveRoleFunc(ctx, in, opts...)
}

// ChangeRole calls the injected ChangeRoleFunc or the real version.
func (asc *AppServiceClient) ChangeRole(
	ctx context.Context, in *apppb.ChangeRoleRequest, opts ...grpc.CallOption,
) (*apppb.ChangeRoleResponse, error) {
	if asc.ChangeRoleFunc == nil {
		return asc.AppServiceClient.ChangeRole(ctx, in, opts...)
	}
	return asc.ChangeRoleFunc(ctx, in, opts...)
}

// ListAuthorizations calls the injected ListAuthorizationsFunc or the real version.
func (asc *AppServiceClient) ListAuthorizations(
	ctx context.Context, in *apppb.ListAuthorizationsRequest, opts ...grpc.CallOption,
) (*apppb.ListAuthorizationsResponse, error) {
	if asc.ListAuthorizationsFunc == nil {
		return asc.AppServiceClient.ListAuthorizations(ctx, in, opts...)
	}
	return asc.ListAuthorizationsFunc(ctx, in, opts...)
}

// CheckPermissions calls the injected CheckPermissionsFunc or the real version.
func (asc *AppServiceClient) CheckPermissions(
	ctx context.Context, in *apppb.CheckPermissionsRequest, opts ...grpc.CallOption,
) (*apppb.CheckPermissionsResponse, error) {
	if asc.CheckPermissionsFunc == nil {
		return asc.AppServiceClient.CheckPermissions(ctx, in, opts...)
	}
	return asc.CheckPermissionsFunc(ctx, in, opts...)
}

// GetRegistryItem calls the injected GetRegistryItemFunc or the real version.
func (asc *AppServiceClient) GetRegistryItem(
	ctx context.Context, in *apppb.GetRegistryItemRequest, opts ...grpc.CallOption,
) (*apppb.GetRegistryItemResponse, error) {
	if asc.GetRegistryItemFunc == nil {
		return asc.AppServiceClient.GetRegistryItem(ctx, in, opts...)
	}
	return asc.GetRegistryItemFunc(ctx, in, opts...)
}

// CreateRegistryItem calls the injected CreateRegistryItemFunc or the real version.
func (asc *AppServiceClient) CreateRegistryItem(
	ctx context.Context, in *apppb.CreateRegistryItemRequest, opts ...grpc.CallOption,
) (*apppb.CreateRegistryItemResponse, error) {
	if asc.CreateRegistryItemFunc == nil {
		return asc.AppServiceClient.CreateRegistryItem(ctx, in, opts...)
	}
	return asc.CreateRegistryItemFunc(ctx, in, opts...)
}

// UpdateRegistryItem calls the injected UpdateRegistryItemFunc or the real version.
func (asc *AppServiceClient) UpdateRegistryItem(
	ctx context.Context, in *apppb.UpdateRegistryItemRequest, opts ...grpc.CallOption,
) (*apppb.UpdateRegistryItemResponse, error) {
	if asc.UpdateRegistryItemFunc == nil {
		return asc.AppServiceClient.UpdateRegistryItem(ctx, in, opts...)
	}
	return asc.UpdateRegistryItemFunc(ctx, in, opts...)
}

// ListRegistryItems calls the injected ListRegistryItemsFunc or the real version.
func (asc *AppServiceClient) ListRegistryItems(
	ctx context.Context, in *apppb.ListRegistryItemsRequest, opts ...grpc.CallOption,
) (*apppb.ListRegistryItemsResponse, error) {
	if asc.ListRegistryItemsFunc == nil {
		return asc.AppServiceClient.ListRegistryItems(ctx, in, opts...)
	}
	return asc.ListRegistryItemsFunc(ctx, in, opts...)
}

// DeleteRegistryItem calls the injected DeleteRegistryItemFunc or the real version.
func (asc *AppServiceClient) DeleteRegistryItem(
	ctx context.Context, in *apppb.DeleteRegistryItemRequest, opts ...grpc.CallOption,
) (*apppb.DeleteRegistryItemResponse, error) {
	if asc.DeleteRegistryItemFunc == nil {
		return asc.AppServiceClient.DeleteRegistryItem(ctx, in, opts...)
	}
	return asc.DeleteRegistryItemFunc(ctx, in, opts...)
}

// TransferRegistryItem calls the injected TransferRegistryItemFunc or the real version.
func (asc *AppServiceClient) TransferRegistryItem(
	ctx context.Context, in *apppb.TransferRegistryItemRequest, opts ...grpc.CallOption,
) (*apppb.TransferRegistryItemResponse, error) {
	if asc.TransferRegistryItemFunc == nil {
		return asc.AppServiceClient.TransferRegistryItem(ctx, in, opts...)
	}
	return asc.TransferRegistryItemFunc(ctx, in, opts...)
}

// CreateModule calls the injected CreateModuleFunc or the real version.
func (asc *AppServiceClient) CreateModule(
	ctx context.Context, in *apppb.CreateModuleRequest, opts ...grpc.CallOption,
) (*apppb.CreateModuleResponse, error) {
	if asc.CreateModuleFunc == nil {
		return asc.AppServiceClient.CreateModule(ctx, in, opts...)
	}
	return asc.CreateModuleFunc(ctx, in, opts...)
}

// UpdateModule calls the injected UpdateModuleFunc or the real version.
func (asc *AppServiceClient) UpdateModule(
	ctx context.Context, in *apppb.UpdateModuleRequest, opts ...grpc.CallOption,
) (*apppb.UpdateModuleResponse, error) {
	if asc.UpdateModuleFunc == nil {
		return asc.AppServiceClient.UpdateModule(ctx, in, opts...)
	}
	return asc.UpdateModuleFunc(ctx, in, opts...)
}

// UploadModuleFile calls the injected UploadModuleFileFunc or the real version.
func (asc *AppServiceClient) UploadModuleFile(
	ctx context.Context, opts ...grpc.CallOption,
) (apppb.AppService_UploadModuleFileClient, error) {
	if asc.UploadModuleFileFunc == nil {
		return asc.AppServiceClient.UploadModuleFile(ctx, opts...)
	}
	return asc.UploadModuleFileFunc(ctx, opts...)
}

// AppServiceUploadModuleFileClient represents a fake instance of a proto AppService_UploadModuleFileClient.
type AppServiceUploadModuleFileClient struct {
	apppb.AppService_UploadModuleFileClient
	SendFunc         func(*apppb.UploadModuleFileRequest) error
	CloseAndRecvFunc func() (*apppb.UploadModuleFileResponse, error)
}

// Send calls the injected SendFunc or the real version.
func (c *AppServiceUploadModuleFileClient) Send(req *apppb.UploadModuleFileRequest) error {
	if c.SendFunc == nil {
		return c.AppService_UploadModuleFileClient.Send(req)
	}
	return c.SendFunc(req)
}

// CloseAndRecv calls the injected CloseAndRecvFunc or the real version.
func (c *AppServiceUploadModuleFileClient) CloseAndRecv() (*apppb.UploadModuleFileResponse, error) {
	if c.CloseAndRecvFunc == nil {
		return c.AppService_UploadModuleFileClient.CloseAndRecv()
	}
	return c.CloseAndRecvFunc()
}

// GetModule calls the injected GetModuleFunc or the real version.
func (asc *AppServiceClient) GetModule(
	ctx context.Context, in *apppb.GetModuleRequest, opts ...grpc.CallOption,
) (*apppb.GetModuleResponse, error) {
	if asc.GetModuleFunc == nil {
		return asc.AppServiceClient.GetModule(ctx, in, opts...)
	}
	return asc.GetModuleFunc(ctx, in, opts...)
}

// ListModules calls the injected ListModulesFunc or the real version.
func (asc *AppServiceClient) ListModules(
	ctx context.Context, in *apppb.ListModulesRequest, opts ...grpc.CallOption,
) (*apppb.ListModulesResponse, error) {
	if asc.ListModulesFunc == nil {
		return asc.AppServiceClient.ListModules(ctx, in, opts...)
	}
	return asc.ListModulesFunc(ctx, in, opts...)
}

// CreateKey calls the injected CreateKeyFunc or the real version.
func (asc *AppServiceClient) CreateKey(
	ctx context.Context, in *apppb.CreateKeyRequest, opts ...grpc.CallOption,
) (*apppb.CreateKeyResponse, error) {
	if asc.CreateKeyFunc == nil {
		return asc.AppServiceClient.CreateKey(ctx, in, opts...)
	}
	return asc.CreateKeyFunc(ctx, in, opts...)
}

// DeleteKey calls the injected DeleteKeyFunc or the real version.
func (asc *AppServiceClient) DeleteKey(
	ctx context.Context, in *apppb.DeleteKeyRequest, opts ...grpc.CallOption,
) (*apppb.DeleteKeyResponse, error) {
	if asc.DeleteKeyFunc == nil {
		return asc.AppServiceClient.DeleteKey(ctx, in, opts...)
	}
	return asc.DeleteKeyFunc(ctx, in, opts...)
}

// ListKeys calls the injected ListKeysFunc or the real version.
func (asc *AppServiceClient) ListKeys(
	ctx context.Context, in *apppb.ListKeysRequest, opts ...grpc.CallOption,
) (*apppb.ListKeysResponse, error) {
	if asc.ListKeysFunc == nil {
		return asc.AppServiceClient.ListKeys(ctx, in, opts...)
	}
	return asc.ListKeysFunc(ctx, in, opts...)
}

// RenameKey calls the injected RenameKeyFunc or the real version.
func (asc *AppServiceClient) RenameKey(
	ctx context.Context, in *apppb.RenameKeyRequest, opts ...grpc.CallOption,
) (*apppb.RenameKeyResponse, error) {
	if asc.RenameKeyFunc == nil {
		return asc.AppServiceClient.RenameKey(ctx, in, opts...)
	}
	return asc.RenameKeyFunc(ctx, in, opts...)
}

// RotateKey calls the injected RotateKeyFunc or the real version.
func (asc *AppServiceClient) RotateKey(
	ctx context.Context, in *apppb.RotateKeyRequest, opts ...grpc.CallOption,
) (*apppb.RotateKeyResponse, error) {
	if asc.RotateKeyFunc == nil {
		return asc.AppServiceClient.RotateKey(ctx, in, opts...)
	}
	return asc.RotateKeyFunc(ctx, in, opts...)
}

// CreateKeyFromExistingKeyAuthorizations calls the injected CreateKeyFromExistingKeyAuthorizationsFunc or the real version.
func (asc *AppServiceClient) CreateKeyFromExistingKeyAuthorizations(
	ctx context.Context, in *apppb.CreateKeyFromExistingKeyAuthorizationsRequest, opts ...grpc.CallOption,
) (*apppb.CreateKeyFromExistingKeyAuthorizationsResponse, error) {
	if asc.CreateKeyFromExistingKeyAuthorizationsFunc == nil {
		return asc.AppServiceClient.CreateKeyFromExistingKeyAuthorizations(ctx, in, opts...)
	}
	return asc.CreateKeyFromExistingKeyAuthorizationsFunc(ctx, in, opts...)
}
