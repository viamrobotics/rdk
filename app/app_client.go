package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	packages "go.viam.com/api/app/packages/v1"
	pb "go.viam.com/api/app/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AppClient struct {
	client pb.AppServiceClient
	logger logging.Logger

	mu sync.Mutex
}

func NewClientFromConn(conn rpc.ClientConn, logger logging.Logger) AppClient {
	return AppClient{client: pb.NewAppServiceClient(conn), logger: logger}
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
	return ProtoToOrganization(resp.Organization), nil
}

// ListOrganizations lists all the organizations.
func (c *AppClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	resp, err := c.client.ListOrganizations(ctx, &pb.ListOrganizationsRequest{})
	if err != nil {
		return nil, err
	}

	var organizations []*Organization
	for _, org := range resp.Organizations {
		organizations = append(organizations, ProtoToOrganization(org))
	}
	return organizations, nil
}

// GetOrganizationsWithAccessToLocation gets all the organizations that have access to a location.
func (c *AppClient) GetOrganizationsWithAccessToLocation(ctx context.Context, locationId string) ([]*OrganizationIdentity, error) {
	resp, err := c.client.GetOrganizationsWithAccessToLocation(ctx, &pb.GetOrganizationsWithAccessToLocationRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}

	var organizations []*OrganizationIdentity
	for _, org := range(resp.OrganizationIdentities) {
		organizations = append(organizations, ProtoToOrganizationIdentity(org))
	}
	return organizations, nil
}

// ListOrganizationsByUser lists all the organizations that a user belongs to.
func (c *AppClient) ListOrganizationsByUser(ctx context.Context, userId string) ([]*OrgDetails, error) {
	resp, err := c.client.ListOrganizationsByUser(ctx, &pb.ListOrganizationsByUserRequest{
		UserId: userId,
	})
	if err != nil {
		return nil, err
	}

	var organizations []*OrgDetails
	for _, org := range(resp.Orgs) {
		organizations = append(organizations, ProtoToOrgDetails(org))
	}
	return organizations, nil
}

