package app

import (
	"fmt"
	"time"

	packages "go.viam.com/api/app/packages/v1"
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

// OrganizationIdentity is used to render an organization's information on the frontend.
type OrganizationIdentity struct {
	ID   string
	Name string
}

// OrgDetails holds the ID and name of the organization.
type OrgDetails struct {
	OrgID   string
	OrgName string
}

// OrganizationMember holds the information of a member of an organization.
type OrganizationMember struct {
	UserID    string
	Emails    []string
	DateAdded *time.Time
	LastLogin *time.Time
}

// OrganizationInvite is the invite to an organization.
type OrganizationInvite struct {
	OrganizationID string
	Email          string
	CreatedOn      *time.Time
	Authorizations []*Authorization
}

// BillingAddress contains billing address details.
type BillingAddress struct {
	AddressLine1 string
	AddressLine2 *string
	City         string
	State        string
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

// LocationOrganization holds information of an organization the location is shared with.
type LocationOrganization struct {
	OrganizationID string
	Primary        bool
}

// StorageConfig holds the GCS region that data is stored in.
type StorageConfig struct {
	Region string
}

// LocationAuth holds the secrets used to authenticate to a location.
type LocationAuth struct {
	LocationID string
	Secrets    []*SharedSecret
}

// Robot holds the information of a machine.
type Robot struct {
	ID         string
	Name       string
	Location   string
	LastAccess *time.Time
	CreatedOn  *time.Time
}

// RoverRentalRobot holds the information of a rover rental robot.
type RoverRentalRobot struct {
	RobotID         string
	LocationID      string
	RobotName       string
	RobotMainPartID string
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

// RobotPartHistoryEntry is a history entry of a robot part.
type RobotPartHistoryEntry struct {
	Part     string
	Robot    string
	When     *time.Time
	Old      *RobotPart
	EditedBy *AuthenticatorInfo
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

// FragmentHistoryEntry is an entry of a fragment's history.
type FragmentHistoryEntry struct {
	Fragment string
	EditedOn *time.Time
	Old      *Fragment
	EditedBy *AuthenticatorInfo
}

// Authorization has the information about a specific authorization.
type Authorization struct {
	AuthorizationType AuthRole
	AuthorizationID   string
	ResourceType      AuthResourceType
	ResourceID        string
	IdentityID        string
	OrganizationID    string
	IdentityType      string
}

// AuthorizedPermissions is authorized permissions.
type AuthorizedPermissions struct {
	ResourceType string
	ResourceID   string
	Permissions  []string
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

// SharedSecret is a secret used for LocationAuth and RobotParts.
type SharedSecret struct {
	ID        string
	CreatedOn *time.Time
	State     SharedSecretState
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

// AuthenticatorInfo holds the information of an authenticator.
type AuthenticatorInfo struct {
	Type          AuthenticationType
	Value         string
	IsDeactivated bool
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

// APIKeyWithAuthorizations is an API Key with its authorizations.
type APIKeyWithAuthorizations struct {
	APIKey         *APIKey
	Authorizations []*AuthorizationDetails
}

// APIKey is a API key to make a request to an API.
type APIKey struct {
	ID        string
	Key       string
	Name      string
	CreatedOn *time.Time
}

// AuthorizationDetails has the details for an authorization.
type AuthorizationDetails struct {
	AuthorizationType string
	AuthorizationID   string
	ResourceType      string
	ResourceID        string
	OrgID             string
}

// RegistryItem has the information of an item in the registry.
type RegistryItem struct {
	ItemID                         string
	OrganizationID                 string
	PublicNamespace                string
	Name                           string
	Type                           PackageType
	Visibility                     Visibility
	URL                            string
	Description                    string
	TotalRobotUsage                int
	TotalExternalRobotUsage        int
	TotalOrganizationUsage         int
	TotalExternalOrganizationUsage int
	Metadata                       isRegistryItemMetadata
	CreatedAt                      *time.Time
	UpdatedAt                      *time.Time
}

// RegistryItemStatus specifies if a registry item is published or in development.
type RegistryItemStatus int

const (
	// RegistryItemStatusUnspecified is an unspecified registry item status.
	RegistryItemStatusUnspecified RegistryItemStatus = iota
	// RegistryItemStatusPublished represents a published registry item.
	RegistryItemStatusPublished
	// RegistryItemStatusInDevelopment represents a registry item still in development.
	RegistryItemStatusInDevelopment
)

// PackageType is the type of package being used.
type PackageType int

const (
	// PackageTypeUnspecified represents an unspecified package type.
	PackageTypeUnspecified PackageType = iota
	// PackageTypeArchive represents an archive package type.
	PackageTypeArchive
	// PackageTypeMLModel represents a ML model package type.
	PackageTypeMLModel
	// PackageTypeModule represents a module package type.
	PackageTypeModule
	// PackageTypeSLAMMap represents a SLAM map package type.
	PackageTypeSLAMMap
	// PackageTypeMLTraining represents a ML training package type.
	PackageTypeMLTraining
)

// Visibility specifies the type of visibility of a registry item.
type Visibility int

const (
	// VisibilityUnspecified represents an unspecified visibility.
	VisibilityUnspecified Visibility = iota
	// VisibilityPrivate are for registry items visible only within the owning org.
	VisibilityPrivate
	// VisibilityPublic are for registry items that are visible to everyone.
	VisibilityPublic
	// VisibilityPublicUnlisted are for registry items usable in everyone's robot but are hidden from the registry page as if they are private.
	VisibilityPublicUnlisted
)

type isRegistryItemMetadata interface {
	isRegistryItemMetadata()
}

type registryItemModuleMetadata struct {
	ModuleMetadata *ModuleMetadata
}

type registryItemMLModelMetadata struct {
	MlModelMetadata *MLModelMetadata
}

type registryItemMLTrainingMetadata struct {
	MlTrainingMetadata *MLTrainingMetadata
}

// ModuleMetadata holds the metadata of a module.
type ModuleMetadata struct {
	Models     []*Model
	Versions   []*ModuleVersion
	Entrypoint string
	FirstRun   *string
}

// Model holds the colon-delimited-triplet of the model and the API implemented by the model.
type Model struct {
	API   string
	Model string
}

// ModuleVersion holds the information of a module version.
type ModuleVersion struct {
	Version    string
	Files      []*Uploads
	Models     []*Model
	Entrypoint string
	FirstRun   *string
}

// Uploads holds the time the file was uploaded and the OS and architecture a module is built to run on.
type Uploads struct {
	Platform   string
	UploadedAt *time.Time
}

// MLModelMetadata holds the metadata for a ML model.
type MLModelMetadata struct {
	Versions       []string
	ModelType      ModelType
	ModelFramework ModelFramework
}

// MLTrainingMetadata is the metadata of an ML Training.
type MLTrainingMetadata struct {
	Versions       []*MLTrainingVersion
	ModelType      ModelType
	ModelFramework ModelFramework
	Draft          bool
}

// MLTrainingVersion is the version of ML Training.
type MLTrainingVersion struct {
	Version   string
	CreatedOn *time.Time
}

// Module holds the information of a module.
type Module struct {
	ModuleID               string
	Name                   string
	Visibility             Visibility
	Versions               []*VersionHistory
	URL                    string
	Description            string
	Models                 []*Model
	TotalRobotUsage        int
	TotalOrganizationUsage int
	OrganizationID         string
	Entrypoint             string
	PublicNamespace        string
	FirstRun               *string
}

// ModuleFileInfo holds the information of a module file.
type ModuleFileInfo struct {
	ModuleID     string
	Version      string
	Platform     string
	PlatformTags []string
}

// VersionHistory holds the history of a version.
type VersionHistory struct {
	Version    string
	Files      []*Uploads
	Models     []*Model
	Entrypoint string
	FirstRun   *string
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

func organizationIdentityFromProto(organizationIdentity *pb.OrganizationIdentity) *OrganizationIdentity {
	return &OrganizationIdentity{
		ID:   organizationIdentity.Id,
		Name: organizationIdentity.Name,
	}
}

func orgDetailsFromProto(orgDetails *pb.OrgDetails) *OrgDetails {
	return &OrgDetails{
		OrgID:   orgDetails.OrgId,
		OrgName: orgDetails.OrgName,
	}
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

func billingAddressToProto(addr *BillingAddress) *pb.BillingAddress {
	return &pb.BillingAddress{
		AddressLine_1: addr.AddressLine1,
		AddressLine_2: addr.AddressLine2,
		City:          addr.City,
		State:         addr.State,
	}
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

func locationOrganizationFromProto(locationOrganization *pb.LocationOrganization) *LocationOrganization {
	return &LocationOrganization{
		OrganizationID: locationOrganization.OrganizationId,
		Primary:        locationOrganization.Primary,
	}
}

func storageConfigFromProto(config *pb.StorageConfig) *StorageConfig {
	return &StorageConfig{Region: config.Region}
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

func roverRentalRobotFromProto(rrRobot *pb.RoverRentalRobot) *RoverRentalRobot {
	return &RoverRentalRobot{
		RobotID:         rrRobot.RobotId,
		LocationID:      rrRobot.LocationId,
		RobotName:       rrRobot.RobotName,
		RobotMainPartID: rrRobot.RobotMainPartId,
	}
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

func fragmentHistoryEntryFromProto(entry *pb.FragmentHistoryEntry) *FragmentHistoryEntry {
	editedOn := entry.EditedOn.AsTime()
	return &FragmentHistoryEntry{
		Fragment: entry.Fragment,
		EditedOn: &editedOn,
		Old:      fragmentFromProto(entry.Old),
		EditedBy: authenticatorInfoFromProto(entry.EditedBy),
	}
}

func authorizationFromProto(authorization *pb.Authorization) *Authorization {
	return &Authorization{
		AuthorizationType: AuthRole(authorization.AuthorizationType),
		AuthorizationID:   authorization.AuthorizationId,
		ResourceType:      AuthResourceType(authorization.ResourceType),
		ResourceID:        authorization.ResourceId,
		IdentityID:        authorization.IdentityId,
		OrganizationID:    authorization.OrganizationId,
		IdentityType:      authorization.IdentityType,
	}
}

func authorizationToProto(authorization *Authorization) *pb.Authorization {
	return &pb.Authorization{
		AuthorizationType: string(authorization.AuthorizationType),
		AuthorizationId:   authorization.AuthorizationID,
		ResourceType:      string(authorization.ResourceType),
		ResourceId:        authorization.ResourceID,
		IdentityId:        authorization.IdentityID,
		OrganizationId:    authorization.OrganizationID,
		IdentityType:      authorization.IdentityType,
	}
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

func sharedSecretFromProto(sharedSecret *pb.SharedSecret) *SharedSecret {
	createdOn := sharedSecret.CreatedOn.AsTime()
	return &SharedSecret{
		ID:        sharedSecret.Id,
		CreatedOn: &createdOn,
		State:     sharedSecretStateFromProto(sharedSecret.State),
	}
}

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

func authenticatorInfoFromProto(info *pb.AuthenticatorInfo) *AuthenticatorInfo {
	return &AuthenticatorInfo{
		Type:          authenticationTypeFromProto(info.Type),
		Value:         info.Value,
		IsDeactivated: info.IsDeactivated,
	}
}

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

func apiKeyFromProto(key *pb.APIKey) *APIKey {
	createdOn := key.CreatedOn.AsTime()
	return &APIKey{
		ID:        key.Id,
		Key:       key.Key,
		Name:      key.Name,
		CreatedOn: &createdOn,
	}
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

func registryItemFromProto(item *pb.RegistryItem) (*RegistryItem, error) {
	var metadata isRegistryItemMetadata
	switch pbMetadata := item.Metadata.(type) {
	case *pb.RegistryItem_ModuleMetadata:
		metadata = &registryItemModuleMetadata{ModuleMetadata: moduleMetadataFromProto(pbMetadata.ModuleMetadata)}
	case *pb.RegistryItem_MlModelMetadata:
		metadata = &registryItemMLModelMetadata{MlModelMetadata: mlModelMetadataFromProto(pbMetadata.MlModelMetadata)}
	case *pb.RegistryItem_MlTrainingMetadata:
		metadata = &registryItemMLTrainingMetadata{MlTrainingMetadata: mlTrainingMetadataFromProto(pbMetadata.MlTrainingMetadata)}
	default:
		return nil, fmt.Errorf("unknown registry item metadata type: %T", item.Metadata)
	}
	createdAt := item.CreatedAt.AsTime()
	updatedAt := item.UpdatedAt.AsTime()
	return &RegistryItem{
		ItemID:                         item.ItemId,
		OrganizationID:                 item.OrganizationId,
		PublicNamespace:                item.PublicNamespace,
		Name:                           item.Name,
		Type:                           packageTypeFromProto(item.Type),
		Visibility:                     visibilityFromProto(item.Visibility),
		URL:                            item.Url,
		Description:                    item.Description,
		TotalRobotUsage:                int(item.TotalRobotUsage),
		TotalExternalRobotUsage:        int(item.TotalExternalRobotUsage),
		TotalOrganizationUsage:         int(item.TotalOrganizationUsage),
		TotalExternalOrganizationUsage: int(item.TotalExternalOrganizationUsage),
		Metadata:                       metadata,
		CreatedAt:                      &createdAt,
		UpdatedAt:                      &updatedAt,
	}, nil
}

func registryItemStatusToProto(status RegistryItemStatus) pb.RegistryItemStatus {
	switch status {
	case RegistryItemStatusUnspecified:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED
	case RegistryItemStatusPublished:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_PUBLISHED
	case RegistryItemStatusInDevelopment:
		return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_IN_DEVELOPMENT
	}
	return pb.RegistryItemStatus_REGISTRY_ITEM_STATUS_UNSPECIFIED
}

func packageTypeFromProto(packageType packages.PackageType) PackageType {
	switch packageType {
	case packages.PackageType_PACKAGE_TYPE_UNSPECIFIED:
		return PackageTypeUnspecified
	case packages.PackageType_PACKAGE_TYPE_ARCHIVE:
		return PackageTypeArchive
	case packages.PackageType_PACKAGE_TYPE_ML_MODEL:
		return PackageTypeMLModel
	case packages.PackageType_PACKAGE_TYPE_MODULE:
		return PackageTypeModule
	case packages.PackageType_PACKAGE_TYPE_SLAM_MAP:
		return PackageTypeSLAMMap
	case packages.PackageType_PACKAGE_TYPE_ML_TRAINING:
		return PackageTypeMLTraining
	}
	return PackageTypeUnspecified
}

func packageTypeToProto(packageType PackageType) packages.PackageType {
	switch packageType {
	case PackageTypeUnspecified:
		return packages.PackageType_PACKAGE_TYPE_UNSPECIFIED
	case PackageTypeArchive:
		return packages.PackageType_PACKAGE_TYPE_ARCHIVE
	case PackageTypeMLModel:
		return packages.PackageType_PACKAGE_TYPE_ML_MODEL
	case PackageTypeModule:
		return packages.PackageType_PACKAGE_TYPE_MODULE
	case PackageTypeSLAMMap:
		return packages.PackageType_PACKAGE_TYPE_SLAM_MAP
	case PackageTypeMLTraining:
		return packages.PackageType_PACKAGE_TYPE_ML_TRAINING
	}
	return packages.PackageType_PACKAGE_TYPE_UNSPECIFIED
}

func visibilityFromProto(visibility pb.Visibility) Visibility {
	switch visibility {
	case pb.Visibility_VISIBILITY_UNSPECIFIED:
		return VisibilityUnspecified
	case pb.Visibility_VISIBILITY_PRIVATE:
		return VisibilityPrivate
	case pb.Visibility_VISIBILITY_PUBLIC:
		return VisibilityPublic
	case pb.Visibility_VISIBILITY_PUBLIC_UNLISTED:
		return VisibilityPublicUnlisted
	}
	return VisibilityUnspecified
}

func visibilityToProto(visibility Visibility) pb.Visibility {
	switch visibility {
	case VisibilityUnspecified:
		return pb.Visibility_VISIBILITY_UNSPECIFIED
	case VisibilityPrivate:
		return pb.Visibility_VISIBILITY_PRIVATE
	case VisibilityPublic:
		return pb.Visibility_VISIBILITY_PUBLIC
	case VisibilityPublicUnlisted:
		return pb.Visibility_VISIBILITY_PUBLIC_UNLISTED
	}
	return pb.Visibility_VISIBILITY_UNSPECIFIED
}

func (*registryItemModuleMetadata) isRegistryItemMetadata() {}

func (*registryItemMLModelMetadata) isRegistryItemMetadata() {}

func (*registryItemMLTrainingMetadata) isRegistryItemMetadata() {}

func moduleMetadataFromProto(md *pb.ModuleMetadata) *ModuleMetadata {
	var models []*Model
	for _, version := range md.Models {
		models = append(models, modelFromProto(version))
	}
	var versions []*ModuleVersion
	for _, version := range md.Versions {
		versions = append(versions, moduleVersionFromProto(version))
	}
	return &ModuleMetadata{
		Models:     models,
		Versions:   versions,
		Entrypoint: md.Entrypoint,
		FirstRun:   md.FirstRun,
	}
}

func modelFromProto(model *pb.Model) *Model {
	return &Model{
		API:   model.Api,
		Model: model.Model,
	}
}

func modelToProto(model *Model) *pb.Model {
	return &pb.Model{
		Api:   model.API,
		Model: model.Model,
	}
}

func moduleVersionFromProto(version *pb.ModuleVersion) *ModuleVersion {
	var files []*Uploads
	for _, file := range version.Files {
		files = append(files, uploadsFromProto(file))
	}
	var models []*Model
	for _, model := range version.Models {
		models = append(models, modelFromProto(model))
	}
	return &ModuleVersion{
		Version:    version.Version,
		Files:      files,
		Models:     models,
		Entrypoint: version.Entrypoint,
		FirstRun:   version.FirstRun,
	}
}

func uploadsFromProto(uploads *pb.Uploads) *Uploads {
	uploadedAt := uploads.UploadedAt.AsTime()
	return &Uploads{
		Platform:   uploads.Platform,
		UploadedAt: &uploadedAt,
	}
}

func mlModelMetadataFromProto(md *pb.MLModelMetadata) *MLModelMetadata {
	return &MLModelMetadata{
		Versions:       md.Versions,
		ModelType:      modelTypeFromProto(md.ModelType),
		ModelFramework: modelFrameworkFromProto(md.ModelFramework),
	}
}

func mlTrainingMetadataFromProto(md *pb.MLTrainingMetadata) *MLTrainingMetadata {
	var versions []*MLTrainingVersion
	for _, version := range md.Versions {
		versions = append(versions, mlTrainingVersionFromProto(version))
	}
	return &MLTrainingMetadata{
		Versions:       versions,
		ModelType:      modelTypeFromProto(md.ModelType),
		ModelFramework: modelFrameworkFromProto(md.ModelFramework),
		Draft:          md.Draft,
	}
}

func mlTrainingVersionFromProto(version *pb.MLTrainingVersion) *MLTrainingVersion {
	createdOn := version.CreatedOn.AsTime()
	return &MLTrainingVersion{
		Version:   version.Version,
		CreatedOn: &createdOn,
	}
}

func moduleFromProto(module *pb.Module) *Module {
	var versions []*VersionHistory
	for _, version := range module.Versions {
		versions = append(versions, versionHistoryFromProto(version))
	}
	var models []*Model
	for _, model := range module.Models {
		models = append(models, modelFromProto(model))
	}
	return &Module{
		ModuleID:               module.ModuleId,
		Name:                   module.Name,
		Visibility:             visibilityFromProto(module.Visibility),
		Versions:               versions,
		URL:                    module.Url,
		Description:            module.Description,
		Models:                 models,
		TotalRobotUsage:        int(module.TotalRobotUsage),
		TotalOrganizationUsage: int(module.TotalOrganizationUsage),
		OrganizationID:         module.OrganizationId,
		Entrypoint:             module.Entrypoint,
		PublicNamespace:        module.PublicNamespace,
		FirstRun:               module.FirstRun,
	}
}

func moduleFileInfoToProto(info *ModuleFileInfo) *pb.ModuleFileInfo {
	return &pb.ModuleFileInfo{
		ModuleId:     info.ModuleID,
		Version:      info.Version,
		Platform:     info.Platform,
		PlatformTags: info.PlatformTags,
	}
}

func versionHistoryFromProto(history *pb.VersionHistory) *VersionHistory {
	var files []*Uploads
	for _, file := range history.Files {
		files = append(files, uploadsFromProto(file))
	}
	var models []*Model
	for _, model := range history.Models {
		models = append(models, modelFromProto(model))
	}
	return &VersionHistory{
		Version:    history.Version,
		Files:      files,
		Models:     models,
		Entrypoint: history.Entrypoint,
		FirstRun:   history.FirstRun,
	}
}

