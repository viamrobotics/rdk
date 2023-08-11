package cli

import (
	"fmt"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/testutils/inject"
)

func TestLoginAction(t *testing.T) {
	ctx, out, errOut := setup(nil)

	test.That(t, LoginAction(ctx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring,
		fmt.Sprintf("already logged in as %q", testEmail))
}

func TestPrintAccessTokenAction(t *testing.T) {
	// asc needed for any Action that calls ensureLoggedIn.
	asc := &inject.AppServiceClient{}
	ctx, out, errOut := setup(asc)

	test.That(t, PrintAccessTokenAction(ctx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, testToken)
}

func TestLogoutAction(t *testing.T) {
	ctx, out, errOut := setup(nil)

	test.That(t, LogoutAction(ctx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring,
		fmt.Sprintf("logged out from %q", testEmail))
}

func TestWhoAmIAction(t *testing.T) {
	ctx, out, errOut := setup(nil)

	test.That(t, WhoAmIAction(ctx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, testEmail)
}
