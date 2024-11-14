package app

import (
	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Organization holds the information of an organization.
type Organization struct {
	ID              string
	Name            string
	CreatedOn       *timestamppb.Timestamp
	PublicNamespace string
	DefaultRegion   string
	Cid             *string
}

func organizationFromProto(organization *pb.Organization) *Organization {
	return &Organization{
		ID:              organization.Id,
		Name:            organization.Name,
		CreatedOn:       organization.CreatedOn,
		PublicNamespace: organization.PublicNamespace,
		DefaultRegion:   organization.DefaultRegion,
		Cid:             organization.Cid,
	}
}

// OrganizationIdentity is used to render an organization's information on the frontend.
type OrganizationIdentity struct {
	ID   string
	Name string
}

func organizationIdentityFromProto(organizationIdentity *pb.OrganizationIdentity) *OrganizationIdentity {
	return &OrganizationIdentity{
		ID:   organizationIdentity.Id,
		Name: organizationIdentity.Name,
	}
}

// OrgDetails holds the ID and name of the organization.
type OrgDetails struct {
	OrgID   string
	OrgName string
}

func orgDetailsFromProto(orgDetails *pb.OrgDetails) *OrgDetails {
	return &OrgDetails{
		OrgID:   orgDetails.OrgId,
		OrgName: orgDetails.OrgName,
	}
}

// OrganizationMember holds the information of a member of an organization.
type OrganizationMember struct {
	UserID    string
	Emails    []string
	DateAdded *timestamppb.Timestamp
	LastLogin *timestamppb.Timestamp
}

func organizationMemberFromProto(organizationMemOrganizationMember *pb.OrganizationMember) *OrganizationMember {
	return &OrganizationMember{
		UserID:    organizationMemOrganizationMember.UserId,
		Emails:    organizationMemOrganizationMember.Emails,
		DateAdded: organizationMemOrganizationMember.DateAdded,
		LastLogin: organizationMemOrganizationMember.LastLogin,
	}
}

// OrganizationInvite is the invite to an organization.
type OrganizationInvite struct {
	OrganizationID string
	Email          string
	CreatedOn      *timestamppb.Timestamp
	Authorizations []*Authorization
}

func organizationInviteFromProto(organizationInvite *pb.OrganizationInvite) *OrganizationInvite {
	var authorizations []*Authorization
	for _, authorization := range organizationInvite.Authorizations {
		authorizations = append(authorizations, authorizationFromProto(authorization))
	}
	return &OrganizationInvite{
		OrganizationID: organizationInvite.OrganizationId,
		Email:          organizationInvite.Email,
		CreatedOn:      organizationInvite.CreatedOn,
		Authorizations: authorizations,
	}
}

// BillingAddress contains billing address details.
type BillingAddress struct {
	AddressLine_1 string 
	AddressLine_2 *string
	City          string 
	State         string 
}

func billingAddressToProto(addr *BillingAddress) *pb.BillingAddress {
	return &pb.BillingAddress{
		AddressLine_1: addr.AddressLine_1,
		AddressLine_2: addr.AddressLine_2,
		City: addr.City,
		State: addr.State,
	}
}
