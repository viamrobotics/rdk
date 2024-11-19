// Package app defines the interfaces that manage a machine fleet with code instead of with the graphical interface of the Viam App.
//
// [fleet management docs]: https://docs.viam.com/appendix/apis/fleet/
package app

import (
	"context"
	"sync"

	packages "go.viam.com/api/app/packages/v1"
	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
)

// AppClient is a gRPC client for method calls to the App API.
//
//nolint:revive // stutter: Ignore the "stuttering" warning for this type name
type AppClient struct {
	client pb.AppServiceClient
	logger logging.Logger

	mu sync.Mutex
}

// NewAppClient constructs a new AppClient using the connection passed in by the viamClient and the provided logger.
func NewAppClient(conn rpc.ClientConn, logger logging.Logger) *AppClient {
	return &AppClient{client: pb.NewAppServiceClient(conn), logger: logger}
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
func (c *AppClient) CreateOrganization(ctx context.Context, name string) (*Organization, error) {
	resp, err := c.client.CreateOrganization(ctx, &pb.CreateOrganizationRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	return organizationFromProto(resp.Organization), nil
}

// ListOrganizations lists all the organizations.
func (c *AppClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	resp, err := c.client.ListOrganizations(ctx, &pb.ListOrganizationsRequest{})
	if err != nil {
		return nil, err
	}

	var organizations []*Organization
	for _, org := range resp.Organizations {
		organizations = append(organizations, organizationFromProto(org))
	}
	return organizations, nil
}

// GetOrganizationsWithAccessToLocation gets all the organizations that have access to a location.
func (c *AppClient) GetOrganizationsWithAccessToLocation(ctx context.Context, locationID string) ([]*OrganizationIdentity, error) {
	resp, err := c.client.GetOrganizationsWithAccessToLocation(ctx, &pb.GetOrganizationsWithAccessToLocationRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}

	var organizations []*OrganizationIdentity
	for _, org := range resp.OrganizationIdentities {
		organizations = append(organizations, organizationIdentityFromProto(org))
	}
	return organizations, nil
}

// ListOrganizationsByUser lists all the organizations that a user belongs to.
func (c *AppClient) ListOrganizationsByUser(ctx context.Context, userID string) ([]*OrgDetails, error) {
	resp, err := c.client.ListOrganizationsByUser(ctx, &pb.ListOrganizationsByUserRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, err
	}

	var organizations []*OrgDetails
	for _, org := range resp.Orgs {
		organizations = append(organizations, orgDetailsFromProto(org))
	}
	return organizations, nil
}

// GetOrganization gets an organization.
func (c *AppClient) GetOrganization(ctx context.Context, orgID string) (*Organization, error) {
	resp, err := c.client.GetOrganization(ctx, &pb.GetOrganizationRequest{
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, err
	}
	return organizationFromProto(resp.Organization), nil
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
func (c *AppClient) UpdateOrganization(ctx context.Context, orgID string, name, namespace, region, cid *string) (*Organization, error) {
	resp, err := c.client.UpdateOrganization(ctx, &pb.UpdateOrganizationRequest{
		OrganizationId:  orgID,
		Name:            name,
		PublicNamespace: namespace,
		Region:          region,
		Cid:             cid,
	})
	if err != nil {
		return nil, err
	}
	return organizationFromProto(resp.Organization), nil
}

// DeleteOrganization deletes an organization.
func (c *AppClient) DeleteOrganization(ctx context.Context, orgID string) error {
	_, err := c.client.DeleteOrganization(ctx, &pb.DeleteOrganizationRequest{
		OrganizationId: orgID,
	})
	return err
}

// ListOrganizationMembers lists all members of an organization and all invited members to the organization.
func (c *AppClient) ListOrganizationMembers(ctx context.Context, orgID string) ([]*OrganizationMember, []*OrganizationInvite, error) {
	resp, err := c.client.ListOrganizationMembers(ctx, &pb.ListOrganizationMembersRequest{
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, nil, err
	}

	var members []*OrganizationMember
	for _, member := range resp.Members {
		members = append(members, organizationMemberFromProto(member))
	}
	var invites []*OrganizationInvite
	for _, invite := range resp.Invites {
		invites = append(invites, organizationInviteFromProto(invite))
	}
	return members, invites, nil
}

// CreateOrganizationInvite creates an organization invite to an organization.
func (c *AppClient) CreateOrganizationInvite(
	ctx context.Context, orgID, email string, authorizations []*Authorization, sendEmailInvite *bool,
) (*OrganizationInvite, error) {
	var pbAuthorizations []*pb.Authorization
	for _, authorization := range authorizations {
		pbAuthorizations = append(pbAuthorizations, authorizationToProto(authorization))
	}
	resp, err := c.client.CreateOrganizationInvite(ctx, &pb.CreateOrganizationInviteRequest{
		OrganizationId:  orgID,
		Email:           email,
		Authorizations:  pbAuthorizations,
		SendEmailInvite: sendEmailInvite,
	})
	if err != nil {
		return nil, err
	}
	return organizationInviteFromProto(resp.Invite), nil
}

// UpdateOrganizationInviteAuthorizations updates the authorizations attached to an organization invite.
func (c *AppClient) UpdateOrganizationInviteAuthorizations(
	ctx context.Context, orgID, email string, addAuthorizations, removeAuthorizations []*Authorization,
) (*OrganizationInvite, error) {
	var pbAddAuthorizations []*pb.Authorization
	for _, authorization := range addAuthorizations {
		pbAddAuthorizations = append(pbAddAuthorizations, authorizationToProto(authorization))
	}
	var pbRemoveAuthorizations []*pb.Authorization
	for _, authorization := range removeAuthorizations {
		pbRemoveAuthorizations = append(pbRemoveAuthorizations, authorizationToProto(authorization))
	}
	resp, err := c.client.UpdateOrganizationInviteAuthorizations(ctx, &pb.UpdateOrganizationInviteAuthorizationsRequest{
		OrganizationId:       orgID,
		Email:                email,
		AddAuthorizations:    pbAddAuthorizations,
		RemoveAuthorizations: pbRemoveAuthorizations,
	})
	if err != nil {
		return nil, err
	}
	return organizationInviteFromProto(resp.Invite), nil
}

// DeleteOrganizationMember deletes an organization member from an organization.
func (c *AppClient) DeleteOrganizationMember(ctx context.Context, orgID, userID string) error {
	_, err := c.client.DeleteOrganizationMember(ctx, &pb.DeleteOrganizationMemberRequest{
		OrganizationId: orgID,
		UserId:         userID,
	})
	return err
}

// DeleteOrganizationInvite deletes an organization invite.
func (c *AppClient) DeleteOrganizationInvite(ctx context.Context, orgID, email string) error {
	_, err := c.client.DeleteOrganizationInvite(ctx, &pb.DeleteOrganizationInviteRequest{
		OrganizationId: orgID,
		Email:          email,
	})
	return err
}

// ResendOrganizationInvite resends an organization invite.
func (c *AppClient) ResendOrganizationInvite(ctx context.Context, orgID, email string) (*OrganizationInvite, error) {
	resp, err := c.client.ResendOrganizationInvite(ctx, &pb.ResendOrganizationInviteRequest{
		OrganizationId: orgID,
		Email:          email,
	})
	if err != nil {
		return nil, err
	}
	return organizationInviteFromProto(resp.Invite), nil
}

// EnableBillingService enables a billing service to an address in an organization.
func (c *AppClient) EnableBillingService(ctx context.Context, orgID string, billingAddress *BillingAddress) error {
	_, err := c.client.EnableBillingService(ctx, &pb.EnableBillingServiceRequest{
		OrgId:          orgID,
		BillingAddress: billingAddressToProto(billingAddress),
	})
	return err
}

// DisableBillingService disables the billing service for an organization.
func (c *AppClient) DisableBillingService(ctx context.Context, orgID string) error {
	_, err := c.client.DisableBillingService(ctx, &pb.DisableBillingServiceRequest{
		OrgId: orgID,
	})
	return err
}

// UpdateBillingService updates the billing service of an organization.
func (c *AppClient) UpdateBillingService(
	ctx context.Context, orgID string, billingAddress *BillingAddress, billingSupportEmail string,
) error {
	_, err := c.client.UpdateBillingService(ctx, &pb.UpdateBillingServiceRequest{
		OrgId:               orgID,
		BillingAddress:      billingAddressToProto(billingAddress),
		BillingSupportEmail: billingSupportEmail,
	})
	return err
}

// OrganizationSetSupportEmail sets an organization's support email.
func (c *AppClient) OrganizationSetSupportEmail(ctx context.Context, orgID, email string) error {
	_, err := c.client.OrganizationSetSupportEmail(ctx, &pb.OrganizationSetSupportEmailRequest{
		OrgId: orgID,
		Email: email,
	})
	return err
}

// OrganizationGetSupportEmail gets an organization's support email.
func (c *AppClient) OrganizationGetSupportEmail(ctx context.Context, orgID string) (string, error) {
	resp, err := c.client.OrganizationGetSupportEmail(ctx, &pb.OrganizationGetSupportEmailRequest{
		OrgId: orgID,
	})
	if err != nil {
		return "", err
	}
	return resp.Email, nil
}

// CreateLocation creates a location.
func (c *AppClient) CreateLocation(ctx context.Context, orgID, name string, parentLocationID *string) (*Location, error) {
	resp, err := c.client.CreateLocation(ctx, &pb.CreateLocationRequest{
		OrganizationId:   orgID,
		Name:             name,
		ParentLocationId: parentLocationID,
	})
	if err != nil {
		return nil, err
	}
	return locationFromProto(resp.Location), nil
}

// GetLocation gets a location.
func (c *AppClient) GetLocation(ctx context.Context, locationID string) (*Location, error) {
	resp, err := c.client.GetLocation(ctx, &pb.GetLocationRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}
	return locationFromProto(resp.Location), nil
}

// UpdateLocation updates a location.
func (c *AppClient) UpdateLocation(ctx context.Context, locationID string, name, parentLocationID, region *string) (*Location, error) {
	resp, err := c.client.UpdateLocation(ctx, &pb.UpdateLocationRequest{
		LocationId:       locationID,
		Name:             name,
		ParentLocationId: parentLocationID,
		Region:           region,
	})
	if err != nil {
		return nil, err
	}
	return locationFromProto(resp.Location), nil
}

// DeleteLocation deletes a location.
func (c *AppClient) DeleteLocation(ctx context.Context, locationID string) error {
	_, err := c.client.DeleteLocation(ctx, &pb.DeleteLocationRequest{
		LocationId: locationID,
	})
	return err
}

// ListLocations gets a list of locations under the specified organization.
func (c *AppClient) ListLocations(ctx context.Context, orgID string) ([]*Location, error) {
	resp, err := c.client.ListLocations(ctx, &pb.ListLocationsRequest{
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, err
	}

	var locations []*Location
	for _, location := range resp.Locations {
		locations = append(locations, locationFromProto(location))
	}
	return locations, nil
}

// ShareLocation shares a location with an organization.
func (c *AppClient) ShareLocation(ctx context.Context, locationID, orgID string) error {
	_, err := c.client.ShareLocation(ctx, &pb.ShareLocationRequest{
		LocationId:     locationID,
		OrganizationId: orgID,
	})
	return err
}

// UnshareLocation stops sharing a location with an organization.
func (c *AppClient) UnshareLocation(ctx context.Context, locationID, orgID string) error {
	_, err := c.client.UnshareLocation(ctx, &pb.UnshareLocationRequest{
		LocationId:     locationID,
		OrganizationId: orgID,
	})
	return err
}

// LocationAuth gets a location's authorization secrets.
func (c *AppClient) LocationAuth(ctx context.Context, locationID string) (*LocationAuth, error) {
	resp, err := c.client.LocationAuth(ctx, &pb.LocationAuthRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}
	return locationAuthFromProto(resp.Auth), nil
}

// CreateLocationSecret creates a new generated secret in the location. Succeeds if there are no more than 2 active secrets after creation.
func (c *AppClient) CreateLocationSecret(ctx context.Context, locationID string) (*LocationAuth, error) {
	resp, err := c.client.CreateLocationSecret(ctx, &pb.CreateLocationSecretRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}
	return locationAuthFromProto(resp.Auth), nil
}

// DeleteLocationSecret deletes a secret from the location.
func (c *AppClient) DeleteLocationSecret(ctx context.Context, locationID, secretID string) error {
	_, err := c.client.DeleteLocationSecret(ctx, &pb.DeleteLocationSecretRequest{
		LocationId: locationID,
		SecretId:   secretID,
	})
	return err
}

// GetRobot gets a specific robot by ID.
func (c *AppClient) GetRobot(ctx context.Context, id string) (*Robot, error) {
	resp, err := c.client.GetRobot(ctx, &pb.GetRobotRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	return robotFromProto(resp.Robot), nil
}

// GetRoverRentalRobots gets rover rental robots within an organization.
func (c *AppClient) GetRoverRentalRobots(ctx context.Context, orgID string) ([]*RoverRentalRobot, error) {
	resp, err := c.client.GetRoverRentalRobots(ctx, &pb.GetRoverRentalRobotsRequest{
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}
	var robots []*RoverRentalRobot
	for _, robot := range resp.Robots {
		robots = append(robots, roverRentalRobotFromProto(robot))
	}
	return robots, nil
}

// GetRobotParts gets a list of all the parts under a specific machine.
func (c *AppClient) GetRobotParts(ctx context.Context, robotID string) ([]*RobotPart, error) {
	resp, err := c.client.GetRobotParts(ctx, &pb.GetRobotPartsRequest{
		RobotId: robotID,
	})
	if err != nil {
		return nil, err
	}
	var parts []*RobotPart
	for _, part := range resp.Parts {
		parts = append(parts, robotPartFromProto(part))
	}
	return parts, nil
}

// GetRobotPart gets a specific robot part and its config by ID.
func (c *AppClient) GetRobotPart(ctx context.Context, id string) (*RobotPart, string, error) {
	resp, err := c.client.GetRobotPart(ctx, &pb.GetRobotPartRequest{
		Id: id,
	})
	if err != nil {
		return nil, "", err
	}
	return robotPartFromProto(resp.Part), resp.ConfigJson, nil
}

// GetRobotPartLogs gets the logs associated with a robot part from a page, defaulting to the most recent page if pageToken is empty.
// Logs of all levels are returned when levels is empty.
func (c *AppClient) GetRobotPartLogs(
	ctx context.Context,
	id string,
	filter,
	pageToken *string,
	levels []string,
	start,
	end *timestamppb.Timestamp,
	limit *int64,
	source *string,
) ([]*LogEntry, string, error) {
	resp, err := c.client.GetRobotPartLogs(ctx, &pb.GetRobotPartLogsRequest{
		Id:        id,
		Filter:    filter,
		PageToken: pageToken,
		Levels:    levels,
		Start:     start,
		End:       end,
		Limit:     limit,
		Source:    source,
	})
	if err != nil {
		return nil, "", err
	}
	var logs []*LogEntry
	for _, log := range resp.Logs {
		logs = append(logs, logEntryFromProto(log))
	}
	return logs, resp.NextPageToken, nil
}

// TailRobotPartLogs gets a stream of log entries for a specific robot part. Logs are ordered by newest first.
func (c *AppClient) TailRobotPartLogs(ctx context.Context, id string, errorsOnly bool, filter *string, ch chan []*LogEntry) error {
	stream := &robotPartLogStream{client: c}

	err := stream.startStream(ctx, id, errorsOnly, filter, ch)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return nil
}

// GetRobotPartHistory gets a specific robot part history by ID.
func (c *AppClient) GetRobotPartHistory(ctx context.Context, id string) ([]*RobotPartHistoryEntry, error) {
	resp, err := c.client.GetRobotPartHistory(ctx, &pb.GetRobotPartHistoryRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	var history []*RobotPartHistoryEntry
	for _, entry := range resp.History {
		history = append(history, robotPartHistoryEntryFromProto(entry))
	}
	return history, nil
}

// UpdateRobotPart updates a robot part.
func (c *AppClient) UpdateRobotPart(ctx context.Context, id, name string, robotConfig interface{}) (*RobotPart, error) {
	config, err := protoutils.StructToStructPb(robotConfig)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.UpdateRobotPart(ctx, &pb.UpdateRobotPartRequest{
		Id:          id,
		Name:        name,
		RobotConfig: config,
	})
	if err != nil {
		return nil, err
	}
	return robotPartFromProto(resp.Part), nil
}

// NewRobotPart creates a new robot part.
func (c *AppClient) NewRobotPart(ctx context.Context, robotID, partName string) (string, error) {
	resp, err := c.client.NewRobotPart(ctx, &pb.NewRobotPartRequest{
		RobotId:  robotID,
		PartName: partName,
	})
	if err != nil {
		return "", err
	}
	return resp.PartId, nil
}

// DeleteRobotPart deletes a robot part.
func (c *AppClient) DeleteRobotPart(ctx context.Context, partID string) error {
	_, err := c.client.DeleteRobotPart(ctx, &pb.DeleteRobotPartRequest{
		PartId: partID,
	})
	return err
}

// GetRobotAPIKeys gets the robot API keys for the robot.
func (c *AppClient) GetRobotAPIKeys(ctx context.Context, robotID string) ([]*APIKeyWithAuthorizations, error) {
	resp, err := c.client.GetRobotAPIKeys(ctx, &pb.GetRobotAPIKeysRequest{
		RobotId: robotID,
	})
	if err != nil {
		return nil, err
	}
	var keys []*APIKeyWithAuthorizations
	for _, key := range resp.ApiKeys {
		keys = append(keys, apiKeyWithAuthorizationsFromProto(key))
	}
	return keys, nil
}

// MarkPartAsMain marks the given part as the main part, and all the others as not.
func (c *AppClient) MarkPartAsMain(ctx context.Context, partID string) error {
	_, err := c.client.MarkPartAsMain(ctx, &pb.MarkPartAsMainRequest{
		PartId: partID,
	})
	return err
}

// MarkPartForRestart marks the given part for restart.
// Once the robot part checks-in with the app the flag is reset on the robot part.
// Calling this multiple times before a robot part checks-in has no effect.
func (c *AppClient) MarkPartForRestart(ctx context.Context, partID string) error {
	_, err := c.client.MarkPartForRestart(ctx, &pb.MarkPartForRestartRequest{
		PartId: partID,
	})
	return err
}

// CreateRobotPartSecret creates a new generated secret in the robot part.
// Succeeds if there are no more than 2 active secrets after creation.
func (c *AppClient) CreateRobotPartSecret(ctx context.Context, partID string) (*RobotPart, error) {
	resp, err := c.client.CreateRobotPartSecret(ctx, &pb.CreateRobotPartSecretRequest{
		PartId: partID,
	})
	if err != nil {
		return nil, err
	}
	return robotPartFromProto(resp.Part), nil
}

// DeleteRobotPartSecret deletes a secret from the robot part.
func (c *AppClient) DeleteRobotPartSecret(ctx context.Context, partID, secretID string) error {
	_, err := c.client.DeleteRobotPartSecret(ctx, &pb.DeleteRobotPartSecretRequest{
		PartId:   partID,
		SecretId: secretID,
	})
	return err
}

// ListRobots gets a list of robots under a location.
func (c *AppClient) ListRobots(ctx context.Context, locationID string) ([]*Robot, error) {
	resp, err := c.client.ListRobots(ctx, &pb.ListRobotsRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}
	var robots []*Robot
	for _, robot := range resp.Robots {
		robots = append(robots, robotFromProto(robot))
	}
	return robots, nil
}

// NewRobot creates a new robot.
func (c *AppClient) NewRobot(ctx context.Context, name, location string) (string, error) {
	resp, err := c.client.NewRobot(ctx, &pb.NewRobotRequest{
		Name:     name,
		Location: location,
	})
	if err != nil {
		return "", err
	}
	return resp.Id, nil
}

// UpdateRobot updates a robot.
func (c *AppClient) UpdateRobot(ctx context.Context, id, name, location string) (*Robot, error) {
	resp, err := c.client.UpdateRobot(ctx, &pb.UpdateRobotRequest{
		Id:       id,
		Name:     name,
		Location: location,
	})
	if err != nil {
		return nil, err
	}
	return robotFromProto(resp.Robot), nil
}

// DeleteRobot deletes a robot.
func (c *AppClient) DeleteRobot(ctx context.Context, id string) error {
	_, err := c.client.DeleteRobot(ctx, &pb.DeleteRobotRequest{
		Id: id,
	})
	return err
}

// ListFragments gets a list of fragments.
func (c *AppClient) ListFragments(
	ctx context.Context, orgID string, showPublic bool, fragmentVisibility []FragmentVisibility,
) ([]*Fragment, error) {
	var visibilities []pb.FragmentVisibility
	for _, visibility := range fragmentVisibility {
		pbFragmentVisibility := fragmentVisibilityToProto(visibility)
		visibilities = append(visibilities, pbFragmentVisibility)
	}
	resp, err := c.client.ListFragments(ctx, &pb.ListFragmentsRequest{
		OrganizationId:     orgID,
		ShowPublic:         showPublic,
		FragmentVisibility: visibilities,
	})
	if err != nil {
		return nil, err
	}
	var fragments []*Fragment
	for _, fragment := range resp.Fragments {
		fragments = append(fragments, fragmentFromProto(fragment))
	}
	return fragments, nil
}

// GetFragment gets a single fragment.
func (c *AppClient) GetFragment(ctx context.Context, id string) (*Fragment, error) {
	resp, err := c.client.GetFragment(ctx, &pb.GetFragmentRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	return fragmentFromProto(resp.Fragment), nil
}

// CreateFragment creates a fragment.
func (c *AppClient) CreateFragment(
	ctx context.Context, name string, config interface{}, orgID string, visibility *FragmentVisibility,
) (*Fragment, error) {
	cfg, err := protoutils.StructToStructPb(config)
	if err != nil {
		return nil, err
	}
	pbFragmentVisibility := fragmentVisibilityToProto(*visibility)
	resp, err := c.client.CreateFragment(ctx, &pb.CreateFragmentRequest{
		Name:           name,
		Config:         cfg,
		OrganizationId: orgID,
		Visibility:     &pbFragmentVisibility,
	})
	if err != nil {
		return nil, err
	}
	return fragmentFromProto(resp.Fragment), nil
}

// UpdateFragment updates a fragment.
func (c *AppClient) UpdateFragment(
	ctx context.Context, id, name string, config map[string]interface{}, public *bool, visibility *FragmentVisibility,
) (*Fragment, error) {
	cfg, err := protoutils.StructToStructPb(config)
	if err != nil {
		return nil, err
	}
	pbVisibility := fragmentVisibilityToProto(*visibility)
	resp, err := c.client.UpdateFragment(ctx, &pb.UpdateFragmentRequest{
		Id:         id,
		Name:       name,
		Config:     cfg,
		Public:     public,
		Visibility: &pbVisibility,
	})
	if err != nil {
		return nil, err
	}
	return fragmentFromProto(resp.Fragment), nil
}

// DeleteFragment deletes a fragment.
func (c *AppClient) DeleteFragment(ctx context.Context, id string) error {
	_, err := c.client.DeleteFragment(ctx, &pb.DeleteFragmentRequest{
		Id: id,
	})
	return err
}

// ListMachineFragments gets top level and nested fragments for a amchine, as well as any other fragments specified by IDs. Additional
// fragments are useful when needing to view fragments that will be provisionally added to the machine alongside existing fragments.
func (c *AppClient) ListMachineFragments(ctx context.Context, machineID string, additionalFragmentIDs []string) ([]*Fragment, error) {
	resp, err := c.client.ListMachineFragments(ctx, &pb.ListMachineFragmentsRequest{
		MachineId:             machineID,
		AdditionalFragmentIds: additionalFragmentIDs,
	})
	if err != nil {
		return nil, err
	}
	var fragments []*Fragment
	for _, fragment := range resp.Fragments {
		fragments = append(fragments, fragmentFromProto(fragment))
	}
	return fragments, nil
}

// GetFragmentHistory gets the fragment's history.
func (c *AppClient) GetFragmentHistory(
	ctx context.Context, id string, pageToken *string, pageLimit *int64,
) ([]*FragmentHistoryEntry, string, error) {
	resp, err := c.client.GetFragmentHistory(ctx, &pb.GetFragmentHistoryRequest{
		Id:        id,
		PageToken: pageToken,
		PageLimit: pageLimit,
	})
	if err != nil {
		return nil, "", err
	}
	var history []*FragmentHistoryEntry
	for _, entry := range resp.History {
		history = append(history, fragmentHistoryEntryFromProto(entry))
	}
	return history, resp.NextPageToken, nil
}

// AddRole creates an identity authorization.
func (c *AppClient) AddRole(ctx context.Context, orgID, identityID, role, resourceType, resourceID string) error {
	authorization, err := createAuthorization(orgID, identityID, "", role, resourceType, resourceID)
	if err != nil {
		return err
	}
	_, err = c.client.AddRole(ctx, &pb.AddRoleRequest{
		Authorization: authorization,
	})
	return err
}

// RemoveRole deletes an identity authorization.
func (c *AppClient) RemoveRole(ctx context.Context, orgID, identityID, role, resourceType, resourceID string) error {
	authorization, err := createAuthorization(orgID, identityID, "", role, resourceType, resourceID)
	if err != nil {
		return err
	}
	_, err = c.client.RemoveRole(ctx, &pb.RemoveRoleRequest{
		Authorization: authorization,
	})
	return err
}

// ChangeRole changes an identity authorization to a new identity authorization.
func (c *AppClient) ChangeRole(
	ctx context.Context,
	oldOrgID,
	oldIdentityID,
	oldRole,
	oldResourceType,
	oldResourceID,
	newOrgID,
	newIdentityID,
	newRole,
	newResourceType,
	newResourceID string,
) error {
	oldAuthorization, err := createAuthorization(oldOrgID, oldIdentityID, "", oldRole, oldResourceType, oldResourceID)
	if err != nil {
		return err
	}
	newAuthorization, err := createAuthorization(newOrgID, newIdentityID, "", newRole, newResourceType, newResourceID)
	if err != nil {
		return err
	}
	_, err = c.client.ChangeRole(ctx, &pb.ChangeRoleRequest{
		OldAuthorization: oldAuthorization,
		NewAuthorization: newAuthorization,
	})
	return err
}

// ListAuthorizations returns all authorization roles for any given resources.
// If no resources are given, all resources within the organization will be included.
func (c *AppClient) ListAuthorizations(ctx context.Context, orgID string, resourceIDs []string) ([]*Authorization, error) {
	resp, err := c.client.ListAuthorizations(ctx, &pb.ListAuthorizationsRequest{
		OrganizationId: orgID,
		ResourceIds:    resourceIDs,
	})
	if err != nil {
		return nil, err
	}
	var authorizations []*Authorization
	for _, authorization := range resp.Authorizations {
		authorizations = append(authorizations, authorizationFromProto(authorization))
	}
	return authorizations, nil
}

// CheckPermissions checks the validity of a list of permissions.
func (c *AppClient) CheckPermissions(ctx context.Context, permissions []*AuthorizedPermissions) ([]*AuthorizedPermissions, error) {
	var pbPermissions []*pb.AuthorizedPermissions
	for _, permission := range permissions {
		pbPermissions = append(pbPermissions, authorizedPermissionsToProto(permission))
	}

	resp, err := c.client.CheckPermissions(ctx, &pb.CheckPermissionsRequest{
		Permissions: pbPermissions,
	})
	if err != nil {
		return nil, err
	}

	var authorizedPermissions []*AuthorizedPermissions
	for _, permission := range resp.AuthorizedPermissions {
		authorizedPermissions = append(authorizedPermissions, authorizedPermissionsFromProto(permission))
	}
	return authorizedPermissions, nil
}

// GetRegistryItem gets a registry item.
func (c *AppClient) GetRegistryItem(ctx context.Context, itemID string) (*RegistryItem, error) {
	resp, err := c.client.GetRegistryItem(ctx, &pb.GetRegistryItemRequest{
		ItemId: itemID,
	})
	if err != nil {
		return nil, err
	}
	item, err := registryItemFromProto(resp.Item)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// CreateRegistryItem creates a registry item.
func (c *AppClient) CreateRegistryItem(ctx context.Context, orgID, name string, packageType PackageType) error {
	_, err := c.client.CreateRegistryItem(ctx, &pb.CreateRegistryItemRequest{
		OrganizationId: orgID,
		Name:           name,
		Type:           packageTypeToProto(packageType),
	})
	return err
}

// UpdateRegistryItem updates a registry item.
func (c *AppClient) UpdateRegistryItem(
	ctx context.Context, itemID string, packageType PackageType, description string, visibility Visibility, url *string,
) error {
	_, err := c.client.UpdateRegistryItem(ctx, &pb.UpdateRegistryItemRequest{
		ItemId:      itemID,
		Type:        packageTypeToProto(packageType),
		Description: description,
		Visibility:  visibilityToProto(visibility),
		Url:         url,
	})
	return err
}

// ListRegistryItems lists the registry items in an organization.
func (c *AppClient) ListRegistryItems(
	ctx context.Context,
	orgID *string,
	types []PackageType,
	visibilities []Visibility,
	platforms []string,
	statuses []RegistryItemStatus,
	searchTerm,
	pageToken *string,
	publicNamespaces []string,
) ([]*RegistryItem, error) {
	var pbTypes []packages.PackageType
	for _, packageType := range types {
		pbTypes = append(pbTypes, packageTypeToProto(packageType))
	}
	var pbVisibilities []pb.Visibility
	for _, visibility := range visibilities {
		pbVisibilities = append(pbVisibilities, visibilityToProto(visibility))
	}
	var pbStatuses []pb.RegistryItemStatus
	for _, status := range statuses {
		pbStatuses = append(pbStatuses, registryItemStatusToProto(status))
	}
	resp, err := c.client.ListRegistryItems(ctx, &pb.ListRegistryItemsRequest{
		OrganizationId:   orgID,
		Types:            pbTypes,
		Visibilities:     pbVisibilities,
		Platforms:        platforms,
		Statuses:         pbStatuses,
		SearchTerm:       searchTerm,
		PageToken:        pageToken,
		PublicNamespaces: publicNamespaces,
	})
	if err != nil {
		return nil, err
	}
	var items []*RegistryItem
	for _, item := range resp.Items {
		i, err := registryItemFromProto(item)
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

// DeleteRegistryItem deletes a registry item given an ID that is formatted as `prefix:name“
// where `prefix“ is the owner's organization ID or namespace.
func (c *AppClient) DeleteRegistryItem(ctx context.Context, itemID string) error {
	_, err := c.client.DeleteRegistryItem(ctx, &pb.DeleteRegistryItemRequest{
		ItemId: itemID,
	})
	return err
}

// TransferRegistryItem transfers a registry item to a namespace.
func (c *AppClient) TransferRegistryItem(ctx context.Context, itemID, newPublicNamespace string) error {
	_, err := c.client.TransferRegistryItem(ctx, &pb.TransferRegistryItemRequest{
		ItemId:             itemID,
		NewPublicNamespace: newPublicNamespace,
	})
	return err
}

// CreateModule creates a module.
func (c *AppClient) CreateModule(ctx context.Context, orgID, name string) (string, string, error) {
	resp, err := c.client.CreateModule(ctx, &pb.CreateModuleRequest{
		OrganizationId: orgID,
		Name:           name,
	})
	if err != nil {
		return "", "", err
	}
	return resp.ModuleId, resp.Url, nil
}

// UpdateModule updates the documentation URL, description, models, entrypoint, and/or the visibility of a module.
// A path to a setup script can be added that is run before a newly downloaded module starts.
func (c *AppClient) UpdateModule(
	ctx context.Context, moduleID string, visibility Visibility, url, description string, models []*Model, entrypoint string, firstRun *string,
) (string, error) {
	var pbModels []*pb.Model
	for _, model := range models {
		pbModels = append(pbModels, modelToProto(model))
	}
	resp, err := c.client.UpdateModule(ctx, &pb.UpdateModuleRequest{
		ModuleId:    moduleID,
		Visibility:  visibilityToProto(visibility),
		Url:         url,
		Description: description,
		Models:      pbModels,
		Entrypoint:  entrypoint,
		FirstRun:    firstRun,
	})
	if err != nil {
		return "", err
	}
	return resp.Url, nil
}

// UploadModuleFile uploads a module file and returns the URL of the uploaded file.
func (c *AppClient) UploadModuleFile(ctx context.Context, fileInfo ModuleFileInfo, file []byte) (string, error) {
	stream := &uploadModuleFileStream{client: c}
	// reqModuleFileInfo := uploadModuleFileRequest{
	// 	ModuleFile: &uploadModuleFileRequestModuleFileInfo{
	// 		ModuleFileInfo: &fileInfo,
	// 	},
	// }
	// reqFile := uploadModuleFileRequest{
	// 	ModuleFile: &uploadModuleFileRequestFile{
	// 		File: file,
	// 	},
	// }
	url, err := stream.startStream(ctx, &fileInfo, file)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return url, nil
}

// GetModule gets a module.
func (c *AppClient) GetModule(ctx context.Context, moduleID string) (*Module, error) {
	resp, err := c.client.GetModule(ctx, &pb.GetModuleRequest{
		ModuleId: moduleID,
	})
	if err != nil {
		return nil, err
	}
	return moduleFromProto(resp.Module), nil
}

// ListModules lists the modules in the organization.
func (c *AppClient) ListModules(ctx context.Context, orgID *string) ([]*Module, error) {
	resp, err := c.client.ListModules(ctx, &pb.ListModulesRequest{
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, err
	}
	var modules []*Module
	for _, module := range resp.Modules {
		modules = append(modules, moduleFromProto(module))
	}
	return modules, nil
}

// CreateKey creates a new API key associated with a list of authorizations.
func (c *AppClient) CreateKey(
	ctx context.Context, orgID string, keyAuthorizations []APIKeyAuthorization, name string,
) (string, string, error) {
	var authorizations []*pb.Authorization
	for _, keyAuthorization := range keyAuthorizations {
		authorization, err := createAuthorization(
			orgID, "", "api-key", keyAuthorization.role, keyAuthorization.resourceType, keyAuthorization.resourceID)
		if err != nil {
			return "", "", err
		}
		authorizations = append(authorizations, authorization)
	}

	resp, err := c.client.CreateKey(ctx, &pb.CreateKeyRequest{
		Authorizations: authorizations,
		Name:           name,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Key, resp.Id, nil
}

// DeleteKey deletes an API key.
func (c *AppClient) DeleteKey(ctx context.Context, id string) error {
	_, err := c.client.DeleteKey(ctx, &pb.DeleteKeyRequest{
		Id: id,
	})
	return err
}

// ListKeys lists all the keys for the organization.
func (c *AppClient) ListKeys(ctx context.Context, orgID string) ([]*APIKeyWithAuthorizations, error) {
	resp, err := c.client.ListKeys(ctx, &pb.ListKeysRequest{
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}
	var apiKeys []*APIKeyWithAuthorizations
	for _, key := range resp.ApiKeys {
		apiKeys = append(apiKeys, apiKeyWithAuthorizationsFromProto(key))
	}
	return apiKeys, nil
}

// RenameKey renames an API key.
func (c *AppClient) RenameKey(ctx context.Context, id, name string) (string, string, error) {
	resp, err := c.client.RenameKey(ctx, &pb.RenameKeyRequest{
		Id:   id,
		Name: name,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Id, resp.Name, nil
}

// RotateKey rotates an API key.
func (c *AppClient) RotateKey(ctx context.Context, id string) (string, string, error) {
	resp, err := c.client.RotateKey(ctx, &pb.RotateKeyRequest{
		Id: id,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Id, resp.Key, nil
}

// CreateKeyFromExistingKeyAuthorizations creates a new API key with an existing key's authorizations.
func (c *AppClient) CreateKeyFromExistingKeyAuthorizations(ctx context.Context, id string) (string, string, error) {
	resp, err := c.client.CreateKeyFromExistingKeyAuthorizations(ctx, &pb.CreateKeyFromExistingKeyAuthorizationsRequest{
		Id: id,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Id, resp.Key, nil
}
