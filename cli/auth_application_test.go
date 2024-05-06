package cli

import (
	"context"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"testing"
)

func TestRegisterAuthApplicationAction(t *testing.T) {
	registerAuthApplicationFunc := func(ctx context.Context, in *apppb.RegisterAuthApplicationRequest,
		opts ...grpc.CallOption,
	) (*apppb.RegisterAuthApplicationResponse, error) {
		return &apppb.RegisterAuthApplicationResponse{
			ApplicationId:   "c6215428-1b73-41c3-b44a-56db0631c8f1",
			ApplicationName: in.ApplicationName,
			Secret:          "reallysecretsecret",
		}, nil
	}

	asc := &inject.EndUserServiceClient{
		RegisterAuthApplicationFunc: registerAuthApplicationFunc,
	}
	flags := make(map[string]any)
	flags[generalFlagOrgID] = "a757fe30-5648-4c5b-ab74-4ecd6bf06e4c"
	flags[authApplicationFlagName] = "pupper_app"
	flags[authApplicationFlagOriginURIs] = []string{"https://woof.com/login", "https://arf.com/"}
	flags[authApplicationFlagRedirectURIs] = []string{"https://woof.com/home", "https://arf.com/home"}
	flags[authApplicationFlagLogoutURI] = "https://woof.com/logout"

	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, nil, asc, "token", flags)
	err := ac.registerAuthApplicationAction(cCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 5)
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

	asc := &inject.EndUserServiceClient{
		UpdateAuthApplicationFunc: updateAuthApplication,
	}
	flags := make(map[string]any)
	flags[generalFlagOrgID] = "a757fe30-5648-4c5b-ab74-4ecd6bf06e4c"
	flags[authApplicationFlagApplicationID] = "a673022c-9916-4238-b8eb-4f7a89885909"
	flags[authApplicationFlagName] = "pupper_app"
	flags[authApplicationFlagOriginURIs] = []string{"https://woof.com/login", "https://arf.com/"}
	flags[authApplicationFlagRedirectURIs] = []string{"https://woof.com/home", "https://arf.com/home"}
	flags[authApplicationFlagLogoutURI] = "https://woof.com/logout"

	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, nil, asc, "token", flags)
	err := ac.updateAuthApplicationAction(cCtx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 3)
}
