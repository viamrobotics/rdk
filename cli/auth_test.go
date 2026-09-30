package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
	shelltestutils "go.viam.com/rdk/services/shell/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestLoginAction(t *testing.T) {
	cCtx, ac, out, errOut := setup(nil, nil, nil, nil, "token")

	test.That(t, ac.loginAction(context.Background(), cCtx), test.ShouldBeNil)
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
	cCtx, ac, out, errOut := setup(asc, nil, nil, map[string]any{"org-id": "my-org"}, "token")

	orgKeyArgs := parseStructFromCtx[organizationsAPIKeyCreateArgs](cCtx)
	test.That(t, ac.organizationsAPIKeyCreateAction(context.Background(), cCtx, orgKeyArgs), test.ShouldBeNil)
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

	flags := make(map[string]any)
	flags[generalFlagOrgID] = fakeOrgID
	flags[generalFlagMachineID] = fakeRobotID
	flags[generalFlagName] = "my-name"
	cCtx, ac, out, errOut := setup(asc, nil, nil, flags, "token")

	test.That(t, ac.robotAPIKeyCreateAction(context.Background(), cCtx, parseStructFromCtx[robotAPIKeyCreateArgs](cCtx)), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, len(out.messages), test.ShouldEqual, 6)
	test.That(t, out.messages[1], test.ShouldContainSubstring, "Successfully created key")
	test.That(t, out.messages[2], test.ShouldContainSubstring, "Key ID: id-xxx")
	test.That(t, out.messages[3], test.ShouldContainSubstring, "Key Value: key-yyy")

	// test that without name still works

	cCtx.Set(generalFlagName, "")
	test.That(t, cCtx.Value(generalFlagName), test.ShouldEqual, "")

	test.That(t, ac.robotAPIKeyCreateAction(context.Background(), cCtx, parseStructFromCtx[robotAPIKeyCreateArgs](cCtx)), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, strings.Join(out.messages, " "), test.ShouldContainSubstring, "using default key name of")

	// test without an orgID
	cCtx.Set(generalFlagOrgID, "")
	test.That(t, cCtx.Value(generalFlagOrgID), test.ShouldEqual, "")

	test.That(t, ac.robotAPIKeyCreateAction(context.Background(), cCtx, parseStructFromCtx[robotAPIKeyCreateArgs](cCtx)), test.ShouldBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)

	allMessages := strings.Join(out.messages, " ")
	test.That(t, allMessages, test.ShouldContainSubstring, "using default key name of ")

	test.That(t, allMessages, test.ShouldContainSubstring, "Successfully created key")
	test.That(t, allMessages, test.ShouldContainSubstring, "Key ID: id-xxx")
	test.That(t, allMessages, test.ShouldContainSubstring, "Key Value: key-yyy")

	// test without a robot ID should fail
	cCtx.Set(generalFlagMachineID, "")
	test.That(t, cCtx.Value(generalFlagMachineID), test.ShouldEqual, "")
	err := ac.robotAPIKeyCreateAction(context.Background(), cCtx, parseStructFromCtx[robotAPIKeyCreateArgs](cCtx))
	test.That(t, err, test.ShouldNotBeNil)

	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot create an api-key for a machine without an ID")

	// test for a location with multiple orgs doesn't work if you don't provide an orgID
	createKeyFunc = func(ctx context.Context, in *apppb.CreateKeyRequest,
		opts ...grpc.CallOption,
	) (*apppb.CreateKeyResponse, error) {
		return nil, errors.New("multiple orgs on the location")
	}

	asc = &inject.AppServiceClient{
		CreateKeyFunc: createKeyFunc,
	}

	flags = make(map[string]any)
	flags[generalFlagMachineID] = fakeRobotID
	flags[generalFlagOrgID] = ""
	flags[generalFlagName] = "test-me"
	cCtx, ac, out, _ = setup(asc, nil, nil, flags, "token")
	err = ac.robotAPIKeyCreateAction(context.Background(), cCtx, parseStructFromCtx[robotAPIKeyCreateArgs](cCtx))
	test.That(t, err, test.ShouldNotBeNil)

	test.That(t, len(out.messages), test.ShouldEqual, 0)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot create the machine api-key as there are multiple orgs on the location.")
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

	flags := make(map[string]any)
	flags[generalFlagLocationID] = ""
	flags[generalFlagOrgID] = ""
	flags[generalFlagName] = "" // testing no locationID

	cCtx, ac, out, errOut := setup(asc, nil, nil, flags, "token")
	err := ac.locationAPIKeyCreateAction(context.Background(), cCtx, parseStructFromCtx[locationAPIKeyCreateArgs](cCtx))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot create an api-key for a location without an ID")

	cCtx.Set(generalFlagLocationID, fakeLocID)
	// will create an api-key with a default name
	locKeyArgs := parseStructFromCtx[locationAPIKeyCreateArgs](cCtx)
	test.That(t, ac.locationAPIKeyCreateAction(context.Background(), cCtx, locKeyArgs), test.ShouldBeNil)
	allMessages := strings.Join(out.messages, " ")

	test.That(t, allMessages, test.ShouldContainSubstring, "using default key name of ")
	test.That(t, allMessages, test.ShouldContainSubstring, "Successfully created key")
	test.That(t, allMessages, test.ShouldContainSubstring, "Key ID: id-xxx")
	test.That(t, allMessages, test.ShouldContainSubstring, "Key Value: key-yyy")

	// test with an orgID is fine
	cCtx.Set(generalFlagOrgID, fakeOrgID)
	test.That(t, ac.c.Value(generalFlagOrgID), test.ShouldNotBeEmpty)
	locKeyArgsWithOrg := parseStructFromCtx[locationAPIKeyCreateArgs](cCtx)
	test.That(t, ac.locationAPIKeyCreateAction(context.Background(), cCtx, locKeyArgsWithOrg), test.ShouldBeNil)
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

	flags = make(map[string]any)
	flags[generalFlagLocationID] = fakeLocID
	flags[generalFlagOrgID] = ""
	flags[generalFlagName] = "test-name"

	cCtx, ac, _, _ = setup(asc, nil, nil, flags, "token")

	err = ac.locationAPIKeyCreateAction(context.Background(), cCtx, parseStructFromCtx[locationAPIKeyCreateArgs](cCtx))
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
		conf := Config{
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
		var newConf Config
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
		conf := Config{
			BaseURL: "https://docs.viam.com:443",
			Auth: &apiKey{
				KeyID:     "42",
				KeyCrypto: "secret",
			},
		}

		bytes, err := json.Marshal(conf)
		test.That(t, err, test.ShouldBeNil)
		var newConf Config
		test.That(t, newConf.tryUnmarshallWithToken(bytes), test.ShouldBeError)
		test.That(t, newConf.tryUnmarshallWithAPIKey(bytes), test.ShouldBeNil)
		test.That(t, newConf.BaseURL, test.ShouldEqual, "https://docs.viam.com:443")
		auth, ok := newConf.Auth.(*apiKey)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, auth.KeyID, test.ShouldEqual, "42")
		test.That(t, auth.KeyCrypto, test.ShouldEqual, "secret")
	})
}

