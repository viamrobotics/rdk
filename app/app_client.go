package app

import (
	"context"
	"errors"
	"fmt"

	pb "go.viam.com/api/app/v1"
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
