package app

import (
	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
