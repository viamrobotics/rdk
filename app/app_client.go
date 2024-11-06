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

func (c *AppClient) GetUserIDByEmail(ctx context.Context, email string) (string, error) {
	resp, err := c.client.GetUserIDByEmail(ctx, &pb.GetUserIDByEmailRequest{
		Email: email,
	})
	if err != nil {
		return "", err
	}
	return resp.UserId, nil
}

func (c *AppClient) CreateOrganization(ctx context.Context, name string) (*pb.Organization, error) {
	resp, err := c.client.CreateOrganization(ctx, &pb.CreateOrganizationRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	return resp.Organization, nil
}

func (c *AppClient) ListOrganizations(ctx context.Context) ([]*pb.Organization, error) {
	resp, err := c.client.ListOrganizations(ctx, &pb.ListOrganizationsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Organizations, nil
}

func (c *AppClient) GetOrganizationsWithAccessToLocation(ctx context.Context, locationId string) ([]*pb.OrganizationIdentity, error) {
	resp, err := c.client.GetOrganizationsWithAccessToLocation(ctx, &pb.GetOrganizationsWithAccessToLocationRequest{
		LocationId: locationId,
	})
	if err != nil {
		return nil, err
	}
	return resp.OrganizationIdentities, nil
}

func (c *AppClient) ListOrganizationsByUser(ctx context.Context, userId string) ([]*pb.OrgDetails, error) {
	resp, err := c.client.ListOrganizationsByUser(ctx, &pb.ListOrganizationsByUserRequest{
		UserId: userId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Orgs, nil
}

func (c *AppClient) GetOrganization(ctx context.Context, orgId string) (*pb.Organization, error) {
	resp, err := c.client.GetOrganization(ctx, &pb.GetOrganizationRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return nil, err
	}
	return resp.Organization, nil
}

func (c *AppClient) GetOrganizationNamespaceAvailability(ctx context.Context, namespace string) (bool, error) {
	resp, err := c.client.GetOrganizationNamespaceAvailability(ctx, &pb.GetOrganizationNamespaceAvailabilityRequest{
		PublicNamespace: namespace,
	})
	if err != nil {
		return false, err
	}
	return resp.Available, nil
}


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

func (c *AppClient) DeleteOrganization(ctx context.Context, orgId string) error {
	_, err := c.client.DeleteOrganization(ctx, &pb.DeleteOrganizationRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *AppClient) ListOrganizationMembers(ctx context.Context, orgId string) ([]*pb.OrganizationMember, []*pb.OrganizationInvite, error) {
	resp, err := c.client.ListOrganizationMembers(ctx, &pb.ListOrganizationMembersRequest{
		OrganizationId: orgId,
	})
	if err != nil {
		return nil, nil, err
	}
	return resp.Members, resp.Invites, nil
}

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

