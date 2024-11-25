package app

import (
	"fmt"
	"time"

	pb "go.viam.com/api/app/v1"
	common "go.viam.com/api/common/v1"
)

// Organization holds the information of an organization.
type Organization struct {
	ID              string
	Name            string
	CreatedOn       *time.Time
	PublicNamespace string
	DefaultRegion   string
	Cid             *string
}

func organizationFromProto(organization *pb.Organization) *Organization {
	createdOn := organization.CreatedOn.AsTime()
	return &Organization{
		ID:              organization.Id,
		Name:            organization.Name,
		CreatedOn:       &createdOn,
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
	DateAdded *time.Time
	LastLogin *time.Time
}

func organizationMemberFromProto(organizationMemOrganizationMember *pb.OrganizationMember) *OrganizationMember {
	dateAdded := organizationMemOrganizationMember.DateAdded.AsTime()
	lastLogin := organizationMemOrganizationMember.LastLogin.AsTime()
	return &OrganizationMember{
		UserID:    organizationMemOrganizationMember.UserId,
		Emails:    organizationMemOrganizationMember.Emails,
		DateAdded: &dateAdded,
		LastLogin: &lastLogin,
	}
}

// OrganizationInvite is the invite to an organization.
type OrganizationInvite struct {
	OrganizationID string
	Email          string
	CreatedOn      *time.Time
	Authorizations []*Authorization
}

func organizationInviteFromProto(organizationInvite *pb.OrganizationInvite) *OrganizationInvite {
	var authorizations []*Authorization
	for _, authorization := range organizationInvite.Authorizations {
		authorizations = append(authorizations, authorizationFromProto(authorization))
	}
	createdOn := organizationInvite.CreatedOn.AsTime()
	return &OrganizationInvite{
		OrganizationID: organizationInvite.OrganizationId,
		Email:          organizationInvite.Email,
		CreatedOn:      &createdOn,
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
	CreatedOn        *time.Time
	RobotCount       int
	Config           *StorageConfig
}

func locationFromProto(location *pb.Location) *Location {
	var organizations []*LocationOrganization
	for _, organization := range location.Organizations {
		organizations = append(organizations, locationOrganizationFromProto(organization))
	}
	createdOn := location.CreatedOn.AsTime()
	return &Location{
		ID:               location.Id,
		Name:             location.Name,
		ParentLocationID: location.ParentLocationId,
		Auth:             locationAuthFromProto(location.Auth),
		Organizations:    organizations,
		CreatedOn:        &createdOn,
		RobotCount:       int(location.RobotCount),
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

// Robot holds the information of a machine.
type Robot struct {
	ID         string
	Name       string
	Location   string
	LastAccess *time.Time
	CreatedOn  *time.Time
}

func robotFromProto(robot *pb.Robot) *Robot {
	lastAccess := robot.LastAccess.AsTime()
	createdOn := robot.CreatedOn.AsTime()
	return &Robot{
		ID:         robot.Id,
		Name:       robot.Name,
		Location:   robot.Location,
		LastAccess: &lastAccess,
		CreatedOn:  &createdOn,
	}
}

// RoverRentalRobot holds the information of a rover rental robot.
type RoverRentalRobot struct {
	RobotID         string
	LocationID      string
	RobotName       string
	RobotMainPartID string
}

func roverRentalRobotFromProto(rrRobot *pb.RoverRentalRobot) *RoverRentalRobot {
	return &RoverRentalRobot{
		RobotID:         rrRobot.RobotId,
		LocationID:      rrRobot.LocationId,
		RobotName:       rrRobot.RobotName,
		RobotMainPartID: rrRobot.RobotMainPartId,
	}
}

// RobotPart is a specific machine part.
type RobotPart struct {
	ID               string
	Name             string
	DNSName          string
	Secret           string
	Robot            string
	LocationID       string
	RobotConfig      *map[string]interface{}
	LastAccess       *time.Time
	UserSuppliedInfo *map[string]interface{}
	MainPart         bool
	FQDN             string
	LocalFQDN        string
	CreatedOn        *time.Time
	Secrets          []*SharedSecret
	LastUpdated      *time.Time
}

func robotPartFromProto(robotPart *pb.RobotPart) *RobotPart {
	var secrets []*SharedSecret
	for _, secret := range robotPart.Secrets {
		secrets = append(secrets, sharedSecretFromProto(secret))
	}
	cfg := robotPart.RobotConfig.AsMap()
	lastAccess := robotPart.LastAccess.AsTime()
	info := robotPart.UserSuppliedInfo.AsMap()
	createdOn := robotPart.CreatedOn.AsTime()
	lastUpdated := robotPart.LastUpdated.AsTime()
	return &RobotPart{
		ID:               robotPart.Id,
		Name:             robotPart.Name,
		DNSName:          robotPart.DnsName,
		Secret:           robotPart.Secret,
		Robot:            robotPart.Robot,
		LocationID:       robotPart.LocationId,
		RobotConfig:      &cfg,
		LastAccess:       &lastAccess,
		UserSuppliedInfo: &info,
		MainPart:         robotPart.MainPart,
		FQDN:             robotPart.Fqdn,
		LocalFQDN:        robotPart.LocalFqdn,
		CreatedOn:        &createdOn,
		Secrets:          secrets,
		LastUpdated:      &lastUpdated,
	}
}

// RobotPartHistoryEntry is a history entry of a robot part.
type RobotPartHistoryEntry struct {
	Part     string
	Robot    string
	When     *time.Time
	Old      *RobotPart
	EditedBy *AuthenticatorInfo
}

func robotPartHistoryEntryFromProto(entry *pb.RobotPartHistoryEntry) *RobotPartHistoryEntry {
	when := entry.When.AsTime()
	return &RobotPartHistoryEntry{
		Part:     entry.Part,
		Robot:    entry.Robot,
		When:     &when,
		Old:      robotPartFromProto(entry.Old),
		EditedBy: authenticatorInfoFromProto(entry.EditedBy),
	}
}

// LogEntry holds the information of a single log entry.
type LogEntry struct {
	Host       string
	Level      string
	Time       *time.Time
	LoggerName string
	Message    string
	Caller     *map[string]interface{}
	Stack      string
	Fields     []*map[string]interface{}
}

func logEntryFromProto(logEntry *common.LogEntry) *LogEntry {
	entryTime := logEntry.Time.AsTime()
	caller := logEntry.Caller.AsMap()
	var fields []*map[string]interface{}
	for _, field := range logEntry.Fields {
		f := field.AsMap()
		fields = append(fields, &f)
	}
	return &LogEntry{
		Host:       logEntry.Host,
		Level:      logEntry.Level,
		Time:       &entryTime,
		LoggerName: logEntry.LoggerName,
		Message:    logEntry.Message,
		Caller:     &caller,
		Stack:      logEntry.Stack,
		Fields:     fields,
	}
}

// Fragment stores the information of a fragment.
type Fragment struct {
	ID                string
	Name              string
	Fragment          *map[string]interface{}
	OrganizationOwner string
	Public            bool
	CreatedOn         *time.Time
	OrganizationName  string
	RobotPartCount    int
	OrganizationCount int
	OnlyUsedByOwner   bool
	Visibility        FragmentVisibility
	LastUpdated       *time.Time
}

func fragmentFromProto(fragment *pb.Fragment) *Fragment {
	f := fragment.Fragment.AsMap()
	createdOn := fragment.CreatedOn.AsTime()
	lastUpdated := fragment.LastUpdated.AsTime()
	return &Fragment{
		ID:                fragment.Id,
		Name:              fragment.Name,
		Fragment:          &f,
		OrganizationOwner: fragment.OrganizationOwner,
		Public:            fragment.Public,
		CreatedOn:         &createdOn,
		OrganizationName:  fragment.OrganizationName,
		RobotPartCount:    int(fragment.RobotPartCount),
		OrganizationCount: int(fragment.OrganizationCount),
		OnlyUsedByOwner:   fragment.OnlyUsedByOwner,
		Visibility:        fragmentVisibilityFromProto(fragment.Visibility),
		LastUpdated:       &lastUpdated,
	}
}

// FragmentVisibility specifies the kind of visibility a fragment has.
type FragmentVisibility int

const (
	// FragmentVisibilityUnspecified is an unspecified visibility.
	FragmentVisibilityUnspecified FragmentVisibility = iota
	// FragmentVisibilityPrivate restricts access to a fragment to its organization.
	FragmentVisibilityPrivate
	// FragmentVisibilityPublic allows the fragment to be accessible to everyone.
	FragmentVisibilityPublic
	// FragmentVisibilityPublicUnlisted allows the fragment to be accessible to everyone but is hidden from public listings like it is private.
	FragmentVisibilityPublicUnlisted
)

func fragmentVisibilityFromProto(visibility pb.FragmentVisibility) FragmentVisibility {
	switch visibility {
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_UNSPECIFIED:
		return FragmentVisibilityUnspecified
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_PRIVATE:
		return FragmentVisibilityPrivate
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC:
		return FragmentVisibilityPublic
	case pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC_UNLISTED:
		return FragmentVisibilityPublicUnlisted
	}
	return FragmentVisibilityUnspecified
}

func fragmentVisibilityToProto(visibility FragmentVisibility) pb.FragmentVisibility {
	switch visibility {
	case FragmentVisibilityUnspecified:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_UNSPECIFIED
	case FragmentVisibilityPrivate:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_PRIVATE
	case FragmentVisibilityPublic:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC
	case FragmentVisibilityPublicUnlisted:
		return pb.FragmentVisibility_FRAGMENT_VISIBILITY_PUBLIC_UNLISTED
	}
	return pb.FragmentVisibility_FRAGMENT_VISIBILITY_UNSPECIFIED
}

// FragmentHistoryEntry is an entry of a fragment's history.
type FragmentHistoryEntry struct {
	Fragment string
	EditedOn *time.Time
	Old      *Fragment
	EditedBy *AuthenticatorInfo
}

func fragmentHistoryEntryFromProto(entry *pb.FragmentHistoryEntry) *FragmentHistoryEntry {
	editedOn := entry.EditedOn.AsTime()
	return &FragmentHistoryEntry{
		Fragment: entry.Fragment,
		EditedOn: &editedOn,
		Old:      fragmentFromProto(entry.Old),
		EditedBy: authenticatorInfoFromProto(entry.EditedBy),
	}
}

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

// AuthRole represents the valid authorizaiton types for an Authorization.
type AuthRole string

const (
	// AuthRoleOwner represents an owner authorization type.
	AuthRoleOwner AuthRole = "owner"
	// AuthRoleOperator represents an operator authorization type.
	AuthRoleOperator AuthRole = "operator"
)

// AuthResourceType represents the valid authorization resource type for an Authorization.
type AuthResourceType string

const (
	// AuthResourceTypeOrganization represents an organization authorization type.
	AuthResourceTypeOrganization = "organization"
	// AuthResourceTypeLocation represents a location authorization type.
	AuthResourceTypeLocation = "location"
	// AuthResourceTypeRobot represents a robot authorization type.
	AuthResourceTypeRobot = "robot"
)

// APIKeyAuthorization is a struct with the necessary authorization data to create an API key.
type APIKeyAuthorization struct {
	role         AuthRole
	resourceType AuthResourceType
	resourceID   string
}

func createAuthorization(
	orgID, identityID, identityType string, role AuthRole, resourceType AuthResourceType, resourceID string,
) *pb.Authorization {
	return &pb.Authorization{
		AuthorizationType: string(role),
		AuthorizationId:   fmt.Sprintf("%s_%s", resourceType, role),
		ResourceType:      string(resourceType),
		ResourceId:        resourceID,
		IdentityId:        identityID,
		OrganizationId:    orgID,
		IdentityType:      identityType,
	}
}

// SharedSecret is a secret used for LocationAuth and RobotParts.
type SharedSecret struct {
	ID        string
	CreatedOn *time.Time
	State     SharedSecretState
}

func sharedSecretFromProto(sharedSecret *pb.SharedSecret) *SharedSecret {
	createdOn := sharedSecret.CreatedOn.AsTime()
	return &SharedSecret{
		ID:        sharedSecret.Id,
		CreatedOn: &createdOn,
		State:     sharedSecretStateFromProto(sharedSecret.State),
	}
}

// SharedSecretState specifies if the secret is enabled, disabled, or unspecified.
type SharedSecretState int

const (
	// SharedSecretStateUnspecified represents an unspecified shared secret state.
	SharedSecretStateUnspecified SharedSecretState = iota
	// SharedSecretStateEnabled represents an enabled secret that can be used in authentication.
	SharedSecretStateEnabled
	// SharedSecretStateDisabled represents a disabled secret that must not be used to authenticate to rpc.
	SharedSecretStateDisabled
)

func sharedSecretStateFromProto(state pb.SharedSecret_State) SharedSecretState {
	switch state {
	case pb.SharedSecret_STATE_UNSPECIFIED:
		return SharedSecretStateUnspecified
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
type AuthenticationType int

const (
	// AuthenticationTypeUnspecified represents an unspecified authentication.
	AuthenticationTypeUnspecified AuthenticationType = iota
	// AuthenticationTypeWebOAuth represents authentication using Web OAuth.
	AuthenticationTypeWebOAuth
	// AuthenticationTypeAPIKey represents authentication using an API key.
	AuthenticationTypeAPIKey
	// AuthenticationTypeRobotPartSecret represents authentication using a robot part secret.
	AuthenticationTypeRobotPartSecret
	// AuthenticationTypeLocationSecret represents authentication using a location secret.
	AuthenticationTypeLocationSecret
)

func authenticationTypeFromProto(authenticationType pb.AuthenticationType) AuthenticationType {
	switch authenticationType {
	case pb.AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED:
		return AuthenticationTypeUnspecified
	case pb.AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH:
		return AuthenticationTypeWebOAuth
	case pb.AuthenticationType_AUTHENTICATION_TYPE_API_KEY:
		return AuthenticationTypeAPIKey
	case pb.AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET:
		return AuthenticationTypeRobotPartSecret
	case pb.AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET:
		return AuthenticationTypeLocationSecret
	}
	return AuthenticationTypeUnspecified
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
	CreatedOn *time.Time
}

func apiKeyFromProto(key *pb.APIKey) *APIKey {
	createdOn := key.CreatedOn.AsTime()
	return &APIKey{
		ID:        key.Id,
		Key:       key.Key,
		Name:      key.Name,
		CreatedOn: &createdOn,
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
