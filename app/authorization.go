package app

import (
	"errors"
	"fmt"

	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createAuthorization(orgID, identityID, identityType, role, resourceType, resourceID string) (*pb.Authorization, error) {
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
		ResourceId:        resourceID,
		IdentityId:        identityID,
		OrganizationId:    orgID,
		IdentityType:      identityType,
	}, nil
}

type AuthenticatorInfo struct {
	Type          AuthenticationType
	Value         string
	IsDeactivated bool
}

func ProtoToAuthenticatorInfo(info *pb.AuthenticatorInfo) (*AuthenticatorInfo, error) {
	authenticationType, err := ProtoToAuthenticationType(info.Type)
	if err != nil {
		return nil, err
	}
	return &AuthenticatorInfo{
		Type:          authenticationType,
		Value:         info.Value,
		IsDeactivated: info.IsDeactivated,
	}, nil
}

func AuthenticatorInfoToProto(info *AuthenticatorInfo) (*pb.AuthenticatorInfo, error) {
	authenticationType, err := AuthenticationTypeToProto(info.Type)
	if err != nil {
		return nil, err
	}
	return &pb.AuthenticatorInfo{
		Type:          authenticationType,
		Value:         info.Value,
		IsDeactivated: info.IsDeactivated,
	}, nil
}

type AuthenticationType int32

const (
	AuthenticationTypeUnspecified     AuthenticationType = 0
	AuthenticationTypeWebOAuth        AuthenticationType = 1
	AuthenticationTypeAPIKey          AuthenticationType = 2
	AuthenticationTypeRobotPartSecret AuthenticationType = 3
	AuthenticationTypeLocationSecret  AuthenticationType = 4
)

func ProtoToAuthenticationType(authenticationType pb.AuthenticationType) (AuthenticationType, error) {
	switch authenticationType {
	case pb.AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED:
		return AuthenticationTypeUnspecified, nil
	case pb.AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH:
		return AuthenticationTypeWebOAuth, nil
	case pb.AuthenticationType_AUTHENTICATION_TYPE_API_KEY:
		return AuthenticationTypeAPIKey, nil
	case pb.AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET:
		return AuthenticationTypeRobotPartSecret, nil
	case pb.AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET:
		return AuthenticationTypeLocationSecret, nil
	default:
		return 0, fmt.Errorf("uknown authentication type: %v", authenticationType)
	}
}

func AuthenticationTypeToProto(authenticationType AuthenticationType) (pb.AuthenticationType, error) {
	switch authenticationType {
	case AuthenticationTypeUnspecified:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED, nil
	case AuthenticationTypeWebOAuth:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH, nil
	case AuthenticationTypeAPIKey:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_API_KEY, nil
	case AuthenticationTypeRobotPartSecret:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET, nil
	case AuthenticationTypeLocationSecret:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET, nil
	default:
		return 0, fmt.Errorf("unknown authentication type: %v", authenticationType)
	}
}

type APIKeyWithAuthorizations struct {
	APIKey         *APIKey
	Authorizations []*AuthorizationDetails
}

func ProtoToAPIKeyWithAuthorizations(key *pb.APIKeyWithAuthorizations) *APIKeyWithAuthorizations {
	var details []*AuthorizationDetails
	for _, detail := range key.Authorizations {
		details = append(details, ProtoToAuthorizationDetails(detail))
	}
	return &APIKeyWithAuthorizations{
		APIKey:         ProtoToAPIKey(key.ApiKey),
		Authorizations: details,
	}
}

func APIKeyWithAuthorizationsToProto(key *APIKeyWithAuthorizations) *pb.APIKeyWithAuthorizations {
	var details []*pb.AuthorizationDetails
	for _, detail := range key.Authorizations {
		details = append(details, AuthorizationDetailsToProto(detail))
	}
	return &pb.APIKeyWithAuthorizations{
		ApiKey:         APIKeyToProto(key.APIKey),
		Authorizations: details,
	}
}

type APIKey struct {
	ID        string
	Key       string
	Name      string
	CreatedOn *timestamppb.Timestamp
}

func ProtoToAPIKey(key *pb.APIKey) *APIKey {
	return &APIKey{
		ID:        key.Id,
		Key:       key.Key,
		Name:      key.Name,
		CreatedOn: key.CreatedOn,
	}
}

func APIKeyToProto(key *APIKey) *pb.APIKey {
	return &pb.APIKey{
		Id:        key.ID,
		Key:       key.Key,
		Name:      key.Name,
		CreatedOn: key.CreatedOn,
	}
}

type AuthorizationDetails struct {
	AuthorizationType string
	AuthorizationID   string
	ResourceType      string
	ResourceID        string
	OrgID             string
}

func ProtoToAuthorizationDetails(details *pb.AuthorizationDetails) *AuthorizationDetails {
	return &AuthorizationDetails{
		AuthorizationType: details.AuthorizationType,
		AuthorizationID:   details.AuthorizationId,
		ResourceType:      details.ResourceType,
		ResourceID:        details.ResourceId,
		OrgID:             details.OrgId,
	}
}

// AuthorizationDetailsToProto converts a AuthorizationDetails struct to protobuf.
func AuthorizationDetailsToProto(details *AuthorizationDetails) *pb.AuthorizationDetails {
	return &pb.AuthorizationDetails{
		AuthorizationType: details.AuthorizationType,
		AuthorizationId:   details.AuthorizationID,
		ResourceType:      details.ResourceType,
		ResourceId:        details.ResourceID,
		OrgId:             details.OrgID,
	}
}

// APIKeyAuthorization is a struct with the necessary authorization data to create an API key.
type APIKeyAuthorization struct {
	// `role`` must be "owner" or "operator"
	role string
	// `resourceType` must be "organization", "location", or "robot"
	resourceType string
	resourceID   string
}
