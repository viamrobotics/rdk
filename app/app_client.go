// Package app contains the interfaces that manage a machine fleet with code instead of with the graphical interface of the Viam App.
//
// [fleet management docs]: https://docs.viam.com/appendix/apis/fleet/
package app

import (
	"context"
	"fmt"
	"time"

	packages "go.viam.com/api/app/packages/v1"
	pb "go.viam.com/api/app/v1"
	common "go.viam.com/api/common/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	Zipcode      string
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
	RobotConfig      map[string]interface{}
	LastAccess       *time.Time
	UserSuppliedInfo map[string]interface{}
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
	Caller     map[string]interface{}
	Stack      string
	Fields     []map[string]interface{}
}

// Fragment stores the information of a fragment.
type Fragment struct {
	ID                string
	Name              string
	Fragment          map[string]interface{}
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

// UpdateOrganizationOptions contains optional parameters for UpdateOrganization.
type UpdateOrganizationOptions struct {
	Name      *string
	Namespace *string
	// Region is the new GCS region to associate the org with.
	Region *string
	CID    *string
}

// CreateOrganizationInviteOptions contains optional parameters for CreateOrganizationInvite.
type CreateOrganizationInviteOptions struct {
	// SendEmailInvite defaults to true to send an email to the receipient of the invite.
	// The user must accept the email to be added to the associated authorizations.
	// If false, the user receives the associated authorization on the next login of the user with the associated email address.
	SendEmailInvite *bool
}

// CreateLocationOptions contains optional parameters for CreateLocation.
type CreateLocationOptions struct {
	// ParentLocationID is the parent location to move the location under.
	ParentLocationID *string
}

// UpdateLocationOptions contains optional parameters for UpdateLocation.
type UpdateLocationOptions struct {
	Name *string
	// PArentLocationID is the new parent location to move the location under.
	ParentLocationID *string
	// Region is the GCS region to associate the location with.
	Region *string
}

// GetRobotPartLogsOptions contains optional parameters for GetRobotPartLogs.
type GetRobotPartLogsOptions struct {
	Filter *string
	// PageToken represents the page to receive logs from. The function defaults to the most recent page if PageToken is empty.
	PageToken *string
	// Levels represents the levels of the logs requested. Logs of all levels are returned when levels is empty.
	Levels []string
	Start  *time.Time
	End    *time.Time
	Limit  *int
	Source *string
}

// TailRobotPartLogsOptions contains optional parameters for TailRobotPartLogs.
type TailRobotPartLogsOptions struct {
	Filter *string
}

// CreateFragmentOptions contains optional parameters for CreateFragment.
type CreateFragmentOptions struct {
	Visibility *FragmentVisibility
}

// UpdateFragmentOptions contains optional parameters for UpdateFragment.
type UpdateFragmentOptions struct {
	Public     *bool
	Visibility *FragmentVisibility
}

// GetFragmentHistoryOptions contains optional parameters for GetFragmentHistory.
type GetFragmentHistoryOptions struct {
	PageToken *string
	PageLimit *int
}

// UpdateRegistryItemOptions contains optional parameters for UpdateRegistryItem.
type UpdateRegistryItemOptions struct {
	URL *string
}

// ListRegistryItemsOptions contains optional parameters for ListRegistryItems.
type ListRegistryItemsOptions struct {
	SearchTerm *string
	PageToken  *string
	// PublicNamespaces are the namespaces to return results for.
	PublicNamespaces []string
}

// UpdateModuleOptions contains optional parameters for UpdateModule.
type UpdateModuleOptions struct {
	// The path to a setup script that is run before a newly downloaded module starts.
	FirstRun *string
}

// ListModulesOptions contains optional parameters for ListModules.
type ListModulesOptions struct {
	// OrgID is the organization to return private modules for.
	OrgID *string
}

// AppClient is a gRPC client for method calls to the App API.
//
//nolint:revive // stutter: Ignore the "stuttering" warning for this type name
type AppClient struct {
	client pb.AppServiceClient
}

func newAppClient(conn rpc.ClientConn) *AppClient {
	return &AppClient{client: pb.NewAppServiceClient(conn)}
}

// GetUserIDByEmail gets the ID of the user with the given email.
func (c *AppClient) GetUserIDByEmail(ctx context.Context, email string) (string, error) {
	resp, err := c.client.GetUserIDByEmail(ctx, &pb.GetUserIDByEmailRequest{
		Email: email,
	})
	if err != nil {
		return "", err
	}
	return resp.UserId, nil
}

// CreateOrganization creates a new organization.
func (c *AppClient) CreateOrganization(ctx context.Context, name string) (*Organization, error) {
	resp, err := c.client.CreateOrganization(ctx, &pb.CreateOrganizationRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	return organizationFromProto(resp.Organization), nil
}

// ListOrganizations lists all the organizations.
func (c *AppClient) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	resp, err := c.client.ListOrganizations(ctx, &pb.ListOrganizationsRequest{})
	if err != nil {
		return nil, err
	}

	var organizations []*Organization
	for _, org := range resp.Organizations {
		organizations = append(organizations, organizationFromProto(org))
	}
	return organizations, nil
}

// GetOrganizationsWithAccessToLocation gets all the organizations that have access to a location.
func (c *AppClient) GetOrganizationsWithAccessToLocation(ctx context.Context, locationID string) ([]*OrganizationIdentity, error) {
	resp, err := c.client.GetOrganizationsWithAccessToLocation(ctx, &pb.GetOrganizationsWithAccessToLocationRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}

	var organizations []*OrganizationIdentity
	for _, org := range resp.OrganizationIdentities {
		organizations = append(organizations, organizationIdentityFromProto(org))
	}
	return organizations, nil
}

// ListOrganizationsByUser lists all the organizations that a user belongs to.
func (c *AppClient) ListOrganizationsByUser(ctx context.Context, userID string) ([]*OrgDetails, error) {
	resp, err := c.client.ListOrganizationsByUser(ctx, &pb.ListOrganizationsByUserRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, err
	}

	var organizations []*OrgDetails
	for _, org := range resp.Orgs {
		organizations = append(organizations, orgDetailsFromProto(org))
	}
	return organizations, nil
}

// GetOrganization gets an organization.
func (c *AppClient) GetOrganization(ctx context.Context, orgID string) (*Organization, error) {
	resp, err := c.client.GetOrganization(ctx, &pb.GetOrganizationRequest{
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, err
	}
	return organizationFromProto(resp.Organization), nil
}

// GetOrganizationNamespaceAvailability checks for namespace availability throughout all organizations.
func (c *AppClient) GetOrganizationNamespaceAvailability(ctx context.Context, namespace string) (bool, error) {
	resp, err := c.client.GetOrganizationNamespaceAvailability(ctx, &pb.GetOrganizationNamespaceAvailabilityRequest{
		PublicNamespace: namespace,
	})
	if err != nil {
		return false, err
	}
	return resp.Available, nil
}

// UpdateOrganization updates an organization.
func (c *AppClient) UpdateOrganization(ctx context.Context, orgID string, opts *UpdateOrganizationOptions) (*Organization, error) {
	var name, namespace, region, cid *string
	if opts != nil {
		name, namespace, region, cid = opts.Name, opts.Namespace, opts.Region, opts.CID
	}
	resp, err := c.client.UpdateOrganization(ctx, &pb.UpdateOrganizationRequest{
		OrganizationId:  orgID,
		Name:            name,
		PublicNamespace: namespace,
		Region:          region,
		Cid:             cid,
	})
	if err != nil {
		return nil, err
	}
	return organizationFromProto(resp.Organization), nil
}

// DeleteOrganization deletes an organization.
func (c *AppClient) DeleteOrganization(ctx context.Context, orgID string) error {
	_, err := c.client.DeleteOrganization(ctx, &pb.DeleteOrganizationRequest{
		OrganizationId: orgID,
	})
	return err
}

// ListOrganizationMembers lists all members of an organization and all invited members to the organization.
func (c *AppClient) ListOrganizationMembers(ctx context.Context, orgID string) ([]*OrganizationMember, []*OrganizationInvite, error) {
	resp, err := c.client.ListOrganizationMembers(ctx, &pb.ListOrganizationMembersRequest{
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, nil, err
	}

	var members []*OrganizationMember
	for _, member := range resp.Members {
		members = append(members, organizationMemberFromProto(member))
	}
	var invites []*OrganizationInvite
	for _, invite := range resp.Invites {
		invites = append(invites, organizationInviteFromProto(invite))
	}
	return members, invites, nil
}

// CreateOrganizationInvite creates an organization invite to an organization.
func (c *AppClient) CreateOrganizationInvite(
	ctx context.Context, orgID, email string, authorizations []*Authorization, opts *CreateOrganizationInviteOptions,
) (*OrganizationInvite, error) {
	var pbAuthorizations []*pb.Authorization
	for _, authorization := range authorizations {
		pbAuthorizations = append(pbAuthorizations, authorizationToProto(authorization))
	}
	var send *bool
	if opts != nil {
		send = opts.SendEmailInvite
	}
	resp, err := c.client.CreateOrganizationInvite(ctx, &pb.CreateOrganizationInviteRequest{
		OrganizationId:  orgID,
		Email:           email,
		Authorizations:  pbAuthorizations,
		SendEmailInvite: send,
	})
	if err != nil {
		return nil, err
	}
	return organizationInviteFromProto(resp.Invite), nil
}

// UpdateOrganizationInviteAuthorizations updates the authorizations attached to an organization invite.
func (c *AppClient) UpdateOrganizationInviteAuthorizations(
	ctx context.Context, orgID, email string, addAuthorizations, removeAuthorizations []*Authorization,
) (*OrganizationInvite, error) {
	var pbAddAuthorizations []*pb.Authorization
	for _, authorization := range addAuthorizations {
		pbAddAuthorizations = append(pbAddAuthorizations, authorizationToProto(authorization))
	}
	var pbRemoveAuthorizations []*pb.Authorization
	for _, authorization := range removeAuthorizations {
		pbRemoveAuthorizations = append(pbRemoveAuthorizations, authorizationToProto(authorization))
	}
	resp, err := c.client.UpdateOrganizationInviteAuthorizations(ctx, &pb.UpdateOrganizationInviteAuthorizationsRequest{
		OrganizationId:       orgID,
		Email:                email,
		AddAuthorizations:    pbAddAuthorizations,
		RemoveAuthorizations: pbRemoveAuthorizations,
	})
	if err != nil {
		return nil, err
	}
	return organizationInviteFromProto(resp.Invite), nil
}

// DeleteOrganizationMember deletes an organization member from an organization.
func (c *AppClient) DeleteOrganizationMember(ctx context.Context, orgID, userID string) error {
	_, err := c.client.DeleteOrganizationMember(ctx, &pb.DeleteOrganizationMemberRequest{
		OrganizationId: orgID,
		UserId:         userID,
	})
	return err
}

// DeleteOrganizationInvite deletes an organization invite.
func (c *AppClient) DeleteOrganizationInvite(ctx context.Context, orgID, email string) error {
	_, err := c.client.DeleteOrganizationInvite(ctx, &pb.DeleteOrganizationInviteRequest{
		OrganizationId: orgID,
		Email:          email,
	})
	return err
}

// ResendOrganizationInvite resends an organization invite.
func (c *AppClient) ResendOrganizationInvite(ctx context.Context, orgID, email string) (*OrganizationInvite, error) {
	resp, err := c.client.ResendOrganizationInvite(ctx, &pb.ResendOrganizationInviteRequest{
		OrganizationId: orgID,
		Email:          email,
	})
	if err != nil {
		return nil, err
	}
	return organizationInviteFromProto(resp.Invite), nil
}

// EnableBillingService enables a billing service to an address in an organization.
func (c *AppClient) EnableBillingService(ctx context.Context, orgID string, billingAddress *BillingAddress) error {
	_, err := c.client.EnableBillingService(ctx, &pb.EnableBillingServiceRequest{
		OrgId:          orgID,
		BillingAddress: billingAddressToProto(billingAddress),
	})
	return err
}

// DisableBillingService disables the billing service for an organization.
func (c *AppClient) DisableBillingService(ctx context.Context, orgID string) error {
	_, err := c.client.DisableBillingService(ctx, &pb.DisableBillingServiceRequest{
		OrgId: orgID,
	})
	return err
}

// UpdateBillingService updates the billing service of an organization.
func (c *AppClient) UpdateBillingService(ctx context.Context, orgID string, billingAddress *BillingAddress) error {
	_, err := c.client.UpdateBillingService(ctx, &pb.UpdateBillingServiceRequest{
		OrgId:          orgID,
		BillingAddress: billingAddressToProto(billingAddress),
	})
	return err
}

// OrganizationSetSupportEmail sets an organization's support email.
func (c *AppClient) OrganizationSetSupportEmail(ctx context.Context, orgID, email string) error {
	_, err := c.client.OrganizationSetSupportEmail(ctx, &pb.OrganizationSetSupportEmailRequest{
		OrgId: orgID,
		Email: email,
	})
	return err
}

// OrganizationGetSupportEmail gets an organization's support email.
func (c *AppClient) OrganizationGetSupportEmail(ctx context.Context, orgID string) (string, error) {
	resp, err := c.client.OrganizationGetSupportEmail(ctx, &pb.OrganizationGetSupportEmailRequest{
		OrgId: orgID,
	})
	if err != nil {
		return "", err
	}
	return resp.Email, nil
}

// GetBillingServiceConfig gets the billing service configuration for an organization.
func (c *AppClient) GetBillingServiceConfig(ctx context.Context, orgID string) (*pb.GetBillingServiceConfigResponse, error) {
	resp, err := c.client.GetBillingServiceConfig(ctx, &pb.GetBillingServiceConfigRequest{
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// OrganizationSetLogo sets an organization's logo.
func (c *AppClient) OrganizationSetLogo(ctx context.Context, orgID string, logo []byte) error {
	_, err := c.client.OrganizationSetLogo(ctx, &pb.OrganizationSetLogoRequest{
		OrgId: orgID,
		Logo:  logo,
	})
	return err
}

// OrganizationGetLogo gets an organization's logo.
func (c *AppClient) OrganizationGetLogo(ctx context.Context, orgID string) (string, error) {
	resp, err := c.client.OrganizationGetLogo(ctx, &pb.OrganizationGetLogoRequest{
		OrgId: orgID,
	})
	if err != nil {
		return "", err
	}
	return resp.Url, nil
}

// ListOAuthApps gets the client's list of OAuth applications.
func (c *AppClient) ListOAuthApps(ctx context.Context, orgID string) ([]string, error) {
	resp, err := c.client.ListOAuthApps(ctx, &pb.ListOAuthAppsRequest{
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}
	return resp.ClientIds, nil
}

// CreateLocation creates a location with the given name under the given organization.
func (c *AppClient) CreateLocation(ctx context.Context, orgID, name string, opts *CreateLocationOptions) (*Location, error) {
	var parentID *string
	if opts != nil {
		parentID = opts.ParentLocationID
	}
	resp, err := c.client.CreateLocation(ctx, &pb.CreateLocationRequest{
		OrganizationId:   orgID,
		Name:             name,
		ParentLocationId: parentID,
	})
	if err != nil {
		return nil, err
	}
	return locationFromProto(resp.Location), nil
}

// GetLocation gets a location.
func (c *AppClient) GetLocation(ctx context.Context, locationID string) (*Location, error) {
	resp, err := c.client.GetLocation(ctx, &pb.GetLocationRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}
	return locationFromProto(resp.Location), nil
}

// UpdateLocation updates a location.
func (c *AppClient) UpdateLocation(ctx context.Context, locationID string, opts *UpdateLocationOptions) (*Location, error) {
	var name, parentID, region *string
	if opts != nil {
		name, parentID, region = opts.Name, opts.ParentLocationID, opts.Region
	}
	resp, err := c.client.UpdateLocation(ctx, &pb.UpdateLocationRequest{
		LocationId:       locationID,
		Name:             name,
		ParentLocationId: parentID,
		Region:           region,
	})
	if err != nil {
		return nil, err
	}
	return locationFromProto(resp.Location), nil
}

// DeleteLocation deletes a location.
func (c *AppClient) DeleteLocation(ctx context.Context, locationID string) error {
	_, err := c.client.DeleteLocation(ctx, &pb.DeleteLocationRequest{
		LocationId: locationID,
	})
	return err
}

// ListLocations gets a list of locations under the specified organization.
func (c *AppClient) ListLocations(ctx context.Context, orgID string) ([]*Location, error) {
	resp, err := c.client.ListLocations(ctx, &pb.ListLocationsRequest{
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, err
	}

	var locations []*Location
	for _, location := range resp.Locations {
		locations = append(locations, locationFromProto(location))
	}
	return locations, nil
}

// ShareLocation shares a location with an organization.
func (c *AppClient) ShareLocation(ctx context.Context, locationID, orgID string) error {
	_, err := c.client.ShareLocation(ctx, &pb.ShareLocationRequest{
		LocationId:     locationID,
		OrganizationId: orgID,
	})
	return err
}

// UnshareLocation stops sharing a location with an organization.
func (c *AppClient) UnshareLocation(ctx context.Context, locationID, orgID string) error {
	_, err := c.client.UnshareLocation(ctx, &pb.UnshareLocationRequest{
		LocationId:     locationID,
		OrganizationId: orgID,
	})
	return err
}

// LocationAuth gets a location's authorization secrets.
func (c *AppClient) LocationAuth(ctx context.Context, locationID string) (*LocationAuth, error) {
	resp, err := c.client.LocationAuth(ctx, &pb.LocationAuthRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}
	return locationAuthFromProto(resp.Auth), nil
}

// CreateLocationSecret creates a new generated secret in the location. Succeeds if there are no more than 2 active secrets after creation.
func (c *AppClient) CreateLocationSecret(ctx context.Context, locationID string) (*LocationAuth, error) {
	resp, err := c.client.CreateLocationSecret(ctx, &pb.CreateLocationSecretRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}
	return locationAuthFromProto(resp.Auth), nil
}

// DeleteLocationSecret deletes a secret from the location.
func (c *AppClient) DeleteLocationSecret(ctx context.Context, locationID, secretID string) error {
	_, err := c.client.DeleteLocationSecret(ctx, &pb.DeleteLocationSecretRequest{
		LocationId: locationID,
		SecretId:   secretID,
	})
	return err
}

// GetRobot gets a specific robot by ID.
func (c *AppClient) GetRobot(ctx context.Context, id string) (*Robot, error) {
	resp, err := c.client.GetRobot(ctx, &pb.GetRobotRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	return robotFromProto(resp.Robot), nil
}

// GetRoverRentalRobots gets rover rental robots within an organization.
func (c *AppClient) GetRoverRentalRobots(ctx context.Context, orgID string) ([]*RoverRentalRobot, error) {
	resp, err := c.client.GetRoverRentalRobots(ctx, &pb.GetRoverRentalRobotsRequest{
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}
	var robots []*RoverRentalRobot
	for _, robot := range resp.Robots {
		robots = append(robots, roverRentalRobotFromProto(robot))
	}
	return robots, nil
}

// GetRobotParts gets a list of all the parts under a specific machine.
func (c *AppClient) GetRobotParts(ctx context.Context, robotID string) ([]*RobotPart, error) {
	resp, err := c.client.GetRobotParts(ctx, &pb.GetRobotPartsRequest{
		RobotId: robotID,
	})
	if err != nil {
		return nil, err
	}
	var parts []*RobotPart
	for _, part := range resp.Parts {
		parts = append(parts, robotPartFromProto(part))
	}
	return parts, nil
}

// GetRobotPart gets a specific robot part and its config by ID.
func (c *AppClient) GetRobotPart(ctx context.Context, id string) (*RobotPart, string, error) {
	resp, err := c.client.GetRobotPart(ctx, &pb.GetRobotPartRequest{
		Id: id,
	})
	if err != nil {
		return nil, "", err
	}
	return robotPartFromProto(resp.Part), resp.ConfigJson, nil
}

// GetRobotPartLogs gets the logs associated with a robot part and the next page token.
func (c *AppClient) GetRobotPartLogs(ctx context.Context, id string, opts *GetRobotPartLogsOptions) ([]*LogEntry, string, error) {
	var filter, token, source *string
	var levels []string
	var start, end *timestamppb.Timestamp
	var limit int64
	if opts != nil {
		filter, token, source = opts.Filter, opts.PageToken, opts.Source
		levels = opts.Levels
		if opts.Start != nil {
			start = timestamppb.New(*opts.Start)
		}
		if opts.End != nil {
			end = timestamppb.New(*opts.End)
		}
		if opts.Limit != nil {
			limit = int64(*opts.Limit)
		}
	}
	resp, err := c.client.GetRobotPartLogs(ctx, &pb.GetRobotPartLogsRequest{
		Id:        id,
		Filter:    filter,
		PageToken: token,
		Levels:    levels,
		Start:     start,
		End:       end,
		Limit:     &limit,
		Source:    source,
	})
	if err != nil {
		return nil, "", err
	}
	var logs []*LogEntry
	for _, log := range resp.Logs {
		logs = append(logs, logEntryFromProto(log))
	}
	return logs, resp.NextPageToken, nil
}

// RobotPartLogStream is a stream with robot part logs.
type RobotPartLogStream struct {
	stream pb.AppService_TailRobotPartLogsClient
}

// Next gets the next slice of robot part log entries.
func (s *RobotPartLogStream) Next() ([]*LogEntry, error) {
	streamResp, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}

	var logs []*LogEntry
	for _, log := range streamResp.Logs {
		logs = append(logs, logEntryFromProto(log))
	}
	return logs, nil
}

// TailRobotPartLogs gets a stream of log entries for a specific robot part. Logs are ordered by newest first.
func (c *AppClient) TailRobotPartLogs(
	ctx context.Context, id string, errorsOnly bool, opts *TailRobotPartLogsOptions,
) (*RobotPartLogStream, error) {
	var filter *string
	if opts != nil {
		filter = opts.Filter
	}
	stream, err := c.client.TailRobotPartLogs(ctx, &pb.TailRobotPartLogsRequest{
		Id:         id,
		ErrorsOnly: errorsOnly,
		Filter:     filter,
	})
	if err != nil {
		return nil, err
	}
	return &RobotPartLogStream{stream: stream}, nil
}

// GetRobotPartHistory gets a specific robot part history by ID.
func (c *AppClient) GetRobotPartHistory(ctx context.Context, id string) ([]*RobotPartHistoryEntry, error) {
	resp, err := c.client.GetRobotPartHistory(ctx, &pb.GetRobotPartHistoryRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	var history []*RobotPartHistoryEntry
	for _, entry := range resp.History {
		history = append(history, robotPartHistoryEntryFromProto(entry))
	}
	return history, nil
}

// UpdateRobotPart updates a robot part.
func (c *AppClient) UpdateRobotPart(ctx context.Context, id, name string, robotConfig interface{}) (*RobotPart, error) {
	config, err := protoutils.StructToStructPb(robotConfig)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.UpdateRobotPart(ctx, &pb.UpdateRobotPartRequest{
		Id:          id,
		Name:        name,
		RobotConfig: config,
	})
	if err != nil {
		return nil, err
	}
	return robotPartFromProto(resp.Part), nil
}

// NewRobotPart creates a new robot part and returns its ID.
func (c *AppClient) NewRobotPart(ctx context.Context, robotID, partName string) (string, error) {
	resp, err := c.client.NewRobotPart(ctx, &pb.NewRobotPartRequest{
		RobotId:  robotID,
		PartName: partName,
	})
	if err != nil {
		return "", err
	}
	return resp.PartId, nil
}

// DeleteRobotPart deletes a robot part.
func (c *AppClient) DeleteRobotPart(ctx context.Context, partID string) error {
	_, err := c.client.DeleteRobotPart(ctx, &pb.DeleteRobotPartRequest{
		PartId: partID,
	})
	return err
}

// GetRobotAPIKeys gets the robot API keys for the robot.
func (c *AppClient) GetRobotAPIKeys(ctx context.Context, robotID string) ([]*APIKeyWithAuthorizations, error) {
	resp, err := c.client.GetRobotAPIKeys(ctx, &pb.GetRobotAPIKeysRequest{
		RobotId: robotID,
	})
	if err != nil {
		return nil, err
	}
	var keys []*APIKeyWithAuthorizations
	for _, key := range resp.ApiKeys {
		keys = append(keys, apiKeyWithAuthorizationsFromProto(key))
	}
	return keys, nil
}

// MarkPartAsMain marks the given part as the main part, and all the others as not.
func (c *AppClient) MarkPartAsMain(ctx context.Context, partID string) error {
	_, err := c.client.MarkPartAsMain(ctx, &pb.MarkPartAsMainRequest{
		PartId: partID,
	})
	return err
}

// MarkPartForRestart marks the given part for restart.
// Once the robot part checks-in with the app the flag is reset on the robot part.
// Calling this multiple times before a robot part checks-in has no effect.
func (c *AppClient) MarkPartForRestart(ctx context.Context, partID string) error {
	_, err := c.client.MarkPartForRestart(ctx, &pb.MarkPartForRestartRequest{
		PartId: partID,
	})
	return err
}

// CreateRobotPartSecret creates a new generated secret in the robot part.
// Succeeds if there are no more than 2 active secrets after creation.
func (c *AppClient) CreateRobotPartSecret(ctx context.Context, partID string) (*RobotPart, error) {
	resp, err := c.client.CreateRobotPartSecret(ctx, &pb.CreateRobotPartSecretRequest{
		PartId: partID,
	})
	if err != nil {
		return nil, err
	}
	return robotPartFromProto(resp.Part), nil
}

// DeleteRobotPartSecret deletes a secret from the robot part.
func (c *AppClient) DeleteRobotPartSecret(ctx context.Context, partID, secretID string) error {
	_, err := c.client.DeleteRobotPartSecret(ctx, &pb.DeleteRobotPartSecretRequest{
		PartId:   partID,
		SecretId: secretID,
	})
	return err
}

// ListRobots gets a list of robots under a location.
func (c *AppClient) ListRobots(ctx context.Context, locationID string) ([]*Robot, error) {
	resp, err := c.client.ListRobots(ctx, &pb.ListRobotsRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}
	var robots []*Robot
	for _, robot := range resp.Robots {
		robots = append(robots, robotFromProto(robot))
	}
	return robots, nil
}

// NewRobot creates a new robot and returns its ID.
func (c *AppClient) NewRobot(ctx context.Context, name, location string) (string, error) {
	resp, err := c.client.NewRobot(ctx, &pb.NewRobotRequest{
		Name:     name,
		Location: location,
	})
	if err != nil {
		return "", err
	}
	return resp.Id, nil
}

// UpdateRobot updates a robot.
func (c *AppClient) UpdateRobot(ctx context.Context, id, name, location string) (*Robot, error) {
	resp, err := c.client.UpdateRobot(ctx, &pb.UpdateRobotRequest{
		Id:       id,
		Name:     name,
		Location: location,
	})
	if err != nil {
		return nil, err
	}
	return robotFromProto(resp.Robot), nil
}

// DeleteRobot deletes a robot.
func (c *AppClient) DeleteRobot(ctx context.Context, id string) error {
	_, err := c.client.DeleteRobot(ctx, &pb.DeleteRobotRequest{
		Id: id,
	})
	return err
}

// ListFragments gets a list of fragments.
func (c *AppClient) ListFragments(
	ctx context.Context, orgID string, showPublic bool, fragmentVisibility []FragmentVisibility,
) ([]*Fragment, error) {
	var visibilities []pb.FragmentVisibility
	for _, visibility := range fragmentVisibility {
		pbFragmentVisibility := fragmentVisibilityToProto(visibility)
		visibilities = append(visibilities, pbFragmentVisibility)
	}
	resp, err := c.client.ListFragments(ctx, &pb.ListFragmentsRequest{
		OrganizationId:     orgID,
		ShowPublic:         showPublic,
		FragmentVisibility: visibilities,
	})
	if err != nil {
		return nil, err
	}
	var fragments []*Fragment
	for _, fragment := range resp.Fragments {
		fragments = append(fragments, fragmentFromProto(fragment))
	}
	return fragments, nil
}

// GetFragment gets a single fragment.
func (c *AppClient) GetFragment(ctx context.Context, id string) (*Fragment, error) {
	resp, err := c.client.GetFragment(ctx, &pb.GetFragmentRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	return fragmentFromProto(resp.Fragment), nil
}

// CreateFragment creates a fragment.
func (c *AppClient) CreateFragment(
	ctx context.Context, orgID, name string, config map[string]interface{}, opts *CreateFragmentOptions,
) (*Fragment, error) {
	pbConfig, err := protoutils.StructToStructPb(config)
	if err != nil {
		return nil, err
	}
	var visibility pb.FragmentVisibility
	if opts != nil && opts.Visibility != nil {
		visibility = fragmentVisibilityToProto(*opts.Visibility)
	}
	resp, err := c.client.CreateFragment(ctx, &pb.CreateFragmentRequest{
		Name:           name,
		Config:         pbConfig,
		OrganizationId: orgID,
		Visibility:     &visibility,
	})
	if err != nil {
		return nil, err
	}
	return fragmentFromProto(resp.Fragment), nil
}

// UpdateFragment updates a fragment.
func (c *AppClient) UpdateFragment(
	ctx context.Context, id, name string, config map[string]interface{}, opts *UpdateFragmentOptions,
) (*Fragment, error) {
	cfg, err := protoutils.StructToStructPb(config)
	if err != nil {
		return nil, err
	}
	var public *bool
	var visibility pb.FragmentVisibility
	if opts != nil {
		public = opts.Public
		if opts.Visibility != nil {
			visibility = fragmentVisibilityToProto(*opts.Visibility)
		}
	}
	resp, err := c.client.UpdateFragment(ctx, &pb.UpdateFragmentRequest{
		Id:         id,
		Name:       name,
		Config:     cfg,
		Public:     public,
		Visibility: &visibility,
	})
	if err != nil {
		return nil, err
	}
	return fragmentFromProto(resp.Fragment), nil
}

// DeleteFragment deletes a fragment.
func (c *AppClient) DeleteFragment(ctx context.Context, id string) error {
	_, err := c.client.DeleteFragment(ctx, &pb.DeleteFragmentRequest{
		Id: id,
	})
	return err
}

// ListMachineFragments gets top level and nested fragments for a amchine, as well as any other fragments specified by IDs.
// Additional fragments are useful to view fragments that will be provisionally added to the machine alongside existing fragments.
func (c *AppClient) ListMachineFragments(ctx context.Context, machineID string, additionalIDs []string) ([]*Fragment, error) {
	resp, err := c.client.ListMachineFragments(ctx, &pb.ListMachineFragmentsRequest{
		MachineId:             machineID,
		AdditionalFragmentIds: additionalIDs,
	})
	if err != nil {
		return nil, err
	}
	var fragments []*Fragment
	for _, fragment := range resp.Fragments {
		fragments = append(fragments, fragmentFromProto(fragment))
	}
	return fragments, nil
}

// GetFragmentHistory gets the fragment's history and the next page token.
func (c *AppClient) GetFragmentHistory(
	ctx context.Context, id string, opts *GetFragmentHistoryOptions,
) ([]*FragmentHistoryEntry, string, error) {
	var token *string
	var limit int64
	if opts != nil {
		token = opts.PageToken
		if opts.PageLimit != nil {
			limit = int64(*opts.PageLimit)
		}
	}
	resp, err := c.client.GetFragmentHistory(ctx, &pb.GetFragmentHistoryRequest{
		Id:        id,
		PageToken: token,
		PageLimit: &limit,
	})
	if err != nil {
		return nil, "", err
	}
	var history []*FragmentHistoryEntry
	for _, entry := range resp.History {
		history = append(history, fragmentHistoryEntryFromProto(entry))
	}
	return history, resp.NextPageToken, nil
}

// AddRole creates an identity authorization.
func (c *AppClient) AddRole(
	ctx context.Context, orgID, identityID string, role AuthRole, resourceType AuthResourceType, resourceID string,
) error {
	authorization := createAuthorization(orgID, identityID, "", role, resourceType, resourceID)
	_, err := c.client.AddRole(ctx, &pb.AddRoleRequest{
		Authorization: authorization,
	})
	return err
}

// RemoveRole deletes an identity authorization.
func (c *AppClient) RemoveRole(ctx context.Context, authorization *Authorization) error {
	_, err := c.client.RemoveRole(ctx, &pb.RemoveRoleRequest{
		Authorization: authorizationToProto(authorization),
	})
	return err
}

// ChangeRole changes an identity authorization to a new identity authorization.
func (c *AppClient) ChangeRole(
	ctx context.Context,
	oldAuthorization *Authorization,
	newOrgID,
	newIdentityID string,
	newRole AuthRole,
	newResourceType AuthResourceType,
	newResourceID string,
) error {
	newAuthorization := createAuthorization(newOrgID, newIdentityID, "", newRole, newResourceType, newResourceID)
	_, err := c.client.ChangeRole(ctx, &pb.ChangeRoleRequest{
		OldAuthorization: authorizationToProto(oldAuthorization),
		NewAuthorization: newAuthorization,
	})
	return err
}

// ListAuthorizations returns all authorization roles for any given resources.
// If no resources are given, all resources within the organization will be included.
func (c *AppClient) ListAuthorizations(ctx context.Context, orgID string, resourceIDs []string) ([]*Authorization, error) {
	resp, err := c.client.ListAuthorizations(ctx, &pb.ListAuthorizationsRequest{
		OrganizationId: orgID,
		ResourceIds:    resourceIDs,
	})
	if err != nil {
		return nil, err
	}
	var authorizations []*Authorization
	for _, authorization := range resp.Authorizations {
		authorizations = append(authorizations, authorizationFromProto(authorization))
	}
	return authorizations, nil
}

// CheckPermissions checks the validity of a list of permissions.
func (c *AppClient) CheckPermissions(ctx context.Context, permissions []*AuthorizedPermissions) ([]*AuthorizedPermissions, error) {
	var pbPermissions []*pb.AuthorizedPermissions
	for _, permission := range permissions {
		pbPermissions = append(pbPermissions, authorizedPermissionsToProto(permission))
	}

	resp, err := c.client.CheckPermissions(ctx, &pb.CheckPermissionsRequest{
		Permissions: pbPermissions,
	})
	if err != nil {
		return nil, err
	}

	var authorizedPermissions []*AuthorizedPermissions
	for _, permission := range resp.AuthorizedPermissions {
		authorizedPermissions = append(authorizedPermissions, authorizedPermissionsFromProto(permission))
	}
	return authorizedPermissions, nil
}

// GetRegistryItem gets a registry item.
func (c *AppClient) GetRegistryItem(ctx context.Context, itemID string) (*RegistryItem, error) {
	resp, err := c.client.GetRegistryItem(ctx, &pb.GetRegistryItemRequest{
		ItemId: itemID,
	})
	if err != nil {
		return nil, err
	}
	item, err := registryItemFromProto(resp.Item)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// CreateRegistryItem creates a registry item.
func (c *AppClient) CreateRegistryItem(ctx context.Context, orgID, name string, packageType PackageType) error {
	_, err := c.client.CreateRegistryItem(ctx, &pb.CreateRegistryItemRequest{
		OrganizationId: orgID,
		Name:           name,
		Type:           packageTypeToProto(packageType),
	})
	return err
}

// UpdateRegistryItem updates a registry item.
func (c *AppClient) UpdateRegistryItem(
	ctx context.Context, itemID string, packageType PackageType, description string, visibility Visibility, opts *UpdateRegistryItemOptions,
) error {
	var siteURL *string
	if opts != nil {
		siteURL = opts.URL
	}
	_, err := c.client.UpdateRegistryItem(ctx, &pb.UpdateRegistryItemRequest{
		ItemId:      itemID,
		Type:        packageTypeToProto(packageType),
		Description: description,
		Visibility:  visibilityToProto(visibility),
		Url:         siteURL,
	})
	return err
}

// ListRegistryItems lists the registry items in an organization.
func (c *AppClient) ListRegistryItems(
	ctx context.Context,
	orgID *string,
	types []PackageType,
	visibilities []Visibility,
	platforms []string,
	statuses []RegistryItemStatus,
	opts *ListRegistryItemsOptions,
) ([]*RegistryItem, error) {
	var pbTypes []packages.PackageType
	for _, packageType := range types {
		pbTypes = append(pbTypes, packageTypeToProto(packageType))
	}
	var pbVisibilities []pb.Visibility
	for _, visibility := range visibilities {
		pbVisibilities = append(pbVisibilities, visibilityToProto(visibility))
	}
	var pbStatuses []pb.RegistryItemStatus
	for _, status := range statuses {
		pbStatuses = append(pbStatuses, registryItemStatusToProto(status))
	}

	var term, token *string
	var namespaces []string
	if opts != nil {
		term, token = opts.SearchTerm, opts.PageToken
		namespaces = opts.PublicNamespaces
	}
	resp, err := c.client.ListRegistryItems(ctx, &pb.ListRegistryItemsRequest{
		OrganizationId:   orgID,
		Types:            pbTypes,
		Visibilities:     pbVisibilities,
		Platforms:        platforms,
		Statuses:         pbStatuses,
		SearchTerm:       term,
		PageToken:        token,
		PublicNamespaces: namespaces,
	})
	if err != nil {
		return nil, err
	}
	var items []*RegistryItem
	for _, item := range resp.Items {
		i, err := registryItemFromProto(item)
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

// DeleteRegistryItem deletes a registry item given an ID that is formatted as `prefix:name“
// where `prefix“ is the owner's organization ID or namespace.
func (c *AppClient) DeleteRegistryItem(ctx context.Context, itemID string) error {
	_, err := c.client.DeleteRegistryItem(ctx, &pb.DeleteRegistryItemRequest{
		ItemId: itemID,
	})
	return err
}

// TransferRegistryItem transfers a registry item to a namespace.
func (c *AppClient) TransferRegistryItem(ctx context.Context, itemID, newPublicNamespace string) error {
	_, err := c.client.TransferRegistryItem(ctx, &pb.TransferRegistryItemRequest{
		ItemId:             itemID,
		NewPublicNamespace: newPublicNamespace,
	})
	return err
}

// CreateModule creates a module and returns its ID and URL.
func (c *AppClient) CreateModule(ctx context.Context, orgID, name string) (string, string, error) {
	resp, err := c.client.CreateModule(ctx, &pb.CreateModuleRequest{
		OrganizationId: orgID,
		Name:           name,
	})
	if err != nil {
		return "", "", err
	}
	return resp.ModuleId, resp.Url, nil
}

// UpdateModule updates the documentation URL, description, models, entrypoint, and/or the visibility of a module and returns its URL.
// A path to a setup script can be added that is run before a newly downloaded module starts.
func (c *AppClient) UpdateModule(
	ctx context.Context,
	moduleID string,
	visibility Visibility,
	url,
	description string,
	models []*Model,
	entrypoint string,
	opts *UpdateModuleOptions,
) (string, error) {
	var pbModels []*pb.Model
	for _, model := range models {
		pbModels = append(pbModels, modelToProto(model))
	}
	var firstRun *string
	if opts != nil {
		firstRun = opts.FirstRun
	}
	resp, err := c.client.UpdateModule(ctx, &pb.UpdateModuleRequest{
		ModuleId:    moduleID,
		Visibility:  visibilityToProto(visibility),
		Url:         url,
		Description: description,
		Models:      pbModels,
		Entrypoint:  entrypoint,
		FirstRun:    firstRun,
	})
	if err != nil {
		return "", err
	}
	return resp.Url, nil
}

// UploadModuleFile uploads a module file and returns the URL of the uploaded file.
func (c *AppClient) UploadModuleFile(ctx context.Context, fileInfo ModuleFileInfo, file []byte) (string, error) {
	stream, err := c.client.UploadModuleFile(ctx)
	if err != nil {
		return "", err
	}

	err = stream.Send(&pb.UploadModuleFileRequest{
		ModuleFile: &pb.UploadModuleFileRequest_ModuleFileInfo{
			ModuleFileInfo: moduleFileInfoToProto(&fileInfo),
		},
	})
	if err != nil {
		return "", err
	}

	for start := 0; start < len(file); start += UploadChunkSize {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		end := start + UploadChunkSize
		if end > len(file) {
			end = len(file)
		}

		chunk := file[start:end]
		err := stream.Send(&pb.UploadModuleFileRequest{
			ModuleFile: &pb.UploadModuleFileRequest_File{
				File: chunk,
			},
		})
		if err != nil {
			return "", err
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return "", err
	}
	return resp.Url, err
}

// GetModule gets a module.
func (c *AppClient) GetModule(ctx context.Context, moduleID string) (*Module, error) {
	resp, err := c.client.GetModule(ctx, &pb.GetModuleRequest{
		ModuleId: moduleID,
	})
	if err != nil {
		return nil, err
	}
	return moduleFromProto(resp.Module), nil
}

// ListModules lists the modules in the organization.
func (c *AppClient) ListModules(ctx context.Context, opts *ListModulesOptions) ([]*Module, error) {
	var orgID *string
	if opts != nil {
		orgID = opts.OrgID
	}
	resp, err := c.client.ListModules(ctx, &pb.ListModulesRequest{
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, err
	}
	var modules []*Module
	for _, module := range resp.Modules {
		modules = append(modules, moduleFromProto(module))
	}
	return modules, nil
}

// CreateKey creates a new API key associated with a list of authorizations and returns its key and ID.
func (c *AppClient) CreateKey(
	ctx context.Context, orgID string, keyAuthorizations []APIKeyAuthorization, name string,
) (string, string, error) {
	var authorizations []*pb.Authorization
	for _, keyAuthorization := range keyAuthorizations {
		authorization := createAuthorization(
			orgID, "", "api-key", keyAuthorization.role, keyAuthorization.resourceType, keyAuthorization.resourceID)
		authorizations = append(authorizations, authorization)
	}

	resp, err := c.client.CreateKey(ctx, &pb.CreateKeyRequest{
		Authorizations: authorizations,
		Name:           name,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Key, resp.Id, nil
}

// DeleteKey deletes an API key.
func (c *AppClient) DeleteKey(ctx context.Context, id string) error {
	_, err := c.client.DeleteKey(ctx, &pb.DeleteKeyRequest{
		Id: id,
	})
	return err
}

// ListKeys lists all the keys for the organization.
func (c *AppClient) ListKeys(ctx context.Context, orgID string) ([]*APIKeyWithAuthorizations, error) {
	resp, err := c.client.ListKeys(ctx, &pb.ListKeysRequest{
		OrgId: orgID,
	})
	if err != nil {
		return nil, err
	}
	var apiKeys []*APIKeyWithAuthorizations
	for _, key := range resp.ApiKeys {
		apiKeys = append(apiKeys, apiKeyWithAuthorizationsFromProto(key))
	}
	return apiKeys, nil
}

// RenameKey renames an API key and returns its ID and name.
func (c *AppClient) RenameKey(ctx context.Context, id, name string) (string, string, error) {
	resp, err := c.client.RenameKey(ctx, &pb.RenameKeyRequest{
		Id:   id,
		Name: name,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Id, resp.Name, nil
}

// RotateKey rotates an API key and returns its ID and key.
func (c *AppClient) RotateKey(ctx context.Context, id string) (string, string, error) {
	resp, err := c.client.RotateKey(ctx, &pb.RotateKeyRequest{
		Id: id,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Id, resp.Key, nil
}

// CreateKeyFromExistingKeyAuthorizations creates a new API key with an existing key's authorizations and returns its ID and key.
func (c *AppClient) CreateKeyFromExistingKeyAuthorizations(ctx context.Context, id string) (string, string, error) {
	resp, err := c.client.CreateKeyFromExistingKeyAuthorizations(ctx, &pb.CreateKeyFromExistingKeyAuthorizationsRequest{
		Id: id,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Id, resp.Key, nil
}

func organizationFromProto(organization *pb.Organization) *Organization {
	if organization == nil {
		return nil
	}
	var createdOn *time.Time
	if organization.CreatedOn != nil {
		t := organization.CreatedOn.AsTime()
		createdOn = &t
	}
	return &Organization{
		ID:              organization.Id,
		Name:            organization.Name,
		CreatedOn:       createdOn,
		PublicNamespace: organization.PublicNamespace,
		DefaultRegion:   organization.DefaultRegion,
		Cid:             organization.Cid,
	}
}

func organizationIdentityFromProto(identity *pb.OrganizationIdentity) *OrganizationIdentity {
	if identity == nil {
		return nil
	}
	return &OrganizationIdentity{
		ID:   identity.Id,
		Name: identity.Name,
	}
}

func orgDetailsFromProto(orgDetails *pb.OrgDetails) *OrgDetails {
	if orgDetails == nil {
		return nil
	}
	return &OrgDetails{
		OrgID:   orgDetails.OrgId,
		OrgName: orgDetails.OrgName,
	}
}

func organizationMemberFromProto(member *pb.OrganizationMember) *OrganizationMember {
	if member == nil {
		return nil
	}
	var dateAdded, lastLogin *time.Time
	if member.DateAdded != nil {
		t := member.DateAdded.AsTime()
		dateAdded = &t
	}
	if member.LastLogin != nil {
		t := member.LastLogin.AsTime()
		lastLogin = &t
	}
	return &OrganizationMember{
		UserID:    member.UserId,
		Emails:    member.Emails,
		DateAdded: dateAdded,
		LastLogin: lastLogin,
	}
}

func organizationInviteFromProto(invite *pb.OrganizationInvite) *OrganizationInvite {
	if invite == nil {
		return nil
	}
	var createdOn *time.Time
	if invite.CreatedOn != nil {
		t := invite.CreatedOn.AsTime()
		createdOn = &t
	}
	var authorizations []*Authorization
	for _, authorization := range invite.Authorizations {
		authorizations = append(authorizations, authorizationFromProto(authorization))
	}
	return &OrganizationInvite{
		OrganizationID: invite.OrganizationId,
		Email:          invite.Email,
		CreatedOn:      createdOn,
		Authorizations: authorizations,
	}
}

func billingAddressToProto(addr *BillingAddress) *pb.BillingAddress {
	if addr == nil {
		return nil
	}
	return &pb.BillingAddress{
		AddressLine_1: addr.AddressLine1,
		AddressLine_2: addr.AddressLine2,
		City:          addr.City,
		State:         addr.State,
		Zipcode:       addr.Zipcode,
	}
}

func locationFromProto(location *pb.Location) *Location {
	if location == nil {
		return nil
	}
	var createdOn *time.Time
	if location.CreatedOn != nil {
		t := location.CreatedOn.AsTime()
		createdOn = &t
	}
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
		CreatedOn:        createdOn,
		RobotCount:       int(location.RobotCount),
		Config:           storageConfigFromProto(location.Config),
	}
}

func locationOrganizationFromProto(locationOrg *pb.LocationOrganization) *LocationOrganization {
	if locationOrg == nil {
		return nil
	}
	return &LocationOrganization{
		OrganizationID: locationOrg.OrganizationId,
		Primary:        locationOrg.Primary,
	}
}

func storageConfigFromProto(config *pb.StorageConfig) *StorageConfig {
	if config == nil {
		return nil
	}
	return &StorageConfig{Region: config.Region}
}

func locationAuthFromProto(auth *pb.LocationAuth) *LocationAuth {
	if auth == nil {
		return nil
	}
	var secrets []*SharedSecret
	for _, secret := range auth.Secrets {
		secrets = append(secrets, sharedSecretFromProto(secret))
	}
	return &LocationAuth{
		LocationID: auth.LocationId,
		Secrets:    secrets,
	}
}

func robotFromProto(robot *pb.Robot) *Robot {
	if robot == nil {
		return nil
	}
	var createdOn, lastAccess *time.Time
	if robot.CreatedOn != nil {
		t := robot.CreatedOn.AsTime()
		createdOn = &t
	}
	if robot.LastAccess != nil {
		t := robot.LastAccess.AsTime()
		lastAccess = &t
	}
	return &Robot{
		ID:         robot.Id,
		Name:       robot.Name,
		Location:   robot.Location,
		LastAccess: lastAccess,
		CreatedOn:  createdOn,
	}
}

func roverRentalRobotFromProto(robot *pb.RoverRentalRobot) *RoverRentalRobot {
	if robot == nil {
		return nil
	}
	return &RoverRentalRobot{
		RobotID:         robot.RobotId,
		LocationID:      robot.LocationId,
		RobotName:       robot.RobotName,
		RobotMainPartID: robot.RobotMainPartId,
	}
}

func robotPartFromProto(part *pb.RobotPart) *RobotPart {
	if part == nil {
		return nil
	}
	var createdOn, lastAccess *time.Time
	if part.CreatedOn != nil {
		t := part.CreatedOn.AsTime()
		createdOn = &t
	}
	if part.LastAccess != nil {
		t := part.LastAccess.AsTime()
		lastAccess = &t
	}
	var secrets []*SharedSecret
	for _, secret := range part.Secrets {
		secrets = append(secrets, sharedSecretFromProto(secret))
	}
	var cfg, info map[string]interface{}
	if part.RobotConfig != nil {
		cfg = part.RobotConfig.AsMap()
	}
	if part.UserSuppliedInfo != nil {
		info = part.UserSuppliedInfo.AsMap()
	}
	lastUpdated := part.LastUpdated.AsTime()
	return &RobotPart{
		ID:               part.Id,
		Name:             part.Name,
		DNSName:          part.DnsName,
		Secret:           part.Secret,
		Robot:            part.Robot,
		LocationID:       part.LocationId,
		RobotConfig:      cfg,
		LastAccess:       lastAccess,
		UserSuppliedInfo: info,
		MainPart:         part.MainPart,
		FQDN:             part.Fqdn,
		LocalFQDN:        part.LocalFqdn,
		CreatedOn:        createdOn,
		Secrets:          secrets,
		LastUpdated:      &lastUpdated,
	}
}

func robotPartHistoryEntryFromProto(entry *pb.RobotPartHistoryEntry) *RobotPartHistoryEntry {
	if entry == nil {
		return nil
	}
	var when *time.Time
	if entry.When != nil {
		t := entry.When.AsTime()
		when = &t
	}
	return &RobotPartHistoryEntry{
		Part:     entry.Part,
		Robot:    entry.Robot,
		When:     when,
		Old:      robotPartFromProto(entry.Old),
		EditedBy: authenticatorInfoFromProto(entry.EditedBy),
	}
}

func logEntryFromProto(log *common.LogEntry) *LogEntry {
	if log == nil {
		return nil
	}
	var entryTime *time.Time
	if log.Time != nil {
		t := log.Time.AsTime()
		entryTime = &t
	}
	var caller map[string]interface{}
	if log.Caller != nil {
		caller = log.Caller.AsMap()
	}
	var fields []map[string]interface{}
	for _, field := range log.Fields {
		if field == nil {
			continue
		}
		fields = append(fields, field.AsMap())
	}
	return &LogEntry{
		Host:       log.Host,
		Level:      log.Level,
		Time:       entryTime,
		LoggerName: log.LoggerName,
		Message:    log.Message,
		Caller:     caller,
		Stack:      log.Stack,
		Fields:     fields,
	}
}

func fragmentFromProto(fragment *pb.Fragment) *Fragment {
	if fragment == nil {
		return nil
	}
	var frag map[string]interface{}
	if fragment.Fragment != nil {
		frag = fragment.Fragment.AsMap()
	}
	var createdOn, lastUpdated *time.Time
	if fragment.CreatedOn != nil {
		t := fragment.CreatedOn.AsTime()
		createdOn = &t
	}
	if fragment.LastUpdated != nil {
		t := fragment.LastUpdated.AsTime()
		lastUpdated = &t
	}
	return &Fragment{
		ID:                fragment.Id,
		Name:              fragment.Name,
		Fragment:          frag,
		OrganizationOwner: fragment.OrganizationOwner,
		Public:            fragment.Public,
		CreatedOn:         createdOn,
		OrganizationName:  fragment.OrganizationName,
		RobotPartCount:    int(fragment.RobotPartCount),
		OrganizationCount: int(fragment.OrganizationCount),
		OnlyUsedByOwner:   fragment.OnlyUsedByOwner,
		Visibility:        fragmentVisibilityFromProto(fragment.Visibility),
		LastUpdated:       lastUpdated,
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
	if entry == nil {
		return nil
	}
	var editedOn *time.Time
	if entry.EditedOn != nil {
		t := entry.EditedOn.AsTime()
		editedOn = &t
	}
	return &FragmentHistoryEntry{
		Fragment: entry.Fragment,
		EditedOn: editedOn,
		Old:      fragmentFromProto(entry.Old),
		EditedBy: authenticatorInfoFromProto(entry.EditedBy),
	}
}

func authorizationFromProto(authorization *pb.Authorization) *Authorization {
	if authorization == nil {
		return nil
	}
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
	if authorization == nil {
		return nil
	}
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
	if permissions == nil {
		return nil
	}
	return &AuthorizedPermissions{
		ResourceType: permissions.ResourceType,
		ResourceID:   permissions.ResourceId,
		Permissions:  permissions.Permissions,
	}
}

func authorizedPermissionsToProto(permissions *AuthorizedPermissions) *pb.AuthorizedPermissions {
	if permissions == nil {
		return nil
	}
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

func sharedSecretFromProto(secret *pb.SharedSecret) *SharedSecret {
	if secret == nil {
		return nil
	}
	var createdOn *time.Time
	if secret.CreatedOn != nil {
		t := secret.CreatedOn.AsTime()
		createdOn = &t
	}
	return &SharedSecret{
		ID:        secret.Id,
		CreatedOn: createdOn,
		State:     sharedSecretStateFromProto(secret.State),
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
	if info == nil {
		return nil
	}
	return &AuthenticatorInfo{
		Type:          authenticationTypeFromProto(info.Type),
		Value:         info.Value,
		IsDeactivated: info.IsDeactivated,
	}
}

func authenticationTypeFromProto(authType pb.AuthenticationType) AuthenticationType {
	switch authType {
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
	if key == nil {
		return nil
	}
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
	if key == nil {
		return nil
	}
	createdOn := key.CreatedOn.AsTime()
	return &APIKey{
		ID:        key.Id,
		Key:       key.Key,
		Name:      key.Name,
		CreatedOn: &createdOn,
	}
}

func authorizationDetailsFromProto(details *pb.AuthorizationDetails) *AuthorizationDetails {
	if details == nil {
		return nil
	}
	return &AuthorizationDetails{
		AuthorizationType: details.AuthorizationType,
		AuthorizationID:   details.AuthorizationId,
		ResourceType:      details.ResourceType,
		ResourceID:        details.ResourceId,
		OrgID:             details.OrgId,
	}
}

func registryItemFromProto(item *pb.RegistryItem) (*RegistryItem, error) {
	if item == nil {
		return nil, nil
	}
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
	var createdAt, updatedAt *time.Time
	if item.CreatedAt != nil {
		t := item.CreatedAt.AsTime()
		createdAt = &t
	}
	if item.UpdatedAt != nil {
		t := item.UpdatedAt.AsTime()
		updatedAt = &t
	}
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
		CreatedAt:                      createdAt,
		UpdatedAt:                      updatedAt,
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
	if md == nil {
		return nil
	}
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
	if model == nil {
		return nil
	}
	return &Model{
		API:   model.Api,
		Model: model.Model,
	}
}

func modelToProto(model *Model) *pb.Model {
	if model == nil {
		return nil
	}
	return &pb.Model{
		Api:   model.API,
		Model: model.Model,
	}
}

func moduleVersionFromProto(version *pb.ModuleVersion) *ModuleVersion {
	if version == nil {
		return nil
	}
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
	if uploads == nil {
		return nil
	}
	var uploadedAt *time.Time
	if uploads.UploadedAt != nil {
		t := uploads.UploadedAt.AsTime()
		uploadedAt = &t
	}
	return &Uploads{
		Platform:   uploads.Platform,
		UploadedAt: uploadedAt,
	}
}

func mlModelMetadataFromProto(md *pb.MLModelMetadata) *MLModelMetadata {
	if md == nil {
		return nil
	}
	return &MLModelMetadata{
		Versions:       md.Versions,
		ModelType:      modelTypeFromProto(md.ModelType),
		ModelFramework: modelFrameworkFromProto(md.ModelFramework),
	}
}

func mlTrainingMetadataFromProto(md *pb.MLTrainingMetadata) *MLTrainingMetadata {
	if md == nil {
		return nil
	}
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
	if version == nil {
		return nil
	}
	var createdOn *time.Time
	if version.CreatedOn != nil {
		t := version.CreatedOn.AsTime()
		createdOn = &t
	}
	return &MLTrainingVersion{
		Version:   version.Version,
		CreatedOn: createdOn,
	}
}

func moduleFromProto(module *pb.Module) *Module {
	if module == nil {
		return nil
	}
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
	if info == nil {
		return nil
	}
	return &pb.ModuleFileInfo{
		ModuleId:     info.ModuleID,
		Version:      info.Version,
		Platform:     info.Platform,
		PlatformTags: info.PlatformTags,
	}
}

func versionHistoryFromProto(history *pb.VersionHistory) *VersionHistory {
	if history == nil {
		return nil
	}
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
