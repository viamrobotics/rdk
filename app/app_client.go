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
	ID        string `json:"id"`
	Key       string `json:"key"`
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
	Apps       []*App
	Versions   []*ModuleVersion
	Entrypoint string
	FirstRun   *string
}

// Model holds the colon-delimited-triplet of the model and the API implemented by the model.
type Model struct {
	API   string
	Model string
}

// App holds the information of an viam app.
type App struct {
	Name       string
	Type       string
	Entrypoint string
}

// ModuleVersion holds the information of a module version.
type ModuleVersion struct {
	Version    string
	Files      []*Uploads
	Models     []*Model
	Apps       []*App
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
	Versions       []*MLModelVersion
	ModelType      ModelType
	ModelFramework ModelFramework
}

// MLModelVersion is the version of a ML model.
type MLModelVersion struct {
	Version   string
	CreatedOn *time.Time
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
	Apps                   []*App
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
	Apps       []*App
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

// LocationSummary contains summaries of machines housed under some location with its ID and name.
type LocationSummary struct {
	LocationID       string
	LocationName     string
	MachineSummaries []*MachineSummary
}

// MachineSummary contains a single machine's ID and name.
type MachineSummary struct {
	MachineID   string
	MachineName string
}

// AppBranding contains metadata relevant to Viam Apps customizations.
//
//nolint:revive // AppBranding is clearer than Branding in context of Viam Apps
type AppBranding struct {
	LogoPath           string
	TextCustomizations map[string]TextOverrides
	FragmentIDs        []string
}

// TextOverrides contains the text Viam App developers want displayed on the Viam Apps "machine picker" page.
type TextOverrides struct {
	Fields map[string]string
}

// AppContent defines where how to retrieve a Viam Apps app from GCS.
//
//nolint:revive // AppContent is clearer than Content in context of Viam Apps
type AppContent struct {
	BlobPath   string
	Entrypoint string
	AppType    int
	Public     bool
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
//
// GetUserIDByEmail example:
//
//	userID, err := cloud.GetUserIDByEmail(context.Background(), "test@example.com")
//
// For more information, see the [GetUserIDByEmail method docs].
//
// [GetUserIDByEmail method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getuseridbyemail
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
//
// CreateOrganization example:
//
//	organization, err := cloud.CreateOrganization(context.Background(), "testOrganization")
//
// For more information, see the [CreateOrganization method docs].
//
// [CreateOrganization method docs]: https://docs.viam.com/dev/reference/apis/fleet/#createorganization
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
//
// ListOrganizations example:
//
//	organizations, err := cloud.ListOrganizations(context.Background())
//
// For more information, see the [ListOrganizations method docs].
//
// [ListOrganizations method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listorganizations
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
//
// GetOrganizationsWithAccessToLocation example:
//
//	organizations, err := cloud.GetOrganizationsWithAccessToLocation(context.Background(), "ab1c2d3e45")
//
// For more information, see the [GetOrganizationsWithAccessToLocation method docs].
//
// [GetOrganizationsWithAccessToLocation method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getorganizationswithaccesstolocation
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
//
// ListOrganizationsByUser example:
//
//	organizations, err := cloud.ListOrganizationsByUser(context.Background(), "1234a56b-1234-1a23-1234-a12bcd3ef4a5")
//
// For more information, see the [ListOrganizationsByUser method docs].
//
// [ListOrganizationsByUser method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listorganizationsbyuser
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
//
// GetOrganization example:
//
//	organization, err := cloud.GetOrganization(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [GetOrganization method docs].
//
// [GetOrganization method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getorganization
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
//
// GetOrganizationNamespaceAvailability example:
//
//	available, err := cloud.GetOrganizationNamespaceAvailability(context.Background(), "test-namespace")
//
// For more information, see the [GetOrganizationNamespaceAvailability method docs].
//
// [GetOrganizationNamespaceAvailability method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getorganizationnamespaceavailability
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
//
// UpdateOrganization example:
//
//	 name := "tests-name"
//		organization, err := cloud.UpdateOrganization(
//			context.Background(),
//			"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//			&UpdateOrganizationOptions{
//				Name: &name,
//			})
//
// For more information, see the [UpdateOrganization method docs].
//
// [UpdateOrganization method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updateorganization
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
//
// DeleteOrganization example:
//
//	err := cloud.DeleteOrganization(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [DeleteOrganization method docs].
//
// [DeleteOrganization method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deleteorganization
func (c *AppClient) DeleteOrganization(ctx context.Context, orgID string) error {
	_, err := c.client.DeleteOrganization(ctx, &pb.DeleteOrganizationRequest{
		OrganizationId: orgID,
	})
	return err
}

// GetOrganizationMetadata gets the user-defined metadata for an organization.
//
// GetOrganizationMetadata example:
//
//	metadata, err := cloud.GetOrganizationMetadata(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [GetOrganizationMetadata method docs].
//
// [GetOrganizationMetadata method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getorganizationmetadata
func (c *AppClient) GetOrganizationMetadata(ctx context.Context, organizationID string) (map[string]interface{}, error) {
	resp, err := c.client.GetOrganizationMetadata(ctx, &pb.GetOrganizationMetadataRequest{
		OrganizationId: organizationID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data.AsMap(), nil
}

// UpdateOrganizationMetadata updates the user-defined metadata for an organization.
//
// UpdateOrganizationMetadata example:
//
//	err := cloud.UpdateOrganizationMetadata(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		map[string]interface{}{
//			"key": "value",
//		},
//	)
//
// For more information, see the [UpdateOrganizationMetadata method docs].
//
// [UpdateOrganizationMetadata method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updateorganizationmetadata
func (c *AppClient) UpdateOrganizationMetadata(ctx context.Context, organizationID string, data interface{}) error {
	d, err := protoutils.StructToStructPb(data)
	if err != nil {
		return err
	}
	_, err = c.client.UpdateOrganizationMetadata(ctx, &pb.UpdateOrganizationMetadataRequest{
		OrganizationId: organizationID,
		Data:           d,
	})
	return err
}

// ListOrganizationMembers lists all members of an organization and all invited members to the organization.
//
// ListOrganizationMembers example:
//
//	members, invites, err := cloud.ListOrganizationMembers(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [ListOrganizationMembers method docs].
//
// [ListOrganizationMembers method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listorganizationmembers
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
//
// CreateOrganizationInvite example:
//
//	func boolPtr(b bool) *bool {
//		return &b
//	}
//
//	invite, err := cloud.CreateOrganizationInvite(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		"test@example.com",
//		[]*Authorization{&Authorization{}},
//		&app.CreateOrganizationInviteOptions{
//			SendEmailInvite: boolPtr(true),
//		})
//
// For more information, see the [CreateOrganizationInvite method docs].
//
// [CreateOrganizationInvite method docs]: https://docs.viam.com/dev/reference/apis/fleet/#createorganizationinvite
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
//
// UpdateOrganizationInviteAuthorizations example:
//
//	invite, err := cloud.UpdateOrganizationInviteAuthorizations(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		"test@example.com",
//		[]*app.Authorization{
//			AuthorizationType: "role",
//			AuthorizationID:   "location_owner",
//			ResourceType:      "location",
//			ResourceID:        LOCATION_ID,
//			OrganizationID:    ORG_ID,
//			IdentityID:        "",
//		},
//		[]*app.Authorization{})
//
// For more information, see the [UpdateOrganizationInviteAuthorizations method docs].
//
// [UpdateOrganizationInviteAuthorizations method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updateorganizationinviteauthorizations
//
// [UpdateOrganizationInviteAuthorizations method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updateorganizationinviteauthorizations
//
//nolint:lll
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
//
// DeleteOrganizationMember example:
//
//	err := cloud.DeleteOrganizationMember(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		"1234a56b-1234-1a23-1234-a12bcd3ef4a5")
//
// For more information, see the [DeleteOrganizationMember method docs].
//
// [DeleteOrganizationMember method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deleteorganizationmember
func (c *AppClient) DeleteOrganizationMember(ctx context.Context, orgID, userID string) error {
	_, err := c.client.DeleteOrganizationMember(ctx, &pb.DeleteOrganizationMemberRequest{
		OrganizationId: orgID,
		UserId:         userID,
	})
	return err
}

// DeleteOrganizationInvite deletes an organization invite.
//
// DeleteOrganizationInvite example:
//
//	err := cloud.DeleteOrganizationInvite(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		"test@example.com")
//
// For more information, see the [DeleteOrganizationInvite method docs].
//
// [DeleteOrganizationInvite method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deleteorganizationinvite
func (c *AppClient) DeleteOrganizationInvite(ctx context.Context, orgID, email string) error {
	_, err := c.client.DeleteOrganizationInvite(ctx, &pb.DeleteOrganizationInviteRequest{
		OrganizationId: orgID,
		Email:          email,
	})
	return err
}

// ResendOrganizationInvite resends an organization invite.
//
// ResendOrganizationInvite example:
//
//	invite, err := cloud.ResendOrganizationInvite(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2", "test@example.com")
//
// For more information, see the [ResendOrganizationInvite method docs].
//
// [ResendOrganizationInvite method docs]: https://docs.viam.com/dev/reference/apis/fleet/#resendorganizationinvite
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
//
// DisableBillingService example:
//
//	err := cloud.DisableBillingService(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [DisableBillingService method docs].
//
// [DisableBillingService method docs]: https://docs.viam.com/dev/reference/apis/fleet/#disablebillingservice
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
//
// OrganizationSetSupportEmail example:
//
//	err := cloud.OrganizationSetSupportEmail(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2", "test@example.com")
//
// For more information, see the [OrganizationSetSupportEmail method docs].
//
// [OrganizationSetSupportEmail method docs]: https://docs.viam.com/dev/reference/apis/fleet/#organizationsetsupportemail
func (c *AppClient) OrganizationSetSupportEmail(ctx context.Context, orgID, email string) error {
	_, err := c.client.OrganizationSetSupportEmail(ctx, &pb.OrganizationSetSupportEmailRequest{
		OrgId: orgID,
		Email: email,
	})
	return err
}

// OrganizationGetSupportEmail gets an organization's support email.
//
// OrganizationGetSupportEmail example:
//
//	email, err := cloud.OrganizationGetSupportEmail(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [OrganizationGetSupportEmail method docs].
//
// [OrganizationGetSupportEmail method docs]: https://docs.viam.com/dev/reference/apis/fleet/#organizationgetsupportemail
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
//
// GetBillingServiceConfig example:
//
//	config, err := cloud.GetBillingServiceConfig(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [GetBillingServiceConfig method docs].
//
// [GetBillingServiceConfig method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getbillingserviceconfig
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
//
// OrganizationSetLogo example:
//
//	logoData, err := os.ReadFile("logo.png")
//	err := cloud.OrganizationSetLogo(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		logoData)
//	)
//
// For more information, see the [OrganizationSetLogo method docs].
//
// [OrganizationSetLogo method docs]: https://docs.viam.com/dev/reference/apis/fleet/#organizationsetlogo
func (c *AppClient) OrganizationSetLogo(ctx context.Context, orgID string, logo []byte) error {
	_, err := c.client.OrganizationSetLogo(ctx, &pb.OrganizationSetLogoRequest{
		OrgId: orgID,
		Logo:  logo,
	})
	return err
}

// OrganizationGetLogo gets an organization's logo.
//
// OrganizationGetLogo example:
//
//	logoURL, err := cloud.OrganizationGetLogo(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//	)
//
// For more information, see the [OrganizationGetLogo method docs].
//
// [OrganizationGetLogo method docs]: https://docs.viam.com/dev/reference/apis/fleet/#organizationgetlogo
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
//
// ListOAuthApps example:
//
//	apps, err := cloud.ListOAuthApps(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//	)
//
// For more information, see the [ListOAuthApps method docs].
//
// [ListOAuthApps method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listoauthapps
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
//
// CreateLocation example:
//
//	 locationID := "ab1c2d3e45"
//		err := cloud.CreateLocation(
//			context.Background(),
//			"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//			"test-location",
//			&app.CreateLocationOptions{
//				ParentLocationID: &locationID,
//		})
//
// For more information, see the [CreateLocation method docs].
//
// [CreateLocation method docs]: https://docs.viam.com/dev/reference/apis/fleet/#createlocation
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
//
// GetLocation example:
//
//	location, err := cloud.GetLocation(context.Background(), "ab1c2d3e45")
//
// For more information, see the [GetLocation method docs].
//
// [GetLocation method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getlocation
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
//
// UpdateLocation example:
//
//	 locationID := "ab1c2d3e45"
//	 name := "test-name"
//		err := cloud.UpdateLocation(
//			context.Background(),
//			"ab1c2d3e45",
//			&app.UpdateLocationOptions{
//				Name: &name,
//				ParentLocationID: &locationID,
//			})
//
// For more information, see the [UpdateLocation method docs].
//
// [UpdateLocation method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updatelocation
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
//
// DeleteLocation example:
//
//	err := cloud.DeleteLocation(context.Background(), "ab1c2d3e45")
//
// For more information, see the [DeleteLocation method docs].
//
// [DeleteLocation method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deletelocation
func (c *AppClient) DeleteLocation(ctx context.Context, locationID string) error {
	_, err := c.client.DeleteLocation(ctx, &pb.DeleteLocationRequest{
		LocationId: locationID,
	})
	return err
}

// ListLocations gets a list of locations under the specified organization.
//
// ListLocations example:
//
//	locations, err := cloud.ListLocations(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [ListLocations method docs].
//
// [ListLocations method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listlocations
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
//
// ShareLocation example:
//
//	err := cloud.ShareLocation(context.Background(), "ab1c2d3e45", "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [ShareLocation method docs].
//
// [ShareLocation method docs]: https://docs.viam.com/dev/reference/apis/fleet/#sharelocation
func (c *AppClient) ShareLocation(ctx context.Context, locationID, orgID string) error {
	_, err := c.client.ShareLocation(ctx, &pb.ShareLocationRequest{
		LocationId:     locationID,
		OrganizationId: orgID,
	})
	return err
}

// UnshareLocation stops sharing a location with an organization.
//
// UnshareLocation example:
//
//	err := cloud.UnshareLocation(context.Background(), "ab1c2d3e45", "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [UnshareLocation method docs].
//
// [UnshareLocation method docs]: https://docs.viam.com/dev/reference/apis/fleet/#unsharelocation
func (c *AppClient) UnshareLocation(ctx context.Context, locationID, orgID string) error {
	_, err := c.client.UnshareLocation(ctx, &pb.UnshareLocationRequest{
		LocationId:     locationID,
		OrganizationId: orgID,
	})
	return err
}

// LocationAuth gets a location's authorization secrets.
//
// LocationAuth example:
//
//	auth, err := cloud.LocationAuth(context.Background(), "ab1c2d3e45")
//
// For more information, see the [LocationAuth method docs].
//
// [LocationAuth method docs]: https://docs.viam.com/dev/reference/apis/fleet/#locationauth
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
//
// CreateLocationSecret example:
//
//	auth, err := cloud.CreateLocationSecret(context.Background(), "ab1c2d3e45")
//
// For more information, see the [CreateLocationSecret method docs].
//
// [CreateLocationSecret method docs]: https://docs.viam.com/dev/reference/apis/fleet/#createlocationsecret
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
//
// DeleteLocationSecret example:
//
//	err := cloud.DeleteLocationSecret(
//		context.Background(),
//		"ab1c2d3e45",
//		"a12bcd3e-a12b-1234-1ab2-abc123d4e5f6")
//	)
//
// For more information, see the [DeleteLocationSecret method docs].
//
// [DeleteLocationSecret method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deletelocationsecret
func (c *AppClient) DeleteLocationSecret(ctx context.Context, locationID, secretID string) error {
	_, err := c.client.DeleteLocationSecret(ctx, &pb.DeleteLocationSecretRequest{
		LocationId: locationID,
		SecretId:   secretID,
	})
	return err
}

// GetLocationMetadata gets the user-defined metadata for a location.
//
// GetLocationMetadata example:
//
//	metadata, err := cloud.GetLocationMetadata(context.Background(), "ab1c2d3e45")
//
// For more information, see the [GetLocationMetadata method docs].
//
// [GetLocationMetadata method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getlocationmetadata
func (c *AppClient) GetLocationMetadata(ctx context.Context, locationID string) (map[string]interface{}, error) {
	resp, err := c.client.GetLocationMetadata(ctx, &pb.GetLocationMetadataRequest{
		LocationId: locationID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data.AsMap(), nil
}

// UpdateLocationMetadata updates the user-defined metadata for a location.
//
// UpdateLocationMetadata example:
//
//	err := cloud.UpdateLocationMetadata(
//		context.Background(),
//		"ab1c2d3e45",
//		map[string]interface{}{
//			"key": "value",
//		},
//	)
//
// For more information, see the [UpdateLocationMetadata method docs].
//
// [UpdateLocationMetadata method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updatelocationmetadata
func (c *AppClient) UpdateLocationMetadata(ctx context.Context, locationID string, data interface{}) error {
	d, err := protoutils.StructToStructPb(data)
	if err != nil {
		return err
	}
	_, err = c.client.UpdateLocationMetadata(ctx, &pb.UpdateLocationMetadataRequest{
		LocationId: locationID,
		Data:       d,
	})
	return err
}

// GetRobot gets a specific robot by ID.
//
// GetRobot example:
//
//	robot, err := cloud.GetRobot(context.Background(), "1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [GetRobot method docs].
//
// [GetRobot method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getrobot
func (c *AppClient) GetRobot(ctx context.Context, id string) (*Robot, error) {
	resp, err := c.client.GetRobot(ctx, &pb.GetRobotRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	return robotFromProto(resp.Robot), nil
}

// GetRobotMetadata gets the user-defined metadata for a robot.
//
// GetRobotMetadata example:
//
//	metadata, err := cloud.GetRobotMetadata(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [GetRobotMetadata method docs].
//
// [GetRobotMetadata method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getrobotmetadata
func (c *AppClient) GetRobotMetadata(ctx context.Context, robotID string) (map[string]interface{}, error) {
	resp, err := c.client.GetRobotMetadata(ctx, &pb.GetRobotMetadataRequest{
		Id: robotID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data.AsMap(), nil
}

// UpdateRobotMetadata updates the user-defined metadata for a robot.
//
// UpdateRobotMetadata example:
//
//	err := cloud.UpdateRobotMetadata(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12",
//		map[string]interface{}{
//			"key": "value",
//		},
//	)
//
// For more information, see the [UpdateRobotMetadata method docs].
//
// [UpdateRobotMetadata method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updaterobotmetadata
func (c *AppClient) UpdateRobotMetadata(ctx context.Context, robotID string, data interface{}) error {
	d, err := protoutils.StructToStructPb(data)
	if err != nil {
		return err
	}
	_, err = c.client.UpdateRobotMetadata(ctx, &pb.UpdateRobotMetadataRequest{
		Id:   robotID,
		Data: d,
	})
	return err
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
//
// GetRobotParts example:
//
//	parts, err := cloud.GetRobotParts(context.Background(), "1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [GetRobotParts method docs].
//
// [GetRobotParts method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getrobotparts
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
//
// GetRobotPart example:
//
//	part, config, err := cloud.GetRobotPart(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12",
//	)
//
// For more information, see the [GetRobotPart method docs].
//
// [GetRobotPart method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getrobotpart
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
//
// GetRobotPartLogs example:
//
//		filter := ""
//		pageToken := ""
//		startTime := time.Now().Add(-720 * time.Hour)
//		endTime := time.Now()
//		limit := 5
//		source := ""
//		partLogs, _, err := cloud.GetRobotPartLogs(
//	        ctx,
//	        PART_ID,
//	       &GetRobotPartLogsOptions{
//				Filter: &filter,
//				PageToken: &pageToken,
//				Levels: []string{"INFO", "WARN", "ERROR"},
//				Start: &startTime,
//				End: &endTime,
//				Limit: &limit,
//				Source: &source,
//			},
//		)
//
// For more information, see the [GetRobotPartLogs method docs].
//
// [GetRobotPartLogs method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getrobotpartlogs
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
//
// TailRobotPartLogs example:
//
//	logFilter := "error"
//	stream, err := cloud.TailRobotPartLogs(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12",
//		true,
//		&app.TailRobotPartLogsOptions{
//			Filter: &logFilter,
//		},
//	)
//
// For more information, see the [TailRobotPartLogs method docs].
//
// [TailRobotPartLogs method docs]: https://docs.viam.com/dev/reference/apis/fleet/#tailrobotpartlogs
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
//
// GetRobotPartHistory example:
//
//	history, err := cloud.GetRobotPartHistory(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [GetRobotPartHistory method docs].
//
// [GetRobotPartHistory method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getrobotparthistory
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
//
// UpdateRobotPart example:
//
//	robotConfig := map[string]interface{}{
//		"components": []map[string]interface{}{
//			{
//				"name":       "camera-1",
//				"api":        "rdk:component:camera",
//				"model":      "rdk:builtin:fake",
//				"attributes": map[string]interface{}{},
//			},
//		},
//	}
//
//	part, err := cloud.UpdateRobotPart(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12",
//		"part_name",
//		map[string]interface{}{
//			"key": "value",
//		},
//	)
//
// For more information, see the [UpdateRobotPart method docs].
//
// [UpdateRobotPart method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updaterobotpart
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
//
// NewRobotPart example:
//
//	partID, err := cloud.NewRobotPart(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12",
//		"part_name")
//
// For more information, see the [NewRobotPart method docs].
//
// [NewRobotPart method docs]: https://docs.viam.com/dev/reference/apis/fleet/#newrobotpart
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
//
// DeleteRobotPart example:
//
//	err := cloud.DeleteRobotPart(context.Background(), "1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [DeleteRobotPart method docs].
//
// [DeleteRobotPart method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deleterobotpart
func (c *AppClient) DeleteRobotPart(ctx context.Context, partID string) error {
	_, err := c.client.DeleteRobotPart(ctx, &pb.DeleteRobotPartRequest{
		PartId: partID,
	})
	return err
}

// GetRobotPartMetadata gets the user-defined metadata for a robot part.
//
// GetRobotPartMetadata example:
//
//	metadata, err := cloud.GetRobotPartMetadata(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [GetRobotPartMetadata method docs].
//
// [GetRobotPartMetadata method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getrobotpartmetadata
func (c *AppClient) GetRobotPartMetadata(ctx context.Context, robotID string) (map[string]interface{}, error) {
	resp, err := c.client.GetRobotPartMetadata(ctx, &pb.GetRobotPartMetadataRequest{
		Id: robotID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data.AsMap(), nil
}

// UpdateRobotPartMetadata updates the user-defined metadata for a robot part.
//
// UpdateRobotPartMetadata example:
//
//	err := cloud.UpdateRobotPartMetadata(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12",
//		map[string]interface{}{
//			"key": "value",
//		},
//	)
//
// For more information, see the [UpdateRobotPartMetadata method docs].
//
// [UpdateRobotPartMetadata method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updaterobotpartmetadata
func (c *AppClient) UpdateRobotPartMetadata(ctx context.Context, robotID string, data interface{}) error {
	d, err := protoutils.StructToStructPb(data)
	if err != nil {
		return err
	}
	_, err = c.client.UpdateRobotPartMetadata(ctx, &pb.UpdateRobotPartMetadataRequest{
		Id:   robotID,
		Data: d,
	})
	return err
}

// GetRobotAPIKeys gets the robot API keys for the robot.
//
// GetRobotAPIKeys example:
//
//	keys, err := cloud.GetRobotAPIKeys(context.Background(), "1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [GetRobotAPIKeys method docs].
//
// [GetRobotAPIKeys method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getrobotapikeys
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
//
// MarkPartAsMain example:
//
//	err := cloud.MarkPartAsMain(context.Background(), "1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [MarkPartAsMain method docs].
//
// [MarkPartAsMain method docs]: https://docs.viam.com/dev/reference/apis/fleet/#markpartasmain
func (c *AppClient) MarkPartAsMain(ctx context.Context, partID string) error {
	_, err := c.client.MarkPartAsMain(ctx, &pb.MarkPartAsMainRequest{
		PartId: partID,
	})
	return err
}

// MarkPartForRestart marks the given part for restart.
// Once the robot part checks-in with the app the flag is reset on the robot part.
// Calling this multiple times before a robot part checks-in has no effect.
//
// MarkPartForRestart example:
//
//	err := cloud.MarkPartForRestart(context.Background(), "1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [MarkPartForRestart method docs].
//
// [MarkPartForRestart method docs]: https://docs.viam.com/dev/reference/apis/fleet/#markpartforrestart
func (c *AppClient) MarkPartForRestart(ctx context.Context, partID string) error {
	_, err := c.client.MarkPartForRestart(ctx, &pb.MarkPartForRestartRequest{
		PartId: partID,
	})
	return err
}

// CreateRobotPartSecret creates a new generated secret in the robot part.
// Succeeds if there are no more than 2 active secrets after creation.
//
// CreateRobotPartSecret example:
//
//	part, err := cloud.CreateRobotPartSecret(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [CreateRobotPartSecret method docs].
//
// [CreateRobotPartSecret method docs]: https://docs.viam.com/dev/reference/apis/fleet/#createrobotpartsecret
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
//
// DeleteRobotPartSecret example:
//
//	err := cloud.DeleteRobotPartSecret(
//		context.Background(),
//		"1ab2345c-a123-1ab2-1abc-1ab234567a12",
//		"a12bcd34-1234-12ab-1ab2-123a4567890b")
//
// For more information, see the [DeleteRobotPartSecret method docs].
//
// [DeleteRobotPartSecret method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deleterobotpartsecret
func (c *AppClient) DeleteRobotPartSecret(ctx context.Context, partID, secretID string) error {
	_, err := c.client.DeleteRobotPartSecret(ctx, &pb.DeleteRobotPartSecretRequest{
		PartId:   partID,
		SecretId: secretID,
	})
	return err
}

// ListRobots gets a list of robots under a location.
//
// ListRobots example:
//
//	robots, err := cloud.ListRobots(context.Background(), "ab1c2d3e45")
//
// For more information, see the [ListRobots method docs].
//
// [ListRobots method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listrobots
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
//
// NewRobot example:
//
//	robotID, err := cloud.NewRobot(context.Background(), "robot_name", "ab1c2d3e45")
//
// For more information, see the [NewRobot method docs].
//
// [NewRobot method docs]: https://docs.viam.com/dev/reference/apis/fleet/#newrobot
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
//
// UpdateRobot example:
//
//	robot, err := cloud.UpdateRobot(context.Background(), "1ab2345c-a123-1ab2-1abc-1ab234567a12", "robot_name", "ab1c2d3e45")
//
// For more information, see the [UpdateRobot method docs].
//
// [UpdateRobot method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updaterobot
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
//
// DeleteRobot example:
//
//	err := cloud.DeleteRobot(context.Background(), "1ab2345c-a123-1ab2-1abc-1ab234567a12")
//
// For more information, see the [DeleteRobot method docs].
//
// [DeleteRobot method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deleterobot
func (c *AppClient) DeleteRobot(ctx context.Context, id string) error {
	_, err := c.client.DeleteRobot(ctx, &pb.DeleteRobotRequest{
		Id: id,
	})
	return err
}

// ListFragments gets a list of fragments.
//
// ListFragments example:
//
//	fragments, err := cloud.ListFragments(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		true,
//		[]app.FragmentVisibility{app.FragmentVisibilityPublic},
//	)
//
// For more information, see the [ListFragments method docs].
//
// [ListFragments method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listfragments
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
//
// GetFragment example:
//
//	fragment, err := cloud.GetFragment(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2", "")
//
// For more information, see the [GetFragment method docs].
//
// [GetFragment method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getfragment
func (c *AppClient) GetFragment(ctx context.Context, id, version string) (*Fragment, error) {
	req := &pb.GetFragmentRequest{
		Id: id,
	}
	if version != "" {
		req.Version = &version
	}

	resp, err := c.client.GetFragment(ctx, req)
	if err != nil {
		return nil, err
	}
	return fragmentFromProto(resp.Fragment), nil
}

// CreateFragment creates a fragment.
//
// CreateFragment example:
//
//	fragmentConfig := map[string]interface{}{
//		"components": []map[string]interface{}{
//			{
//				"name":       "camera-1",
//				"api":        "rdk:component:camera",
//				"model":      "rdk:builtin:fake",
//				"attributes": map[string]interface{}{},
//			},
//		},
//	}
//
//	fragment, err := cloud.CreateFragment(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		"My Fragment",
//		fragmentConfig,
//		&app.CreateFragmentOptions{Visibility: &app.FragmentVisibilityPublic},
//	)
//
// For more information, see the [CreateFragment method docs].
//
// [CreateFragment method docs]: https://docs.viam.com/dev/reference/apis/fleet/#createfragment
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
//
// UpdateFragment example:
//
//	fragmentConfig := map[string]interface{}{
//		"components": []map[string]interface{}{
//			{
//				"name":       "camera-1",
//				"api":        "rdk:component:camera",
//				"model":      "rdk:builtin:fake",
//				"attributes": map[string]interface{}{},
//			},
//		},
//	}
//
//	fragment, err := cloud.UpdateFragment(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		"My Fragment",
//		fragmentConfig,
//		&app.UpdateFragmentOptions{Visibility: &app.FragmentVisibilityPublic})
//
// For more information, see the [UpdateFragment method docs].
//
// [UpdateFragment method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updatefragment
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
//
// DeleteFragment example:
//
//	err := cloud.DeleteFragment(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [DeleteFragment method docs].
//
// [DeleteFragment method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deletefragment
func (c *AppClient) DeleteFragment(ctx context.Context, id string) error {
	_, err := c.client.DeleteFragment(ctx, &pb.DeleteFragmentRequest{
		Id: id,
	})
	return err
}

// ListMachineFragments gets top level and nested fragments for a machine, as well as any other fragments specified by IDs.
// Additional fragments are useful to view fragments that will be provisionally added to the machine alongside existing fragments.
//
// ListMachineFragments example:
//
//	fragments, err := cloud.ListMachineFragments(context.Background(), "1ab2345c-a123-1ab2-1abc-1ab234567a12", []string{})
//
// For more information, see the [ListMachineFragments method docs].
//
// [ListMachineFragments method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listmachinefragments
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
//
// GetFragmentHistory example:
//
//	 limit := 10
//		history, token, err := cloud.GetFragmentHistory(
//			context.Background(),
//			"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//			&app.GetFragmentHistoryOptions{PageLimit: &limit})
//
// For more information, see the [GetFragmentHistory method docs].
//
// [GetFragmentHistory method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getfragmenthistory
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
//
// ListAuthorizations example:
//
//	err := cloud.ListAuthorizations(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		[]string{"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2"},
//	)
//
// For more information, see the [ListAuthorizations method docs].
//
// [ListAuthorizations method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listauthorizations
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
//
// CheckPermissions example:
//
//	err := cloud.CheckPermissions(
//		context.Background(),
//		[]*app.AuthorizedPermissions{
//			{
//				ResourceType: app.AuthResourceTypeLocation,
//				ResourceID:   "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//				Permissions:  []string{"control_robot", "read_robot_logs"},
//			},
//		},
//	)
//
// For more information, see the [CheckPermissions method docs].
//
// [CheckPermissions method docs]: https://docs.viam.com/dev/reference/apis/fleet/#checkpermissions
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
//
// GetRegistryItem example:
//
//	registryItem, err := cloud.GetRegistryItem(context.Background(), "namespace:name")
//
// For more information, see the [GetRegistryItem method docs].
//
// [GetRegistryItem method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getregistryitem
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
//
// CreateRegistryItem example:
//
//	err := cloud.CreateRegistryItem(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		"registry_item_name",
//		app.PackageTypeMLModel)
//
// For more information, see the [CreateRegistryItem method docs].
//
// [CreateRegistryItem method docs]: https://docs.viam.com/dev/reference/apis/fleet/#createregistryitem
func (c *AppClient) CreateRegistryItem(ctx context.Context, orgID, name string, packageType PackageType) error {
	_, err := c.client.CreateRegistryItem(ctx, &pb.CreateRegistryItemRequest{
		OrganizationId: orgID,
		Name:           name,
		Type:           packageTypeToProto(packageType),
	})
	return err
}

// UpdateRegistryItem updates a registry item.
//
// UpdateRegistryItem example:
//
//	siteURL := "https://example.com"
//	err := cloud.UpdateRegistryItem(
//		context.Background(),
//		"namespace:name",
//		app.PackageTypeMLModel,
//		"description",
//		app.VisibilityPrivate,
//		&app.UpdateRegistryItemOptions{URL: &siteURL},
//	)
//
// For more information, see the [UpdateRegistryItem method docs].
//
// [UpdateRegistryItem method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updateregistryitem
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
//
// ListRegistryItems example:
//
//	organizationID := "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2"
//	searchTerm := ""
//	pageToken := ""
//	namespaces := []string{}
//	items, err := cloud.ListRegistryItems(
//		context.Background(),
//		&organizationID,
//		[]app.PackageType{app.PackageTypeModule},
//		[]app.Visibility{app.VisibilityPublic},
//		[]string{"linux/any"},
//		[]app.RegistryItemStatus{app.RegistryItemStatusPublished},
//		&app.ListRegistryItemsOptions{
//			SearchTerm: &searchTerm,
//			PageToken: &pageToken,
//			PublicNamespaces: namespaces,
//		},
//	)
//
// For more information, see the [ListRegistryItems method docs].
//
// [ListRegistryItems method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listregistryitems
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

// DeleteRegistryItem deletes a registry item given an ID that is formatted as `prefix:name"
// where `prefix" is the owner's organization ID or namespace.
//
// DeleteRegistryItem example:
//
//	err := cloud.DeleteRegistryItem(context.Background(), "namespace:name")
//
// For more information, see the [DeleteRegistryItem method docs].
//
// [DeleteRegistryItem method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deleteregistryitem
func (c *AppClient) DeleteRegistryItem(ctx context.Context, itemID string) error {
	_, err := c.client.DeleteRegistryItem(ctx, &pb.DeleteRegistryItemRequest{
		ItemId: itemID,
	})
	return err
}

// TransferRegistryItem transfers a registry item to a namespace.
//
// TransferRegistryItem example:
//
//	err := cloud.TransferRegistryItem(context.Background(), "namespace:name", "new_namespace")
//
// For more information, see the [TransferRegistryItem method docs].
//
// [TransferRegistryItem method docs]: https://docs.viam.com/dev/reference/apis/fleet/#transferregistryitem
func (c *AppClient) TransferRegistryItem(ctx context.Context, itemID, newPublicNamespace string) error {
	_, err := c.client.TransferRegistryItem(ctx, &pb.TransferRegistryItemRequest{
		ItemId:             itemID,
		NewPublicNamespace: newPublicNamespace,
	})
	return err
}

// CreateModule creates a module and returns its ID and URL.
//
// CreateModule example:
//
//	moduleID, url, err := cloud.CreateModule(
//		context.Background(),
//		"a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2",
//		"module_name",
//	)
//
// For more information, see the [CreateModule method docs].
//
// [CreateModule method docs]: https://docs.viam.com/dev/reference/apis/fleet/#createmodule
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
//
// UpdateModule example:
//
//	model := &app.Model{
//		API:   "rdk:service:generic",
//		Model: "docs-test:new_test_module:test_model",
//	}
//	app := &app.App{
//		Name:       "app_name",
//		Type:       "app_type",
//		Entrypoint: "entrypoint",
//	}
//	firstRun := "first_run.sh"
//	url, err := cloud.UpdateModule(
//		context.Background(),
//		"namespace:name",
//		app.VisibilityPublic,
//		"https://example.com",
//		"description",
//		[]*app.Model{model},
//		[]*app.App{app},
//		"entrypoint",
//		&app.UpdateModuleOptions{FirstRun: &firstRun},
//	)
//
// For more information, see the [UpdateModule method docs].
//
// [UpdateModule method docs]: https://docs.viam.com/dev/reference/apis/fleet/#updatemodule
func (c *AppClient) UpdateModule(
	ctx context.Context,
	moduleID string,
	visibility Visibility,
	url,
	description string,
	models []*Model,
	apps []*App,
	entrypoint string,
	opts *UpdateModuleOptions,
) (string, error) {
	var pbModels []*pb.Model
	for _, model := range models {
		pbModels = append(pbModels, modelToProto(model))
	}
	var pbApps []*pb.App
	for _, app := range apps {
		pbApps = append(pbApps, appToProto(app))
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
		Apps:        pbApps,
		Entrypoint:  entrypoint,
		FirstRun:    firstRun,
	})
	if err != nil {
		return "", err
	}
	return resp.Url, nil
}

// UploadModuleFile uploads a module file and returns the URL of the uploaded file.
//
// UploadModuleFile example:
//
//	moduleFileInfo := app.ModuleFileInfo{
//		ModuleID: "namespace:name",
//		Version:  "1.0.0",
//		Platform: "darwin/arm64",
//	}
//	fileURL, err := cloud.UploadModuleFile(context.Background(), fileInfo, []byte("empty.txt"))
//
// For more information, see the [UploadModuleFile method docs].
//
// [UploadModuleFile method docs]: https://docs.viam.com/dev/reference/apis/fleet/#uploadmodulefile
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
//
// GetModule example:
//
//	module, err := cloud.GetModule(context.Background(), "namespace:name")
//
// For more information, see the [GetModule method docs].
//
// [GetModule method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getmodule
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
//
// ListModules example:
//
//	orgID := "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2"
//	modules, err := cloud.ListModules(context.Background(), &app.ListModulesOptions{OrgID: &orgID})
//
// For more information, see the [ListModules method docs].
//
// [ListModules method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listmodules
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
//
// DeleteKey example:
//
//	err := cloud.DeleteKey(context.Background(), "a1bcdefghi2jklmnopqrstuvw3xyzabc")
//
// For more information, see the [DeleteKey method docs].
//
// [DeleteKey method docs]: https://docs.viam.com/dev/reference/apis/fleet/#deletekey
func (c *AppClient) DeleteKey(ctx context.Context, id string) error {
	_, err := c.client.DeleteKey(ctx, &pb.DeleteKeyRequest{
		Id: id,
	})
	return err
}

// ListKeys example:
//
//	keys, err := cloud.ListKeys(context.Background(), "a1b2c345-abcd-1a2b-abc1-a1b23cd4561e2")
//
// For more information, see the [ListKeys method docs].
//
// [ListKeys method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listkeys
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
//
// RenameKey example:
//
//	_, name, err := cloud.RenameKey(context.Background(), "a1bcdefghi2jklmnopqrstuvw3xyzabc", "new_name")
//
// For more information, see the [RenameKey method docs].
//
// [RenameKey method docs]: https://docs.viam.com/dev/reference/apis/fleet/#renamekey
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
//
// RotateKey example:
//
//	id, key, err := cloud.RotateKey(context.Background(), "a1bcdefghi2jklmnopqrstuvw3xyzabc")
//
// For more information, see the [RotateKey method docs].
//
// [RotateKey method docs]: https://docs.viam.com/dev/reference/apis/fleet/#rotatekey
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
//
// CreateKeyFromExistingKeyAuthorizations example:
//
//	id, key, err := cloud.CreateKeyFromExistingKeyAuthorizations(context.Background(), "a1bcdefghi2jklmnopqrstuvw3xyzabc")
//
// For more information, see the [CreateKeyFromExistingKeyAuthorizations method docs].
//
// [CreateKeyFromExistingKeyAuthorizations method docs]: https://docs.viam.com/dev/reference/apis/fleet/#createkeyfromexistingkeyauthorizations
//
//nolint:lll
func (c *AppClient) CreateKeyFromExistingKeyAuthorizations(ctx context.Context, id string) (string, string, error) {
	resp, err := c.client.CreateKeyFromExistingKeyAuthorizations(ctx, &pb.CreateKeyFromExistingKeyAuthorizationsRequest{
		Id: id,
	})
	if err != nil {
		return "", "", err
	}
	return resp.Id, resp.Key, nil
}

// ListMachineSummaries lists machine summaries, optionally limited, under an organization, organized by provided location ID's.
//
// ListMachineSummaries example:
//
//	 summaries, err := cloud.ListMachineSummaries(
//			context.Background(),
//		   	"a1bcdefghi2jklmnopqrstuvw3xyzabc",
//		   	[]string{locationID},
//		   	[]string{fragmetnID},
//		  	0,
//	 )
//
// For more information, see the [ListMachineSummaries method docs].
//
// [ListMachineSummaries method docs]: https://docs.viam.com/dev/reference/apis/fleet/#listmachinesummaries
func (c *AppClient) ListMachineSummaries(
	ctx context.Context,
	organizationID string,
	fragmentIDs, locationIDs []string,
	limit int32,
) ([]*LocationSummary, error) {
	req := &pb.ListMachineSummariesRequest{
		OrganizationId: organizationID,
		FragmentIds:    fragmentIDs,
		LocationIds:    locationIDs,
		Limit:          &limit,
	}

	resp, err := c.client.ListMachineSummaries(ctx, req)
	if err != nil {
		return nil, err
	}

	summaries := make([]*LocationSummary, 0, len(resp.LocationSummaries))
	for _, LocationSummary := range resp.LocationSummaries {
		summaries = append(summaries, locationSummaryFromProto(LocationSummary))
	}

	return summaries, nil
}

// GetAppBranding gets the branding for an app.
//
// GetAppBranding example:
//
//	branding, err := cloud.GetAppBranding(context.Background(), "my-org", "my-app")
//
// For more information, see the [GetAppBranding method docs].
//
// [GetAppBranding method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getappbranding
func (c *AppClient) GetAppBranding(ctx context.Context, orgPublicNamespace, appName string) (*AppBranding, error) {
	req := &pb.GetAppBrandingRequest{
		PublicNamespace: orgPublicNamespace,
		Name:            appName,
	}

	resp, err := c.client.GetAppBranding(ctx, req)
	if err != nil {
		return nil, err
	}

	return appBrandingFromProto(resp), nil
}

// GetAppContent gets the content for an app.
//
// GetAppContent example:
//
//	content, err := cloud.GetAppContent(context.Background(), "my-org", "my-app")
//
// For more information, see the [GetAppContent method docs].
//
// [GetAppContent method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getappcontent
func (c *AppClient) GetAppContent(ctx context.Context, orgPublicNamespace, appName string) (*AppContent, error) {
	req := &pb.GetAppContentRequest{
		PublicNamespace: orgPublicNamespace,
		Name:            appName,
	}

	resp, err := c.client.GetAppContent(ctx, req)
	if err != nil {
		return nil, err
	}

	return appContentFromProto(resp), nil
}

// GetRobotPartByNameAndLocation gets a robot part by name and location.
//
// GetRobotPartByNameAndLocation example:
//
//	robotPart, err := cloud.GetRobotPartByNameAndLocation(context.Background(), "my-robot-main", "ab1c2d3e45")
//
// For more information, see the [GetRobotPartByNameAndLocation method docs].
//
// [GetRobotPartByNameAndLocation method docs]: https://docs.viam.com/dev/reference/apis/fleet/#getrobotpartbynameandlocation
func (c *AppClient) GetRobotPartByNameAndLocation(ctx context.Context, name, locationID string) (*RobotPart, error) {
	req := &pb.GetRobotPartByNameAndLocationRequest{
		Name:       name,
		LocationId: locationID,
	}

	resp, err := c.client.GetRobotPartByNameAndLocation(ctx, req)
	if err != nil {
		return nil, err
	}

	return robotPartFromProto(resp.GetPart()), nil
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
	var apps []*App
	for _, app := range md.Apps {
		apps = append(apps, appFromProto(app))
	}
	var versions []*ModuleVersion
	for _, version := range md.Versions {
		versions = append(versions, moduleVersionFromProto(version))
	}
	return &ModuleMetadata{
		Models:     models,
		Apps:       apps,
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

func appFromProto(app *pb.App) *App {
	if app == nil {
		return nil
	}
	return &App{
		Name:       app.Name,
		Type:       app.Type,
		Entrypoint: app.Entrypoint,
	}
}

func appToProto(app *App) *pb.App {
	if app == nil {
		return nil
	}
	return &pb.App{
		Name:       app.Name,
		Type:       app.Type,
		Entrypoint: app.Entrypoint,
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
	var apps []*App
	for _, app := range version.Apps {
		apps = append(apps, appFromProto(app))
	}
	return &ModuleVersion{
		Version:    version.Version,
		Files:      files,
		Models:     models,
		Apps:       apps,
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
	var versions []*MLModelVersion
	for _, version := range md.DetailedVersions {
		versions = append(versions, mlModelVersionFromProto(version))
	}
	return &MLModelMetadata{
		Versions:       versions,
		ModelType:      modelTypeFromProto(md.ModelType),
		ModelFramework: modelFrameworkFromProto(md.ModelFramework),
	}
}

func mlModelVersionFromProto(version *pb.MLModelVersion) *MLModelVersion {
	if version == nil {
		return nil
	}
	var createdOn *time.Time
	if version.CreatedOn != nil {
		t := version.CreatedOn.AsTime()
		createdOn = &t
	}
	return &MLModelVersion{
		Version:   version.Version,
		CreatedOn: createdOn,
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
	var apps []*App
	for _, app := range module.Apps {
		apps = append(apps, appFromProto(app))
	}
	return &Module{
		ModuleID:               module.ModuleId,
		Name:                   module.Name,
		Visibility:             visibilityFromProto(module.Visibility),
		Versions:               versions,
		URL:                    module.Url,
		Description:            module.Description,
		Models:                 models,
		Apps:                   apps,
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
	var apps []*App
	for _, app := range history.Apps {
		apps = append(apps, appFromProto(app))
	}
	return &VersionHistory{
		Version:    history.Version,
		Files:      files,
		Models:     models,
		Apps:       apps,
		Entrypoint: history.Entrypoint,
		FirstRun:   history.FirstRun,
	}
}

func locationSummaryFromProto(locationSummary *pb.LocationSummary) *LocationSummary {
	machineSummaries := locationSummary.GetMachineSummaries()
	machines := make([]*MachineSummary, 0, len(machineSummaries))
	for _, machineSummary := range machineSummaries {
		machines = append(machines, machineSummaryFromProto(machineSummary))
	}

	return &LocationSummary{
		LocationID:       locationSummary.GetLocationId(),
		LocationName:     locationSummary.GetLocationName(),
		MachineSummaries: machines,
	}
}

func machineSummaryFromProto(machineSummary *pb.MachineSummary) *MachineSummary {
	return &MachineSummary{
		MachineID:   machineSummary.GetMachineId(),
		MachineName: machineSummary.GetMachineName(),
	}
}

func appBrandingFromProto(resp *pb.GetAppBrandingResponse) *AppBranding {
	var logoPath string
	if resp.LogoPath != nil {
		logoPath = resp.GetLogoPath()
	}

	textCustomizations := make(map[string]TextOverrides, len(resp.TextCustomizations))
	for k, v := range resp.GetTextCustomizations() {
		textCustomizations[k] = TextOverrides{
			Fields: v.GetFields(),
		}
	}

	return &AppBranding{
		LogoPath:           logoPath,
		TextCustomizations: textCustomizations,
		FragmentIDs:        resp.GetFragmentIds(),
	}
}

func appContentFromProto(resp *pb.GetAppContentResponse) *AppContent {
	return &AppContent{
		BlobPath:   resp.GetBlobPath(),
		Entrypoint: resp.GetEntrypoint(),
		AppType:    int(resp.GetAppType()),
		Public:     resp.GetPublic(),
	}
}
