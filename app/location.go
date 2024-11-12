package app

import (
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
	Config *pb.StorageConfig
}

func ProtoToLocation(location *pb.Location) *Location {
	var organizations []*LocationOrganization
	for _, organization := range(location.Organizations) {
		organizations = append(organizations, ProtoToLocationOrganization(organization))
	}
	return &Location{
		Id: location.Id,
		Name: location.Name,
		ParentLocationId: location.ParentLocationId,
		Auth: ProtoToLocationAuth(location.Auth),
		Organizations: organizations,
		CreatedOn: location.CreatedOn,
		RobotCount: location.RobotCount,
		Config: location.Config,
	}
}

func LocationToProto(location *Location) *pb.Location {
	var organizations []*pb.LocationOrganization
	for _, organization := range(location.Organizations) {
		organizations = append(organizations, LocationOrganizationToProto(organization))
	}
	return &pb.Location{
		Id: location.Id,
		Name: location.Name,
		ParentLocationId: location.ParentLocationId,
		Auth: LocationAuthToProto(location.Auth),
		Organizations: organizations,
		CreatedOn: location.CreatedOn,
		RobotCount: location.RobotCount,
		Config: location.Config,
	}
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

type LocationAuth struct {
	LocationId string
	Secrets []*pb.SharedSecret
}

func ProtoToLocationAuth(locationAuth *pb.LocationAuth) *LocationAuth {
	return &LocationAuth{
		LocationId: locationAuth.LocationId,
		Secrets: locationAuth.Secrets,
	}
}

func LocationAuthToProto(locationAuth *LocationAuth) *pb.LocationAuth {
	return &pb.LocationAuth{
		LocationId: locationAuth.LocationId,
		Secrets: locationAuth.Secrets,
	}
}

type SharedSecret struct {
	Id string
	CreatedOn *timestamppb.Timestamp
	State SharedSecret_State
}

func ProtoToSharedSecret(sharedSecret *pb.SharedSecret) *SharedSecret {
	return &SharedSecret{
		Id: sharedSecret.Id,
		CreatedOn: sharedSecret.CreatedOn,
		State: ProtoToSharedSecretState(sharedSecret.State),
	}
}

func SharedSecretToProto(sharedSecret *SharedSecret) *pb.SharedSecret {
	return &pb.SharedSecret{
		Id: sharedSecret.Id,
		CreatedOn: sharedSecret.CreatedOn,
		State: SharedSecretStateToProto(sharedSecret.State),
	}
}

type SharedSecret_State int32

const (
	SharedSecret_STATE_UNSPECIFIED SharedSecret_State = 0
	SharedSecret_STATE_ENABLED SharedSecret_State = 1
	SharedSecret_STATE_DISABLED SharedSecret_State = 2
)

func ProtoToSharedSecretState(sharedSecretState pb.SharedSecret_State) SharedSecret_State {
	switch sharedSecretState{
	case pb.SharedSecret_STATE_UNSPECIFIED:
		return SharedSecret_STATE_UNSPECIFIED
	case pb.SharedSecret_STATE_ENABLED:
		return SharedSecret_STATE_ENABLED
	case pb.SharedSecret_STATE_DISABLED:
		return SharedSecret_STATE_DISABLED
	}
}

func SharedSecretStateToProto(sharedSecretState SharedSecret_State) pb.SharedSecret_State {
	switch sharedSecretState{
	case SharedSecret_STATE_UNSPECIFIED:
		return pb.SharedSecret_STATE_UNSPECIFIED
	case SharedSecret_STATE_ENABLED:
		return pb.SharedSecret_STATE_ENABLED
	case SharedSecret_STATE_DISABLED:
		return pb.SharedSecret_STATE_DISABLED
	}
}
