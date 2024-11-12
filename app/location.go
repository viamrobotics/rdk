package app

import (
	"fmt"

	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Location struct {
	Id string
	Name string
	ParentLocationId string
	Auth *LocationAuth
	Organizations []*LocationOrganization
	CreatedOn *timestamppb.Timestamp
	RobotCount int32
	Config *StorageConfig
}

func ProtoToLocation(location *pb.Location) (*Location, error) {
	var organizations []*LocationOrganization
	for _, organization := range(location.Organizations) {
		organizations = append(organizations, ProtoToLocationOrganization(organization))
	}
	auth, err := ProtoToLocationAuth(location.Auth)
	if err != nil {
		return nil, err
	}
	return &Location{
		Id: location.Id,
		Name: location.Name,
		ParentLocationId: location.ParentLocationId,
		Auth: auth,
		Organizations: organizations,
		CreatedOn: location.CreatedOn,
		RobotCount: location.RobotCount,
		Config: ProtoToStorageConfig(location.Config),
	}, nil
}

func LocationToProto(location *Location) (*pb.Location, error) {
	var organizations []*pb.LocationOrganization
	for _, organization := range(location.Organizations) {
		organizations = append(organizations, LocationOrganizationToProto(organization))
	}
	auth, err := LocationAuthToProto(location.Auth)
	if err != nil {
		return nil, err
	}
	return &pb.Location{
		Id: location.Id,
		Name: location.Name,
		ParentLocationId: location.ParentLocationId,
		Auth: auth,
		Organizations: organizations,
		CreatedOn: location.CreatedOn,
		RobotCount: location.RobotCount,
		Config: StorageConfigToProto(location.Config),
	}, nil
}

type LocationOrganization struct {
	OrganizationId string
	Primary bool
}

func ProtoToLocationOrganization(locationOrganization *pb.LocationOrganization) *LocationOrganization {
	return &LocationOrganization{
		OrganizationId: locationOrganization.OrganizationId,
		Primary: locationOrganization.Primary,
	}
}

func LocationOrganizationToProto(locationOrganization *LocationOrganization) *pb.LocationOrganization {
	return &pb.LocationOrganization{
		OrganizationId: locationOrganization.OrganizationId,
		Primary: locationOrganization.Primary,
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
	LocationId string
	Secrets []*SharedSecret
}

func ProtoToLocationAuth(locationAuth *pb.LocationAuth) (*LocationAuth, error) {
	var secrets []*SharedSecret
	for _, secret := range(locationAuth.Secrets) {
		s, err := ProtoToSharedSecret(secret)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	return &LocationAuth{
		LocationId: locationAuth.LocationId,
		Secrets: secrets,
	}, nil
}

func LocationAuthToProto(locationAuth *LocationAuth) (*pb.LocationAuth, error) {
	var secrets []*pb.SharedSecret
	for _, secret := range(locationAuth.Secrets) {
		s, err := SharedSecretToProto(secret)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	return &pb.LocationAuth{
		LocationId: locationAuth.LocationId,
		Secrets: secrets,
	}, nil
}

type SharedSecret struct {
	Id string
	CreatedOn *timestamppb.Timestamp
	State SharedSecret_State
}

func ProtoToSharedSecret(sharedSecret *pb.SharedSecret) (*SharedSecret, error) {
	state, err := ProtoToSharedSecretState(sharedSecret.State)
	if err != nil {
		return nil, err
	}
	return &SharedSecret{
		Id: sharedSecret.Id,
		CreatedOn: sharedSecret.CreatedOn,
		State: state,
	}, nil
}

func SharedSecretToProto(sharedSecret *SharedSecret) (*pb.SharedSecret, error) {
	state, err := SharedSecretStateToProto(sharedSecret.State)
	if err != nil {
		return nil, err
	}
	return &pb.SharedSecret{
		Id: sharedSecret.Id,
		CreatedOn: sharedSecret.CreatedOn,
		State: state,
	}, nil
}

type SharedSecret_State int32

const (
	SharedSecret_STATE_UNSPECIFIED SharedSecret_State = 0
	SharedSecret_STATE_ENABLED SharedSecret_State = 1
	SharedSecret_STATE_DISABLED SharedSecret_State = 2
)

func ProtoToSharedSecretState(state pb.SharedSecret_State) (SharedSecret_State, error) {
	switch state{
	case pb.SharedSecret_STATE_UNSPECIFIED:
		return SharedSecret_STATE_UNSPECIFIED, nil
	case pb.SharedSecret_STATE_ENABLED:
		return SharedSecret_STATE_ENABLED, nil
	case pb.SharedSecret_STATE_DISABLED:
		return SharedSecret_STATE_DISABLED, nil
	default:
		return 0, fmt.Errorf("uknown secret state: %v", state)
	}
}

func SharedSecretStateToProto(state SharedSecret_State) (pb.SharedSecret_State, error) {
	switch state{
	case SharedSecret_STATE_UNSPECIFIED:
		return pb.SharedSecret_STATE_UNSPECIFIED, nil
	case SharedSecret_STATE_ENABLED:
		return pb.SharedSecret_STATE_ENABLED, nil
	case SharedSecret_STATE_DISABLED:
		return pb.SharedSecret_STATE_DISABLED, nil
	default:
		return 0, fmt.Errorf("unknown secret state: %v", state)
	}
}
