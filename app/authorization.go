package app

import (
	"errors"
	"fmt"

	pb "go.viam.com/api/app/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Authorization has the information about a specific authorization.
type Authorization struct {
	AuthorizationType string
	AuthorizationID   string
	ResourceType      string
	ResourceID        string
	IdentityID        string
	OrganizationID    string
	IdentityType      string
}

func authorizationFromProto(authorization *pb.Authorization) *Authorization {
	return &Authorization{
		AuthorizationType: authorization.AuthorizationType,
		AuthorizationID:   authorization.AuthorizationId,
		ResourceType:      authorization.ResourceType,
		ResourceID:        authorization.ResourceId,
		IdentityID:        authorization.IdentityId,
		OrganizationID:    authorization.OrganizationId,
		IdentityType:      authorization.IdentityType,
	}
}

func authorizationToProto(authorization *Authorization) *pb.Authorization {
	return &pb.Authorization{
		AuthorizationType: authorization.AuthorizationType,
		AuthorizationId:   authorization.AuthorizationID,
		ResourceType:      authorization.ResourceType,
		ResourceId:        authorization.ResourceID,
		IdentityId:        authorization.IdentityID,
		OrganizationId:    authorization.OrganizationID,
		IdentityType:      authorization.IdentityType,
	}
}

// AuthorizedPermissions is authorized permissions.
type AuthorizedPermissions struct {
	ResourceType string
	ResourceID   string
	Permissions  []string
}

func authorizedPermissionsFromProto(permissions *pb.AuthorizedPermissions) *AuthorizedPermissions {
	return &AuthorizedPermissions{
		ResourceType: permissions.ResourceType,
		ResourceID:   permissions.ResourceId,
		Permissions:  permissions.Permissions,
	}
}

func authorizedPermissionsToProto(permissions *AuthorizedPermissions) *pb.AuthorizedPermissions {
	return &pb.AuthorizedPermissions{
		ResourceType: permissions.ResourceType,
		ResourceId:   permissions.ResourceID,
		Permissions:  permissions.Permissions,
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

// SharedSecret is a secret used for LocationAuth and RobotParts.
type SharedSecret struct {
	ID        string
	CreatedOn *timestamppb.Timestamp
	State     SharedSecretState
}

func sharedSecretFromProto(sharedSecret *pb.SharedSecret) *SharedSecret {
	return &SharedSecret{
		ID:        sharedSecret.Id,
		CreatedOn: sharedSecret.CreatedOn,
		State:     sharedSecretStateFromProto(sharedSecret.State),
	}
}

// SharedSecretState specifies if the secret is enabled, disabled, or unspecified.
type SharedSecretState int32

const (
	// SharedSecretUnspecified represents an unspecified shared secret state.
	SharedSecretStateUnspecified SharedSecretState = 0
	// SharedSecretEnabled represents an enabled secret that can be used in authentication.
	SharedSecretStateEnabled SharedSecretState = 1
	// SharedSecretDisabled represents a disabled secret that must not be used to authenticate to rpc.
	SharedSecretStateDisabled SharedSecretState = 2
)

func sharedSecretStateFromProto(state pb.SharedSecret_State) SharedSecretState {
	switch state {
	case pb.SharedSecret_STATE_ENABLED:
		return SharedSecretStateEnabled
	case pb.SharedSecret_STATE_DISABLED:
		return SharedSecretStateDisabled
	default:
		return SharedSecretStateUnspecified
	}
}

// AuthenticatorInfo holds the information of an authenticator.
type AuthenticatorInfo struct {
	Type          AuthenticationType
	Value         string
	IsDeactivated bool
}

func authenticatorInfoFromProto(info *pb.AuthenticatorInfo) *AuthenticatorInfo {
	return &AuthenticatorInfo{
		Type:          authenticationTypeFromProto(info.Type),
		Value:         info.Value,
		IsDeactivated: info.IsDeactivated,
	}
}

// AuthenticationType specifies the type of authentication.
type AuthenticationType int32

const (
	// AuthenticationTypeUnspecified represents an unspecified authentication.
	AuthenticationTypeUnspecified AuthenticationType = 0
	// AuthenticationTypeWebOAuth represents authentication using Web OAuth.
	AuthenticationTypeWebOAuth AuthenticationType = 1
	// AuthenticationTypeAPIKey represents authentication using an API key.
	AuthenticationTypeAPIKey AuthenticationType = 2
	// AuthenticationTypeRobotPartSecret represents authentication using a robot part secret.
	AuthenticationTypeRobotPartSecret AuthenticationType = 3
	// AuthenticationTypeLocationSecret represents authentication using a location secret.
	AuthenticationTypeLocationSecret AuthenticationType = 4
)

func authenticationTypeFromProto(authenticationType pb.AuthenticationType) AuthenticationType {
	switch authenticationType {
	case pb.AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH:
		return AuthenticationTypeWebOAuth
	case pb.AuthenticationType_AUTHENTICATION_TYPE_API_KEY:
		return AuthenticationTypeAPIKey
	case pb.AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET:
		return AuthenticationTypeRobotPartSecret
	case pb.AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET:
		return AuthenticationTypeLocationSecret
	default:
		return AuthenticationTypeUnspecified
	}
}

// APIKeyWithAuthorizations is an API Key with its authorizations.
type APIKeyWithAuthorizations struct {
	APIKey         *APIKey
	Authorizations []*AuthorizationDetails
}

func apiKeyWithAuthorizationsFromProto(key *pb.APIKeyWithAuthorizations) *APIKeyWithAuthorizations {
	var details []*AuthorizationDetails
	for _, detail := range key.Authorizations {
		details = append(details, authorizationDetailsFromProto(detail))
	}
	return &APIKeyWithAuthorizations{
		APIKey:         apiKeyFromProto(key.ApiKey),
		Authorizations: details,
	}
}

// APIKey is a API key to make a request to an API.
type APIKey struct {
	ID        string
	Key       string
	Name      string
	CreatedOn *timestamppb.Timestamp
}

func apiKeyFromProto(key *pb.APIKey) *APIKey {
	return &APIKey{
		ID:        key.Id,
		Key:       key.Key,
		Name:      key.Name,
		CreatedOn: key.CreatedOn,
	}
}

// AuthorizationDetails has the details for an authorization.
type AuthorizationDetails struct {
	AuthorizationType string
	AuthorizationID   string
	ResourceType      string
	ResourceID        string
	OrgID             string
}

func authorizationDetailsFromProto(details *pb.AuthorizationDetails) *AuthorizationDetails {
	return &AuthorizationDetails{
		AuthorizationType: details.AuthorizationType,
		AuthorizationID:   details.AuthorizationId,
		ResourceType:      details.ResourceType,
		ResourceID:        details.ResourceId,
		OrgID:             details.OrgId,
	}
}
