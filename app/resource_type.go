package app

import (
	pb "go.viam.com/api/app/v1"
	common "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Organization holds the information of an organization.
type Organization struct {
	ID              string
	Name            string
	CreatedOn       *timestamppb.Timestamp
	PublicNamespace string
	DefaultRegion   string
	Cid             *string
}

func organizationFromProto(organization *pb.Organization) *Organization {
	return &Organization{
		ID:              organization.Id,
		Name:            organization.Name,
		CreatedOn:       organization.CreatedOn,
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
	DateAdded *timestamppb.Timestamp
	LastLogin *timestamppb.Timestamp
}

func organizationMemberFromProto(organizationMemOrganizationMember *pb.OrganizationMember) *OrganizationMember {
	return &OrganizationMember{
		UserID:    organizationMemOrganizationMember.UserId,
		Emails:    organizationMemOrganizationMember.Emails,
		DateAdded: organizationMemOrganizationMember.DateAdded,
		LastLogin: organizationMemOrganizationMember.LastLogin,
	}
}

// OrganizationInvite is the invite to an organization.
type OrganizationInvite struct {
	OrganizationID string
	Email          string
	CreatedOn      *timestamppb.Timestamp
	Authorizations []*Authorization
}

func organizationInviteFromProto(organizationInvite *pb.OrganizationInvite) *OrganizationInvite {
	var authorizations []*Authorization
	for _, authorization := range organizationInvite.Authorizations {
		authorizations = append(authorizations, authorizationFromProto(authorization))
	}
	return &OrganizationInvite{
		OrganizationID: organizationInvite.OrganizationId,
		Email:          organizationInvite.Email,
		CreatedOn:      organizationInvite.CreatedOn,
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
	CreatedOn        *timestamppb.Timestamp
	RobotCount       int32
	Config           *StorageConfig
}

func locationFromProto(location *pb.Location) *Location {
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
		CreatedOn:        location.CreatedOn,
		RobotCount:       location.RobotCount,
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
	LastAccess *timestamppb.Timestamp
	CreatedOn  *timestamppb.Timestamp
}

func robotFromProto(robot *pb.Robot) *Robot {
	return &Robot{
		ID:         robot.Id,
		Name:       robot.Name,
		Location:   robot.Location,
		LastAccess: robot.LastAccess,
		CreatedOn:  robot.CreatedOn,
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
	LastAccess       *timestamppb.Timestamp
	UserSuppliedInfo *map[string]interface{}
	MainPart         bool
	FQDN             string
	LocalFQDN        string
	CreatedOn        *timestamppb.Timestamp
	Secrets          []*SharedSecret
	LastUpdated      *timestamppb.Timestamp
}

func robotPartFromProto(robotPart *pb.RobotPart) *RobotPart {
	var secrets []*SharedSecret
	for _, secret := range robotPart.Secrets {
		secrets = append(secrets, sharedSecretFromProto(secret))
	}
	cfg := robotPart.RobotConfig.AsMap()
	info := robotPart.UserSuppliedInfo.AsMap()
	return &RobotPart{
		ID:               robotPart.Id,
		Name:             robotPart.Name,
		DNSName:          robotPart.DnsName,
		Secret:           robotPart.Secret,
		Robot:            robotPart.Robot,
		LocationID:       robotPart.LocationId,
		RobotConfig:      &cfg,
		LastAccess:       robotPart.LastAccess,
		UserSuppliedInfo: &info,
		MainPart:         robotPart.MainPart,
		FQDN:             robotPart.Fqdn,
		LocalFQDN:        robotPart.LocalFqdn,
		CreatedOn:        robotPart.CreatedOn,
		Secrets:          secrets,
		LastUpdated:      robotPart.LastUpdated,
	}
}

// RobotPartHistoryEntry is a history entry of a robot part.
type RobotPartHistoryEntry struct {
	Part     string
	Robot    string
	When     *timestamppb.Timestamp
	Old      *RobotPart
	EditedBy *AuthenticatorInfo
}

func robotPartHistoryEntryFromProto(entry *pb.RobotPartHistoryEntry) *RobotPartHistoryEntry {
	return &RobotPartHistoryEntry{
		Part:     entry.Part,
		Robot:    entry.Robot,
		When:     entry.When,
		Old:      robotPartFromProto(entry.Old),
		EditedBy: authenticatorInfoFromProto(entry.EditedBy),
	}
}

// LogEntry holds the information of a single log entry.
type LogEntry struct {
	Host       string
	Level      string
	Time       *timestamppb.Timestamp
	LoggerName string
	Message    string
	Caller     *map[string]interface{}
	Stack      string
	Fields     []*map[string]interface{}
}

func logEntryFromProto(logEntry *common.LogEntry) *LogEntry {
	var fields []*map[string]interface{}
	for _, field := range logEntry.Fields {
		f := field.AsMap()
		fields = append(fields, &f)
	}
	caller := logEntry.Caller.AsMap()
	return &LogEntry{
		Host:       logEntry.Host,
		Level:      logEntry.Level,
		Time:       logEntry.Time,
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
	CreatedOn         *timestamppb.Timestamp
	OrganizationName  string
	RobotPartCount    int32
	OrganizationCount int32
	OnlyUsedByOwner   bool
	Visibility        FragmentVisibility
	LastUpdated       *timestamppb.Timestamp
}

func fragmentFromProto(fragment *pb.Fragment) *Fragment {
	f := fragment.Fragment.AsMap()
	return &Fragment{
		ID:                fragment.Id,
		Name:              fragment.Name,
		Fragment:          &f,
		OrganizationOwner: fragment.OrganizationOwner,
		Public:            fragment.Public,
		CreatedOn:         fragment.CreatedOn,
		OrganizationName:  fragment.OrganizationName,
		RobotPartCount:    fragment.RobotPartCount,
		OrganizationCount: fragment.OrganizationCount,
		OnlyUsedByOwner:   fragment.OnlyUsedByOwner,
		Visibility:        fragmentVisibilityFromProto(fragment.Visibility),
		LastUpdated:       fragment.LastUpdated,
	}
}

// FragmentVisibility specifies the kind of visibility a fragment has.
type FragmentVisibility int32

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
	EditedOn *timestamppb.Timestamp
	Old      *Fragment
	EditedBy *AuthenticatorInfo
}

func fragmentHistoryEntryFromProto(entry *pb.FragmentHistoryEntry) *FragmentHistoryEntry {
	return &FragmentHistoryEntry{
		Fragment: entry.Fragment,
		EditedOn: entry.EditedOn,
		Old:      fragmentFromProto(entry.Old),
		EditedBy: authenticatorInfoFromProto(entry.EditedBy),
	}
}
