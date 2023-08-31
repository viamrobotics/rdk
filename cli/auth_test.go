package cli

import (
	"context"
	"fmt"
	"strings"
	"testing"

	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

func TestLoginAction(t *testing.T) {
	cCtx, ac, out, errOut := setup(nil, nil)

	test.That(t, ac.loginAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring,
		fmt.Sprintf("already logged in as %q", testEmail))
}

func TestPrintAccessTokenAction(t *testing.T) {
	// AppServiceClient needed for any Action that calls ensureLoggedIn.
	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil)

	test.That(t, ac.printAccessTokenAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, testToken)
}

func TestAPIKeyCreateAction(t *testing.T) {
	// AppServiceClient needed for any Action that calls ensureLoggedIn.
	createKeyFunc := func(ctx context.Context, in *apppb.CreateKeyRequest,
		opts ...grpc.CallOption,
	) (*apppb.CreateKeyResponse, error) {
		return &apppb.CreateKeyResponse{Id: "id-xxx", Key: "key-yyy"}, nil
	}
	asc := &inject.AppServiceClient{
		CreateKeyFunc: createKeyFunc,
	}
	cCtx, ac, out, errOut := setup(asc, nil)

	test.That(t, ac.organizationAPIKeyCreateAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 8)
	test.That(t, strings.Join(out.messages, ""), test.ShouldContainSubstring, "id-xxx")
	test.That(t, strings.Join(out.messages, ""), test.ShouldContainSubstring, "key-yyy")
}

func TestLogoutAction(t *testing.T) {
	cCtx, ac, out, errOut := setup(nil, nil)

	test.That(t, ac.logoutAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring,
		fmt.Sprintf("logged out from %q", testEmail))
}

func TestWhoAmIAction(t *testing.T) {
	cCtx, ac, out, errOut := setup(nil, nil)

	test.That(t, ac.whoAmIAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, testEmail)
}
