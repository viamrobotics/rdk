package app

import (
	"context"
	"errors"
	"fmt"

	pb "go.viam.com/api/app/v1"
	common "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AppClient struct {
	client pb.AppServiceClient
}

// GetUserIDByEmail gets the ID of the user with the given email.
func (c *AppClient) GetUserIDByEmail(ctx context.Context, email string) (string, error) {
	resp, err := c.client.GetUserIDByEmail(ctx, &pb.GetUserIDByEmailRequest{
		Email: email,
	})
	if err != nil {
		return "", err
	}
	return resp.UserId, nil
}

// CreateOrganization creates a new organization.
func (c *AppClient) CreateOrganization(ctx context.Context, name string) (*pb.Organization, error) {
	resp, err := c.client.CreateOrganization(ctx, &pb.CreateOrganizationRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Organization, nil
}

// ListOrganizations lists all the organizations.
func (c *AppClient) ListOrganizations(ctx context.Context) ([]*pb.Organization, error) {
	resp, err := c.client.ListOrganizations(ctx, &pb.ListOrganizationsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Organizations, nil
}

// GetOrganizationsWithAccessToLocation gets all the organizations that have access to a location.
func (c *AppClient) GetOrganizationsWithAccessToLocation(ctx context.Context, locationId string) ([]*pb.OrganizationIdentity, error) {
	resp, err := c.client.GetOrganizationsWithAccessToLocation(ctx, &pb.GetOrganizationsWithAccessToLocationRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}
	return resp.OrganizationIdentities, nil
}

// ListOrganizationsByUser lists all the organizations that a user belongs to.
func (c *AppClient) ListOrganizationsByUser(ctx context.Context, userId string) ([]*pb.OrgDetails, error) {
	resp, err := c.client.ListOrganizationsByUser(ctx, &pb.ListOrganizationsByUserRequest{
		UserId: userId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Orgs, nil
}

// GetOrganization gets an organization.
func (c *AppClient) GetOrganization(ctx context.Context, orgId string) (*pb.Organization, error) {
	resp, err := c.client.GetOrganization(ctx, &pb.GetOrganizationRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Organization, nil
}

// GetOrganizationNamespaceAvailability checks for namespace availability throughout all organizations.
func (c *AppClient) GetOrganizationNamespaceAvailability(ctx context.Context, namespace string) (bool, error) {
	resp, err := c.client.GetOrganizationNamespaceAvailability(ctx, &pb.GetOrganizationNamespaceAvailabilityRequest{
		PublicNamespace: namespace,
	})
	if err != nil {
		return false, err
	}
	return resp.Available, nil
}


// UpdateOrganization updates an organization.
func (c *AppClient) UpdateOrganization(ctx context.Context, orgId string, name *string, namespace *string, region *string, cid *string) (*pb.Organization, error) {
	resp, err := c.client.UpdateOrganization(ctx, &pb.UpdateOrganizationRequest{
		OrganizationId: orgId,
		Name: name,
		PublicNamespace: namespace,
		Region: region,
		Cid: cid,
	})
	if err != nil {
		return nil, err
	}
	return resp.Organization, nil
}

// DeleteOrganization deletes an organization.
func (c *AppClient) DeleteOrganization(ctx context.Context, orgId string) error {
	_, err := c.client.DeleteOrganization(ctx, &pb.DeleteOrganizationRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return err
	}
	return nil
}

// ListOrganizationMembers lists all members of an organization and all invited members to the organization.
func (c *AppClient) ListOrganizationMembers(ctx context.Context, orgId string) ([]*pb.OrganizationMember, []*pb.OrganizationInvite, error) {
	resp, err := c.client.ListOrganizationMembers(ctx, &pb.ListOrganizationMembersRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return nil, nil, err
	}
	return resp.Members, resp.Invites, nil
}

// CreateOrganizaitonInvite creates an organization invite to an organization.
func (c *AppClient) CreateOrganizationInvite(ctx context.Context, orgId string, email string, authorizations []*pb.Authorization, sendEmailInvite *bool) (*pb.OrganizationInvite, error) {
	resp, err := c.client.CreateOrganizationInvite(ctx, &pb.CreateOrganizationInviteRequest{
		OrganizationId: orgId,
		Email: email,
		Authorizations: authorizations,
		SendEmailInvite: sendEmailInvite,
	})
	if err != nil {
		return nil, err
	}
	return resp.Invite, nil
}

// UpdateOrganizationInviteAuthorizations updates the authorizations attached to an organization invite.
func (c *AppClient) UpdateOrganizationInviteAuthorizations(ctx context.Context, orgId string, email string, addAuthorizations []*pb.Authorization, removeAuthorizations []*pb.Authorization) (*pb.OrganizationInvite, error) {
	resp, err := c.client.UpdateOrganizationInviteAuthorizations(ctx, &pb.UpdateOrganizationInviteAuthorizationsRequest{
		OrganizationId: orgId,
		Email: email,
		AddAuthorizations: addAuthorizations,
		RemoveAuthorizations: removeAuthorizations,
	})
	if err != nil {
		return nil, err
	}
	return resp.Invite, nil
}

// DeleteOrganizationMember deletes an organization member from an organization.
func (c *AppClient) DeleteOrganizationMember(ctx context.Context, orgId string, userId string) error {
	_, err := c.client.DeleteOrganizationMember(ctx, &pb.DeleteOrganizationMemberRequest{
		OrganizationId: orgId,
		UserId: userId,
	})
	if err != nil {
		return err
	}
	return nil
}

// DeleteOrganizationInvite deletes an organization invite.
func (c *AppClient) DeleteOrganizationInvite(ctx context.Context, orgId string, email string) error {
	_, err := c.client.DeleteOrganizationInvite(ctx, &pb.DeleteOrganizationInviteRequest{
		OrganizationId: orgId,
		Email: email,
	})
	if err != nil {
		return err
	}
	return nil
}

// ResendOrganizationInvite resends an organization invite.
func (c *AppClient) ResendOrganizationInvite(ctx context.Context, orgId string, email string) (*pb.OrganizationInvite, error) {
	resp, err := c.client.ResendOrganizationInvite(ctx, &pb.ResendOrganizationInviteRequest{
		OrganizationId: orgId,
		Email: email,
	})
	if err != nil {
		return nil, err
	}
	return resp.Invite, nil
}

// CreateLocation creates a location.
func (c *AppClient) CreateLocation(ctx context.Context, orgId string, name string, parentLocationId *string) (*pb.Location, error) {
	resp, err := c.client.CreateLocation(ctx, &pb.CreateLocationRequest{
		OrganizationId: orgId,
		Name: name,
		ParentLocationId: parentLocationId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Location, nil
}

// GetLocation gets a location.
func (c *AppClient) GetLocation(ctx context.Context, locationId string) (*pb.Location, error) {
	resp, err := c.client.GetLocation(ctx, &pb.GetLocationRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Location, nil
}

// UpdateLocation updates a location.
func (c *AppClient) UpdateLocation(ctx context.Context, locationId string, name *string, parentLocationId *string, region *string) (*pb.Location, error) {
	resp, err := c.client.UpdateLocation(ctx, &pb.UpdateLocationRequest{
		LocationId: locationId,
		Name: name,
		ParentLocationId: parentLocationId,
		Region: region,
	})
	if err != nil {
		return nil, err
	}
	return resp.Location, nil
}

// DeleteLocation deletes a location.
func (c *AppClient) DeleteLocation(ctx context.Context, locationId string) error {
	_, err := c.client.DeleteLocation(ctx, &pb.DeleteLocationRequest{
		LocationId: locationId,
	})
	if err != nil {
		return err
	}
	return nil
}

// ListLocations gets a list of locations under the specified organization.
func (c *AppClient) ListLocations(ctx context.Context, orgId string) ([]*pb.Location, error) {
	resp, err := c.client.ListLocations(ctx, &pb.ListLocationsRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Locations, nil
}

// ShareLocation shares a location with an organization.
func (c *AppClient) ShareLocation(ctx context.Context, locationId string, orgId string) error {
	_, err := c.client.ShareLocation(ctx, &pb.ShareLocationRequest{
		LocationId: locationId,
		OrganizationId: orgId,
	})
	if err != nil {
		return err
	}
	return nil
}

// UnshareLocation stops sharing a location with an organization.
func (c *AppClient) UnshareLocation(ctx context.Context, locationId string, orgId string) error {
	_, err := c.client.UnshareLocation(ctx, &pb.UnshareLocationRequest{
		LocationId: locationId,
		OrganizationId: orgId,
	})
	if err != nil {
		return err
	}
	return nil
}

// LocationAuth gets a location's authorization secrets.
func (c *AppClient) LocationAuth(ctx context.Context, locationId string) (*pb.LocationAuth, error) {
	resp, err := c.client.LocationAuth(ctx, &pb.LocationAuthRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Auth, nil
}

// CreateLocationSecret creates a new generated secret in the location. Succeeds if there are no more than 2 active secrets after creation.
func (c *AppClient) CreateLocationSecret(ctx context.Context, locationId string) (*pb.LocationAuth, error) {
	resp, err := c.client.CreateLocationSecret(ctx, &pb.CreateLocationSecretRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Auth, nil
}

// Delete a secret from the location.
func (c *AppClient) DeleteLocationSecret(ctx context.Context, locationId string, secretId string) error {
	_, err := c.client.DeleteLocationSecret(ctx, &pb.DeleteLocationSecretRequest{
		LocationId: locationId,
		SecretId: secretId,
	})
	if err != nil {
		return err
	}
	return nil
}

// GetRobot gets a specific robot by ID.
func (c *AppClient) GetRobot(ctx context.Context, id string) (*pb.Robot, error) {
	resp, err := c.client.GetRobot(ctx, &pb.GetRobotRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	return resp.Robot, nil
}

// GetRoverRentalRobots gets rover rental robots within an organization.
func (c *AppClient) GetRoverRentalRobots(ctx context.Context, orgId string) ([]*pb.RoverRentalRobot, error) {
	resp, err := c.client.GetRoverRentalRobots(ctx, &pb.GetRoverRentalRobotsRequest{
		OrgId: orgId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Robots, nil
}

// GetRobotParts gets a list of all the parts under a specific machine.
func (c *AppClient) GetRobotParts(ctx context.Context, robotId string) ([]*pb.RobotPart, error) {
	resp, err := c.client.GetRobotParts(ctx, &pb.GetRobotPartsRequest{
		RobotId: robotId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Parts, nil
}

// GetRobotPart gets a specific robot part and its config by ID.
func (c *AppClient) GetRobotPart(ctx context.Context, id string) (*pb.RobotPart, string, error) {
	resp, err := c.client.GetRobotPart(ctx, &pb.GetRobotPartRequest{
		Id: id,
	})
	if err != nil {
		return nil, "", err
	}
	return resp.Part, resp.ConfigJson, nil
}

// GetRobotPartLogs gets the logs associated with a robot part from a page, defaulting to the most recent page if pageToken is empty. Logs of all levels are returned when levels is empty.
func (c *AppClient) GetRobotPartLogs(ctx context.Context, id string, filter *string, pageToken *string, levels []string, start *timestamppb.Timestamp, end *timestamppb.Timestamp, limit *int64, source *string) ([]*common.LogEntry, string, error) {
	resp, err := c.client.GetRobotPartLogs(ctx, &pb.GetRobotPartLogsRequest{
		Id: id,
		Filter: filter,
		PageToken: pageToken,
		Levels: levels,
		Start: start,
		End: end,
		Limit: limit,
		Source: source,
	})
	if err != nil {
		return nil, "", err
	}
	return resp.Logs, resp.NextPageToken, nil
}

// // TailRobotPartLogs gets a stream of log entries for a specific robot part. Logs are ordered by newest first.
// func (c *AppClient) TailRobotPartLogs(ctx context.Context) (AppService_TailRobotPartLogsClient, error) {
// 	resp, err := c.client.
// 	if err != nil {
// 		return nil, err
// 	}
// 	return resp, nil
// }

// GetRobotPartHistory gets a specific robot part history by ID.
func (c *AppClient) GetRobotPartHistory(ctx context.Context, id string) ([]*pb.RobotPartHistoryEntry, error) {
	resp, err := c.client.GetRobotPartHistory(ctx, &pb.GetRobotPartHistoryRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	return resp.History, nil
}

// UpdaetRobotPart updates a robot part.
func (c *AppClient) UpdateRobotPart(ctx context.Context, id string, name string, robotConfig *structpb.Struct) (*pb.RobotPart, error) {
	resp, err := c.client.UpdateRobotPart(ctx, &pb.UpdateRobotPartRequest{
		Id: id,
		Name: name,
		RobotConfig: robotConfig,
	})
	if err != nil {
		return nil, err
	}
	return resp.Part, nil
}

// NewRobotPart creates a new robot part.
func (c *AppClient) NewRobotPart(ctx context.Context, robotId string, partName string) (string, error) {
	resp, err := c.client.NewRobotPart(ctx, &pb.NewRobotPartRequest{
		RobotId: robotId,
		PartName: partName,
	})
	if err != nil {
		return "", err
	}
	return resp.PartId, nil
}

// DeleteRobotPart deletes a robot part.
func (c *AppClient) DeleteRobotPart(ctx context.Context, partId string) error {
	_, err := c.client.DeleteRobotPart(ctx, &pb.DeleteRobotPartRequest{
		PartId: partId,
	})
	if err != nil {
		return err
	}
	return nil
}

// GetRobotAPIKeys gets the robot API keys for the robot.
func (c *AppClient) GetRobotAPIKeys(ctx context.Context, robotId string) ([]*pb.APIKeyWithAuthorizations, error) {
	resp, err := c.client.GetRobotAPIKeys(ctx, &pb.GetRobotAPIKeysRequest{
		RobotId: robotId,
	})
	if err != nil {
		return nil, err
	}
	return resp.ApiKeys, nil
}

// MarkPartAsMain marks the given part as the main part, and all the others as not.
func (c *AppClient) MarkPartAsMain(ctx context.Context, partId string) error {
	_, err := c.client.MarkPartAsMain(ctx, &pb.MarkPartAsMainRequest{
		PartId: partId,
	})
	if err != nil {
		return err
	}
	return nil
}

// MarkPartForRestart marks the given part for restart. Once the robot part checks-in with the app the flag is reset on the robot part. Calling this multiple times before a robot part checks-in has no effect.
func (c *AppClient) MarkPartForRestart(ctx context.Context, partId string) error {
	_, err := c.client.MarkPartForRestart(ctx, &pb.MarkPartForRestartRequest{
		PartId: partId,
	})
	if err != nil {
		return err
	}
	return nil
}

// CreateRobotPartSecret creates a new generated secret in the robot part. Succeeds if there are no more than 2 active secrets after creation.
func (c *AppClient) CreateRobotPartSecret(ctx context.Context, partId string) (*pb.RobotPart, error) {
	resp, err := c.client.CreateRobotPartSecret(ctx, &pb.CreateRobotPartSecretRequest{
		PartId: partId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Part, nil
}

// DeleteRobotPartSecret deletes a secret from the robot part.
func (c *AppClient) DeleteRobotPartSecret(ctx context.Context, partId string, secretId string) error {
	_, err := c.client.DeleteRobotPartSecret(ctx, &pb.DeleteRobotPartSecretRequest{
		PartId: partId,
		SecretId: secretId,
	})
	if err != nil {
		return err
	}
	return nil
}

// ListRobots gets a list of robots under a location.
func (c *AppClient) ListRobots(ctx context.Context, locationId string) ([]*pb.Robot, error) {
	resp, err := c.client.ListRobots(ctx, &pb.ListRobotsRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Robots, nil
}

// NewRobot creates a new robot.
func (c *AppClient) NewRobot(ctx context.Context, name string, location string) (string, error) {
	resp, err := c.client.NewRobot(ctx, &pb.NewRobotRequest{
		Name: name,
		Location: location,
	})
	if err != nil {
		return "", err
	}
	return resp.Id, nil
}

// UpdateRobot updates a robot.
func (c *AppClient) UpdateRobot(ctx context.Context, id string, name string, location string) (*pb.Robot, error) {
	resp, err := c.client.UpdateRobot(ctx, &pb.UpdateRobotRequest{
		Id: id,
		Name: name,
		Location: location,
	})
	if err != nil {
		return nil, err
	}
	return resp.Robot, nil
}

// DeleteRobot deletes a robot.
func (c *AppClient) DeleteRobot(ctx context.Context, id string) error {
	_, err := c.client.DeleteRobot(ctx, &pb.DeleteRobotRequest{
		Id: id,
	})
	if err != nil {
		return err
	}
	return nil
}

// ListFragments gets a list of fragments.
func (c *AppClient) ListFragments(ctx context.Context, orgId string, showPublic bool, fragmentVisibility []pb.FragmentVisibility) ([]*pb.Fragment, error) {
	resp, err := c.client.ListFragments(ctx, &pb.ListFragmentsRequest{
		OrganizationId: orgId,
		ShowPublic: showPublic,
		FragmentVisibility: fragmentVisibility,
	})
	if err != nil {
		return nil, err
	}
	return resp.Fragments, nil
}

// GetFragment gets a single fragment.
func (c *AppClient) GetFragment(ctx context.Context, id string) (*pb.Fragment, error) {
	resp, err := c.client.GetFragment(ctx, &pb.GetFragmentRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	return resp.Fragment, nil
}

// CreateFragment creates a fragment.
func (c *AppClient) CreateFragment(ctx context.Context, name string, config *structpb.Struct, orgId string, visibility *pb.FragmentVisibility) (*pb.Fragment, error) {
	resp, err := c.client.CreateFragment(ctx, &pb.CreateFragmentRequest{
		Name: name,
		Config: config,
		OrganizationId: orgId,
		Visibility: visibility,
	})
	if err != nil {
		return nil, err
	}
	return resp.Fragment, nil
}

// UpdateFragment updates a fragment.
func (c *AppClient) UpdateFragment(ctx context.Context, id string, name string, config *structpb.Struct, public *bool, visibility *pb.FragmentVisibility) (*pb.Fragment, error) {
	resp, err := c.client.UpdateFragment(ctx, &pb.UpdateFragmentRequest{
		Id: id,
		Name: name,
		Config: config,
		Public: public,
		Visibility: visibility,
	})
	if err != nil {
		return nil, err
	}
	return resp.Fragment, nil
}

// DeleteFragment deletes a fragment.
func (c *AppClient) DeleteFragment(ctx context.Context, id string) error {
	_, err := c.client.DeleteFragment(ctx, &pb.DeleteFragmentRequest{
		Id: id,
	})
	if err != nil {
		return err
	}
	return nil
}

// ListMachineFragments gets top level and nested fragments for a amchine, as well as any other fragments specified by IDs. Additional fragments are useful when needing to view fragments that will be provisionally added to the machine alongside existing fragments.
func (c *AppClient) ListMachineFragments(ctx context.Context, machineId string, additionalFragmentIds []string) ([]*pb.Fragment, error) {
	resp, err := c.client.ListMachineFragments(ctx, &pb.ListMachineFragmentsRequest{
		MachineId: machineId,
		AdditionalFragmentIds: additionalFragmentIds,
	})
	if err != nil {
		return nil, err
	}
	return resp.Fragments, nil
}

// GetFragmentHistory gets the fragment's history.
func (c *AppClient) GetFragmentHistory(ctx context.Context, id string, pageToken *string, pageLimit *int64) ([]*pb.FragmentHistoryEntry, string, error) {
	resp, err := c.client.GetFragmentHistory(ctx, &pb.GetFragmentHistoryRequest{
		Id: id,
		PageToken: pageToken,
		PageLimit: pageLimit,
	})
	if err != nil {
		return nil, "", err
	}
	return resp.History, resp.NextPageToken, nil
}

func createAuthorization(orgId string, identityId string, identityType string, role string, resourceType string, resourceId string) (*pb.Authorization, error) {
	if role != "owner" && role != "operator" {
		return nil, errors.New("role string must be 'owner' or 'operator'")
	}
	if resourceType != "organization" && resourceType != "location" && resourceType != "robot" {
		return nil, errors.New("resourceType must be 'organization', 'location', or 'robot'")
	}

	return &pb.Authorization{
		AuthorizationType: role,
		AuthorizationId: fmt.Sprintf("%s_%s", resourceType, role),
		ResourceType: resourceType,
		ResourceId: resourceId,
		IdentityId: identityId,
		OrganizationId: orgId,
		IdentityType: identityType,
	}, nil
}

// AddRole creates an identity authorization.
func (c *AppClient) AddRole(ctx context.Context, orgId string, identityId string, role string, resourceType string, resourceId string) error {
	authorization, err := createAuthorization(orgId, identityId, "", role, resourceType, resourceId)
	if err != nil {
		return err
	}
	_, err = c.client.AddRole(ctx, &pb.AddRoleRequest{
		Authorization: authorization,
	})
	if err != nil {
		return err
	}
	return nil
}

// RemoveRole deletes an identity authorization.
func (c *AppClient) RemoveRole(ctx context.Context, orgId string, identityId string, role string, resourceType string, resourceId string) error {
	authorization, err := createAuthorization(orgId, identityId, "", role, resourceType, resourceId)
	if err != nil {
		return err
	}
	_, err = c.client.RemoveRole(ctx, &pb.RemoveRoleRequest{
		Authorization: authorization,
	})
	if err != nil {
		return err
	}
	return nil
}

// ChangeRole changes an identity authorization to a new identity authorization.
func (c *AppClient) ChangeRole(ctx context.Context, oldOrgId string, oldIdentityId string, oldRole string, oldResourceType string, oldResourceId string, newOrgId string, newIdentityId string, newRole string, newResourceType string, newResourceId string) error {
	oldAuthorization, err := createAuthorization(oldOrgId, oldIdentityId, "", oldRole, oldResourceType, oldResourceId)
	if err != nil {
		return err
	}
	newAuthorization, err := createAuthorization(newOrgId, newIdentityId, "", newRole, newResourceType, newResourceId)
	if err != nil {
		return err
	}
	_, err = c.client.ChangeRole(ctx, &pb.ChangeRoleRequest{
		OldAuthorization: oldAuthorization,
		NewAuthorization: newAuthorization,
	})
	if err != nil {
		return err
	}
	return nil
}

// listAuthorizations returns all authorization roles for any given resources. If no resources are given, all resources within the organization will be included.
func (c *AppClient) ListAuthorizations(ctx context.Context, orgId string, resourceIds []string) ([]*pb.Authorization, error) {
	resp, err := c.client.ListAuthorizations(ctx, &pb.ListAuthorizationsRequest{
		OrganizationId: orgId,
		ResourceIds: resourceIds,
	})
	if err != nil {
		return nil, err
	}
	return resp.Authorizations, nil
}

// CheckPermissions checks the validity of a list of permissions.
func (c *AppClient) CheckPermissions(ctx context.Context, permissions []*pb.AuthorizedPermissions) ([]*pb.AuthorizedPermissions, error) {
	resp, err := c.client.CheckPermissions(ctx, &pb.CheckPermissionsRequest{
		Permissions: permissions,
	})
	if err != nil {
		return nil, err
	}
	return resp.AuthorizedPermissions, nil
}
