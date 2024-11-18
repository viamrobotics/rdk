package app

import (
	"context"
	"fmt"
	"testing"

	mlTraining "go.viam.com/api/app/mltraining/v1"
	packages "go.viam.com/api/app/packages/v1"
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
	organizationID2                = "organization_id_2"
	email                          = "email"
	userID                         = "user_id"
	locationID                     = "location_id"
	available                      = true
	authorizationType              = "owner"
	authorizationType2             = "operator"
	badAuthorizationType           = "authorization_type"
	resourceType                   = "organization"
	resourceType2                  = "location"
	badResourceType                = "resource_type"
	resourceID                     = "resource_id"
	resourceID2                    = "resource_id_2"
	identityID                     = "identity_id"
	identityID2                    = "identity_id_2"
	identityType                   = ""
	secretID                       = "secret_ids"
	primary                        = true
	robotCount                     = 1
	robotID                        = "robot_id"
	robotLocation                  = "robot_location"
	partID                         = "part_id"
	dnsName                        = "dns_name"
	secret                         = "secret"
	mainPart                       = false
	fqdn                           = "fqdn"
	localFQDN                      = "local_fqdn"
	configJSON                     = "configJson"
	host                           = "host"
	level                          = "level"
	loggerName                     = "logger_name"
	message                        = "message"
	stack                          = "stack"
	value                          = "value"
	isDeactivated                  = false
	keyID                          = "key_id"
	key                            = "key"
	fragmentID                     = "fragment_id"
	organizationOwner              = "organization_owner"
	robotPartCount                 = 5
	onlyUsedByOwner                = false
	organizationCount              = 2
	permission                     = "permission"
	itemID                         = "item_id"
	description                    = "description"
	packageType                    = PackageTypeMLTraining
	visibility                     = VisibilityPublic
	totalRobotUsage                = 4
	totalExternalRobotUsage        = 2
	totalOrganizationUsage         = 40
	totalExternalOrganizationUsage = 52
	version                        = "version"
	modelType                      = ModelTypeObjectDetection
	modelFramework                 = ModelFrameworkPyTorch
	draft                          = false
	platform                       = "platform"
	registryItemStatus             = RegistryItemStatusPublished
)

