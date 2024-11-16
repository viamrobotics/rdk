package app

import (
	"context"
	"testing"

	pb "go.viam.com/api/app/v1"
	common "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/testutils/inject"
)

const (
	organizationID    = "organization_id"
	email             = "email"
	userID            = "user_id"
	locationID        = "location_id"
	available         = true
	authorizationType = "authorization_type"
	authorizationID   = "authorization_id"
	resourceType      = "resource_type"
	resourceID        = "resource_id"
	identityID        = "identity_id"
	identityType      = "identity_type"
	secretID          = "secret_ids"
	primary           = true
	robotCount        = 1
	robotID           = "robot_id"
	robotLocation     = "robot_location"
	partID            = "part_id"
	dnsName           = "dns_name"
	secret            = "secret"
	mainPart          = false
	fqdn              = "fqdn"
	localFQDN         = "local_fqdn"
	configJSON        = "configJson"
	host              = "host"
	level             = "level"
	loggerName        = "logger_name"
	message           = "message"
	stack             = "stack"
	value             = "value"
	isDeactivated     = false
	keyID             = "key_id"
	key               = "key"
)

var (
	name         = "name"
	region       = "region"
	namespace    = "public_namespace"
	cid          = "cid"
	dateAdded    = timestamppb.Timestamp{Seconds: 0, Nanos: 50}
	organization = Organization{
		ID:              organizationID,
		Name:            name,
		CreatedOn:       &dateAdded,
		PublicNamespace: namespace,
		DefaultRegion:   region,
		Cid:             &cid,
	}
	pbOrganization = pb.Organization{
		Id:              organization.ID,
		Name:            organization.Name,
		CreatedOn:       organization.CreatedOn,
		PublicNamespace: organization.PublicNamespace,
		DefaultRegion:   organization.DefaultRegion,
		Cid:             organization.Cid,
	}
	organizationIdentity = OrganizationIdentity{
		ID:   organizationID,
		Name: name,
	}
	orgDetails = OrgDetails{
		OrgID:   organizationID,
		OrgName: name,
	}
	lastLogin     = timestamppb.Timestamp{Seconds: 0, Nanos: 100}
	createdOn     = timestamppb.Timestamp{Seconds: 0, Nanos: 0}
	authorization = Authorization{
		AuthorizationType: authorizationType,
		AuthorizationID:   authorizationID,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		IdentityID:        identityID,
		OrganizationID:    organizationID,
		IdentityType:      identityType,
	}
	authorizations   = []*Authorization{&authorization}
	pbAuthorizations = []*pb.Authorization{
		{
			AuthorizationType: authorization.AuthorizationType,
			AuthorizationId:   authorization.AuthorizationID,
			ResourceType:      authorization.ResourceType,
			ResourceId:        authorization.ResourceID,
			IdentityId:        authorization.IdentityID,
			OrganizationId:    authorization.OrganizationID,
			IdentityType:      authorization.IdentityType,
		},
	}
	member = OrganizationMember{
		UserID:    userID,
		Emails:    []string{email},
		DateAdded: &dateAdded,
		LastLogin: &lastLogin,
	}
	invite = OrganizationInvite{
		OrganizationID: organizationID,
		Email:          email,
		CreatedOn:      &createdOn,
		Authorizations: authorizations,
	}
	pbInvite = pb.OrganizationInvite{
		OrganizationId: invite.OrganizationID,
		Email:          invite.Email,
		CreatedOn:      invite.CreatedOn,
		Authorizations: pbAuthorizations,
	}
	sendEmailInvite = true
	addressLine2    = "address_line_2"
	address         = BillingAddress{
		AddressLine1: "address_line_1",
		AddressLine2: &addressLine2,
		City:         "city",
		State:        "state",
	}
	pbAddress = pb.BillingAddress{
		AddressLine_1: address.AddressLine1,
		AddressLine_2: address.AddressLine2,
		City:          address.City,
		State:         address.State,
	}
	parentLocationID = "parent_location_id"
	sharedSecret     = SharedSecret{
		ID:        secretID,
		CreatedOn: &createdOn,
		State:     SharedSecretStateEnabled,
	}
	sharedSecrets = []*SharedSecret{&sharedSecret}
	pbSecret      = pb.SharedSecret{
		Id:        sharedSecret.ID,
		CreatedOn: sharedSecret.CreatedOn,
		State:     sharedSecretStateToProto(sharedSecret.State),
	}
	pbSecrets    = []*pb.SharedSecret{&pbSecret}
	locationAuth = LocationAuth{
		LocationID: locationID,
		Secrets:    sharedSecrets,
	}
	pbLocationAuth = pb.LocationAuth{
		LocationId: locationAuth.LocationID,
		Secrets:    pbSecrets,
	}
	locationOrg = LocationOrganization{
		OrganizationID: organizationID,
		Primary:        primary,
	}
	storageConfig = StorageConfig{
		Region: region,
	}
	location = Location{
		ID:               locationID,
		Name:             name,
		ParentLocationID: parentLocationID,
		Auth:             &locationAuth,
		Organizations:    []*LocationOrganization{&locationOrg},
		CreatedOn:        &createdOn,
		RobotCount:       robotCount,
		Config:           &storageConfig,
	}
	pbLocation = pb.Location{
		Id:               location.ID,
		Name:             location.Name,
		ParentLocationId: location.ParentLocationID,
		Auth:             &pbLocationAuth,
		Organizations: []*pb.LocationOrganization{
			{
				OrganizationId: locationOrg.OrganizationID,
				Primary:        locationOrg.Primary,
			},
		},
		CreatedOn:  location.CreatedOn,
		RobotCount: location.RobotCount,
		Config: &pb.StorageConfig{
			Region: storageConfig.Region,
		},
	}
	lastAccess = timestamppb.Timestamp{Seconds: 0, Nanos: 110}
	robot      = Robot{
		ID:         robotID,
		Name:       name,
		Location:   robotLocation,
		LastAccess: &lastAccess,
		CreatedOn:  &createdOn,
	}
	pbRobot = pb.Robot{
		Id:         robot.ID,
		Name:       robot.Name,
		Location:   robot.Location,
		LastAccess: robot.LastAccess,
		CreatedOn:  robot.CreatedOn,
	}
	roverRentalRobot = RoverRentalRobot{
		RobotID:         robotID,
		LocationID:      locationID,
		RobotName:       name,
		RobotMainPartID: partID,
	}
	lastUpdated           = timestamppb.Timestamp{Seconds: 0, Nanos: 130}
	robotConfig           = map[string]interface{}{"name": name, "ID": robotID}
	pbRobotConfig, _      = protoutils.StructToStructPb(*robotPart.RobotConfig)
	pbUserSuppliedInfo, _ = protoutils.StructToStructPb(*robotPart.UserSuppliedInfo)
	userSuppliedInfo      = map[string]interface{}{"userID": userID}
	robotPart             = RobotPart{
		ID:               partID,
		Name:             name,
		DNSName:          dnsName,
		Secret:           secret,
		Robot:            robotID,
		LocationID:       locationID,
		RobotConfig:      &robotConfig,
		LastAccess:       &lastAccess,
		UserSuppliedInfo: &userSuppliedInfo,
		MainPart:         mainPart,
		FQDN:             fqdn,
		LocalFQDN:        localFQDN,
		CreatedOn:        &createdOn,
		Secrets:          sharedSecrets,
		LastUpdated:      &lastUpdated,
	}
	pbRobotPart = pb.RobotPart{
		Id:               robotPart.ID,
		Name:             robotPart.Name,
		DnsName:          robotPart.DNSName,
		Secret:           robotPart.Secret,
		Robot:            robotPart.Robot,
		LocationId:       robotPart.LocationID,
		RobotConfig:      pbRobotConfig,
		LastAccess:       robotPart.LastAccess,
		UserSuppliedInfo: pbUserSuppliedInfo,
		MainPart:         robotPart.MainPart,
		Fqdn:             robotPart.FQDN,
		LocalFqdn:        robotPart.LocalFQDN,
		CreatedOn:        robotPart.CreatedOn,
		Secrets:          pbSecrets,
		LastUpdated:      robotPart.LastUpdated,
	}
	pageToken       = "page_token"
	levels          = []string{level}
	start           = timestamppb.Timestamp{Seconds: 92, Nanos: 0}
	end             = timestamppb.Timestamp{Seconds: 99, Nanos: 999}
	limit     int64 = 2
	source          = "source"
	filter          = "filter"
	time            = timestamppb.Timestamp{Seconds: 11, Nanos: 15}
	caller          = map[string]interface{}{"name": name}
	field           = map[string]interface{}{"key": "value"}
	logEntry        = LogEntry{
		Host:       host,
		Level:      level,
		Time:       &time,
		LoggerName: loggerName,
		Message:    message,
		Caller:     &caller,
		Stack:      stack,
		Fields:     []*map[string]interface{}{&field},
	}
	authenticatorInfo = AuthenticatorInfo{
		Type:          AuthenticationTypeAPIKey,
		Value:         value,
		IsDeactivated: isDeactivated,
	}
	robotPartHistoryEntry = RobotPartHistoryEntry{
		Part:     partID,
		Robot:    robotID,
		When:     &time,
		Old:      &robotPart,
		EditedBy: &authenticatorInfo,
	}
	authorizationDetails = AuthorizationDetails{
		AuthorizationType: authorizationType,
		AuthorizationID:   authorizationID,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		OrgID:             organizationID,
	}
	apiKeyWithAuthorizations = APIKeyWithAuthorizations{
		APIKey: &APIKey{
			ID:        keyID,
			Key:       key,
			Name:      name,
			CreatedOn: &createdOn,
		},
		Authorizations: []*AuthorizationDetails{&authorizationDetails},
	}
	pbAPIKeyWithAuthorizations = pb.APIKeyWithAuthorizations{
		ApiKey: &pb.APIKey{
			Id:        apiKeyWithAuthorizations.APIKey.ID,
			Key:       apiKeyWithAuthorizations.APIKey.Key,
			Name:      apiKeyWithAuthorizations.APIKey.Name,
			CreatedOn: apiKeyWithAuthorizations.APIKey.CreatedOn,
		},
		Authorizations: []*pb.AuthorizationDetails{
			{
				AuthorizationType: authorizationDetails.AuthorizationType,
				AuthorizationId:   authorizationDetails.AuthorizationID,
				ResourceType:      authorizationDetails.ResourceType,
				ResourceId:        authorizationDetails.ResourceID,
				OrgId:             authorizationDetails.OrgID,
			},
		},
	}
)

