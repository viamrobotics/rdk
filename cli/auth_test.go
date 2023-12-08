package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

func TestLoginAction(t *testing.T) {
	cCtx, ac, out, errOut := setup(nil, nil, nil, nil, "token")

	test.That(t, ac.loginAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring,
		fmt.Sprintf("Already logged in as %q", testEmail))
}

func TestAPIKeyAuth(t *testing.T) {
	_, ac, _, errOut := setup(nil, nil, nil, nil, "apiKey")
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	APIKey, isAPIKey := ac.conf.Auth.(*apiKey)
	test.That(t, isAPIKey, test.ShouldBeTrue)
	test.That(t, APIKey.KeyID, test.ShouldEqual, testKeyID)
	test.That(t, APIKey.KeyCrypto, test.ShouldEqual, testKeyCrypto)
}

func TestPrintAccessTokenAction(t *testing.T) {
	// AppServiceClient needed for any Action that calls ensureLoggedIn.
	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, nil, nil, "token")

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
	cCtx, ac, out, errOut := setup(asc, nil, nil, nil, "token")

	test.That(t, ac.organizationsAPIKeyCreateAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 8)
	test.That(t, strings.Join(out.messages, ""), test.ShouldContainSubstring, "id-xxx")
	test.That(t, strings.Join(out.messages, ""), test.ShouldContainSubstring, "key-yyy")
}

func TestRobotAPIKeyCreateAction(t *testing.T) {
	createKeyFunc := func(ctx context.Context, in *apppb.CreateKeyRequest,
		opts ...grpc.CallOption,
	) (*apppb.CreateKeyResponse, error) {
		return &apppb.CreateKeyResponse{Id: "id-xxx", Key: "key-yyy"}, nil
	}

	fakeOrgID := "fake-org-id"
	fakeRobotID := "fake-robot"

	asc := &inject.AppServiceClient{
		CreateKeyFunc: createKeyFunc,
	}

	flags := make(map[string]string)
	flags[apiKeyCreateFlagOrgID] = fakeOrgID
	flags[apiKeyFlagRobotID] = fakeRobotID
	flags[apiKeyCreateFlagName] = "my-name"
	cCtx, ac, out, errOut := setup(asc, nil, nil, &flags, "token")

	test.That(t, ac.robotAPIKeyCreateAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 6)
	test.That(t, out.messages[1], test.ShouldContainSubstring, "Successfully created key")
	test.That(t, out.messages[2], test.ShouldContainSubstring, "Key ID: id-xxx")
	test.That(t, out.messages[3], test.ShouldContainSubstring, "Key Value: key-yyy")

	// test that without name still works

	cCtx.Set(apiKeyCreateFlagName, "")
	test.That(t, cCtx.Value(apiKeyCreateFlagName), test.ShouldEqual, "")

	test.That(t, ac.robotAPIKeyCreateAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, strings.Join(out.messages, " "), test.ShouldContainSubstring, "using default key name of")

	// test without an orgID
	cCtx.Set(apiKeyCreateFlagOrgID, "")
	test.That(t, cCtx.Value(apiKeyCreateFlagOrgID), test.ShouldEqual, "")

	test.That(t, ac.robotAPIKeyCreateAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)

	allMessages := strings.Join(out.messages, " ")
	test.That(t, allMessages, test.ShouldContainSubstring, "using default key name of ")

	test.That(t, allMessages, test.ShouldContainSubstring, "Successfully created key")
	test.That(t, allMessages, test.ShouldContainSubstring, "Key ID: id-xxx")
	test.That(t, allMessages, test.ShouldContainSubstring, "Key Value: key-yyy")

	// test without a robot ID should fail
	cCtx.Set(apiKeyFlagRobotID, "")
	test.That(t, cCtx.Value(apiKeyFlagRobotID), test.ShouldEqual, "")
	err := ac.robotAPIKeyCreateAction(cCtx)
	test.That(t, err, test.ShouldNotBeNil)

	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot create an api-key for a robot without an ID")

	// test for a location with multiple orgs doesn't work if you don't provide an orgID
	createKeyFunc = func(ctx context.Context, in *apppb.CreateKeyRequest,
		opts ...grpc.CallOption,
	) (*apppb.CreateKeyResponse, error) {
		return nil, errors.New("multiple orgs on the location")
	}

	asc = &inject.AppServiceClient{
		CreateKeyFunc: createKeyFunc,
	}

	flags = make(map[string]string)
	flags[apiKeyFlagRobotID] = fakeRobotID
	flags[apiKeyCreateFlagOrgID] = ""
	flags[apiKeyCreateFlagName] = "test-me"
	cCtx, ac, out, _ = setup(asc, nil, nil, &flags, "token")
	err = ac.robotAPIKeyCreateAction(cCtx)
	test.That(t, err, test.ShouldNotBeNil)

	test.That(t, len(out.messages), test.ShouldEqual, 0)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot create the robot api-key as there are multiple orgs on the location.")
}

