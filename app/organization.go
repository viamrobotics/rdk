package app

import (
	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Organization struct {
	Id string
	Name string
	CreatedOn *timestamppb.Timestamp
	PublicNamespace string
	DefaultRegion string
	Cid *string
}

func ProtoToOrganization(organization *pb.Organization) *Organization {
	return &Organization{
		Id: organization.Id,
		Name: organization.Name,
		CreatedOn: organization.CreatedOn,
		PublicNamespace: organization.PublicNamespace,
		DefaultRegion: organization.DefaultRegion,
		Cid: organization.Cid,
	}
}

func OrganizationToProto(organization *Organization) *pb.Organization {
	return &pb.Organization{
		Id: organization.Id,
		Name: organization.Name,
		CreatedOn: organization.CreatedOn,
		PublicNamespace: organization.PublicNamespace,
		DefaultRegion: organization.DefaultRegion,
		Cid: organization.Cid,
	}
}

type OrganizationIdentity struct {
	Id string
	Name string
}

func ProtoToOrganizationIdentity(organizationIdentity *pb.OrganizationIdentity) *OrganizationIdentity {
	return &OrganizationIdentity{
		Id: organizationIdentity.Id,
		Name: organizationIdentity.Name,
	}
}

func OrganizationIdentityToProto(organizationIdentity *OrganizationIdentity) (*pb.OrganizationIdentity, error) {
	return &pb.OrganizationIdentity{
		Id: organizationIdentity.Id,
		Name: organizationIdentity.Name,
	}, nil
}

type OrgDetails struct {
	OrgId string
	OrgName string
}

func ProtoToOrgDetails(orgDetails *pb.OrgDetails) *OrgDetails {
	return &OrgDetails{
		OrgId: orgDetails.OrgId,
		OrgName: orgDetails.OrgName,
	}
}

func OrgDetailsToProto(orgDetails *OrgDetails) (*pb.OrgDetails, error) {
	return &pb.OrgDetails{
		OrgId: orgDetails.OrgId,
		OrgName: orgDetails.OrgName,
	}, nil
}

type OrganizationMember struct {
	UserId string
	Emails []string
	DateAdded *timestamppb.Timestamp
	LastLogin *timestamppb.Timestamp
}

func ProtoToOrganizationMember(organizationMemOrganizationMember *pb.OrganizationMember) *OrganizationMember {
	return &OrganizationMember{
		UserId: organizationMemOrganizationMember.UserId,
		Emails: organizationMemOrganizationMember.Emails,
		DateAdded: organizationMemOrganizationMember.DateAdded,
		LastLogin: organizationMemOrganizationMember.LastLogin,
	}
}

func OrganizationMemberToProto(organizationMemOrganizationMember *OrganizationMember) (*pb.OrganizationMember, error) {
	return &pb.OrganizationMember{
		UserId: organizationMemOrganizationMember.UserId,
		Emails: organizationMemOrganizationMember.Emails,
		DateAdded: organizationMemOrganizationMember.DateAdded,
		LastLogin: organizationMemOrganizationMember.LastLogin,
	}, nil
}

type OrganizationInvite struct {
	OrganizationId string
	Email string
	CreatedOn *timestamppb.Timestamp
	Authorizations []*Authorization
}

func ProtoToOrganizationInvite(organizationInvite *pb.OrganizationInvite) *OrganizationInvite {
	var authorizations []*Authorization
	for _, authorization := range(organizationInvite.Authorizations) {
		authorizations = append(authorizations, ProtoToAuthorization(authorization))
	}
	return &OrganizationInvite{
		OrganizationId: organizationInvite.OrganizationId,
		Email: organizationInvite.Email,
		CreatedOn: organizationInvite.CreatedOn,
		Authorizations: authorizations,
	}
}

func OrganizationInviteToProto(organizationInvite *OrganizationInvite) (*pb.OrganizationInvite, error) {
	var authorizations []*pb.Authorization
	for _, authorization := range(organizationInvite.Authorizations) {
		authorizations = append(authorizations, AuthorizationToProto(authorization))
	}
	return &pb.OrganizationInvite{
		OrganizationId: organizationInvite.OrganizationId,
		Email: organizationInvite.Email,
		CreatedOn: organizationInvite.CreatedOn,
		Authorizations: authorizations,
	}, nil
}

type Authorization struct {
	AuthorizationType string
	AuthorizationId string
	ResourceType string
	ResourceId string
	IdentityId string
	OrganizationId string
	IdentityType string
}

func ProtoToAuthorization(authorization *pb.Authorization) *Authorization {
	return &Authorization{
		AuthorizationType: authorization.AuthorizationType,
		AuthorizationId: authorization.AuthorizationId,
		ResourceType: authorization.ResourceType,
		ResourceId: authorization.ResourceId,
		IdentityId: authorization.IdentityId,
		OrganizationId: authorization.OrganizationId,
		IdentityType: authorization.IdentityType,
	}
}

func AuthorizationToProto(authorization *Authorization) *pb.Authorization {
	return &pb.Authorization{
		AuthorizationType: authorization.AuthorizationType,
		AuthorizationId: authorization.AuthorizationId,
		ResourceType: authorization.ResourceType,
		ResourceId: authorization.ResourceId,
		IdentityId: authorization.IdentityId,
		OrganizationId: authorization.OrganizationId,
		IdentityType: authorization.IdentityType,
	}
}

type AuthorizedPermissions struct {
	ResourceType string
	ResourceId string
	Permissions []string
}

func ProtoToAuthorizedPermissions(permissions *pb.AuthorizedPermissions) *AuthorizedPermissions {
	return &AuthorizedPermissions{
		ResourceType: permissions.ResourceType,
		ResourceId: permissions.ResourceId,
		Permissions: permissions.Permissions,
	}
}

func AuthorizedPermissionsToProto(permissions *AuthorizedPermissions) *pb.AuthorizedPermissions {
	return &pb.AuthorizedPermissions{
		ResourceType: permissions.ResourceType,
		ResourceId: permissions.ResourceId,
		Permissions: permissions.Permissions,
	}
}