func sharedSecretStateToProto(state SharedSecretState) pb.SharedSecret_State {
	switch state {
	case SharedSecretStateEnabled:
		return pb.SharedSecret_STATE_ENABLED
	case SharedSecretStateDisabled:
		return pb.SharedSecret_STATE_DISABLED
	default:
		return pb.SharedSecret_STATE_UNSPECIFIED
	}
}

func authenticationTypeToProto(authType AuthenticationType) pb.AuthenticationType {
	switch authType {
	case AuthenticationTypeWebOAuth:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH
	case AuthenticationTypeAPIKey:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_API_KEY
	case AuthenticationTypeRobotPartSecret:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET
	case AuthenticationTypeLocationSecret:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET
	default:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED
	}
}

func testOrganizationResponse(t *testing.T, actualOrg, expectedOrg *Organization) {
	test.That(t, actualOrg.ID, test.ShouldEqual, expectedOrg.ID)
	test.That(t, actualOrg.Name, test.ShouldEqual, expectedOrg.Name)
	test.That(t, actualOrg.PublicNamespace, test.ShouldEqual, expectedOrg.PublicNamespace)
	test.That(t, actualOrg.DefaultRegion, test.ShouldEqual, expectedOrg.DefaultRegion)
	test.That(t, actualOrg.Cid, test.ShouldEqual, expectedOrg.Cid)
}