func TestLocationAPIKeyCreateAction(t *testing.T) {
	fakeLocID := "fake-loc-id"
	fakeOrgID := "fake-org-id"

	createKeyFunc := func(ctx context.Context, in *apppb.CreateKeyRequest,
		opts ...grpc.CallOption,
	) (*apppb.CreateKeyResponse, error) {
		return &apppb.CreateKeyResponse{Id: "id-xxx", Key: "key-yyy"}, nil
	}

	asc := &inject.AppServiceClient{
		CreateKeyFunc: createKeyFunc,
	}

	flags := make(map[string]string)
	flags[apiKeyFlagLocationID] = ""
	flags[apiKeyCreateFlagOrgID] = ""
	flags[apiKeyCreateFlagName] = "" // testing no locationID

	cCtx, ac, out, errOut := setup(asc, nil, nil, &flags, "token")
	err := ac.locationAPIKeyCreateAction(cCtx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot create an api-key for a location without an ID")

	cCtx.Set(apiKeyFlagLocationID, fakeLocID)
	// will create an api-key with a default name
	test.That(t, ac.locationAPIKeyCreateAction(cCtx), test.ShouldBeNil)
	allMessages := strings.Join(out.messages, " ")

	test.That(t, allMessages, test.ShouldContainSubstring, "using default key name of ")
	test.That(t, allMessages, test.ShouldContainSubstring, "Successfully created key")
	test.That(t, allMessages, test.ShouldContainSubstring, "Key ID: id-xxx")
	test.That(t, allMessages, test.ShouldContainSubstring, "Key Value: key-yyy")

	// test with an orgID is fine
	cCtx.Set(apiKeyCreateFlagOrgID, fakeOrgID)
	test.That(t, ac.c.Value(apiKeyCreateFlagOrgID), test.ShouldNotBeEmpty)
	test.That(t, ac.locationAPIKeyCreateAction(cCtx), test.ShouldBeNil)
	allMessages = strings.Join(out.messages, " ")

	test.That(t, allMessages, test.ShouldContainSubstring, "Successfully created key")
	test.That(t, allMessages, test.ShouldContainSubstring, "Key ID: id-xxx")
	test.That(t, allMessages, test.ShouldContainSubstring, "Key Value: key-yyy")
	// test that multiple organizations on the location will error out}
	createKeyFunc = func(ctx context.Context, in *apppb.CreateKeyRequest,
		opts ...grpc.CallOption,
	) (*apppb.CreateKeyResponse, error) {
		return nil, errors.New("multiple orgs on the location")
	}

	asc = &inject.AppServiceClient{
		CreateKeyFunc: createKeyFunc,
	}

	flags = make(map[string]string)
	flags[apiKeyFlagLocationID] = fakeLocID
	flags[apiKeyCreateFlagOrgID] = ""
	flags[apiKeyCreateFlagName] = "test-name"

	cCtx, ac, _, _ = setup(asc, nil, nil, &flags, "token")

	err = ac.locationAPIKeyCreateAction(cCtx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring,
		fmt.Sprintf("cannot create api-key for location: %s as there are multiple orgs on the location", fakeLocID))
}

func TestLogoutAction(t *testing.T) {
	cCtx, ac, out, errOut := setup(nil, nil, nil, nil, "token")

	test.That(t, ac.logoutAction(cCtx), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 1)
	test.That(t, out.messages[0], test.ShouldContainSubstring,
		fmt.Sprintf("Logged out from %q", testEmail))
}

func TestWhoAmIAction(t *testing.T) {
	cCtx, ac, out, errOut := setup(nil, nil, nil, nil, "token")

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
