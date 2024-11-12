package app

import (
	"errors"
	"fmt"

	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createAuthorization(orgId, identityId, identityType, role, resourceType, resourceId string) (*pb.Authorization, error) {
	if role != "owner" && role != "operator" {
		return nil, errors.New("role string must be 'owner' or 'operator'")
	}
	if resourceType != "organization" && resourceType != "location" && resourceType != "robot" {
		return nil, errors.New("resourceType must be 'organization', 'location', or 'robot'")
	}

	return &pb.Authorization{
		AuthorizationType: role,
		AuthorizationId:   fmt.Sprintf("%s_%s", resourceType, role),
		ResourceType:      resourceType,
		ResourceId:        resourceId,
		IdentityId:        identityId,
		OrganizationId:    orgId,
		IdentityType:      identityType,
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
		return 0, fmt.Errorf("uknown authentication type: %v", authenticationType)
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
		return 0, fmt.Errorf("unknown authentication type: %v", authenticationType)
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

// APIKeyAuthorization is a struct with the necessary authorization data to create an API key.
type APIKeyAuthorization struct {
	// `role`` must be "owner" or "operator"
	role string
	// `resourceType` must be "organization", "location", or "robot"
	resourceType string
	resourceId   string
}
