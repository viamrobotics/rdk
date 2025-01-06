package cli

import (
	"context"
	"testing"

	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

func TestRegisterAuthApplicationAction(t *testing.T) {
	registerAuthApplicationFunc := func(ctx context.Context, in *apppb.RegisterAuthApplicationRequest,
		opts ...grpc.CallOption,
	) (*apppb.RegisterAuthApplicationResponse, error) {
		return &apppb.RegisterAuthApplicationResponse{
			ApplicationId:   "c6215428-1b73-41c3-b44a-56db0631c8f1",
			ApplicationName: in.ApplicationName,
			ClientSecret:    "reallysecretsecret",
		}, nil
	}

	eusc := &inject.EndUserServiceClient{
		RegisterAuthApplicationFunc: registerAuthApplicationFunc,
	}
	flags := make(map[string]any)
	flags[generalFlagOrgID] = "a757fe30-5648-4c5b-ab74-4ecd6bf06e4c"
	flags[authApplicationFlagName] = "pupper_app"
	flags[authApplicationFlagOriginURIs] = []string{"https://woof.com/login", "https://arf.com/"}
	flags[authApplicationFlagRedirectURIs] = []string{"https://woof.com/home", "https://arf.com/home"}
	flags[authApplicationFlagLogoutURI] = "https://woof.com/logout"

	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, nil, eusc, flags, "token")
	err := ac.registerAuthApplicationAction(cCtx, parseStructFromCtx[registerAuthApplicationArgs](cCtx))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 5)

	expectedResponseString := "{\n\t\"application_id\": \"c6215428-1b73-41c3-b44a-56db0631c8f1\"," +
		"\n\t\"application_name\": \"pupper_app\",\n\t\"client_secret\": \"reallysecretsecret\"\n}\n"
	test.That(t, out.messages[2], test.ShouldEqual, expectedResponseString)
}

func TestUpdateAuthApplicationAction(t *testing.T) {
	updateAuthApplication := func(ctx context.Context, in *apppb.UpdateAuthApplicationRequest,
		opts ...grpc.CallOption,
	) (*apppb.UpdateAuthApplicationResponse, error) {
		return &apppb.UpdateAuthApplicationResponse{
			ApplicationId:   "c6215428-1b73-41c3-b44a-56db0631c8f1",
			ApplicationName: in.ApplicationName,
		}, nil
	}

	eusc := &inject.EndUserServiceClient{
		UpdateAuthApplicationFunc: updateAuthApplication,
	}
	flags := make(map[string]any)
	flags[generalFlagOrgID] = "a757fe30-5648-4c5b-ab74-4ecd6bf06e4c"
	flags[authApplicationFlagApplicationID] = "a673022c-9916-4238-b8eb-4f7a89885909"
	flags[authApplicationFlagName] = "pupper_app"
	flags[authApplicationFlagOriginURIs] = []string{"https://woof.com/login", "https://arf.com/"}
	flags[authApplicationFlagRedirectURIs] = []string{"https://woof.com/home", "https://arf.com/home"}
	flags[authApplicationFlagLogoutURI] = "https://woof.com/logout"

	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, nil, eusc, flags, "token")
	err := ac.updateAuthApplicationAction(cCtx, parseStructFromCtx[updateAuthApplicationArgs](cCtx))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 3)

	expectedResponseString := "{\n\t\"application_id\": \"c6215428-1b73-41c3-b44a-56db0631c8f1\"," +
		"\n\t\"application_name\": \"pupper_app\"\n}\n"
	test.That(t, out.messages[2], test.ShouldEqual, expectedResponseString)
}

func TestGetAuthApplicationAction(t *testing.T) {
	getAuthApplication := func(ctx context.Context, in *apppb.GetAuthApplicationRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetAuthApplicationResponse, error) {
		return &apppb.GetAuthApplicationResponse{
			ApplicationId:   "c6215428-1b73-41c3-b44a-56db0631c8f1",
			ApplicationName: "my_app",
			ClientSecret:    "supersupersecretsecret",
			OriginUris:      []string{"https://woof.com/login", "https://arf.com/"},
			RedirectUris:    []string{"https://woof.com/home", "https://arf.com/home"},
			LogoutUri:       "https://woof.com/logout",
		}, nil
	}

	eusc := &inject.EndUserServiceClient{
		GetAuthApplicationFunc: getAuthApplication,
	}
	flags := make(map[string]any)
	flags[generalFlagOrgID] = "a757fe30-5648-4c5b-ab74-4ecd6bf06e4c"
	flags[authApplicationFlagApplicationID] = "a673022c-9916-4238-b8eb-4f7a89885909"

	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, nil, eusc, flags, "token")
	err := ac.getAuthApplicationAction(cCtx, parseStructFromCtx[getAuthApplicationArgs](cCtx))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 3)

	expectedResponseString := "{\n\t\"" +
		"application_id\": \"c6215428-1b73-41c3-b44a-56db0631c8f1\"," +
		"\n\t\"application_name\": \"my_app\"," +
		"\n\t\"client_secret\": \"supersupersecretsecret\"," +
		"\n\t\"origin_uris\": [\n\t\t\"https://woof.com/login\"," +
		"\n\t\t\"https://arf.com/\"\n\t]," +
		"\n\t\"redirect_uris\": [\n\t\t\"https://woof.com/home\",\n\t\t\"https://arf.com/home\"\n\t]," +
		"\n\t\"logout_uri\": \"https://woof.com/logout\"\n}\n"
	test.That(t, out.messages[0], test.ShouldEqual, expectedResponseString)
}
