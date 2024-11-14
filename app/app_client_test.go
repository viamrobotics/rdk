package app

import (
	"context"
	"fmt"
	"testing"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	organizationID = "organization_id"
	email = "email"
	userID = "user_id"
	locationID = "location_id"
	available = true
	authorizationType = "authorization_type"
	authorizationID = "authorization_id"
	resourceType = "resource_type"
	resourceID = "resource_id"
	identityID = "identity_id"
	identityType = "identity_type"
	secretID = "secret_ids"
	primary = true
	robotCount = 1
)

var (
	name = "name"
	region = "region"
	namespace = "public_namespace"
	cid = "cid"
	dateAdded = timestamppb.Timestamp{Seconds: 0, Nanos: 50}
	organization = Organization{
		ID: organizationID,
		Name: name,
		CreatedOn: &dateAdded,
		PublicNamespace: namespace,
		DefaultRegion: region,
		Cid: &cid,
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
		ID: organizationID,
		Name: name,
	}
	orgDetails = OrgDetails{
		OrgID: organizationID,
		OrgName: name,
	}
	lastLogin = timestamppb.Timestamp{Seconds: 0, Nanos: 100}
	createdOn = timestamppb.Timestamp{Seconds: 0, Nanos: 0}
	authorization = Authorization{
		AuthorizationType: authorizationType,
		AuthorizationID: authorizationID,
		ResourceType: resourceType,
		ResourceID: resourceID,
		IdentityID: identityID,
		OrganizationID: organizationID,
		IdentityType: identityType,
	}
	pbAuthorization = pb.Authorization{
		AuthorizationType: authorization.AuthorizationType,
		AuthorizationId: authorization.AuthorizationID,
		ResourceType: authorization.ResourceType,
		ResourceId: authorization.ResourceID,
		IdentityId: authorization.IdentityID,
		OrganizationId: authorization.OrganizationID,
		IdentityType: authorization.IdentityType,
	}
	authorizations = []*Authorization{&authorization}
	pbAuthorizations = []*pb.Authorization{&pbAuthorization}
	member = OrganizationMember{
		UserID: userID,
		Emails: []string{email},
		DateAdded: &dateAdded,
		LastLogin: &lastLogin,
	}
	invite = OrganizationInvite{
		OrganizationID: organizationID,
		Email: email,
		CreatedOn: &createdOn,
		Authorizations: authorizations,
	}
	pbInvite = pb.OrganizationInvite{
		OrganizationId: invite.OrganizationID,
		Email: invite.Email,
		CreatedOn: invite.CreatedOn,
		Authorizations: pbAuthorizations,
	}
	sendEmailInvite = true
	addressLine2 = "address_line_2"
	address = BillingAddress{
		AddressLine_1: "address_line_1",
		AddressLine_2: &addressLine2,
		City: "city",
		State: "state",
	}
	pbAddress = pb.BillingAddress{
		AddressLine_1: address.AddressLine_1,
		AddressLine_2: address.AddressLine_2,
		City: address.City,
		State: address.State,
	}
	parentLocationID = "parent_location_id"
	secret = SharedSecret{
		ID: secretID,
		CreatedOn: &createdOn,
		State: SharedSecretStateEnabled,
	}
	pbSharedSecretState, _ = sharedSecretStateToProto(secret.State)
	pbSecret = pb.SharedSecret{
		Id: secret.ID,
		CreatedOn: secret.CreatedOn,
		State: pbSharedSecretState,
	}
	locationAuth = LocationAuth{
		LocationID: locationID,
		Secrets: []*SharedSecret{&secret},
	}
	pbLocationAuth = pb.LocationAuth{
		LocationId: locationAuth.LocationID,
		Secrets: []*pb.SharedSecret{&pbSecret},
	}
	locationOrg = LocationOrganization{
		OrganizationID: organizationID,
		Primary: primary,
	}
	pbLocationOrg = pb.LocationOrganization{
		OrganizationId: locationOrg.OrganizationID,
		Primary: locationOrg.Primary,
	}
	storageConfig = StorageConfig{
		Region: region,
	}
	pbStorageConfig = pb.StorageConfig{
		Region: storageConfig.Region,
	}
	location = Location{
		ID: locationID,
		Name: name,
		ParentLocationID: parentLocationID,
		Auth: &locationAuth,
		Organizations: []*LocationOrganization{&locationOrg},
		CreatedOn: &createdOn,
		RobotCount: robotCount,
		Config: &storageConfig,
	}
	pbLocation = pb.Location{
		Id: location.ID,
		Name: location.Name,
		ParentLocationId: location.ParentLocationID,
		Auth: &pbLocationAuth,
		Organizations: []*pb.LocationOrganization{&pbLocationOrg},
		CreatedOn: location.CreatedOn,
		RobotCount: location.RobotCount,
		Config: &pbStorageConfig,
	}
)