// GetOrganization gets an organization.
func (c *AppClient) GetOrganization(ctx context.Context, orgId string) (*Organization, error) {
	resp, err := c.client.GetOrganization(ctx, &pb.GetOrganizationRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return nil, err
	}
	return ProtoToOrganization(resp.Organization), nil
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
func (c *AppClient) UpdateOrganization(ctx context.Context, orgId string, name, namespace, region, cid *string) (*Organization, error) {
	resp, err := c.client.UpdateOrganization(ctx, &pb.UpdateOrganizationRequest{
		OrganizationId:  orgId,
		Name:            name,
		PublicNamespace: namespace,
		Region:          region,
		Cid:             cid,
	})
	if err != nil {
		return nil, err
	}
	return ProtoToOrganization(resp.Organization), nil
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
func (c *AppClient) ListOrganizationMembers(ctx context.Context, orgId string) ([]*OrganizationMember, []*OrganizationInvite, error) {
	resp, err := c.client.ListOrganizationMembers(ctx, &pb.ListOrganizationMembersRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return nil, nil, err
	}

	var members []*OrganizationMember
	for _, member := range(resp.Members) {
		members = append(members, ProtoToOrganizationMember(member))
	}
	var invites []*OrganizationInvite
	for _, invite := range(resp.Invites) {
		invites = append(invites, ProtoToOrganizationInvite(invite))
	}
	return members, invites, nil
}

// CreateOrganizaitonInvite creates an organization invite to an organization.
func (c *AppClient) CreateOrganizationInvite(ctx context.Context, orgId, email string, authorizations []*pb.Authorization, sendEmailInvite *bool) (*OrganizationInvite, error) {
	resp, err := c.client.CreateOrganizationInvite(ctx, &pb.CreateOrganizationInviteRequest{
		OrganizationId:  orgId,
		Email:           email,
		Authorizations:  authorizations,
		SendEmailInvite: sendEmailInvite,
	})
	if err != nil {
		return nil, err
	}
	return ProtoToOrganizationInvite(resp.Invite), nil
}

// UpdateOrganizationInviteAuthorizations updates the authorizations attached to an organization invite.
func (c *AppClient) UpdateOrganizationInviteAuthorizations(ctx context.Context, orgId, email string, addAuthorizations, removeAuthorizations []*pb.Authorization) (*OrganizationInvite, error) {
	resp, err := c.client.UpdateOrganizationInviteAuthorizations(ctx, &pb.UpdateOrganizationInviteAuthorizationsRequest{
		OrganizationId:       orgId,
		Email:                email,
		AddAuthorizations:    addAuthorizations,
		RemoveAuthorizations: removeAuthorizations,
	})
	if err != nil {
		return nil, err
	}
	return ProtoToOrganizationInvite(resp.Invite), nil
}

// DeleteOrganizationMember deletes an organization member from an organization.
func (c *AppClient) DeleteOrganizationMember(ctx context.Context, orgId, userId string) error {
	_, err := c.client.DeleteOrganizationMember(ctx, &pb.DeleteOrganizationMemberRequest{
		OrganizationId: orgId,
		UserId:         userId,
	})
	if err != nil {
		return err
	}
	return nil
}

// DeleteOrganizationInvite deletes an organization invite.
func (c *AppClient) DeleteOrganizationInvite(ctx context.Context, orgId, email string) error {
	_, err := c.client.DeleteOrganizationInvite(ctx, &pb.DeleteOrganizationInviteRequest{
		OrganizationId: orgId,
		Email:          email,
	})
	if err != nil {
		return err
	}
	return nil
}

// ResendOrganizationInvite resends an organization invite.
func (c *AppClient) ResendOrganizationInvite(ctx context.Context, orgId, email string) (*OrganizationInvite, error) {
	resp, err := c.client.ResendOrganizationInvite(ctx, &pb.ResendOrganizationInviteRequest{
		OrganizationId: orgId,
		Email:          email,
	})
	if err != nil {
		return nil, err
	}
	return ProtoToOrganizationInvite(resp.Invite), nil
}

// CreateLocation creates a location.
func (c *AppClient) CreateLocation(ctx context.Context, orgId, name string, parentLocationId *string) (*Location, error) {
	resp, err := c.client.CreateLocation(ctx, &pb.CreateLocationRequest{
		OrganizationId:   orgId,
		Name:             name,
		ParentLocationId: parentLocationId,
	})
	if err != nil {
		return nil, err
	}
	location, err := ProtoToLocation(resp.Location)
	if err != nil {
		return nil, err
	}
	return location, nil
}

// GetLocation gets a location.
func (c *AppClient) GetLocation(ctx context.Context, locationId string) (*Location, error) {
	resp, err := c.client.GetLocation(ctx, &pb.GetLocationRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}
	location, err := ProtoToLocation(resp.Location)
	if err != nil {
		return nil, err
	}
	return location, nil
}

// UpdateLocation updates a location.
func (c *AppClient) UpdateLocation(ctx context.Context, locationId string, name, parentLocationId, region *string) (*Location, error) {
	resp, err := c.client.UpdateLocation(ctx, &pb.UpdateLocationRequest{
		LocationId:       locationId,
		Name:             name,
		ParentLocationId: parentLocationId,
		Region:           region,
	})
	if err != nil {
		return nil, err
	}
	location, err := ProtoToLocation(resp.Location)
	if err != nil {
		return nil, err
	}
	return location, nil
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
func (c *AppClient) ListLocations(ctx context.Context, orgId string) ([]*Location, error) {
	resp, err := c.client.ListLocations(ctx, &pb.ListLocationsRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return nil, err
	}

	var locations []*Location
	for _, location := range(resp.Locations) {
		l, err := ProtoToLocation(location)
		if err != nil {
			return nil, err
		}
		locations = append(locations, l)
	}
	return locations, nil
}

// ShareLocation shares a location with an organization.
func (c *AppClient) ShareLocation(ctx context.Context, locationId, orgId string) error {
	_, err := c.client.ShareLocation(ctx, &pb.ShareLocationRequest{
		LocationId:     locationId,
		OrganizationId: orgId,
	})
	if err != nil {
		return err
	}
	return nil
}

// UnshareLocation stops sharing a location with an organization.
func (c *AppClient) UnshareLocation(ctx context.Context, locationId, orgId string) error {
	_, err := c.client.UnshareLocation(ctx, &pb.UnshareLocationRequest{
		LocationId:     locationId,
		OrganizationId: orgId,
	})
	if err != nil {
		return err
	}
	return nil
}

// LocationAuth gets a location's authorization secrets.
func (c *AppClient) LocationAuth(ctx context.Context, locationId string) (*LocationAuth, error) {
	resp, err := c.client.LocationAuth(ctx, &pb.LocationAuthRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}
	auth, err := ProtoToLocationAuth(resp.Auth)
	if err != nil {
		return nil, err
	}
	return auth, nil
}

// CreateLocationSecret creates a new generated secret in the location. Succeeds if there are no more than 2 active secrets after creation.
func (c *AppClient) CreateLocationSecret(ctx context.Context, locationId string) (*LocationAuth, error) {
	resp, err := c.client.CreateLocationSecret(ctx, &pb.CreateLocationSecretRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}
	auth, err := ProtoToLocationAuth(resp.Auth)
	if err != nil {
		return nil, err
	}
	return auth, nil
}

// Delete a secret from the location.
func (c *AppClient) DeleteLocationSecret(ctx context.Context, locationId, secretId string) error {
	_, err := c.client.DeleteLocationSecret(ctx, &pb.DeleteLocationSecretRequest{
		LocationId: locationId,
		SecretId:   secretId,
	})
	if err != nil {
		return err
	}
	return nil
}

// GetRobot gets a specific robot by ID.
func (c *AppClient) GetRobot(ctx context.Context, id string) (*Robot, error) {
	resp, err := c.client.GetRobot(ctx, &pb.GetRobotRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	return ProtoToRobot(resp.Robot), nil
}

// GetRoverRentalRobots gets rover rental robots within an organization.
func (c *AppClient) GetRoverRentalRobots(ctx context.Context, orgId string) ([]*RoverRentalRobot, error) {
	resp, err := c.client.GetRoverRentalRobots(ctx, &pb.GetRoverRentalRobotsRequest{
		OrgId: orgId,
	})
	if err != nil {
		return nil, err
	}
	var robots []*RoverRentalRobot
	for _, robot := range(resp.Robots) {
		robots = append(robots, ProtoToRoverRentalRobot(robot))
	}
	return robots, nil
}

// GetRobotParts gets a list of all the parts under a specific machine.
func (c *AppClient) GetRobotParts(ctx context.Context, robotId string) ([]*RobotPart, error) {
	resp, err := c.client.GetRobotParts(ctx, &pb.GetRobotPartsRequest{
		RobotId: robotId,
	})
	if err != nil {
		return nil, err
	}
	var parts []*RobotPart
	for _, part := range(resp.Parts) {
		p, err := ProtoToRobotPart(part)
		if err != nil {
			return nil, err
		}
		parts = append(parts, p)
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
	part, err := ProtoToRobotPart(resp.Part)
	if err != nil {
		return nil, "", err
	}
	return part, resp.ConfigJson, nil
}

// GetRobotPartLogs gets the logs associated with a robot part from a page, defaulting to the most recent page if pageToken is empty. Logs of all levels are returned when levels is empty.
func (c *AppClient) GetRobotPartLogs(ctx context.Context, id string, filter, pageToken *string, levels []string, start, end *timestamppb.Timestamp, limit *int64, source *string) ([]*LogEntry, string, error) {
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
	for _, log := range(resp.Logs){
		l, err := ProtoToLogEntry(log)
		if err != nil {
			return nil, "", err
		}
		logs = append(logs, l)
	}
	return logs, resp.NextPageToken, nil
}

// TailRobotPartLogs gets a stream of log entries for a specific robot part. Logs are ordered by newest first.
func (c *AppClient) TailRobotPartLogs(ctx context.Context, id string, errorsOnly bool, filter *string, ch chan []*LogEntry) error {
	stream := &logStream {client: c}

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
	for _, entry := range(resp.History){
		e, err := ProtoToRobotPartHistoryEntry(entry)
		if err != nil {
			return nil, err
		}
		history = append(history, e)
	}
	return history, nil
}

// UpdaetRobotPart updates a robot part.
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
	part, err := ProtoToRobotPart(resp.Part)
	if err != nil {
		return nil, err
	}
	return part, nil
}

// NewRobotPart creates a new robot part.
func (c *AppClient) NewRobotPart(ctx context.Context, robotId, partName string) (string, error) {
	resp, err := c.client.NewRobotPart(ctx, &pb.NewRobotPartRequest{
		RobotId:  robotId,
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
func (c *AppClient) GetRobotAPIKeys(ctx context.Context, robotId string) ([]*APIKeyWithAuthorizations, error) {
	resp, err := c.client.GetRobotAPIKeys(ctx, &pb.GetRobotAPIKeysRequest{
		RobotId: robotId,
	})
	if err != nil {
		return nil, err
	}
	var keys []*APIKeyWithAuthorizations
	for _, key := range(resp.ApiKeys) {
		keys = append(keys, ProtoToAPIKeyWithAuthorizations(key))
	}
	return keys, nil
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
func (c *AppClient) CreateRobotPartSecret(ctx context.Context, partId string) (*RobotPart, error) {
	resp, err := c.client.CreateRobotPartSecret(ctx, &pb.CreateRobotPartSecretRequest{
		PartId: partId,
	})
	if err != nil {
		return nil, err
	}
	part, err := ProtoToRobotPart(resp.Part)
	if err != nil {
		return nil, err
	}
	return part, nil
}

// DeleteRobotPartSecret deletes a secret from the robot part.
func (c *AppClient) DeleteRobotPartSecret(ctx context.Context, partId, secretId string) error {
	_, err := c.client.DeleteRobotPartSecret(ctx, &pb.DeleteRobotPartSecretRequest{
		PartId:   partId,
		SecretId: secretId,
	})
	if err != nil {
		return err
	}
	return nil
}

// ListRobots gets a list of robots under a location.
func (c *AppClient) ListRobots(ctx context.Context, locationId string) ([]*Robot, error) {
	resp, err := c.client.ListRobots(ctx, &pb.ListRobotsRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}
	var robots []*Robot
	for _, robot := range(resp.Robots) {
		robots = append(robots, ProtoToRobot(robot))
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
	return ProtoToRobot(resp.Robot), nil
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
		OrganizationId:     orgId,
		ShowPublic:         showPublic,
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
func (c *AppClient) CreateFragment(ctx context.Context, name string, config interface{}, orgId string, visibility *pb.FragmentVisibility) (*pb.Fragment, error) {
	cfg, err := protoutils.StructToStructPb(config)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.CreateFragment(ctx, &pb.CreateFragmentRequest{
		Name:           name,
		Config:         cfg,
		OrganizationId: orgId,
		Visibility:     visibility,
	})
	if err != nil {
		return nil, err
	}
	return resp.Fragment, nil
}

// UpdateFragment updates a fragment.
func (c *AppClient) UpdateFragment(ctx context.Context, id, name string, config interface{}, public *bool, visibility *pb.FragmentVisibility) (*pb.Fragment, error) {
	cfg, err := protoutils.StructToStructPb(config)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.UpdateFragment(ctx, &pb.UpdateFragmentRequest{
		Id:         id,
		Name:       name,
		Config:     cfg,
		Public:     public,
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
		MachineId:             machineId,
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
		Id:        id,
		PageToken: pageToken,
		PageLimit: pageLimit,
	})
	if err != nil {
		return nil, "", err
	}
	return resp.History, resp.NextPageToken, nil
}

func createAuthorization(orgId, identityId, identityType, role, resourceType, resourceId string) (*pb.Authorization, error) {
	if role != "owner" && role != "operator" {
		return nil, errors.New("role string must be 'owner' or 'operator'")
	}
	if resourceType != "organization" && resourceType != "location" && resourceType != "robot" {
		return nil, errors.New("resourceType must be 'organization', 'location', or 'robot'")
	}

	return &pb.Authorization{
		AuthorizationType: role,
		AuthorizationId:   fmt.Sprintf("%s_%s", resourceType, role),
		ResourceType:      resourceType,
		ResourceId:        resourceId,
		IdentityId:        identityId,
		OrganizationId:    orgId,
		IdentityType:      identityType,
	}, nil
}

// AddRole creates an identity authorization.
func (c *AppClient) AddRole(ctx context.Context, orgId, identityId, role, resourceType, resourceId string) error {
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
func (c *AppClient) RemoveRole(ctx context.Context, orgId, identityId, role, resourceType, resourceId string) error {
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
func (c *AppClient) ChangeRole(ctx context.Context, oldOrgId, oldIdentityId, oldRole, oldResourceType, oldResourceId, newOrgId, newIdentityId, newRole, newResourceType, newResourceId string) error {
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
		ResourceIds:    resourceIds,
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

// GetRegistryItem gets a registry item.
func (c *AppClient) GetRegistryItem(ctx context.Context, itemId string) (*pb.RegistryItem, error) {
	resp, err := c.client.GetRegistryItem(ctx, &pb.GetRegistryItemRequest{
		ItemId: itemId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Item, nil
}

// CreateRegistryItem creates a registry item.
func (c *AppClient) CreateRegistryItem(ctx context.Context, orgId, name string, packageType packages.PackageType) error {
	_, err := c.client.CreateRegistryItem(ctx, &pb.CreateRegistryItemRequest{
		OrganizationId: orgId,
		Name:           name,
		Type:           packageType,
	})
	if err != nil {
		return err
	}
	return nil
}

// UpdateRegistryItem updates a registry item.
func (c *AppClient) UpdateRegistryItem(ctx context.Context, itemId string, packageType packages.PackageType, description string, visibility pb.Visibility, url *string) error {
	_, err := c.client.UpdateRegistryItem(ctx, &pb.UpdateRegistryItemRequest{
		ItemId:      itemId,
		Type:        packageType,
		Description: description,
		Visibility:  visibility,
		Url:         url,
	})
	if err != nil {
		return err
	}
	return nil
}

// ListRegistryItems lists the registry items in an organization.
func (c *AppClient) ListRegistryItems(ctx context.Context, orgId *string, types []packages.PackageType, visibilities []pb.Visibility, platforms []string, statuses []pb.RegistryItemStatus, searchTerm, pageToken *string, publicNamespaces []string) ([]*pb.RegistryItem, error) {
	resp, err := c.client.ListRegistryItems(ctx, &pb.ListRegistryItemsRequest{
		OrganizationId:   orgId,
		Types:            types,
		Visibilities:     visibilities,
		Platforms:        platforms,
		Statuses:         statuses,
		SearchTerm:       searchTerm,
		PageToken:        pageToken,
		PublicNamespaces: publicNamespaces,
	})
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// DeleteRegistryItem deletes a registry item given an ID that is formatted as `prefix:name“ where `prefix“ is the owner's organization ID or namespace.
func (c *AppClient) DeleteRegistryItem(ctx context.Context, itemId string) error {
	_, err := c.client.DeleteRegistryItem(ctx, &pb.DeleteRegistryItemRequest{
		ItemId: itemId,
	})
	if err != nil {
		return err
	}
	return nil
}

// TransferRegistryItem transfers a registry item to a namespace.
func (c *AppClient) TransferRegistryItem(ctx context.Context, itemId, newPublicNamespace string) error {
	_, err := c.client.TransferRegistryItem(ctx, &pb.TransferRegistryItemRequest{
		ItemId:             itemId,
		NewPublicNamespace: newPublicNamespace,
	})
	if err != nil {
		return err
	}
	return nil
}

// CreateModule creates a module.
func (c *AppClient) CreateModule(ctx context.Context, orgId, name string) (string, string, error) {
	resp, err := c.client.CreateModule(ctx, &pb.CreateModuleRequest{
		OrganizationId: orgId,
		Name:           name,
	})
	if err != nil {
		return "", "", err
	}
	return resp.ModuleId, resp.Url, nil
}

// UpdateModule updates the documentation URL, description, models, entrypoint, and/or the visibility of a module. A path to a setup script can be added that is run before a newly downloaded module starts.
func (c *AppClient) UpdateModule(ctx context.Context, moduleId string, visibility pb.Visibility, url, description string, models []*pb.Model, entrypoint string, firstRun *string) (string, error) {
	resp, err := c.client.UpdateModule(ctx, &pb.UpdateModuleRequest{
		ModuleId:    moduleId,
		Visibility:  visibility,
		Url:         url,
		Description: description,
		Models:      models,
		Entrypoint:  entrypoint,
		FirstRun:    firstRun,
	})
	if err != nil {
		return "", err
	}
	return resp.Url, nil
}

// type isModuleFile interface {
// 	isUploadModuleFileRequest_ModuleFile()
// }

// type UploadModuleFileRequest_ModuleFileInfo struct {
// 	*pb.UploadModuleFileRequest_ModuleFileInfo
// }

// func (UploadModuleFileRequest_ModuleFileInfo) isUploadModuleFileRequest_ModuleFile() {}

// type UploadModuleFileRequest_File struct {
// 	*pb.UploadModuleFileRequest_File
// }

// func (UploadModuleFileRequest_File) isUploadModuleFileRequest_ModuleFile() {}

// type uploadStream struct {
// 	gostream.
// }

// func (c *AppClient) UploadModuleFile(ctx context.Context, moduleFile isModuleFile, ch ) (string, error) {
// 	c.mu.Lock()
// 	streamCtx, stream, 
	
	// stream := &uploadStream{client: c}

	// err = stream.startStream(ctx, moduleFile, ch)
	
	// var req *pb.UploadModuleFileRequest
	// switch moduleFileInfo := moduleFile.(type) {
	// case UploadModuleFileRequest_ModuleFileInfo:
	// 	req = &pb.UploadModuleFileRequest{
	// 		ModuleFile: &pb.UploadModuleFileRequest_ModuleFileInfo{
	// 			ModuleFileInfo: moduleFileInfo.ModuleFileInfo,
	// 		},
	// 	}
	// case UploadModuleFileRequest_File:
	// 	req = &pb.UploadModuleFileRequest{
	// 		ModuleFile: &pb.UploadModuleFileRequest_File{
	// 			File: moduleFileInfo.File,
	// 		},
	// 	}
	// }
	
	// resp, err := c.client.UploadModuleFile(ctx, req)
	// if err != nil {
	// 	return "", err
	// }
	// return resp.Url, nil
// }

// GetModule gets a module.
func (c *AppClient) GetModule(ctx context.Context, moduleId string) (*pb.Module, error) {
	resp, err := c.client.GetModule(ctx, &pb.GetModuleRequest{
		ModuleId: moduleId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Module, nil
}

// ListModules lists the modules in the organization.
func (c *AppClient) ListModules(ctx context.Context, orgId *string) ([]*pb.Module, error) {
	resp, err := c.client.ListModules(ctx, &pb.ListModulesRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Modules, nil
}

// APIKeyAuthorization is a struct with the necessary authorization data to create an API key.
type APIKeyAuthorization struct {
	// `role`` must be "owner" or "operator"
	role string
	// `resourceType` must be "organization", "location", or "robot"
	resourceType string
	resourceId   string
}

// CreateKey creates a new API key associated with a list of authorizations.
func (c *AppClient) CreateKey(ctx context.Context, orgId string, keyAuthorizations []APIKeyAuthorization, name string) (string, string, error) {
	var authorizations []*pb.Authorization
	for _, keyAuthorization := range keyAuthorizations {
		authorization, err := createAuthorization(orgId, "", "api-key", keyAuthorization.role, keyAuthorization.resourceType, keyAuthorization.resourceId)
		if err != nil {
			return "", "", nil
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
	if err != nil {
		return err
	}
	return nil
}

// ListKeys lists all the keys for the organization.
func (c *AppClient) ListKeys(ctx context.Context, orgId string) ([]*pb.APIKeyWithAuthorizations, error) {
	resp, err := c.client.ListKeys(ctx, &pb.ListKeysRequest{
		OrgId: orgId,
	})
	if err != nil {
		return nil, err
	}
	return resp.ApiKeys, nil
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
