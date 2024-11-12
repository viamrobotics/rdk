package app

import (
	"fmt"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Robot struct {
	Id string
	Name string
	Location string
	LastAccess *timestamppb.Timestamp
	CreatedOn *timestamppb.Timestamp
}

func ProtoToRobot(robot *pb.Robot) *Robot {
	return &Robot{
		Id: robot.Id,
		Name: robot.Name,
		Location: robot.Location,
		LastAccess: robot.LastAccess,
		CreatedOn: robot.CreatedOn,
	}
}

func RobotToProto(robot *Robot) *pb.Robot {
	return &pb.Robot{
		Id: robot.Id,
		Name: robot.Name,
		Location: robot.Location,
		LastAccess: robot.LastAccess,
		CreatedOn: robot.CreatedOn,
	}
}

type RoverRentalRobot struct {
	RobotId string
	LocationId string
	RobotName string
	RobotMainPartId string
}

func ProtoToRoverRentalRobot(rrRobot *pb.RoverRentalRobot) *RoverRentalRobot {
	return &RoverRentalRobot{
		RobotId: rrRobot.RobotId,
		LocationId: rrRobot.LocationId,
		RobotName: rrRobot.RobotName,
		RobotMainPartId: rrRobot.RobotMainPartId,
	}
}

func RoverRentalRobotToProto(rrRobot *RoverRentalRobot) *pb.RoverRentalRobot {
	return &pb.RoverRentalRobot{
		RobotId: rrRobot.RobotId,
		LocationId: rrRobot.LocationId,
		RobotName: rrRobot.RobotName,
		RobotMainPartId: rrRobot.RobotMainPartId,
	}
}

type RobotPart struct {
	Id string
	Name string
	DnsName string
	Secret string
	Robot string
	LocationId string
	RobotConfig map[string]interface{}
	LastAccess *timestamppb.Timestamp
	UserSuppliedInfo map[string]interface{}
	MainPart bool
	Fqdn string
	LocalFqdn string
	CreatedOn *timestamppb.Timestamp
	Secrets []*SharedSecret
	LastUpdated *timestamppb.Timestamp
}

func ProtoToRobotPart(robotPart *pb.RobotPart) (*RobotPart, error) {
	var secrets []*SharedSecret
	for _, secret := range(robotPart.Secrets) {
		s, err := ProtoToSharedSecret(secret)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	return &RobotPart{
		Id: robotPart.Id,
		Name: robotPart.Name,
		DnsName: robotPart.DnsName,
		Secret: robotPart.Secret,
		Robot: robotPart.DnsName,
		LocationId: robotPart.LocationId,
		RobotConfig: robotPart.RobotConfig.AsMap(),
		LastAccess: robotPart.LastAccess,
		UserSuppliedInfo: robotPart.UserSuppliedInfo.AsMap(),
		MainPart: robotPart.MainPart,
		Fqdn: robotPart.Fqdn,
		LocalFqdn: robotPart.LocalFqdn,
		CreatedOn: robotPart.CreatedOn,
		Secrets: secrets,
		LastUpdated: robotPart.LastUpdated,
	}, nil
}

func RobotPartToProto(robotPart *RobotPart) (*pb.RobotPart, error) {
	var secrets []*pb.SharedSecret
	for _, secret := range(robotPart.Secrets) {
		s, err := SharedSecretToProto(secret)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}
	robotConfig, err := protoutils.StructToStructPb(robotPart.RobotConfig)
	if err != nil {
		return nil, err
	}
	userSuppliedInfo, err := protoutils.StructToStructPb(robotPart.UserSuppliedInfo)
	if err != nil {
		return nil, err
	}
	return &pb.RobotPart{
		Id: robotPart.Id,
		Name: robotPart.Name,
		DnsName: robotPart.DnsName,
		Secret: robotPart.Secret,
		Robot: robotPart.DnsName,
		LocationId: robotPart.LocationId,
		RobotConfig: robotConfig,
		LastAccess: robotPart.LastAccess,
		UserSuppliedInfo: userSuppliedInfo,
		MainPart: robotPart.MainPart,
		Fqdn: robotPart.Fqdn,
		LocalFqdn: robotPart.LocalFqdn,
		CreatedOn: robotPart.CreatedOn,
		Secrets: secrets,
		LastUpdated: robotPart.LastUpdated,
	}, nil
}

type RobotPartHistoryEntry struct {
	Part string
	Robot string
	When *timestamppb.Timestamp
	Old *RobotPart
	EditedBy *AuthenticatorInfo
}

func ProtoToRobotPartHistoryEntry(entry *pb.RobotPartHistoryEntry) (*RobotPartHistoryEntry, error) {
	old, err := ProtoToRobotPart(entry.Old)
	if err != nil {
		return nil, err
	}
	info, err := ProtoToAuthenticatorInfo(entry.EditedBy)
	if err != nil {
		return nil, err
	}
	return &RobotPartHistoryEntry{
		Part: entry.Part,
		Robot: entry.Robot,
		When: entry.When,
		Old: old,
		EditedBy: info,
	}, nil
}

type AuthenticatorInfo struct {
	Type AuthenticationType
	Value string
	IsDeactivated bool
}

func ProtoToAuthenticatorInfo(info *pb.AuthenticatorInfo) (*AuthenticatorInfo, error){
	authenticationType, err := ProtoToAuthenticationType(info.Type)
	if err != nil {
		return nil, err
	}
	return &AuthenticatorInfo{
		Type: authenticationType,
		Value: info.Value,
		IsDeactivated: info.IsDeactivated,
	}, nil
}

func AuthenticatorInfoToProto(info *AuthenticatorInfo) (*pb.AuthenticatorInfo, error){
	authenticationType, err := AuthenticationTypeToProto(info.Type)
	if err != nil {
		return nil, err
	}
	return &pb.AuthenticatorInfo{
		Type: authenticationType,
		Value: info.Value,
		IsDeactivated: info.IsDeactivated,
	}, nil
}

type AuthenticationType int32

const (
	AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED       AuthenticationType = 0
	AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH         AuthenticationType = 1
	AuthenticationType_AUTHENTICATION_TYPE_API_KEY           AuthenticationType = 2
	AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET AuthenticationType = 3
	AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET   AuthenticationType = 4
)


func ProtoToAuthenticationType(authenticationType pb.AuthenticationType) (AuthenticationType, error) {
	switch authenticationType{
	case pb.AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED:
		return AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED, nil
	case pb.AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH:
		return AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH, nil
	case pb.AuthenticationType_AUTHENTICATION_TYPE_API_KEY:
		return AuthenticationType_AUTHENTICATION_TYPE_API_KEY, nil
	case pb.AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET:
		return AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET, nil
	case pb.AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET:
		return AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET, nil
	default:
		return 0, fmt.Errorf("uknown secret state: %v", authenticationType)
	}
}

func AuthenticationTypeToProto(authenticationType AuthenticationType) (pb.AuthenticationType, error) {
	switch authenticationType{
	case AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED, nil
	case AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH, nil
	case AuthenticationType_AUTHENTICATION_TYPE_API_KEY:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_API_KEY, nil
	case AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET, nil
	case AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET, nil
	default:
		return 0, fmt.Errorf("unknown secret state: %v", authenticationType)
	}
}

type APIKeyWithAuthorizations struct {
	ApiKey *APIKey
	Authorizations []*AuthorizationDetails
}

func ProtoToAPIKeyWithAuthorizations(key *pb.APIKeyWithAuthorizations) *APIKeyWithAuthorizations {
	var details []*AuthorizationDetails
	for _, detail := range(key.Authorizations){
		details = append(details, ProtoToAuthorizationDetails(detail))
	}
	return &APIKeyWithAuthorizations{
		ApiKey: ProtoToAPIKey(key.ApiKey),
		Authorizations: details,
	}
}

func APIKeyWithAuthorizationsToProto(key *APIKeyWithAuthorizations) *pb.APIKeyWithAuthorizations {
	var details []*pb.AuthorizationDetails
	for _, detail := range(key.Authorizations){
		details = append(details, AuthorizationDetailsToProto(detail))
	}
	return &pb.APIKeyWithAuthorizations{
		ApiKey: APIKeyToProto(key.ApiKey),
		Authorizations: details,
	}
}

type APIKey struct {
	Id string
	Key string
	Name string
	CreatedOn *timestamppb.Timestamp
}

func ProtoToAPIKey(key *pb.APIKey) *APIKey {
	return &APIKey{
		Id: key.Id,
		Key: key.Key,
		Name: key.Name,
		CreatedOn: key.CreatedOn,
	}
}

func APIKeyToProto(key *APIKey) *pb.APIKey {
	return &pb.APIKey{
		Id: key.Id,
		Key: key.Key,
		Name: key.Name,
		CreatedOn: key.CreatedOn,
	}
}

type AuthorizationDetails struct {
	AuthorizationType string
	AuthorizationId string
	ResourceType string
	ResourceId string
	OrgId string
}

func ProtoToAuthorizationDetails(details *pb.AuthorizationDetails) *AuthorizationDetails {
	return &AuthorizationDetails{
		AuthorizationType: details.AuthorizationType,
		AuthorizationId: details.AuthorizationId,
		ResourceType: details.ResourceType,
		ResourceId: details.ResourceId,
		OrgId: details.OrgId,
	}
}

func AuthorizationDetailsToProto(details *AuthorizationDetails) *pb.AuthorizationDetails {
	return &pb.AuthorizationDetails{
		AuthorizationType: details.AuthorizationType,
		AuthorizationId: details.AuthorizationId,
		ResourceType: details.ResourceType,
		ResourceId: details.ResourceId,
		OrgId: details.OrgId,
	}
}