func sharedSecretStateToProto(state SharedSecretState) (pb.SharedSecret_State, error) {
	switch state {
	case SharedSecretStateUnspecified:
		return pb.SharedSecret_STATE_UNSPECIFIED, nil
	case SharedSecretStateEnabled:
		return pb.SharedSecret_STATE_ENABLED, nil
	case SharedSecretStateDisabled:
		return pb.SharedSecret_STATE_DISABLED, nil
	default:
		return 0, fmt.Errorf("uknown secret state: %v", state)
	}
}

func createGrpcClient() *inject.AppServiceClient {
	return &inject.AppServiceClient{}
}

func TestAppClient(t *testing.T) {
	grpcClient := createGrpcClient()
	client := Client{client: grpcClient}

	t.Run("GetUserIDByEmail", func(t *testing.T) {
		grpcClient.GetUserIDByEmailFunc = func(ctx context.Context, in *pb.GetUserIDByEmailRequest, opts ...grpc.CallOption) (*pb.GetUserIDByEmailResponse, error) {
			test.That(t, in.Email, test.ShouldEqual, email)
			return &pb.GetUserIDByEmailResponse{
				UserId: userID,
			}, nil
		}
		resp, _ := client.GetUserIDByEmail(context.Background(), email)
		test.That(t, resp, test.ShouldEqual, userID)
	})

	t.Run("CreateOrganization", func(t *testing.T) {
		grpcClient.CreateOrganizationFunc = func(ctx context.Context, in *pb.CreateOrganizationRequest, opts ...grpc.CallOption) (*pb.CreateOrganizationResponse, error) {
			test.That(t, in.Name, test.ShouldEqual, name)
			return &pb.CreateOrganizationResponse{
				Organization: &pbOrganization,
			}, nil
		}
		resp, _ := client.CreateOrganization(context.Background(), name)
		test.That(t, resp.ID, test.ShouldEqual, organization.ID)
		test.That(t, resp.Name, test.ShouldEqual, organization.Name)
		test.That(t, resp.PublicNamespace, test.ShouldEqual, organization.PublicNamespace)
		test.That(t, resp.DefaultRegion, test.ShouldEqual, organization.DefaultRegion)
		test.That(t, resp.Cid, test.ShouldEqual, organization.Cid)
	})

	t.Run("ListOrganizations", func(t *testing.T) {
		organizations := []Organization{organization}
		grpcClient.ListOrganizationsFunc = func(ctx context.Context, in *pb.ListOrganizationsRequest, opts ...grpc.CallOption) (*pb.ListOrganizationsResponse, error) {
			return &pb.ListOrganizationsResponse{
				Organizations: []*pb.Organization{&pbOrganization},
			}, nil
		}
		resp, _ := client.ListOrganizations(context.Background())
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp[0].ID, test.ShouldEqual, organizations[0].ID)
		test.That(t, resp[0].Name, test.ShouldEqual, organizations[0].Name)
		test.That(t, resp[0].PublicNamespace, test.ShouldEqual, organizations[0].PublicNamespace)
		test.That(t, resp[0].DefaultRegion, test.ShouldEqual, organizations[0].DefaultRegion)
		test.That(t, resp[0].Cid, test.ShouldEqual, organizations[0].Cid)
	})

	t.Run("GetOrganizationsWithAccessToLocation", func(t *testing.T) {
		pbOrganizationIdentity := pb.OrganizationIdentity{
			Id: organizationIdentity.ID,
			Name: organizationIdentity.Name,
		}
		grpcClient.GetOrganizationsWithAccessToLocationFunc = func(ctx context.Context, in *pb.GetOrganizationsWithAccessToLocationRequest, opts ...grpc.CallOption) (*pb.GetOrganizationsWithAccessToLocationResponse, error) {
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
			OrgId: orgDetails.OrgID,
			OrgName: orgDetails.OrgName,
		}
		grpcClient.ListOrganizationsByUserFunc = func(ctx context.Context, in *pb.ListOrganizationsByUserRequest, opts ...grpc.CallOption) (*pb.ListOrganizationsByUserResponse, error) {
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
		grpcClient.GetOrganizationFunc = func(ctx context.Context, in *pb.GetOrganizationRequest, opts ...grpc.CallOption) (*pb.GetOrganizationResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.GetOrganizationResponse{
				Organization: &pbOrganization,
			}, nil
		}
		resp, _ := client.GetOrganization(context.Background(), organizationID)
		test.That(t, resp.ID, test.ShouldEqual, organization.ID)
		test.That(t, resp.Name, test.ShouldEqual, organization.Name)
		test.That(t, resp.PublicNamespace, test.ShouldEqual, organization.PublicNamespace)
		test.That(t, resp.DefaultRegion, test.ShouldEqual, organization.DefaultRegion)
		test.That(t, resp.Cid, test.ShouldEqual, organization.Cid)
	})

	t.Run("GetOrganizationNamespaceAvailability", func(t *testing.T) {
		grpcClient.GetOrganizationNamespaceAvailabilityFunc = func(ctx context.Context, in *pb.GetOrganizationNamespaceAvailabilityRequest, opts ...grpc.CallOption) (*pb.GetOrganizationNamespaceAvailabilityResponse, error) {
			test.That(t, in.PublicNamespace, test.ShouldEqual, namespace)
			return &pb.GetOrganizationNamespaceAvailabilityResponse{
				Available: available,
			}, nil
		}
		resp, _ := client.GetOrganizationNamespaceAvailability(context.Background(), namespace)
		test.That(t, resp, test.ShouldEqual, available)
	})

	t.Run("UpdateOrganization", func(t *testing.T) {
		grpcClient.UpdateOrganizationFunc = func(ctx context.Context, in *pb.UpdateOrganizationRequest, opts ...grpc.CallOption) (*pb.UpdateOrganizationResponse, error) {
			test.That(t, in.PublicNamespace, test.ShouldEqual, &namespace)
			return &pb.UpdateOrganizationResponse{
				Organization: &pbOrganization,
			}, nil
		}
		resp, _ := client.UpdateOrganization(context.Background(), organizationID, &name, &namespace, &region, &cid)
		test.That(t, resp.ID, test.ShouldEqual, organization.ID)
		test.That(t, resp.Name, test.ShouldEqual, organization.Name)
		test.That(t, resp.PublicNamespace, test.ShouldEqual, organization.PublicNamespace)
		test.That(t, resp.DefaultRegion, test.ShouldEqual, organization.DefaultRegion)
		test.That(t, resp.Cid, test.ShouldEqual, organization.Cid)
	})

	t.Run("DeleteOrganization", func(t *testing.T) {
		grpcClient.DeleteOrganizationFunc = func(ctx context.Context, in *pb.DeleteOrganizationRequest, opts ...grpc.CallOption) (*pb.DeleteOrganizationResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.DeleteOrganizationResponse{}, nil
		}
		client.DeleteOrganization(context.Background(), organizationID)
	})

	t.Run("ListOrganizationMembers", func(t *testing.T) {
		expectedMembers := []OrganizationMember{member}
		pbMember := pb.OrganizationMember{
			UserId: member.UserID,
			Emails: member.Emails,
			DateAdded: member.DateAdded,
			LastLogin: member.LastLogin,
		}
		expectedInvites := []OrganizationInvite{invite}
		grpcClient.ListOrganizationMembersFunc = func(ctx context.Context, in *pb.ListOrganizationMembersRequest, opts ...grpc.CallOption) (*pb.ListOrganizationMembersResponse, error) {
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
		grpcClient.CreateOrganizationInviteFunc = func(ctx context.Context, in *pb.CreateOrganizationInviteRequest, opts ...grpc.CallOption) (*pb.CreateOrganizationInviteResponse, error) {
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
		grpcClient.UpdateOrganizationInviteAuthorizationsFunc = func(ctx context.Context, in *pb.UpdateOrganizationInviteAuthorizationsRequest, opts ...grpc.CallOption) (*pb.UpdateOrganizationInviteAuthorizationsResponse, error) {
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
		grpcClient.DeleteOrganizationMemberFunc = func(ctx context.Context, in *pb.DeleteOrganizationMemberRequest, opts ...grpc.CallOption) (*pb.DeleteOrganizationMemberResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.UserId, test.ShouldEqual, userID)
			return &pb.DeleteOrganizationMemberResponse{}, nil
		}
		client.DeleteOrganizationMember(context.Background(), organizationID, userID)
	})

	t.Run("DeleteOrganizationInvite", func(t *testing.T) {
		grpcClient.DeleteOrganizationInviteFunc = func(ctx context.Context, in *pb.DeleteOrganizationInviteRequest, opts ...grpc.CallOption) (*pb.DeleteOrganizationInviteResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.Email, test.ShouldEqual, email)
			return &pb.DeleteOrganizationInviteResponse{}, nil
		}
		client.DeleteOrganizationInvite(context.Background(), organizationID, email)
	})

	t.Run("ResendOrganizationInvite", func(t *testing.T) {
		grpcClient.ResendOrganizationInviteFunc = func(ctx context.Context, in *pb.ResendOrganizationInviteRequest, opts ...grpc.CallOption) (*pb.ResendOrganizationInviteResponse, error) {
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
		grpcClient.EnableBillingServiceFunc = func(ctx context.Context, in *pb.EnableBillingServiceRequest, opts ...grpc.CallOption) (*pb.EnableBillingServiceResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			test.That(t, in.BillingAddress, test.ShouldResemble, &pbAddress)
			return &pb.EnableBillingServiceResponse{}, nil
		}
		client.EnableBillingService(context.Background(), organizationID, &address)
	})

	t.Run("DisableBillingService", func(t *testing.T) {
		grpcClient.DisableBillingServiceFunc = func(ctx context.Context, in *pb.DisableBillingServiceRequest, opts ...grpc.CallOption) (*pb.DisableBillingServiceResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return &pb.DisableBillingServiceResponse{}, nil
		}
		client.DisableBillingService(context.Background(), organizationID)
	})

	t.Run("UpdateBillingService", func(t *testing.T) {
		grpcClient.UpdateBillingServiceFunc = func(ctx context.Context, in *pb.UpdateBillingServiceRequest, opts ...grpc.CallOption) (*pb.UpdateBillingServiceResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			test.That(t, in.BillingAddress, test.ShouldResemble, &pbAddress)
			test.That(t, in.BillingSupportEmail, test.ShouldResemble, email)
			return &pb.UpdateBillingServiceResponse{}, nil
		}
		client.UpdateBillingService(context.Background(), organizationID, &address, email)
	})

	t.Run("OrganizationSetSupportEmail", func(t *testing.T) {
		grpcClient.OrganizationSetSupportEmailFunc = func(ctx context.Context, in *pb.OrganizationSetSupportEmailRequest, opts ...grpc.CallOption) (*pb.OrganizationSetSupportEmailResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			test.That(t, in.Email, test.ShouldResemble, email)
			return &pb.OrganizationSetSupportEmailResponse{}, nil
		}
		client.OrganizationSetSupportEmail(context.Background(), organizationID, email)
	})

	t.Run("OrganizationGetSupportEmail", func(t *testing.T) {
		grpcClient.OrganizationGetSupportEmailFunc = func(ctx context.Context, in *pb.OrganizationGetSupportEmailRequest, opts ...grpc.CallOption) (*pb.OrganizationGetSupportEmailResponse, error) {
			test.That(t, in.OrgId, test.ShouldEqual, organizationID)
			return &pb.OrganizationGetSupportEmailResponse{
				Email: email,
			}, nil
		}
		resp, _ := client.OrganizationGetSupportEmail(context.Background(), organizationID)
		test.That(t, resp, test.ShouldEqual, email)
	})

	t.Run("CreateLocation", func(t *testing.T) {
		grpcClient.CreateLocationFunc = func(ctx context.Context, in *pb.CreateLocationRequest, opts ...grpc.CallOption) (*pb.CreateLocationResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			test.That(t, in.Name, test.ShouldEqual, name)
			test.That(t, in.ParentLocationId, test.ShouldEqual, &parentLocationID)
			return &pb.CreateLocationResponse{
				Location: &pbLocation,
			}, nil
		}
		resp, _ := client.CreateLocation(context.Background(), organizationID, name, &parentLocationID)
		test.That(t, resp.ID, test.ShouldEqual, location.ID)
		test.That(t, resp.Name, test.ShouldEqual, location.Name)
		test.That(t, resp.ParentLocationID, test.ShouldEqual, location.ParentLocationID)
		test.That(t, resp.Auth, test.ShouldResemble, location.Auth)
		test.That(t, resp.Organizations, test.ShouldResemble, location.Organizations)
		test.That(t, resp.CreatedOn, test.ShouldEqual, location.CreatedOn)
		test.That(t, resp.RobotCount, test.ShouldEqual, location.RobotCount)
		test.That(t, resp.Config, test.ShouldResemble, location.Config)
	})

	t.Run("GetLocation", func(t *testing.T) {
		grpcClient.GetLocationFunc = func(ctx context.Context, in *pb.GetLocationRequest, opts ...grpc.CallOption) (*pb.GetLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.GetLocationResponse{
				Location: &pbLocation,
			}, nil
		}
		resp, _ := client.GetLocation(context.Background(), locationID)
		test.That(t, resp.ID, test.ShouldEqual, location.ID)
		test.That(t, resp.Name, test.ShouldEqual, location.Name)
		test.That(t, resp.ParentLocationID, test.ShouldEqual, location.ParentLocationID)
		test.That(t, resp.Auth, test.ShouldResemble, location.Auth)
		test.That(t, resp.Organizations, test.ShouldResemble, location.Organizations)
		test.That(t, resp.CreatedOn, test.ShouldEqual, location.CreatedOn)
		test.That(t, resp.RobotCount, test.ShouldEqual, location.RobotCount)
		test.That(t, resp.Config, test.ShouldResemble, location.Config)
	})

	t.Run("UpdateLocation", func(t *testing.T) {
		grpcClient.UpdateLocationFunc = func(ctx context.Context, in *pb.UpdateLocationRequest, opts ...grpc.CallOption) (*pb.UpdateLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			test.That(t, in.Name, test.ShouldEqual, &name)
			test.That(t, in.ParentLocationId, test.ShouldEqual, &parentLocationID)
			test.That(t, in.Region, test.ShouldEqual, &region)
			return &pb.UpdateLocationResponse{
				Location: &pbLocation,
			}, nil
		}
		resp, _ := client.UpdateLocation(context.Background(), locationID, &name, &parentLocationID, &region)
		test.That(t, resp.ID, test.ShouldEqual, location.ID)
		test.That(t, resp.Name, test.ShouldEqual, location.Name)
		test.That(t, resp.ParentLocationID, test.ShouldEqual, location.ParentLocationID)
		test.That(t, resp.Auth, test.ShouldResemble, location.Auth)
		test.That(t, resp.Organizations, test.ShouldResemble, location.Organizations)
		test.That(t, resp.CreatedOn, test.ShouldEqual, location.CreatedOn)
		test.That(t, resp.RobotCount, test.ShouldEqual, location.RobotCount)
		test.That(t, resp.Config, test.ShouldResemble, location.Config)
	})

	t.Run("DeleteLocation", func(t *testing.T) {
		grpcClient.DeleteLocationFunc = func(ctx context.Context, in *pb.DeleteLocationRequest, opts ...grpc.CallOption) (*pb.DeleteLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.DeleteLocationResponse{}, nil
		}
		client.DeleteLocation(context.Background(), locationID)
	})

	t.Run("ListLocations", func(t *testing.T) {
		expectedLocations := []Location{location}
		grpcClient.ListLocationsFunc = func(ctx context.Context, in *pb.ListLocationsRequest, opts ...grpc.CallOption) (*pb.ListLocationsResponse, error) {
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.ListLocationsResponse{
				Locations: []*pb.Location{&pbLocation},
			}, nil
		}
		resp, _ := client.ListLocations(context.Background(), organizationID)
		test.That(t, len(resp), test.ShouldEqual, len(expectedLocations))
		test.That(t, resp[0].ID, test.ShouldEqual, expectedLocations[0].ID)
		test.That(t, resp[0].Name, test.ShouldEqual, expectedLocations[0].Name)
		test.That(t, resp[0].ParentLocationID, test.ShouldEqual, expectedLocations[0].ParentLocationID)
		test.That(t, resp[0].Auth, test.ShouldResemble, expectedLocations[0].Auth)
		test.That(t, resp[0].Organizations, test.ShouldResemble, expectedLocations[0].Organizations)
		test.That(t, resp[0].CreatedOn, test.ShouldEqual, expectedLocations[0].CreatedOn)
		test.That(t, resp[0].RobotCount, test.ShouldEqual, expectedLocations[0].RobotCount)
		test.That(t, resp[0].Config, test.ShouldResemble, expectedLocations[0].Config)
	})

	t.Run("ShareLocation", func(t *testing.T) {
		grpcClient.ShareLocationFunc = func(ctx context.Context, in *pb.ShareLocationRequest, opts ...grpc.CallOption) (*pb.ShareLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.ShareLocationResponse{}, nil
		}
		client.ShareLocation(context.Background(), locationID, organizationID)
	})

	t.Run("UnshareLocation", func(t *testing.T) {
		grpcClient.UnshareLocationFunc = func(ctx context.Context, in *pb.UnshareLocationRequest, opts ...grpc.CallOption) (*pb.UnshareLocationResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			test.That(t, in.OrganizationId, test.ShouldEqual, organizationID)
			return &pb.UnshareLocationResponse{}, nil
		}
		client.UnshareLocation(context.Background(), locationID, organizationID)
	})

	t.Run("LocationAuth", func(t *testing.T) {
		grpcClient.LocationAuthFunc = func(ctx context.Context, in *pb.LocationAuthRequest, opts ...grpc.CallOption) (*pb.LocationAuthResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.LocationAuthResponse{
				Auth: &pbLocationAuth,
			}, nil
		}
		resp, _ := client.LocationAuth(context.Background(), locationID)
		test.That(t, resp, test.ShouldResemble, &locationAuth)
	})

	t.Run("CreateLocationSecret", func(t *testing.T) {
		grpcClient.CreateLocationSecretFunc = func(ctx context.Context, in *pb.CreateLocationSecretRequest, opts ...grpc.CallOption) (*pb.CreateLocationSecretResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			return &pb.CreateLocationSecretResponse{
				Auth: &pbLocationAuth,
			}, nil
		}
		resp, _ := client.CreateLocationSecret(context.Background(), locationID)
		test.That(t, resp, test.ShouldResemble, &locationAuth)
	})

	t.Run("DeleteLocationSecret", func(t *testing.T) {
		grpcClient.DeleteLocationSecretFunc = func(ctx context.Context, in *pb.DeleteLocationSecretRequest, opts ...grpc.CallOption) (*pb.DeleteLocationSecretResponse, error) {
			test.That(t, in.LocationId, test.ShouldEqual, locationID)
			test.That(t, in.SecretId, test.ShouldEqual, secretID)
			return &pb.DeleteLocationSecretResponse{}, nil
		}
		client.DeleteLocationSecret(context.Background(), locationID, secretID)
	})
}