func testLocationResponse(t *testing.T, actualLocation, expectedLocation *Location) {
	test.That(t, actualLocation.ID, test.ShouldEqual, expectedLocation.ID)
	test.That(t, actualLocation.Name, test.ShouldEqual, expectedLocation.Name)
	test.That(t, actualLocation.ParentLocationID, test.ShouldEqual, expectedLocation.ParentLocationID)
	test.That(t, actualLocation.Auth, test.ShouldResemble, expectedLocation.Auth)
	test.That(t, actualLocation.Organizations, test.ShouldResemble, expectedLocation.Organizations)
	test.That(t, actualLocation.CreatedOn, test.ShouldEqual, expectedLocation.CreatedOn)
	test.That(t, actualLocation.RobotCount, test.ShouldEqual, expectedLocation.RobotCount)
	test.That(t, actualLocation.Config, test.ShouldResemble, expectedLocation.Config)
}

func testRobotPartResponse(t *testing.T, actualRobotPart, expectedRobotPart *RobotPart) {
	test.That(t, actualRobotPart.ID, test.ShouldEqual, expectedRobotPart.ID)
	test.That(t, actualRobotPart.Name, test.ShouldEqual, expectedRobotPart.Name)
	test.That(t, actualRobotPart.DNSName, test.ShouldEqual, expectedRobotPart.DNSName)
	test.That(t, actualRobotPart.Secret, test.ShouldEqual, expectedRobotPart.Secret)
	test.That(t, actualRobotPart.Robot, test.ShouldEqual, expectedRobotPart.Robot)
	test.That(t, actualRobotPart.LocationID, test.ShouldEqual, expectedRobotPart.LocationID)
	test.That(t, actualRobotPart.RobotConfig, test.ShouldResemble, expectedRobotPart.RobotConfig)
	test.That(t, actualRobotPart.LastAccess, test.ShouldEqual, expectedRobotPart.LastAccess)
	test.That(t, actualRobotPart.UserSuppliedInfo, test.ShouldResemble, expectedRobotPart.UserSuppliedInfo)
	test.That(t, actualRobotPart.MainPart, test.ShouldEqual, expectedRobotPart.MainPart)
	test.That(t, actualRobotPart.FQDN, test.ShouldEqual, expectedRobotPart.FQDN)
	test.That(t, actualRobotPart.LocalFQDN, test.ShouldEqual, expectedRobotPart.LocalFQDN)
	test.That(t, actualRobotPart.CreatedOn, test.ShouldEqual, expectedRobotPart.CreatedOn)
	test.That(t, len(actualRobotPart.Secrets), test.ShouldEqual, len(expectedRobotPart.Secrets))
	test.That(t, actualRobotPart.Secrets[0], test.ShouldResemble, expectedRobotPart.Secrets[0])
	test.That(t, actualRobotPart.LastUpdated, test.ShouldEqual, expectedRobotPart.LastUpdated)
}

func createGrpcClient() *inject.AppServiceClient {
	return &inject.AppServiceClient{}
}

