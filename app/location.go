package app

import (
	"fmt"

	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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

func ProtoToLocation(location *pb.Location) (*Location, error) {
	var organizations []*LocationOrganization
	for _, organization := range location.Organizations {
		organizations = append(organizations, ProtoToLocationOrganization(organization))
	}
	auth, err := ProtoToLocationAuth(location.Auth)
	if err != nil {
		return nil, err
	}
	return &Location{
		ID:               location.Id,
		Name:             location.Name,
		ParentLocationID: location.ParentLocationId,
		Auth:             auth,
		Organizations:    organizations,
		CreatedOn:        location.CreatedOn,
		RobotCount:       location.RobotCount,
		Config:           ProtoToStorageConfig(location.Config),
	}, nil
}

func LocationToProto(location *Location) (*pb.Location, error) {
	var organizations []*pb.LocationOrganization
	for _, organization := range location.Organizations {
		organizations = append(organizations, LocationOrganizationToProto(organization))
	}
	auth, err := LocationAuthToProto(location.Auth)
	if err != nil {
		return nil, err
	}
	return &pb.Location{
		Id:               location.ID,
		Name:             location.Name,
		ParentLocationId: location.ParentLocationID,
		Auth:             auth,
		Organizations:    organizations,
		CreatedOn:        location.CreatedOn,
		RobotCount:       location.RobotCount,
		Config:           StorageConfigToProto(location.Config),
	}, nil
}

type LocationOrganization struct {
	OrganizationID string
	Primary        bool
}

func ProtoToLocationOrganization(locationOrganization *pb.LocationOrganization) *LocationOrganization {
	return &LocationOrganization{
		OrganizationID: locationOrganization.OrganizationId,
		Primary:        locationOrganization.Primary,
	}
}

func LocationOrganizationToProto(locationOrganization *LocationOrganization) *pb.LocationOrganization {
	return &pb.LocationOrganization{
		OrganizationId: locationOrganization.OrganizationID,
		Primary:        locationOrganization.Primary,
	}
}

type StorageConfig struct {
	Region string
}

func ProtoToStorageConfig(config *pb.StorageConfig) *StorageConfig {
	return &StorageConfig{Region: config.Region}
}

func StorageConfigToProto(config *StorageConfig) *pb.StorageConfig {
	return &pb.StorageConfig{Region: config.Region}
}

type LocationAuth struct {
	LocationID string
	Secrets    []*SharedSecret
}

func ProtoToLocationAuth(locationAuth *pb.LocationAuth) (*LocationAuth, error) {
	var secrets []*SharedSecret
	for _, secret := range locationAuth.Secrets {
		s, err := ProtoToSharedSecret(secret)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	return &LocationAuth{
		LocationID: locationAuth.LocationId,
		Secrets:    secrets,
	}, nil
}

func LocationAuthToProto(locationAuth *LocationAuth) (*pb.LocationAuth, error) {
	var secrets []*pb.SharedSecret
	for _, secret := range locationAuth.Secrets {
		s, err := SharedSecretToProto(secret)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	return &pb.LocationAuth{
		LocationId: locationAuth.LocationID,
		Secrets:    secrets,
	}, nil
}

type SharedSecret struct {
	ID        string
	CreatedOn *timestamppb.Timestamp
	State     SharedSecretState
}

func ProtoToSharedSecret(sharedSecret *pb.SharedSecret) (*SharedSecret, error) {
	state, err := ProtoToSharedSecretState(sharedSecret.State)
	if err != nil {
		return nil, err
	}
	return &SharedSecret{
		ID:        sharedSecret.Id,
		CreatedOn: sharedSecret.CreatedOn,
		State:     state,
	}, nil
}

func SharedSecretToProto(sharedSecret *SharedSecret) (*pb.SharedSecret, error) {
	state, err := SharedSecretStateToProto(sharedSecret.State)
	if err != nil {
		return nil, err
	}
	return &pb.SharedSecret{
		Id:        sharedSecret.ID,
		CreatedOn: sharedSecret.CreatedOn,
		State:     state,
	}, nil
}

type SharedSecretState int32

const (
	SharedSecretUnspecified SharedSecretState = 0
	SharedSecretEnabled     SharedSecretState = 1
	SharedSecretDisabled    SharedSecretState = 2
)

func ProtoToSharedSecretState(state pb.SharedSecret_State) (SharedSecretState, error) {
	switch state {
	case pb.SharedSecret_STATE_UNSPECIFIED:
		return SharedSecretUnspecified, nil
	case pb.SharedSecret_STATE_ENABLED:
		return SharedSecretEnabled, nil
	case pb.SharedSecret_STATE_DISABLED:
		return SharedSecretDisabled, nil
	default:
		return 0, fmt.Errorf("uknown secret state: %v", state)
	}
}

func SharedSecretStateToProto(state SharedSecretState) (pb.SharedSecret_State, error) {
	switch state {
	case SharedSecretUnspecified:
		return pb.SharedSecret_STATE_UNSPECIFIED, nil
	case SharedSecretEnabled:
		return pb.SharedSecret_STATE_ENABLED, nil
	case SharedSecretDisabled:
		return pb.SharedSecret_STATE_DISABLED, nil
	default:
		return 0, fmt.Errorf("unknown secret state: %v", state)
	}
}