var (
	organizationID = "organization_id"
	name           = "name"
	region         = "region"
	namespace      = "public_namespace"
	cid            = "cid"
	dateAdded      = timestamppb.Timestamp{Seconds: 0, Nanos: 50}
	organization   = Organization{
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
	pbAuthorization = pb.Authorization{
		AuthorizationType: authorization.AuthorizationType,
		AuthorizationId:   authorization.AuthorizationID,
		ResourceType:      authorization.ResourceType,
		ResourceId:        authorization.ResourceID,
		IdentityId:        authorization.IdentityID,
		OrganizationId:    authorization.OrganizationID,
		IdentityType:      authorization.IdentityType,
	}
	authorization2 = Authorization{
		AuthorizationType: authorizationType2,
		AuthorizationID:   authorizationID2,
		ResourceType:      resourceType2,
		ResourceID:        resourceID2,
		IdentityID:        identityID2,
		OrganizationID:    organizationID2,
		IdentityType:      identityType,
	}
	pbAuthorization2 = pb.Authorization{
		AuthorizationType: authorization2.AuthorizationType,
		AuthorizationId:   authorization2.AuthorizationID,
		ResourceType:      authorization2.ResourceType,
		ResourceId:        authorization2.ResourceID,
		IdentityId:        authorization2.IdentityID,
		OrganizationId:    authorization2.OrganizationID,
		IdentityType:      authorization2.IdentityType,
	}
	authorizations   = []*Authorization{&authorization, &authorization2}
	pbAuthorizations = []*pb.Authorization{&pbAuthorization, &pbAuthorization2}
	member           = OrganizationMember{
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
	pbAuthenticatorInfo = pb.AuthenticatorInfo{
		Type:          authenticationTypeToProto(authenticatorInfo.Type),
		Value:         authenticatorInfo.Value,
		IsDeactivated: authenticatorInfo.IsDeactivated,
	}
	robotPartHistoryEntry = RobotPartHistoryEntry{
		Part:     partID,
		Robot:    robotID,
		When:     &time,
		Old:      &robotPart,
		EditedBy: &authenticatorInfo,
	}
	authorizationID      = fmt.Sprintf("%s_%s", resourceType, authorizationType)
	authorizationID2     = fmt.Sprintf("%s_%s", resourceType2, authorizationType2)
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
	public             = true
	fragmentVisibility = FragmentVisibilityPublic
	f                  = map[string]interface{}{"name": name, "id": fragmentID}
	pbF, _             = protoutils.StructToStructPb(f)
	fragment           = Fragment{
		ID:                fragmentID,
		Name:              name,
		Fragment:          &f,
		OrganizationOwner: organizationOwner,
		Public:            public,
		CreatedOn:         &createdOn,
		OrganizationName:  name,
		RobotPartCount:    robotPartCount,
		OrganizationCount: organizationCount,
		OnlyUsedByOwner:   onlyUsedByOwner,
		Visibility:        fragmentVisibility,
		LastUpdated:       &lastUpdated,
	}
	pbFragment = pb.Fragment{
		Id:                fragment.ID,
		Name:              fragment.Name,
		Fragment:          pbF,
		OrganizationOwner: fragment.OrganizationOwner,
		Public:            fragment.Public,
		CreatedOn:         fragment.CreatedOn,
		OrganizationName:  fragment.OrganizationName,
		RobotPartCount:    fragment.RobotPartCount,
		OrganizationCount: fragment.OrganizationCount,
		OnlyUsedByOwner:   fragment.OnlyUsedByOwner,
		Visibility:        fragmentVisibilityToProto(fragment.Visibility),
		LastUpdated:       fragment.LastUpdated,
	}
	fragmentConfig       = map[string]interface{}{"organizationCount": 4}
	editedOn             = timestamppb.Timestamp{Seconds: 8, Nanos: 278}
	fragmentHistoryEntry = FragmentHistoryEntry{
		Fragment: fragmentID,
		EditedOn: &editedOn,
		Old:      &fragment,
		EditedBy: &authenticatorInfo,
	}
	resourceIDs = []string{resourceID, resourceID2}
	permissions = []*AuthorizedPermissions{
		{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Permissions:  []string{permission},
		},
	}
	siteURL  = "url"
	metadata = RegistryItemMLTrainingMetadata{
		MlTrainingMetadata: &MLTrainingMetadata{
			Versions: []*MLTrainingVersion{
				{
					Version:   version,
					CreatedOn: &createdOn,
				},
			},
			ModelType:      modelType,
			ModelFramework: modelFramework,
			Draft:          draft,
		},
	}
	registryItem = RegistryItem{
		ItemID:                         itemID,
		OrganizationID:                 organizationID,
		PublicNamespace:                namespace,
		Name:                           name,
		Type:                           packageType,
		Visibility:                     visibility,
		URL:                            siteURL,
		Description:                    description,
		TotalRobotUsage:                totalRobotUsage,
		TotalExternalRobotUsage:        totalExternalRobotUsage,
		TotalOrganizationUsage:         totalOrganizationUsage,
		TotalExternalOrganizationUsage: totalExternalOrganizationUsage,
		Metadata:                       &metadata,
		CreatedAt:                      &createdOn,
		UpdatedAt:                      &lastUpdated,
	}
	pbRegistryItem, _ = registryItemToProto(&registryItem)
	searchTerm        = "search_term"
)

func sharedSecretStateToProto(state SharedSecretState) pb.SharedSecret_State {
	switch state {
	case SharedSecretStateUnspecified:
		return pb.SharedSecret_STATE_UNSPECIFIED
	case SharedSecretStateEnabled:
		return pb.SharedSecret_STATE_ENABLED
	case SharedSecretStateDisabled:
		return pb.SharedSecret_STATE_DISABLED
	}
	return pb.SharedSecret_STATE_UNSPECIFIED
}

func authenticationTypeToProto(authType AuthenticationType) pb.AuthenticationType {
	switch authType {
	case AuthenticationTypeUnspecified:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED
	case AuthenticationTypeWebOAuth:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_WEB_OAUTH
	case AuthenticationTypeAPIKey:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_API_KEY
	case AuthenticationTypeRobotPartSecret:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_ROBOT_PART_SECRET
	case AuthenticationTypeLocationSecret:
		return pb.AuthenticationType_AUTHENTICATION_TYPE_LOCATION_SECRET
	}
	return pb.AuthenticationType_AUTHENTICATION_TYPE_UNSPECIFIED
}

func modelTypeToProto(modelType ModelType) mlTraining.ModelType {
	switch modelType {
	case ModelTypeUnspecified:
		return mlTraining.ModelType_MODEL_TYPE_UNSPECIFIED
	case ModelTypeSingleLabelClassification:
		return mlTraining.ModelType_MODEL_TYPE_SINGLE_LABEL_CLASSIFICATION
	case ModelTypeMultiLabelClassification:
		return mlTraining.ModelType_MODEL_TYPE_MULTI_LABEL_CLASSIFICATION
	case ModelTypeObjectDetection:
		return mlTraining.ModelType_MODEL_TYPE_OBJECT_DETECTION
	}
	return mlTraining.ModelType_MODEL_TYPE_UNSPECIFIED
}

func modelFrameworkToProto(framework ModelFramework) mlTraining.ModelFramework {
	switch framework {
	case ModelFrameworkUnspecified:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED
	case ModelFrameworkTFLite:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_TFLITE
	case ModelFrameworkTensorFlow:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_TENSORFLOW
	case ModelFrameworkPyTorch:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_PYTORCH
	case ModelFrameworkONNX:
		return mlTraining.ModelFramework_MODEL_FRAMEWORK_ONNX
	}
	return mlTraining.ModelFramework_MODEL_FRAMEWORK_UNSPECIFIED
}

func mlTrainingVersionToProto(version *MLTrainingVersion) *pb.MLTrainingVersion {
	return &pb.MLTrainingVersion{
		Version:   version.Version,
		CreatedOn: version.CreatedOn,
	}
}

func mlTrainingMetadataToProto(md MLTrainingMetadata) *pb.MLTrainingMetadata {
	var versions []*pb.MLTrainingVersion
	for _, version := range md.Versions {
		versions = append(versions, mlTrainingVersionToProto(version))
	}
	return &pb.MLTrainingMetadata{
		Versions:       versions,
		ModelType:      modelTypeToProto(md.ModelType),
		ModelFramework: modelFrameworkToProto(md.ModelFramework),
		Draft:          md.Draft,
	}
}

func registryItemToProto(item *RegistryItem) (*pb.RegistryItem, error) {
	switch metadata := item.Metadata.(type) {
	case *RegistryItemModuleMetadata:
		return &pb.RegistryItem{
			ItemId:                         item.ItemID,
			OrganizationId:                 item.OrganizationID,
			PublicNamespace:                item.PublicNamespace,
			Name:                           item.Name,
			Type:                           packageTypeToProto(item.Type),
			Visibility:                     visibilityToProto(item.Visibility),
			Url:                            item.URL,
			Description:                    item.Description,
			TotalRobotUsage:                item.TotalRobotUsage,
			TotalExternalRobotUsage:        item.TotalExternalRobotUsage,
			TotalOrganizationUsage:         item.TotalOrganizationUsage,
			TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
			Metadata:                       &pb.RegistryItem_ModuleMetadata{ModuleMetadata: &pb.ModuleMetadata{}},
			CreatedAt:                      item.CreatedAt,
			UpdatedAt:                      item.UpdatedAt,
		}, nil
	case *RegistryItemMLModelMetadata:
		return &pb.RegistryItem{
			ItemId:                         item.ItemID,
			OrganizationId:                 item.OrganizationID,
			PublicNamespace:                item.PublicNamespace,
			Name:                           item.Name,
			Type:                           packageTypeToProto(item.Type),
			Visibility:                     visibilityToProto(item.Visibility),
			Url:                            item.URL,
			Description:                    item.Description,
			TotalRobotUsage:                item.TotalRobotUsage,
			TotalExternalRobotUsage:        item.TotalExternalRobotUsage,
			TotalOrganizationUsage:         item.TotalOrganizationUsage,
			TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
			Metadata:                       &pb.RegistryItem_ModuleMetadata{ModuleMetadata: &pb.ModuleMetadata{}},
			CreatedAt:                      item.CreatedAt,
			UpdatedAt:                      item.UpdatedAt,
		}, nil
	case *RegistryItemMLTrainingMetadata:
		protoMetadata := mlTrainingMetadataToProto(*metadata.MlTrainingMetadata)
		return &pb.RegistryItem{
			ItemId:                         item.ItemID,
			OrganizationId:                 item.OrganizationID,
			PublicNamespace:                item.PublicNamespace,
			Name:                           item.Name,
			Type:                           packageTypeToProto(item.Type),
			Visibility:                     visibilityToProto(item.Visibility),
			Url:                            item.URL,
			Description:                    item.Description,
			TotalRobotUsage:                item.TotalRobotUsage,
			TotalExternalRobotUsage:        item.TotalExternalRobotUsage,
			TotalOrganizationUsage:         item.TotalOrganizationUsage,
			TotalExternalOrganizationUsage: item.TotalExternalOrganizationUsage,
			Metadata:                       &pb.RegistryItem_MlTrainingMetadata{MlTrainingMetadata: protoMetadata},
			CreatedAt:                      item.CreatedAt,
			UpdatedAt:                      item.UpdatedAt,
		}, nil
	default:
		return nil, fmt.Errorf("unknown registry item metadata type: %T", item.Metadata)
	}
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
		resp, err := client.GetUserIDByEmail(context.Background(), email)
		test.That(t, err, test.ShouldBeNil)
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
		resp, err := client.CreateOrganization(context.Background(), name)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &organization)
	})

	t.Run("ListOrganizations", func(t *testing.T) {
		expectedOrganizations := []*Organization{&organization}
		grpcClient.ListOrganizationsFunc = func(
			ctx context.Context, in *pb.ListOrganizationsRequest, opts ...grpc.CallOption,
		) (*pb.ListOrganizationsResponse, error) {
			return &pb.ListOrganizationsResponse{
				Organizations: []*pb.Organization{&pbOrganization},
			}, nil
		}
		resp, err := client.ListOrganizations(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedOrganizations)
	})

	t.Run("GetOrganizationsWithAccessToLocation", func(t *testing.T) {
		expectedOrganizationIdentities := []*OrganizationIdentity{&organizationIdentity}
		grpcClient.GetOrganizationsWithAccessToLocationFunc = func(
			ctx context.Context, in *pb.GetOrganizationsWithAccessToLocationRequest, opts ...grpc.CallOption,
		) (*pb.GetOrganizationsWithAccessToLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.GetOrganizationsWithAccessToLocationResponse{
				OrganizationIdentities: []*pb.OrganizationIdentity{
					{
						Id:   organizationIdentity.ID,
						Name: organizationIdentity.Name,
					},
				},
			}, nil
		}
		resp, err := client.GetOrganizationsWithAccessToLocation(context.Background(), locationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedOrganizationIdentities)
	})

	t.Run("ListOrganizationsByUser", func(t *testing.T) {
		expectedOrgDetailsList := []*OrgDetails{&orgDetails}
		grpcClient.ListOrganizationsByUserFunc = func(
			ctx context.Context, in *pb.ListOrganizationsByUserRequest, opts ...grpc.CallOption,
		) (*pb.ListOrganizationsByUserResponse, error) {
			test.That(t, in.UserId, test.ShouldEqual, userID)
			return &pb.ListOrganizationsByUserResponse{
				Orgs: []*pb.OrgDetails{
					{
						OrgId:   orgDetails.OrgID,
						OrgName: orgDetails.OrgName,
					},
				},
			}, nil
		}
		resp, err := client.ListOrganizationsByUser(context.Background(), userID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedOrgDetailsList)
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
		resp, err := client.GetOrganization(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &organization)
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
		resp, err := client.GetOrganizationNamespaceAvailability(context.Background(), namespace)
		test.That(t, err, test.ShouldBeNil)
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
		resp, err := client.UpdateOrganization(context.Background(), organizationID, &name, &namespace, &region, &cid)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &organization)
	})

	t.Run("DeleteOrganization", func(t *testing.T) {
		grpcClient.DeleteOrganizationFunc = func(
			ctx context.Context, in *pb.DeleteOrganizationRequest, opts ...grpc.CallOption,
		) (*pb.DeleteOrganizationResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.DeleteOrganizationResponse{}, nil
		}
		err := client.DeleteOrganization(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ListOrganizationMembers", func(t *testing.T) {
		expectedMembers := []*OrganizationMember{&member}
		expectedInvites := []*OrganizationInvite{&invite}
		grpcClient.ListOrganizationMembersFunc = func(
			ctx context.Context, in *pb.ListOrganizationMembersRequest, opts ...grpc.CallOption,
		) (*pb.ListOrganizationMembersResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.ListOrganizationMembersResponse{
				Members: []*pb.OrganizationMember{
					{
						UserId:    member.UserID,
						Emails:    member.Emails,
						DateAdded: member.DateAdded,
						LastLogin: member.LastLogin,
					},
				},
				Invites: []*pb.OrganizationInvite{&pbInvite},
			}, nil
		}
		members, invites, err := client.ListOrganizationMembers(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, members, test.ShouldResemble, expectedMembers)
		test.That(t, invites, test.ShouldResemble, expectedInvites)
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
		resp, err := client.CreateOrganizationInvite(context.Background(), organizationID, email, authorizations, &sendEmailInvite)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &invite)
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
		resp, err := client.UpdateOrganizationInviteAuthorizations(context.Background(), organizationID, email, authorizations, authorizations)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &invite)
	})

	t.Run("DeleteOrganizationMember", func(t *testing.T) {
		grpcClient.DeleteOrganizationMemberFunc = func(
			ctx context.Context, in *pb.DeleteOrganizationMemberRequest, opts ...grpc.CallOption,
		) (*pb.DeleteOrganizationMemberResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.UserId, test.ShouldEqual, userID)
			return &pb.DeleteOrganizationMemberResponse{}, nil
		}
		err := client.DeleteOrganizationMember(context.Background(), organizationID, userID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("DeleteOrganizationInvite", func(t *testing.T) {
		grpcClient.DeleteOrganizationInviteFunc = func(
			ctx context.Context, in *pb.DeleteOrganizationInviteRequest, opts ...grpc.CallOption,
		) (*pb.DeleteOrganizationInviteResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.Email, test.ShouldEqual, email)
			return &pb.DeleteOrganizationInviteResponse{}, nil
		}
		err := client.DeleteOrganizationInvite(context.Background(), organizationID, email)
		test.That(t, err, test.ShouldBeNil)
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
		resp, err := client.ResendOrganizationInvite(context.Background(), organizationID, email)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &invite)
	})

	t.Run("EnableBillingService", func(t *testing.T) {
		grpcClient.EnableBillingServiceFunc = func(
			ctx context.Context, in *pb.EnableBillingServiceRequest, opts ...grpc.CallOption,
		) (*pb.EnableBillingServiceResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			test.That(t, in.BillingAddress, test.ShouldResemble, &pbAddress)
			return &pb.EnableBillingServiceResponse{}, nil
		}
		err := client.EnableBillingService(context.Background(), organizationID, &address)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("DisableBillingService", func(t *testing.T) {
		grpcClient.DisableBillingServiceFunc = func(
			ctx context.Context, in *pb.DisableBillingServiceRequest, opts ...grpc.CallOption,
		) (*pb.DisableBillingServiceResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return &pb.DisableBillingServiceResponse{}, nil
		}
		err := client.DisableBillingService(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
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
		err := client.UpdateBillingService(context.Background(), organizationID, &address, email)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("OrganizationSetSupportEmail", func(t *testing.T) {
		grpcClient.OrganizationSetSupportEmailFunc = func(
			ctx context.Context, in *pb.OrganizationSetSupportEmailRequest, opts ...grpc.CallOption,
		) (*pb.OrganizationSetSupportEmailResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			test.That(t, in.Email, test.ShouldResemble, email)
			return &pb.OrganizationSetSupportEmailResponse{}, nil
		}
		err := client.OrganizationSetSupportEmail(context.Background(), organizationID, email)
		test.That(t, err, test.ShouldBeNil)
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
		resp, err := client.OrganizationGetSupportEmail(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
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
		resp, err := client.CreateLocation(context.Background(), organizationID, name, &parentLocationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &location)
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
		resp, err := client.GetLocation(context.Background(), locationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &location)
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
		resp, err := client.UpdateLocation(context.Background(), locationID, &name, &parentLocationID, &region)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &location)
	})

	t.Run("DeleteLocation", func(t *testing.T) {
		grpcClient.DeleteLocationFunc = func(
			ctx context.Context, in *pb.DeleteLocationRequest, opts ...grpc.CallOption,
		) (*pb.DeleteLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.DeleteLocationResponse{}, nil
		}
		err := client.DeleteLocation(context.Background(), locationID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ListLocations", func(t *testing.T) {
		expectedLocations := []*Location{&location}
		grpcClient.ListLocationsFunc = func(
			ctx context.Context, in *pb.ListLocationsRequest, opts ...grpc.CallOption,
		) (*pb.ListLocationsResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.ListLocationsResponse{
				Locations: []*pb.Location{&pbLocation},
			}, nil
		}
		resp, err := client.ListLocations(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedLocations)
	})

	t.Run("ShareLocation", func(t *testing.T) {
		grpcClient.ShareLocationFunc = func(
			ctx context.Context, in *pb.ShareLocationRequest, opts ...grpc.CallOption,
		) (*pb.ShareLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.ShareLocationResponse{}, nil
		}
		err := client.ShareLocation(context.Background(), locationID, organizationID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("UnshareLocation", func(t *testing.T) {
		grpcClient.UnshareLocationFunc = func(
			ctx context.Context, in *pb.UnshareLocationRequest, opts ...grpc.CallOption,
		) (*pb.UnshareLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.UnshareLocationResponse{}, nil
		}
		err := client.UnshareLocation(context.Background(), locationID, organizationID)
		test.That(t, err, test.ShouldBeNil)
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
		resp, err := client.LocationAuth(context.Background(), locationID)
		test.That(t, err, test.ShouldBeNil)
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
		resp, err := client.CreateLocationSecret(context.Background(), locationID)
		test.That(t, err, test.ShouldBeNil)
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
		err := client.DeleteLocationSecret(context.Background(), locationID, secretID)
		test.That(t, err, test.ShouldBeNil)
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
		resp, err := client.GetRobot(context.Background(), robotID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &robot)
	})

	t.Run("GetRoverRentalRobots", func(t *testing.T) {
		expectedRobots := []*RoverRentalRobot{&roverRentalRobot}
		grpcClient.GetRoverRentalRobotsFunc = func(
			ctx context.Context, in *pb.GetRoverRentalRobotsRequest, opts ...grpc.CallOption,
		) (*pb.GetRoverRentalRobotsResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return &pb.GetRoverRentalRobotsResponse{
				Robots: []*pb.RoverRentalRobot{
					{
						RobotId:         roverRentalRobot.RobotID,
						LocationId:      roverRentalRobot.LocationID,
						RobotName:       roverRentalRobot.RobotName,
						RobotMainPartId: partID,
					},
				},
			}, nil
		}
		resp, err := client.GetRoverRentalRobots(context.Background(), organizationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedRobots)
	})

	t.Run("GetRobotParts", func(t *testing.T) {
		expectedRobotParts := []*RobotPart{&robotPart}
		grpcClient.GetRobotPartsFunc = func(
			ctx context.Context, in *pb.GetRobotPartsRequest, opts ...grpc.CallOption,
		) (*pb.GetRobotPartsResponse, error) {
			test.That(t, in.RobotId, test.ShouldEqual, robotID)
			return &pb.GetRobotPartsResponse{
				Parts: []*pb.RobotPart{&pbRobotPart},
			}, nil
		}
		resp, err := client.GetRobotParts(context.Background(), robotID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedRobotParts)
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
		part, json, err := client.GetRobotPart(context.Background(), partID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, json, test.ShouldEqual, configJSON)
		test.That(t, part, test.ShouldResemble, &robotPart)
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
		logs, token, err := client.GetRobotPartLogs(context.Background(), partID, &filter, &pageToken, levels, &start, &end, &limit, &source)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, token, test.ShouldEqual, pageToken)
		test.That(t, logs, test.ShouldResemble, expectedLogs)
	})

	t.Run("GetRobotPartHistory", func(t *testing.T) {
		expectedEntries := []*RobotPartHistoryEntry{&robotPartHistoryEntry}
		grpcClient.GetRobotPartHistoryFunc = func(
			ctx context.Context, in *pb.GetRobotPartHistoryRequest, opts ...grpc.CallOption,
		) (*pb.GetRobotPartHistoryResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, partID)
			return &pb.GetRobotPartHistoryResponse{
				History: []*pb.RobotPartHistoryEntry{
					{
						Part:     robotPartHistoryEntry.Part,
						Robot:    robotPartHistoryEntry.Robot,
						When:     robotPartHistoryEntry.When,
						Old:      &pbRobotPart,
						EditedBy: &pbAuthenticatorInfo,
					},
				},
			}, nil
		}
		resp, err := client.GetRobotPartHistory(context.Background(), partID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedEntries)
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
		resp, err := client.UpdateRobotPart(context.Background(), partID, name, robotConfig)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &robotPart)
	})

	t.Run("NewRobotPart", func(t *testing.T) {
		grpcClient.NewRobotPartFunc = func(
			ctx context.Context, in *pb.NewRobotPartRequest, opts ...grpc.CallOption,
		) (*pb.NewRobotPartResponse, error) {
			test.That(t, in.RobotId, test.ShouldEqual, robotID)
			test.That(t, in.PartName, test.ShouldEqual, name)
			return &pb.NewRobotPartResponse{
				PartId: partID,
			}, nil
		}
		resp, err := client.NewRobotPart(context.Background(), robotID, name)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, partID)
	})

	t.Run("DeleteRobotPart", func(t *testing.T) {
		grpcClient.DeleteRobotPartFunc = func(
			ctx context.Context, in *pb.DeleteRobotPartRequest, opts ...grpc.CallOption,
		) (*pb.DeleteRobotPartResponse, error) {
			test.That(t, in.PartId, test.ShouldEqual, partID)
			return &pb.DeleteRobotPartResponse{}, nil
		}
		err := client.DeleteRobotPart(context.Background(), partID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("GetRobotAPIKeys", func(t *testing.T) {
		expectedAPIKeyWithAuthorizations := []*APIKeyWithAuthorizations{&apiKeyWithAuthorizations}
		grpcClient.GetRobotAPIKeysFunc = func(
			ctx context.Context, in *pb.GetRobotAPIKeysRequest, opts ...grpc.CallOption,
		) (*pb.GetRobotAPIKeysResponse, error) {
			test.That(t, in.RobotId, test.ShouldEqual, robotID)
			return &pb.GetRobotAPIKeysResponse{
				ApiKeys: []*pb.APIKeyWithAuthorizations{&pbAPIKeyWithAuthorizations},
			}, nil
		}
		resp, err := client.GetRobotAPIKeys(context.Background(), robotID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedAPIKeyWithAuthorizations)
	})

	t.Run("MarkPartForRestart", func(t *testing.T) {
		grpcClient.MarkPartForRestartFunc = func(
			ctx context.Context, in *pb.MarkPartForRestartRequest, opts ...grpc.CallOption,
		) (*pb.MarkPartForRestartResponse, error) {
			test.That(t, in.PartId, test.ShouldEqual, partID)
			return &pb.MarkPartForRestartResponse{}, nil
		}
		err := client.MarkPartForRestart(context.Background(), partID)
		test.That(t, err, test.ShouldBeNil)
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
		resp, err := client.CreateRobotPartSecret(context.Background(), partID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &robotPart)
	})

	t.Run("DeleteRobotPartSecret", func(t *testing.T) {
		grpcClient.DeleteRobotPartSecretFunc = func(
			ctx context.Context, in *pb.DeleteRobotPartSecretRequest, opts ...grpc.CallOption,
		) (*pb.DeleteRobotPartSecretResponse, error) {
			test.That(t, in.PartId, test.ShouldEqual, partID)
			test.That(t, in.SecretId, test.ShouldEqual, secretID)
			return &pb.DeleteRobotPartSecretResponse{}, nil
		}
		err := client.DeleteRobotPartSecret(context.Background(), partID, secretID)
		test.That(t, err, test.ShouldBeNil)
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
		resp, err := client.ListRobots(context.Background(), locationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedRobots)
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
		resp, err := client.NewRobot(context.Background(), name, locationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldEqual, robotID)
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
		resp, err := client.UpdateRobot(context.Background(), robotID, name, locationID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &robot)
	})

	t.Run("DeleteRobot", func(t *testing.T) {
		grpcClient.DeleteRobotFunc = func(
			ctx context.Context, in *pb.DeleteRobotRequest, opts ...grpc.CallOption,
		) (*pb.DeleteRobotResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, robotID)
			return &pb.DeleteRobotResponse{}, nil
		}
		err := client.DeleteRobot(context.Background(), robotID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("GetFragment", func(t *testing.T) {
		grpcClient.GetFragmentFunc = func(
			ctx context.Context, in *pb.GetFragmentRequest, opts ...grpc.CallOption,
		) (*pb.GetFragmentResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, fragmentID)
			return &pb.GetFragmentResponse{
				Fragment: &pbFragment,
			}, nil
		}
		resp, err := client.GetFragment(context.Background(), fragmentID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &fragment)
	})

	t.Run("UpdateFragment", func(t *testing.T) {
		pbFragmentConfig, _ := protoutils.StructToStructPb(fragmentConfig)
		pbFragmentVisibility := fragmentVisibilityToProto(fragmentVisibility)
		grpcClient.UpdateFragmentFunc = func(
			ctx context.Context, in *pb.UpdateFragmentRequest, opts ...grpc.CallOption,
		) (*pb.UpdateFragmentResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, fragmentID)
			test.That(t, in.Name, test.ShouldEqual, name)
			test.That(t, in.Config, test.ShouldResemble, pbFragmentConfig)
			test.That(t, in.Public, test.ShouldEqual, &public)
			test.That(t, in.Visibility, test.ShouldResemble, &pbFragmentVisibility)
			return &pb.UpdateFragmentResponse{
				Fragment: &pbFragment,
			}, nil
		}
		resp, err := client.UpdateFragment(context.Background(), fragmentID, name, fragmentConfig, &public, &fragmentVisibility)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &fragment)
	})

	t.Run("DeleteFragment", func(t *testing.T) {
		grpcClient.DeleteFragmentFunc = func(
			ctx context.Context, in *pb.DeleteFragmentRequest, opts ...grpc.CallOption,
		) (*pb.DeleteFragmentResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, fragmentID)
			return &pb.DeleteFragmentResponse{}, nil
		}
		err := client.DeleteFragment(context.Background(), fragmentID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ListMachineFragments", func(t *testing.T) {
		expectedFragments := []*Fragment{&fragment}
		additionalFragmentIDs := []string{fragmentID}
		grpcClient.ListMachineFragmentsFunc = func(
			ctx context.Context, in *pb.ListMachineFragmentsRequest, opts ...grpc.CallOption,
		) (*pb.ListMachineFragmentsResponse, error) {
			test.That(t, in.MachineId, test.ShouldEqual, robotID)
			test.That(t, in.AdditionalFragmentIds, test.ShouldResemble, additionalFragmentIDs)
			return &pb.ListMachineFragmentsResponse{
				Fragments: []*pb.Fragment{&pbFragment},
			}, nil
		}
		resp, err := client.ListMachineFragments(context.Background(), robotID, additionalFragmentIDs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedFragments)
	})

	t.Run("GetFragmentHistory", func(t *testing.T) {
		expectedHistory := []*FragmentHistoryEntry{&fragmentHistoryEntry}
		grpcClient.GetFragmentHistoryFunc = func(
			ctx context.Context, in *pb.GetFragmentHistoryRequest, opts ...grpc.CallOption,
		) (*pb.GetFragmentHistoryResponse, error) {
			test.That(t, in.Id, test.ShouldEqual, fragmentID)
			test.That(t, in.PageToken, test.ShouldResemble, &pageToken)
			test.That(t, in.PageLimit, test.ShouldResemble, &limit)
			return &pb.GetFragmentHistoryResponse{
				History: []*pb.FragmentHistoryEntry{
					{
						Fragment: fragmentHistoryEntry.Fragment,
						EditedOn: fragmentHistoryEntry.EditedOn,
						Old:      &pbFragment,
						EditedBy: &pbAuthenticatorInfo,
					},
				},
				NextPageToken: pageToken,
			}, nil
		}
		resp, token, err := client.GetFragmentHistory(context.Background(), fragmentID, &pageToken, &limit)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, token, test.ShouldEqual, pageToken)
		test.That(t, resp, test.ShouldResemble, expectedHistory)
	})

	t.Run("createAuthorization", func(t *testing.T) {
		resp, err := createAuthorization(authorization.OrganizationID,
			authorization.IdentityID,
			authorization.IdentityType,
			authorization.AuthorizationType,
			authorization.ResourceType,
			authorization.ResourceID,
		)
		test.That(t, resp, test.ShouldResemble, &pbAuthorization)
		test.That(t, err, test.ShouldBeNil)
		resp, err = createAuthorization(
			authorization.OrganizationID,
			authorization.IdentityID,
			authorization.IdentityType,
			badAuthorizationType,
			authorization.ResourceType,
			authorization.ResourceID,
		)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "role string must be 'owner' or 'operator'")
		resp, err = createAuthorization(
			authorization.OrganizationID,
			authorization.IdentityID,
			authorization.IdentityType,
			authorization.AuthorizationType,
			badResourceType,
			authorization.ResourceID,
		)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "resourceType must be 'organization', 'location', or 'robot'")
	})

	t.Run("AddRole", func(t *testing.T) {
		grpcClient.AddRoleFunc = func(
			ctx context.Context, in *pb.AddRoleRequest, opts ...grpc.CallOption,
		) (*pb.AddRoleResponse, error) {
			test.That(t, in.Authorization, test.ShouldResemble, &pbAuthorization)
			return &pb.AddRoleResponse{}, nil
		}
		err := client.AddRole(
			context.Background(),
			authorization.OrganizationID,
			authorization.IdentityID,
			authorization.AuthorizationType,
			authorization.ResourceType,
			authorization.ResourceID,
		)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("RemoveRole", func(t *testing.T) {
		grpcClient.RemoveRoleFunc = func(
			ctx context.Context, in *pb.RemoveRoleRequest, opts ...grpc.CallOption,
		) (*pb.RemoveRoleResponse, error) {
			test.That(t, in.Authorization, test.ShouldResemble, &pbAuthorization)
			return &pb.RemoveRoleResponse{}, nil
		}
		err := client.RemoveRole(
			context.Background(),
			authorization.OrganizationID,
			authorization.IdentityID,
			authorization.AuthorizationType,
			authorization.ResourceType,
			authorization.ResourceID,
		)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ChangeRole", func(t *testing.T) {
		grpcClient.ChangeRoleFunc = func(
			ctx context.Context, in *pb.ChangeRoleRequest, opts ...grpc.CallOption,
		) (*pb.ChangeRoleResponse, error) {
			test.That(t, in.OldAuthorization, test.ShouldResemble, &pbAuthorization)
			test.That(t, in.NewAuthorization, test.ShouldResemble, &pbAuthorization2)
			return &pb.ChangeRoleResponse{}, nil
		}
		err := client.ChangeRole(
			context.Background(),
			authorization.OrganizationID,
			authorization.IdentityID,
			authorization.AuthorizationType,
			authorization.ResourceType,
			authorization.ResourceID,
			authorization2.OrganizationID,
			authorization2.IdentityID,
			authorization2.AuthorizationType,
			authorization2.ResourceType,
			authorization2.ResourceID,
		)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ListAuthorizations", func(t *testing.T) {
		grpcClient.ListAuthorizationsFunc = func(
			ctx context.Context, in *pb.ListAuthorizationsRequest, opts ...grpc.CallOption,
		) (*pb.ListAuthorizationsResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldResemble, organizationID)
			test.That(t, in.ResourceIds, test.ShouldResemble, resourceIDs)
			return &pb.ListAuthorizationsResponse{
				Authorizations: pbAuthorizations,
			}, nil
		}
		resp, err := client.ListAuthorizations(context.Background(), organizationID, resourceIDs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, authorizations)
	})

	t.Run("CheckPermissions", func(t *testing.T) {
		pbPermissions := []*pb.AuthorizedPermissions{
			{
				ResourceType: permissions[0].ResourceType,
				ResourceId:   permissions[0].ResourceID,
				Permissions:  permissions[0].Permissions,
			},
		}
		grpcClient.CheckPermissionsFunc = func(
			ctx context.Context, in *pb.CheckPermissionsRequest, opts ...grpc.CallOption,
		) (*pb.CheckPermissionsResponse, error) {
			test.That(t, in.Permissions, test.ShouldResemble, pbPermissions)
			return &pb.CheckPermissionsResponse{
				AuthorizedPermissions: pbPermissions,
			}, nil
		}
		resp, err := client.CheckPermissions(context.Background(), permissions)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, permissions)
	})

	t.Run("GetRegistryItem", func(t *testing.T) {
		grpcClient.GetRegistryItemFunc = func(
			ctx context.Context, in *pb.GetRegistryItemRequest, opts ...grpc.CallOption,
		) (*pb.GetRegistryItemResponse, error) {
			test.That(t, in.ItemId, test.ShouldResemble, itemID)
			return &pb.GetRegistryItemResponse{
				Item: pbRegistryItem,
			}, nil
		}
		resp, err := client.GetRegistryItem(context.Background(), itemID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, &registryItem)
	})

	t.Run("CreateRegistryItem", func(t *testing.T) {
		grpcClient.CreateRegistryItemFunc = func(
			ctx context.Context, in *pb.CreateRegistryItemRequest, opts ...grpc.CallOption,
		) (*pb.CreateRegistryItemResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldResemble, registryItem.OrganizationID)
			test.That(t, in.Name, test.ShouldResemble, registryItem.Name)
			test.That(t, in.Type, test.ShouldResemble, pbRegistryItem.Type)
			return &pb.CreateRegistryItemResponse{}, nil
		}
		err := client.CreateRegistryItem(context.Background(), registryItem.OrganizationID, registryItem.Name, registryItem.Type)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("UpdateRegistryItem", func(t *testing.T) {
		grpcClient.UpdateRegistryItemFunc = func(
			ctx context.Context, in *pb.UpdateRegistryItemRequest, opts ...grpc.CallOption,
		) (*pb.UpdateRegistryItemResponse, error) {
			test.That(t, in.ItemId, test.ShouldResemble, itemID)
			test.That(t, in.Type, test.ShouldResemble, packageTypeToProto(packageType))
			test.That(t, in.Description, test.ShouldResemble, description)
			test.That(t, in.Visibility, test.ShouldResemble, visibilityToProto(visibility))
			test.That(t, in.Url, test.ShouldResemble, &siteURL)
			return &pb.UpdateRegistryItemResponse{}, nil
		}
		err := client.UpdateRegistryItem(context.Background(), registryItem.ItemID, packageType, description, visibility, &siteURL)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ListRegistryItems", func(t *testing.T) {
		platforms := []string{platform}
		namespaces := []string{namespace}
		expectedRegistryItems := []*RegistryItem{&registryItem}
		grpcClient.ListRegistryItemsFunc = func(
			ctx context.Context, in *pb.ListRegistryItemsRequest, opts ...grpc.CallOption,
		) (*pb.ListRegistryItemsResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldResemble, &organizationID)
			test.That(t, in.Types, test.ShouldResemble, []packages.PackageType{packageTypeToProto(packageType)})
			test.That(t, in.Visibilities, test.ShouldResemble, []pb.Visibility{visibilityToProto(visibility)})
			test.That(t, in.Platforms, test.ShouldResemble, platforms)
			test.That(t, in.Statuses, test.ShouldResemble, []pb.RegistryItemStatus{pb.RegistryItemStatus(registryItemStatus)})
			test.That(t, in.SearchTerm, test.ShouldResemble, &searchTerm)
			test.That(t, in.PageToken, test.ShouldResemble, &pageToken)
			test.That(t, in.PublicNamespaces, test.ShouldResemble, namespaces)
			return &pb.ListRegistryItemsResponse{
				Items: []*pb.RegistryItem{pbRegistryItem},
			}, nil
		}
		resp, err := client.ListRegistryItems(
			context.Background(),
			&organizationID,
			[]PackageType{packageType},
			[]Visibility{visibility},
			platforms,
			[]RegistryItemStatus{registryItemStatus},
			&searchTerm,
			&pageToken,
			namespaces,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, expectedRegistryItems)
	})

	t.Run("DeleteRegistryItem", func(t *testing.T) {
		grpcClient.DeleteRegistryItemFunc = func(
			ctx context.Context, in *pb.DeleteRegistryItemRequest, opts ...grpc.CallOption,
		) (*pb.DeleteRegistryItemResponse, error) {
			test.That(t, in.ItemId, test.ShouldResemble, itemID)
			return &pb.DeleteRegistryItemResponse{}, nil
		}
		err := client.DeleteRegistryItem(context.Background(), registryItem.ItemID)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("TransferRegistryItem", func(t *testing.T) {
		grpcClient.TransferRegistryItemFunc = func(
			ctx context.Context, in *pb.TransferRegistryItemRequest, opts ...grpc.CallOption,
		) (*pb.TransferRegistryItemResponse, error) {
			test.That(t, in.ItemId, test.ShouldResemble, itemID)
			test.That(t, in.NewPublicNamespace, test.ShouldResemble, namespace)
			return &pb.TransferRegistryItemResponse{}, nil
		}
		err := client.TransferRegistryItem(context.Background(), registryItem.ItemID, namespace)
		test.That(t, err, test.ShouldBeNil)
	})
}
