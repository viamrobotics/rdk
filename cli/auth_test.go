package cli

import (
	"context"
	"encoding/json"
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
		fmt.Sprintf("Already logged in as %q", testEmail))
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
	createKeyFunc := func(ctx context.Context, in *apppb.CreateKeyRequest,
		opts ...grpc.CallOption,
	) (*apppb.CreateKeyResponse, error) {
		return &apppb.CreateKeyResponse{Id: "id-xxx", Key: "key-yyy"}, nil
	}
	asc := &inject.AppServiceClient{
		CreateKeyFunc: createKeyFunc,
	}
	cCtx, ac, out, errOut := setup(asc, nil)

	test.That(t, ac.organizationsAPIKeyCreateAction(cCtx), test.ShouldBeNil)
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
		fmt.Sprintf("Logged out from %q", testEmail))
}

func TestWhoAmIAction(t *testing.T) {
	cCtx, ac, out, errOut := setup(nil, nil)

	test.That(t, ac.whoAmIAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring, testEmail)
}

func TestConfigMarshalling(t *testing.T) {
	t.Run("token config", func(t *testing.T) {
		conf := config{
			BaseURL: "https://guthib.com:443",
			Auth: &token{
				AccessToken: "secret-token",
				User: userData{
					Email:   "tipsy@viam.com",
					Subject: "MAIV",
				},
			},
		}

		bytes, err := json.Marshal(conf)
		test.That(t, err, test.ShouldBeNil)
		var newConf config
		test.That(t, newConf.tryUnmarshallWithAPIKey(bytes), test.ShouldBeError)
		test.That(t, newConf.tryUnmarshallWithToken(bytes), test.ShouldBeNil)
		test.That(t, newConf.BaseURL, test.ShouldEqual, "https://guthib.com:443")
		auth, ok := newConf.Auth.(*token)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, auth.AccessToken, test.ShouldEqual, "secret-token")
		test.That(t, auth.User.Email, test.ShouldEqual, "tipsy@viam.com")
		test.That(t, auth.User.Subject, test.ShouldEqual, "MAIV")
	})

	t.Run("api-key config", func(t *testing.T) {
		conf := config{
			BaseURL: "https://docs.viam.com:443",
			Auth: &apiKey{
				KeyID:     "42",
				KeyCrypto: "secret",
			},
		}

		bytes, err := json.Marshal(conf)
		test.That(t, err, test.ShouldBeNil)
		var newConf config
		test.That(t, newConf.tryUnmarshallWithToken(bytes), test.ShouldBeError)
		test.That(t, newConf.tryUnmarshallWithAPIKey(bytes), test.ShouldBeNil)
		test.That(t, newConf.BaseURL, test.ShouldEqual, "https://docs.viam.com:443")
		auth, ok := newConf.Auth.(*apiKey)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, auth.KeyID, test.ShouldEqual, "42")
		test.That(t, auth.KeyCrypto, test.ShouldEqual, "secret")
	})
}
