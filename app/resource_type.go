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
	AddressLine1 string
	AddressLine2 *string
	City         string
	State        string
}

func billingAddressToProto(addr *BillingAddress) *pb.BillingAddress {
	return &pb.BillingAddress{
		AddressLine_1: addr.AddressLine1,
		AddressLine_2: addr.AddressLine2,
		City:          addr.City,
		State:         addr.State,
	}
}


// Location holds the information of a specific location.
type Location struct {
	ID               string
	Name             string
	ParentLocationID string
	Auth             *LocationAuth
	Organizations    []*LocationOrganization
	CreatedOn        *timestamppb.Timestamp
	RobotCount       int32
	Config           *StorageConfig
}

func locationFromProto(location *pb.Location) *Location {
	var organizations []*LocationOrganization
	for _, organization := range location.Organizations {
		organizations = append(organizations, locationOrganizationFromProto(organization))
	}
	return &Location{
		ID:               location.Id,
		Name:             location.Name,
		ParentLocationID: location.ParentLocationId,
		Auth:             locationAuthFromProto(location.Auth),
		Organizations:    organizations,
		CreatedOn:        location.CreatedOn,
		RobotCount:       location.RobotCount,
		Config:           storageConfigFromProto(location.Config),
	}
}

// LocationOrganization holds information of an organization the location is shared with.
type LocationOrganization struct {
	OrganizationID string
	Primary        bool
}

func locationOrganizationFromProto(locationOrganization *pb.LocationOrganization) *LocationOrganization {
	return &LocationOrganization{
		OrganizationID: locationOrganization.OrganizationId,
		Primary:        locationOrganization.Primary,
	}
}

// StorageConfig holds the GCS region that data is stored in.
type StorageConfig struct {
	Region string
}

func storageConfigFromProto(config *pb.StorageConfig) *StorageConfig {
	return &StorageConfig{Region: config.Region}
}

// LocationAuth holds the secrets used to authenticate to a location.
type LocationAuth struct {
	LocationID string
	Secrets    []*SharedSecret
}

func locationAuthFromProto(locationAuth *pb.LocationAuth) *LocationAuth {
	var secrets []*SharedSecret
	for _, secret := range locationAuth.Secrets {
		secrets = append(secrets, sharedSecretFromProto(secret))
	}
	return &LocationAuth{
		LocationID: locationAuth.LocationId,
		Secrets:    secrets,
	}
}