func TestAppClient(t *testing.T) {
	grpcClient := createGrpcClient()
	client := Client{client: grpcClient}

	t.Run("GetUserIDByEmail", func(t *testing.T) {
		grpcClient.GetUserIDByEmailFunc = func(
			ctx context.Context, in *pb.GetUserIDByEmailRequest, opts ...grpc.CallOption,
		) (*pb.GetUserIDByEmailResponse, error) {
			test.That(t, in.Email, test.ShouldEqual, email)
			return &pb.GetUserIDByEmailResponse{
				UserId: userID,
			}, nil
		}
		resp, _ := client.GetUserIDByEmail(context.Background(), email)
		test.That(t, resp, test.ShouldEqual, userID)
	})

	t.Run("CreateOrganization", func(t *testing.T) {
		grpcClient.CreateOrganizationFunc = func(
			ctx context.Context, in *pb.CreateOrganizationRequest, opts ...grpc.CallOption,
		) (*pb.CreateOrganizationResponse, error) {
			test.That(t, in.Name, test.ShouldEqual, name)
			return &pb.CreateOrganizationResponse{
				Organization: &pbOrganization,
			}, nil
		}
		resp, _ := client.CreateOrganization(context.Background(), name)
		testOrganizationResponse(t, resp, &organization)
	})

	t.Run("ListOrganizations", func(t *testing.T) {
		expectedOrganizations := []Organization{organization}
		grpcClient.ListOrganizationsFunc = func(
			ctx context.Context, in *pb.ListOrganizationsRequest, opts ...grpc.CallOption,
		) (*pb.ListOrganizationsResponse, error) {
			return &pb.ListOrganizationsResponse{
				Organizations: []*pb.Organization{&pbOrganization},
			}, nil
		}
		resp, _ := client.ListOrganizations(context.Background())
		test.That(t, len(resp), test.ShouldEqual, len(expectedOrganizations))
		testOrganizationResponse(t, resp[0], &expectedOrganizations[0])
	})

	t.Run("GetOrganizationsWithAccessToLocation", func(t *testing.T) {
		pbOrganizationIdentity := pb.OrganizationIdentity{
			Id:   organizationIdentity.ID,
			Name: organizationIdentity.Name,
		}
		grpcClient.GetOrganizationsWithAccessToLocationFunc = func(
			ctx context.Context, in *pb.GetOrganizationsWithAccessToLocationRequest, opts ...grpc.CallOption,
		) (*pb.GetOrganizationsWithAccessToLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.GetOrganizationsWithAccessToLocationResponse{
				OrganizationIdentities: []*pb.OrganizationIdentity{&pbOrganizationIdentity},
			}, nil
		}
		resp, _ := client.GetOrganizationsWithAccessToLocation(context.Background(), locationID)
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp[0].ID, test.ShouldEqual, organizationIdentity.ID)
		test.That(t, resp[0].Name, test.ShouldEqual, organizationIdentity.Name)
	})

	t.Run("ListOrganizationsByUser", func(t *testing.T) {
		orgDetailsList := []OrgDetails{orgDetails}
		pbOrgDetails := pb.OrgDetails{
			OrgId:   orgDetails.OrgID,
			OrgName: orgDetails.OrgName,
		}
		grpcClient.ListOrganizationsByUserFunc = func(
			ctx context.Context, in *pb.ListOrganizationsByUserRequest, opts ...grpc.CallOption,
		) (*pb.ListOrganizationsByUserResponse, error) {
			test.That(t, in.UserId, test.ShouldEqual, userID)
			return &pb.ListOrganizationsByUserResponse{
				Orgs: []*pb.OrgDetails{&pbOrgDetails},
			}, nil
		}
		resp, _ := client.ListOrganizationsByUser(context.Background(), userID)
		test.That(t, len(resp), test.ShouldEqual, len(orgDetailsList))
		test.That(t, resp[0].OrgID, test.ShouldEqual, orgDetailsList[0].OrgID)
		test.That(t, resp[0].OrgName, test.ShouldEqual, orgDetailsList[0].OrgName)
	})

	t.Run("GetOrganization", func(t *testing.T) {
		grpcClient.GetOrganizationFunc = func(
			ctx context.Context, in *pb.GetOrganizationRequest, opts ...grpc.CallOption,
		) (*pb.GetOrganizationResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.GetOrganizationResponse{
				Organization: &pbOrganization,
			}, nil
		}
		resp, _ := client.GetOrganization(context.Background(), organizationID)
		testOrganizationResponse(t, resp, &organization)
	})

	t.Run("GetOrganizationNamespaceAvailability", func(t *testing.T) {
		grpcClient.GetOrganizationNamespaceAvailabilityFunc = func(
			ctx context.Context, in *pb.GetOrganizationNamespaceAvailabilityRequest, opts ...grpc.CallOption,
		) (*pb.GetOrganizationNamespaceAvailabilityResponse, error) {
			test.That(t, in.PublicNamespace, test.ShouldEqual, namespace)
			return &pb.GetOrganizationNamespaceAvailabilityResponse{
				Available: available,
			}, nil
		}
		resp, _ := client.GetOrganizationNamespaceAvailability(context.Background(), namespace)
		test.That(t, resp, test.ShouldEqual, available)
	})

	t.Run("UpdateOrganization", func(t *testing.T) {
		grpcClient.UpdateOrganizationFunc = func(
			ctx context.Context, in *pb.UpdateOrganizationRequest, opts ...grpc.CallOption,
		) (*pb.UpdateOrganizationResponse, error) {
			test.That(t, in.PublicNamespace, test.ShouldEqual, &namespace)
			return &pb.UpdateOrganizationResponse{
				Organization: &pbOrganization,
			}, nil
		}
		resp, _ := client.UpdateOrganization(context.Background(), organizationID, &name, &namespace, &region, &cid)
		testOrganizationResponse(t, resp, &organization)
	})

	t.Run("DeleteOrganization", func(t *testing.T) {
		grpcClient.DeleteOrganizationFunc = func(
			ctx context.Context, in *pb.DeleteOrganizationRequest, opts ...grpc.CallOption,
		) (*pb.DeleteOrganizationResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.DeleteOrganizationResponse{}, nil
		}
		client.DeleteOrganization(context.Background(), organizationID)
	})

	t.Run("ListOrganizationMembers", func(t *testing.T) {
		expectedMembers := []OrganizationMember{member}
		pbMember := pb.OrganizationMember{
			UserId:    member.UserID,
			Emails:    member.Emails,
			DateAdded: member.DateAdded,
			LastLogin: member.LastLogin,
		}
		expectedInvites := []OrganizationInvite{invite}
		grpcClient.ListOrganizationMembersFunc = func(
			ctx context.Context, in *pb.ListOrganizationMembersRequest, opts ...grpc.CallOption,
		) (*pb.ListOrganizationMembersResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.ListOrganizationMembersResponse{
				Members: []*pb.OrganizationMember{&pbMember},
				Invites: []*pb.OrganizationInvite{&pbInvite},
			}, nil
		}
		members, invites, _ := client.ListOrganizationMembers(context.Background(), organizationID)
		test.That(t, len(members), test.ShouldEqual, len(expectedMembers))
		test.That(t, members[0].UserID, test.ShouldEqual, expectedMembers[0].UserID)
		test.That(t, members[0].Emails, test.ShouldResemble, expectedMembers[0].Emails)
		test.That(t, members[0].DateAdded, test.ShouldEqual, expectedMembers[0].DateAdded)
		test.That(t, members[0].LastLogin, test.ShouldEqual, expectedMembers[0].LastLogin)
		test.That(t, len(invites), test.ShouldEqual, len(expectedInvites))
		test.That(t, invites[0].OrganizationID, test.ShouldEqual, expectedInvites[0].OrganizationID)
		test.That(t, invites[0].Email, test.ShouldResemble, expectedInvites[0].Email)
		test.That(t, invites[0].CreatedOn, test.ShouldEqual, expectedInvites[0].CreatedOn)
		test.That(t, len(invites[0].Authorizations), test.ShouldEqual, len(expectedInvites[0].Authorizations))
		test.That(t, invites[0].Authorizations[0], test.ShouldResemble, expectedInvites[0].Authorizations[0])
	})

	t.Run("CreateOrganizationInvite", func(t *testing.T) {
		grpcClient.CreateOrganizationInviteFunc = func(
			ctx context.Context, in *pb.CreateOrganizationInviteRequest, opts ...grpc.CallOption,
		) (*pb.CreateOrganizationInviteResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.Email, test.ShouldEqual, email)
			test.That(t, in.Authorizations, test.ShouldResemble, pbAuthorizations)
			test.That(t, in.SendEmailInvite, test.ShouldEqual, &sendEmailInvite)
			return &pb.CreateOrganizationInviteResponse{
				Invite: &pbInvite,
			}, nil
		}
		resp, _ := client.CreateOrganizationInvite(context.Background(), organizationID, email, authorizations, &sendEmailInvite)
		test.That(t, resp.OrganizationID, test.ShouldEqual, invite.OrganizationID)
		test.That(t, resp.Email, test.ShouldResemble, invite.Email)
		test.That(t, resp.CreatedOn, test.ShouldEqual, invite.CreatedOn)
		test.That(t, len(resp.Authorizations), test.ShouldEqual, len(invite.Authorizations))
		test.That(t, resp.Authorizations[0], test.ShouldResemble, invite.Authorizations[0])
	})

	t.Run("UpdateOrganizationInviteAuthorizations", func(t *testing.T) {
		grpcClient.UpdateOrganizationInviteAuthorizationsFunc = func(
			ctx context.Context, in *pb.UpdateOrganizationInviteAuthorizationsRequest, opts ...grpc.CallOption,
		) (*pb.UpdateOrganizationInviteAuthorizationsResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.UpdateOrganizationInviteAuthorizationsResponse{
				Invite: &pbInvite,
			}, nil
		}
		resp, _ := client.UpdateOrganizationInviteAuthorizations(context.Background(), organizationID, email, authorizations, authorizations)
		test.That(t, resp.OrganizationID, test.ShouldEqual, invite.OrganizationID)
		test.That(t, resp.Email, test.ShouldResemble, invite.Email)
		test.That(t, resp.CreatedOn, test.ShouldEqual, invite.CreatedOn)
		test.That(t, len(resp.Authorizations), test.ShouldResemble, len(invite.Authorizations))
		test.That(t, resp.Authorizations[0], test.ShouldResemble, invite.Authorizations[0])
	})

	t.Run("DeleteOrganizationMember", func(t *testing.T) {
		grpcClient.DeleteOrganizationMemberFunc = func(
			ctx context.Context, in *pb.DeleteOrganizationMemberRequest, opts ...grpc.CallOption,
		) (*pb.DeleteOrganizationMemberResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.UserId, test.ShouldEqual, userID)
			return &pb.DeleteOrganizationMemberResponse{}, nil
		}
		client.DeleteOrganizationMember(context.Background(), organizationID, userID)
	})

	t.Run("DeleteOrganizationInvite", func(t *testing.T) {
		grpcClient.DeleteOrganizationInviteFunc = func(
			ctx context.Context, in *pb.DeleteOrganizationInviteRequest, opts ...grpc.CallOption,
		) (*pb.DeleteOrganizationInviteResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.Email, test.ShouldEqual, email)
			return &pb.DeleteOrganizationInviteResponse{}, nil
		}
		client.DeleteOrganizationInvite(context.Background(), organizationID, email)
	})

	t.Run("ResendOrganizationInvite", func(t *testing.T) {
		grpcClient.ResendOrganizationInviteFunc = func(
			ctx context.Context, in *pb.ResendOrganizationInviteRequest, opts ...grpc.CallOption,
		) (*pb.ResendOrganizationInviteResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.Email, test.ShouldEqual, email)
			return &pb.ResendOrganizationInviteResponse{
				Invite: &pbInvite,
			}, nil
		}
		resp, _ := client.ResendOrganizationInvite(context.Background(), organizationID, email)
		test.That(t, resp.OrganizationID, test.ShouldEqual, invite.OrganizationID)
		test.That(t, resp.Email, test.ShouldResemble, invite.Email)
		test.That(t, resp.CreatedOn, test.ShouldEqual, invite.CreatedOn)
		test.That(t, len(resp.Authorizations), test.ShouldEqual, len(invite.Authorizations))
		test.That(t, resp.Authorizations[0], test.ShouldResemble, invite.Authorizations[0])
	})

	t.Run("EnableBillingService", func(t *testing.T) {
		grpcClient.EnableBillingServiceFunc = func(
			ctx context.Context, in *pb.EnableBillingServiceRequest, opts ...grpc.CallOption,
		) (*pb.EnableBillingServiceResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			test.That(t, in.BillingAddress, test.ShouldResemble, &pbAddress)
			return &pb.EnableBillingServiceResponse{}, nil
		}
		client.EnableBillingService(context.Background(), organizationID, &address)
	})

	t.Run("DisableBillingService", func(t *testing.T) {
		grpcClient.DisableBillingServiceFunc = func(
			ctx context.Context, in *pb.DisableBillingServiceRequest, opts ...grpc.CallOption,
		) (*pb.DisableBillingServiceResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return &pb.DisableBillingServiceResponse{}, nil
		}
		client.DisableBillingService(context.Background(), organizationID)
	})

	t.Run("UpdateBillingService", func(t *testing.T) {
		grpcClient.UpdateBillingServiceFunc = func(
			ctx context.Context, in *pb.UpdateBillingServiceRequest, opts ...grpc.CallOption,
		) (*pb.UpdateBillingServiceResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			test.That(t, in.BillingAddress, test.ShouldResemble, &pbAddress)
			test.That(t, in.BillingSupportEmail, test.ShouldResemble, email)
			return &pb.UpdateBillingServiceResponse{}, nil
		}
		client.UpdateBillingService(context.Background(), organizationID, &address, email)
	})

	t.Run("OrganizationSetSupportEmail", func(t *testing.T) {
		grpcClient.OrganizationSetSupportEmailFunc = func(
			ctx context.Context, in *pb.OrganizationSetSupportEmailRequest, opts ...grpc.CallOption,
		) (*pb.OrganizationSetSupportEmailResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			test.That(t, in.Email, test.ShouldResemble, email)
			return &pb.OrganizationSetSupportEmailResponse{}, nil
		}
		client.OrganizationSetSupportEmail(context.Background(), organizationID, email)
	})

	t.Run("OrganizationGetSupportEmail", func(t *testing.T) {
		grpcClient.OrganizationGetSupportEmailFunc = func(
			ctx context.Context, in *pb.OrganizationGetSupportEmailRequest, opts ...grpc.CallOption,
		) (*pb.OrganizationGetSupportEmailResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return &pb.OrganizationGetSupportEmailResponse{
				Email: email,
			}, nil
		}
		resp, _ := client.OrganizationGetSupportEmail(context.Background(), organizationID)
		test.That(t, resp, test.ShouldEqual, email)
	})

	t.Run("CreateLocation", func(t *testing.T) {
		grpcClient.CreateLocationFunc = func(
			ctx context.Context, in *pb.CreateLocationRequest, opts ...grpc.CallOption,
		) (*pb.CreateLocationResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.Name, test.ShouldEqual, name)
			test.That(t, in.ParentLocationId, test.ShouldEqual, &parentLocationID)
			return &pb.CreateLocationResponse{
				Location: &pbLocation,
			}, nil
		}
		resp, _ := client.CreateLocation(context.Background(), organizationID, name, &parentLocationID)
		testLocationResponse(t, resp, &location)
	})

	t.Run("GetLocation", func(t *testing.T) {
		grpcClient.GetLocationFunc = func(
			ctx context.Context, in *pb.GetLocationRequest, opts ...grpc.CallOption,
		) (*pb.GetLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.GetLocationResponse{
				Location: &pbLocation,
			}, nil
		}
		resp, _ := client.GetLocation(context.Background(), locationID)
		testLocationResponse(t, resp, &location)
	})

	t.Run("UpdateLocation", func(t *testing.T) {
		grpcClient.UpdateLocationFunc = func(
			ctx context.Context, in *pb.UpdateLocationRequest, opts ...grpc.CallOption,
		) (*pb.UpdateLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			test.That(t, in.Name, test.ShouldEqual, &name)
			test.That(t, in.ParentLocationId, test.ShouldEqual, &parentLocationID)
			test.That(t, in.Region, test.ShouldEqual, &region)
			return &pb.UpdateLocationResponse{
				Location: &pbLocation,
			}, nil
		}
		resp, _ := client.UpdateLocation(context.Background(), locationID, &name, &parentLocationID, &region)
		testLocationResponse(t, resp, &location)
	})

	t.Run("DeleteLocation", func(t *testing.T) {
		grpcClient.DeleteLocationFunc = func(
			ctx context.Context, in *pb.DeleteLocationRequest, opts ...grpc.CallOption,
		) (*pb.DeleteLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.DeleteLocationResponse{}, nil
		}
		client.DeleteLocation(context.Background(), locationID)
	})

	t.Run("ListLocations", func(t *testing.T) {
		expectedLocations := []Location{location}
		grpcClient.ListLocationsFunc = func(
			ctx context.Context, in *pb.ListLocationsRequest, opts ...grpc.CallOption,
		) (*pb.ListLocationsResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.ListLocationsResponse{
				Locations: []*pb.Location{&pbLocation},
			}, nil
		}
		resp, _ := client.ListLocations(context.Background(), organizationID)
		test.That(t, len(resp), test.ShouldEqual, len(expectedLocations))
		testLocationResponse(t, resp[0], &expectedLocations[0])
	})

	t.Run("ShareLocation", func(t *testing.T) {
		grpcClient.ShareLocationFunc = func(
			ctx context.Context, in *pb.ShareLocationRequest, opts ...grpc.CallOption,
		) (*pb.ShareLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.ShareLocationResponse{}, nil
		}
		client.ShareLocation(context.Background(), locationID, organizationID)
	})

	t.Run("UnshareLocation", func(t *testing.T) {
		grpcClient.UnshareLocationFunc = func(
			ctx context.Context, in *pb.UnshareLocationRequest, opts ...grpc.CallOption,
		) (*pb.UnshareLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.UnshareLocationResponse{}, nil
		}
		client.UnshareLocation(context.Background(), locationID, organizationID)
	})

	t.Run("LocationAuth", func(t *testing.T) {
		grpcClient.LocationAuthFunc = func(
			ctx context.Context, in *pb.LocationAuthRequest, opts ...grpc.CallOption,
		) (*pb.LocationAuthResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.LocationAuthResponse{
				Auth: &pbLocationAuth,
			}, nil
		}
		resp, _ := client.LocationAuth(context.Background(), locationID)
		test.That(t, resp, test.ShouldResemble, &locationAuth)
	})

	t.Run("CreateLocationSecret", func(t *testing.T) {
		grpcClient.CreateLocationSecretFunc = func(
			ctx context.Context, in *pb.CreateLocationSecretRequest, opts ...grpc.CallOption,
		) (*pb.CreateLocationSecretResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.CreateLocationSecretResponse{
				Auth: &pbLocationAuth,
			}, nil
		}
		resp, _ := client.CreateLocationSecret(context.Background(), locationID)
		test.That(t, resp, test.ShouldResemble, &locationAuth)
	})

	t.Run("DeleteLocationSecret", func(t *testing.T) {
		grpcClient.DeleteLocationSecretFunc = func(
			ctx context.Context, in *pb.DeleteLocationSecretRequest, opts ...grpc.CallOption,
		) (*pb.DeleteLocationSecretResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			test.That(t, in.SecretId, test.ShouldEqual, secretID)
			return &pb.DeleteLocationSecretResponse{}, nil
		}
		client.DeleteLocationSecret(context.Background(), locationID, secretID)
	})

	t.Run("GetRobot", func(t *testing.T) {
		grpcClient.GetRobotFunc = func(
			ctx context.Context, in *pb.GetRobotRequest, opts ...grpc.CallOption,
		) (*pb.GetRobotResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, robotID)
			return &pb.GetRobotResponse{
				Robot: &pbRobot,
			}, nil
		}
		resp, _ := client.GetRobot(context.Background(), robotID)
		test.That(t, resp, test.ShouldResemble, &robot)
	})

	t.Run("GetRoverRentalRobots", func(t *testing.T) {
		expectedRobots := []RoverRentalRobot{roverRentalRobot}
		pbRobot := pb.RoverRentalRobot{
			RobotId:         roverRentalRobot.RobotID,
			LocationId:      roverRentalRobot.LocationID,
			RobotName:       roverRentalRobot.RobotName,
			RobotMainPartId: partID,
		}
		grpcClient.GetRoverRentalRobotsFunc = func(
			ctx context.Context, in *pb.GetRoverRentalRobotsRequest, opts ...grpc.CallOption,
		) (*pb.GetRoverRentalRobotsResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return &pb.GetRoverRentalRobotsResponse{
				Robots: []*pb.RoverRentalRobot{&pbRobot},
			}, nil
		}
		resp, _ := client.GetRoverRentalRobots(context.Background(), organizationID)
		test.That(t, len(resp), test.ShouldEqual, len(expectedRobots))
		test.That(t, resp[0], test.ShouldResemble, &expectedRobots[0])
	})

	t.Run("GetRobotParts", func(t *testing.T) {
		expectedRobotParts := []RobotPart{robotPart}
		grpcClient.GetRobotPartsFunc = func(
			ctx context.Context, in *pb.GetRobotPartsRequest, opts ...grpc.CallOption,
		) (*pb.GetRobotPartsResponse, error) {
			test.That(t, in.RobotId, test.ShouldEqual, robotID)
			return &pb.GetRobotPartsResponse{
				Parts: []*pb.RobotPart{&pbRobotPart},
			}, nil
		}
		resp, _ := client.GetRobotParts(context.Background(), robotID)
		test.That(t, len(resp), test.ShouldEqual, len(expectedRobotParts))
		testRobotPartResponse(t, resp[0], &expectedRobotParts[0])
	})

	t.Run("GetRobotPart", func(t *testing.T) {
		grpcClient.GetRobotPartFunc = func(
			ctx context.Context, in *pb.GetRobotPartRequest, opts ...grpc.CallOption,
		) (*pb.GetRobotPartResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, partID)
			return &pb.GetRobotPartResponse{
				Part:       &pbRobotPart,
				ConfigJson: configJSON,
			}, nil
		}
		part, json, _ := client.GetRobotPart(context.Background(), partID)
		test.That(t, json, test.ShouldEqual, configJSON)
		testRobotPartResponse(t, part, &robotPart)
	})

	t.Run("GetRobotPartLogs", func(t *testing.T) {
		expectedLogs := []*LogEntry{&logEntry}
		pbCaller, _ := protoutils.StructToStructPb(*logEntry.Caller)
		pbField, _ := protoutils.StructToStructPb(field)
		pbLogEntry := common.LogEntry{
			Host:       logEntry.Host,
			Level:      logEntry.Level,
			Time:       logEntry.Time,
			LoggerName: logEntry.LoggerName,
			Message:    logEntry.Message,
			Caller:     pbCaller,
			Stack:      logEntry.Stack,
			Fields:     []*structpb.Struct{pbField},
		}
		grpcClient.GetRobotPartLogsFunc = func(
			ctx context.Context, in *pb.GetRobotPartLogsRequest, opts ...grpc.CallOption,
		) (*pb.GetRobotPartLogsResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, partID)
			test.That(t, in.Filter, test.ShouldEqual, &filter)
			test.That(t, in.PageToken, test.ShouldEqual, &pageToken)
			test.That(t, in.Levels, test.ShouldResemble, levels)
			test.That(t, in.Start, test.ShouldEqual, &start)
			test.That(t, in.End, test.ShouldEqual, &end)
			test.That(t, in.Limit, test.ShouldEqual, &limit)
			test.That(t, in.Source, test.ShouldEqual, &source)
			return &pb.GetRobotPartLogsResponse{
				Logs:          []*common.LogEntry{&pbLogEntry},
				NextPageToken: pageToken,
			}, nil
		}
		logs, token, _ := client.GetRobotPartLogs(context.Background(), partID, &filter, &pageToken, levels, &start, &end, &limit, &source)
		test.That(t, token, test.ShouldEqual, pageToken)
		test.That(t, len(logs), test.ShouldEqual, len(expectedLogs))
		test.That(t, logs[0].Host, test.ShouldEqual, expectedLogs[0].Host)
		test.That(t, logs[0].Level, test.ShouldEqual, expectedLogs[0].Level)
		test.That(t, logs[0].Time, test.ShouldEqual, expectedLogs[0].Time)
		test.That(t, logs[0].LoggerName, test.ShouldEqual, expectedLogs[0].LoggerName)
		test.That(t, logs[0].Message, test.ShouldEqual, expectedLogs[0].Message)
		test.That(t, logs[0].Caller, test.ShouldResemble, expectedLogs[0].Caller)
		test.That(t, logs[0].Stack, test.ShouldEqual, expectedLogs[0].Stack)
		test.That(t, len(logs[0].Fields), test.ShouldEqual, len(expectedLogs[0].Fields))
		test.That(t, logs[0].Fields[0], test.ShouldResemble, expectedLogs[0].Fields[0])
	})

	t.Run("GetRobotPartHistory", func(t *testing.T) {
		expectedEntries := []*RobotPartHistoryEntry{&robotPartHistoryEntry}
		pbAuthenticatorInfo := pb.AuthenticatorInfo{
			Type:          authenticationTypeToProto(authenticatorInfo.Type),
			Value:         authenticatorInfo.Value,
			IsDeactivated: authenticatorInfo.IsDeactivated,
		}
		pbRobotPartHistoryEntry := pb.RobotPartHistoryEntry{
			Part:     robotPartHistoryEntry.Part,
			Robot:    robotPartHistoryEntry.Robot,
			When:     robotPartHistoryEntry.When,
			Old:      &pbRobotPart,
			EditedBy: &pbAuthenticatorInfo,
		}
		grpcClient.GetRobotPartHistoryFunc = func(
			ctx context.Context, in *pb.GetRobotPartHistoryRequest, opts ...grpc.CallOption,
		) (*pb.GetRobotPartHistoryResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, partID)
			return &pb.GetRobotPartHistoryResponse{
				History: []*pb.RobotPartHistoryEntry{&pbRobotPartHistoryEntry},
			}, nil
		}
		resp, _ := client.GetRobotPartHistory(context.Background(), partID)
		test.That(t, resp[0].Part, test.ShouldEqual, expectedEntries[0].Part)
		test.That(t, resp[0].Robot, test.ShouldEqual, expectedEntries[0].Robot)
		test.That(t, resp[0].When, test.ShouldEqual, expectedEntries[0].When)
		testRobotPartResponse(t, resp[0].Old, expectedEntries[0].Old)
		test.That(t, resp[0].EditedBy, test.ShouldResemble, expectedEntries[0].EditedBy)
	})

	t.Run("UpdateRobotPart", func(t *testing.T) {
		grpcClient.UpdateRobotPartFunc = func(
			ctx context.Context, in *pb.UpdateRobotPartRequest, opts ...grpc.CallOption,
		) (*pb.UpdateRobotPartResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, partID)
			test.That(t, in.Name, test.ShouldEqual, name)
			test.That(t, in.RobotConfig, test.ShouldResemble, pbRobotConfig)
			return &pb.UpdateRobotPartResponse{
				Part: &pbRobotPart,
			}, nil
		}
		resp, _ := client.UpdateRobotPart(context.Background(), partID, name, robotConfig)
		testRobotPartResponse(t, resp, &robotPart)
	})

	t.Run("NewRobotPart", func(t *testing.T) {
		grpcClient.NewRobotPartFunc = func(
			ctx context.Context, in *pb.NewRobotPartRequest, opts ...grpc.CallOption,
		) (*pb.NewRobotPartResponse, error) {
			test.That(t, in.RobotId, test.ShouldEqual, robotID)
			test.That(t, in.PartName, test.ShouldEqual, name)
			return &pb.NewRobotPartResponse{}, nil
		}
		client.NewRobotPart(context.Background(), robotID, name)
	})

	t.Run("DeleteRobotPart", func(t *testing.T) {
		grpcClient.DeleteRobotPartFunc = func(
			ctx context.Context, in *pb.DeleteRobotPartRequest, opts ...grpc.CallOption,
		) (*pb.DeleteRobotPartResponse, error) {
			test.That(t, in.PartId, test.ShouldEqual, partID)
			return &pb.DeleteRobotPartResponse{}, nil
		}
		client.DeleteRobotPart(context.Background(), partID)
	})

	t.Run("GetRobotAPIKeys", func(t *testing.T) {
		expectedAPIKeyWithAuthorizations := []APIKeyWithAuthorizations{apiKeyWithAuthorizations}
		grpcClient.GetRobotAPIKeysFunc = func(
			ctx context.Context, in *pb.GetRobotAPIKeysRequest, opts ...grpc.CallOption,
		) (*pb.GetRobotAPIKeysResponse, error) {
			test.That(t, in.RobotId, test.ShouldEqual, robotID)
			return &pb.GetRobotAPIKeysResponse{
				ApiKeys: []*pb.APIKeyWithAuthorizations{&pbAPIKeyWithAuthorizations},
			}, nil
		}
		resp, _ := client.GetRobotAPIKeys(context.Background(), robotID)
		test.That(t, len(resp), test.ShouldEqual, len(expectedAPIKeyWithAuthorizations))
		test.That(t, resp[0].APIKey, test.ShouldResemble, expectedAPIKeyWithAuthorizations[0].APIKey)
		test.That(t, resp[0].Authorizations, test.ShouldResemble, expectedAPIKeyWithAuthorizations[0].Authorizations)
	})

	t.Run("MarkPartForRestart", func(t *testing.T) {
		grpcClient.MarkPartForRestartFunc = func(
			ctx context.Context, in *pb.MarkPartForRestartRequest, opts ...grpc.CallOption,
		) (*pb.MarkPartForRestartResponse, error) {
			test.That(t, in.PartId, test.ShouldEqual, partID)
			return &pb.MarkPartForRestartResponse{}, nil
		}
		client.MarkPartForRestart(context.Background(), partID)
	})

	t.Run("CreateRobotPartSecret", func(t *testing.T) {
		grpcClient.CreateRobotPartSecretFunc = func(
			ctx context.Context, in *pb.CreateRobotPartSecretRequest, opts ...grpc.CallOption,
		) (*pb.CreateRobotPartSecretResponse, error) {
			test.That(t, in.PartId, test.ShouldEqual, partID)
			return &pb.CreateRobotPartSecretResponse{
				Part: &pbRobotPart,
			}, nil
		}
		resp, _ := client.CreateRobotPartSecret(context.Background(), partID)
		testRobotPartResponse(t, resp, &robotPart)
	})

	t.Run("DeleteRobotPartSecret", func(t *testing.T) {
		grpcClient.DeleteRobotPartSecretFunc = func(
			ctx context.Context, in *pb.DeleteRobotPartSecretRequest, opts ...grpc.CallOption,
		) (*pb.DeleteRobotPartSecretResponse, error) {
			test.That(t, in.PartId, test.ShouldEqual, partID)
			test.That(t, in.SecretId, test.ShouldEqual, secretID)
			return &pb.DeleteRobotPartSecretResponse{}, nil
		}
		client.DeleteRobotPartSecret(context.Background(), partID, secretID)
	})

	t.Run("ListRobots", func(t *testing.T) {
		expectedRobots := []*Robot{&robot}
		grpcClient.ListRobotsFunc = func(
			ctx context.Context, in *pb.ListRobotsRequest, opts ...grpc.CallOption,
		) (*pb.ListRobotsResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.ListRobotsResponse{
				Robots: []*pb.Robot{&pbRobot},
			}, nil
		}
		resp, _ := client.ListRobots(context.Background(), locationID)
		test.That(t, len(resp), test.ShouldEqual, len(expectedRobots))
		test.That(t, resp[0], test.ShouldResemble, expectedRobots[0])
	})

	t.Run("NewRobot", func(t *testing.T) {
		grpcClient.NewRobotFunc = func(
			ctx context.Context, in *pb.NewRobotRequest, opts ...grpc.CallOption,
		) (*pb.NewRobotResponse, error) {
			test.That(t, in.Name, test.ShouldEqual, name)
			test.That(t, in.Location, test.ShouldEqual, locationID)
			return &pb.NewRobotResponse{
				Id: robotID,
			}, nil
		}
		resp, _ := client.NewRobot(context.Background(), name, locationID)
		test.That(t, resp, test.ShouldResemble, robotID)
	})

	t.Run("UpdateRobot", func(t *testing.T) {
		grpcClient.UpdateRobotFunc = func(
			ctx context.Context, in *pb.UpdateRobotRequest, opts ...grpc.CallOption,
		) (*pb.UpdateRobotResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, robotID)
			test.That(t, in.Name, test.ShouldEqual, name)
			test.That(t, in.Location, test.ShouldEqual, locationID)
			return &pb.UpdateRobotResponse{
				Robot: &pbRobot,
			}, nil
		}
		resp, _ := client.UpdateRobot(context.Background(), robotID, name, locationID)
		test.That(t, resp, test.ShouldResemble, &robot)
	})

	t.Run("DeleteRobot", func(t *testing.T) {
		grpcClient.DeleteRobotFunc = func(
			ctx context.Context, in *pb.DeleteRobotRequest, opts ...grpc.CallOption,
		) (*pb.DeleteRobotResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, robotID)
			return &pb.DeleteRobotResponse{}, nil
		}
		client.DeleteRobot(context.Background(), robotID)
	})
}