// starts a token endpoint that answers any refresh grant with accessToken, and reports
// how many times it was called.
func newRefreshTokenServer(t *testing.T, accessToken string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		resp := tokenResponse{
			AccessToken:  accessToken,
			RefreshToken: "fresh-refresh-token",
			IDToken:      makeIDToken(t, testEmail, "user-123"),
			ExpiresIn:    3600,
			TokenType:    tokenTypeUserOAuthToken,
		}
		test.That(t, json.NewEncoder(w).Encode(resp), test.ShouldBeNil)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// expiredToken builds a user login that expired an hour ago.
func expiredToken(tokenURL string) *token {
	t := &token{
		AccessToken: "stale-access-token",
		TokenType:   tokenTypeUserOAuthToken,
		ExpiresAt:   time.Now().Add(-time.Hour),
		User:        userData{Email: testEmail},
	}
	if tokenURL != "" {
		t.RefreshToken = "old-refresh-token"
		t.TokenURL = tokenURL
		t.ClientID = "client-123"
	}
	return t
}

// useTempCLICache points the CLI cache at a temp dir.
func useTempCLICache(t *testing.T) {
	t.Helper()
	origViamDotDir := utils.ViamDotDir
	utils.ViamDotDir = t.TempDir()
	t.Cleanup(func() { utils.ViamDotDir = origViamDotDir })
}

func TestRefreshAuthIfExpired(t *testing.T) {
	t.Run("unexpired token is left alone", func(t *testing.T) {
		useTempCLICache(t)
		srv, hits := newRefreshTokenServer(t, "fresh-access-token")

		_, ac, _, _ := setup(&inject.AppServiceClient{}, nil, nil, nil, "token")
		ac.authFlow = newCLIAuthFlow(io.Discard, true)
		ac.conf.Auth.(*token).TokenURL = srv.URL

		refreshed, err := ac.refreshAuthIfExpired(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, refreshed, test.ShouldBeFalse)
		test.That(t, hits.Load(), test.ShouldEqual, 0)
		test.That(t, ac.conf.Auth.(*token).AccessToken, test.ShouldEqual, testToken)
	})

	t.Run("expired token is refreshed", func(t *testing.T) {
		useTempCLICache(t)
		srv, hits := newRefreshTokenServer(t, "fresh-access-token")

		_, ac, _, _ := setup(&inject.AppServiceClient{}, nil, nil, nil, "token")
		ac.authFlow = newCLIAuthFlow(io.Discard, true)
		ac.conf.Auth = expiredToken(srv.URL)

		refreshed, err := ac.refreshAuthIfExpired(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, refreshed, test.ShouldBeTrue)
		test.That(t, hits.Load(), test.ShouldEqual, 1)
		test.That(t, ac.conf.Auth.(*token).AccessToken, test.ShouldEqual, "fresh-access-token")
	})

	t.Run("api keys need no refresh", func(t *testing.T) {
		useTempCLICache(t)

		_, ac, _, _ := setup(&inject.AppServiceClient{}, nil, nil, nil, "apiKey")

		refreshed, err := ac.refreshAuthIfExpired(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, refreshed, test.ShouldBeFalse)
		_, isAPIKey := ac.conf.Auth.(*apiKey)
		test.That(t, isAPIKey, test.ShouldBeTrue)
	})

	t.Run("unrefreshable token logs out", func(t *testing.T) {
		useTempCLICache(t)

		_, ac, _, _ := setup(&inject.AppServiceClient{}, nil, nil, nil, "token")
		ac.conf.Auth = expiredToken("")

		_, err := ac.refreshAuthIfExpired(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, errors.Is(err, errLoggedOut), test.ShouldBeTrue)
		test.That(t, err.Error(), test.ShouldContainSubstring, "token expired and cannot be refreshed")
		test.That(t, ac.conf.Auth, test.ShouldBeNil)
	})

	t.Run("failed refresh logs out and returns errLoggedOut", func(t *testing.T) {
		useTempCLICache(t)

		// Refresh server returns 500 to simulate a transient (or permanent) server error.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"server_error","error_description":"internal error"}`, http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)

		_, ac, _, _ := setup(&inject.AppServiceClient{}, nil, nil, nil, "token")
		ac.authFlow = newCLIAuthFlow(io.Discard, true)
		ac.conf.Auth = expiredToken(srv.URL)

		_, err := ac.refreshAuthIfExpired(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, errors.Is(err, errLoggedOut), test.ShouldBeTrue)
		test.That(t, ac.conf.Auth, test.ShouldBeNil)
	})
}

func TestPrepareDialRefreshesExpiredToken(t *testing.T) {
	// Ensures prepareDialInner can refresh.

	useTempCLICache(t)
	srv, hits := newRefreshTokenServer(t, "fresh-access-token")

	_, ac, _, errOut := setup(&inject.AppServiceClient{}, nil, nil, nil, "token")
	ac.authFlow = newCLIAuthFlow(io.Discard, true)
	ac.conf.Auth = expiredToken(srv.URL)
	ac.conf.BaseURL = srv.URL
	var err error
	ac.baseURL, _, err = utils.ParseBaseURL(ac.conf.BaseURL, false)
	test.That(t, err, test.ShouldBeNil)

	_, fqdn, rpcOpts, err := ac.prepareDialInner(context.Background(), "part.fqdn", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fqdn, test.ShouldEqual, "part.fqdn")
	test.That(t, len(rpcOpts), test.ShouldBeGreaterThan, 0)
	test.That(t, hits.Load(), test.ShouldEqual, 1)
	test.That(t, ac.conf.Auth.(*token).AccessToken, test.ShouldEqual, "fresh-access-token")

	// The refreshed token is good for another hour, so a subsequent dial reuses it.
	_, _, rpcOpts, err = ac.prepareDialInner(context.Background(), "part.fqdn", false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(rpcOpts), test.ShouldBeGreaterThan, 0)
	test.That(t, hits.Load(), test.ShouldEqual, 1)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
}

func TestPrepareDialRefreshesBetweenCopyRetries(t *testing.T) {
	// Ensures `machine part cp` can refresh a token between retries. E2E of above test.

	useTempCLICache(t)
	srv, hits := newRefreshTokenServer(t, "fresh-access-token")

	partFqdn := uuid.NewString()
	asc := &inject.AppServiceClient{
		ListOrganizationsFunc: func(ctx context.Context, in *apppb.ListOrganizationsRequest,
			opts ...grpc.CallOption,
		) (*apppb.ListOrganizationsResponse, error) {
			return &apppb.ListOrganizationsResponse{
				Organizations: []*apppb.Organization{{Name: "jedi", Id: uuid.NewString(), PublicNamespace: "anakin"}},
			}, nil
		},
		ListLocationsFunc: func(ctx context.Context, in *apppb.ListLocationsRequest,
			opts ...grpc.CallOption,
		) (*apppb.ListLocationsResponse, error) {
			return &apppb.ListLocationsResponse{Locations: []*apppb.Location{{Name: "naboo"}}}, nil
		},
		ListRobotsFunc: func(ctx context.Context, in *apppb.ListRobotsRequest,
			opts ...grpc.CallOption,
		) (*apppb.ListRobotsResponse, error) {
			return &apppb.ListRobotsResponse{Robots: []*apppb.Robot{{Name: "r2d2"}}}, nil
		},
		GetRobotPartsFunc: func(ctx context.Context, in *apppb.GetRobotPartsRequest,
			opts ...grpc.CallOption,
		) (*apppb.GetRobotPartsResponse, error) {
			return &apppb.GetRobotPartsResponse{Parts: []*apppb.RobotPart{{Name: "main", Fqdn: partFqdn}}}, nil
		},
	}

	partFlags := map[string]any{
		"organization": "jedi",
		"location":     "naboo",
		"robot":        "r2d2",
		"part":         "main",
	}

	logger := logging.NewTestLogger(t)
	tfs := shelltestutils.SetupTestFileSystem(t)
	tempDir := t.TempDir()

	args := []string{"machine:" + tfs.SingleFileNested, tempDir}
	cCtx, ac, _, errOut := setupWithRunningPart(t, asc, nil, nil, partFlags, "token", partFqdn, args...)

	ac.authFlow = newCLIAuthFlow(io.Discard, true)
	ac.conf.Auth = expiredToken(srv.URL)

	realDial := ac.dialOverride
	var dialCalls atomic.Int32
	ac.dialOverride = func(ctx context.Context, fqdn string, rpcOpts []rpc.DialOption,
		l logging.Logger,
	) (*client.RobotClient, error) {
		tok := ac.conf.Auth.(*token)
		if tok.isExpired() {
			return nil, errors.New("unauthenticated: token is expired")
		}
		if dialCalls.Add(1) == 1 {
			// Manually expire the token on the first dial. The second attempt should still
			// succeed.
			tok.ExpiresAt = time.Now().Add(-time.Hour)
			return nil, errors.New("connection reset during transfer")
		}
		return realDial(ctx, fqdn, rpcOpts, l)
	}

	err := ac.machinesPartCopyFilesAction(context.Background(), cCtx, parseStructFromCtx[machinesPartCopyFilesArgs](cCtx), logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, hits.Load(), test.ShouldEqual, 2)
	test.That(t, ac.conf.Auth.(*token).AccessToken, test.ShouldEqual, "fresh-access-token")

	rd, err := os.ReadFile(filepath.Join(tempDir, filepath.Base(tfs.SingleFileNested)))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rd, test.ShouldResemble, tfs.SingleFileNestedData)
	test.That(t, len(errOut.messages), test.ShouldEqual, 0)
}
