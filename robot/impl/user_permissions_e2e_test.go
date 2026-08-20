package robotimpl

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/jwk"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/camera"
	camerafake "go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/x264"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/web"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

const (
	authzTestFQDN      = "authz-e2e-test"
	authzTestAppUserID = "a1b2c3d4-0000-4000-8000-000000000001"
	authzGetImages     = "/viam.component.camera.v1.CameraService/GetImages"
	authzAddStream     = "/proto.stream.v1.StreamService/AddStream"
	authzRemoveStream  = "/proto.stream.v1.StreamService/RemoveStream"
	authzExternalKeyID = "external-auth-key-id"
)

// signExternalTokenWithAppUserID mints an app-style external auth token: RS256-signed,
// FusionAuth user ID as the subject, and the user's app user ID in the auth metadata claim.
func signExternalTokenWithAppUserID(key *rsa.PrivateKey, subject, appUserID, aud string) (string, error) {
	token := &jwt.Token{
		Header: map[string]interface{}{
			"typ": "JWT",
			"alg": jwt.SigningMethodRS256.Alg(),
			"kid": authzExternalKeyID,
		},
		Claims: rpc.JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Audience: []string{aud},
				Issuer:   "authz-e2e-iss",
				Subject:  subject,
				IssuedAt: jwt.NewNumericDate(time.Now()),
			},
			AuthCredentialsType: rpc.CredentialsTypeExternal,
			AuthMetadata:        map[string]string{"app_user_id": appUserID},
		},
		Method: jwt.SigningMethodRS256,
	}
	return token.SignedString(key)
}

// TestUserPermissionsE2E asserts that an API-key-ID user, an app-user-id user, and a
// default user each authenticate to a machine with user_permissions defined and are
// authorized against their own entry: each may get images from and add a live stream
// of the fake camera its entry grants, and is denied a camera granted only to a
// different user (which also proves the identity resolved to the right entry).
func TestUserPermissionsE2E(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	fakeCamCfg := func(name string) resource.Config {
		return resource.Config{
			Name:  name,
			API:   resource.NewAPI("rdk", "component", "camera"),
			Model: resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &camerafake.Config{
				Animated: true,
				Width:    100,
				Height:   50,
			},
		}
	}
	// listedCam is granted to the explicitly-listed users (API key + app user ID); defaultCam
	// is granted only to the default user. Each user is denied the other's camera, so a
	// mis-resolved identity (e.g. an app-user-id user silently falling through to the default
	// user) would fail its assertions instead of passing on identical grants.
	robotCfg := &config.Config{Components: []resource.Config{
		fakeCamCfg("listedCam"),
		fakeCamCfg("defaultCam"),
	}}

	// The robot's own web service needs a video encoder to serve live streams.
	r := setupLocalRobot(t, ctx, robotCfg, logger,
		WithWebOptions(web.WithStreamConfig(gostream.StreamConfig{
			VideoEncoderFactory: x264.NewEncoderFactory(),
		})))

	// External auth (app-user-id user): the machine trusts tokens signed by this key, as it
	// would trust app-signed tokens via its configured JWKS.
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	test.That(t, err, test.ShouldBeNil)
	keyset := jwk.NewSet()
	pubKey, err := jwk.New(privKey.PublicKey)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pubKey.Set("alg", "RS256"), test.ShouldBeNil)
	test.That(t, pubKey.Set(jwk.KeyIDKey, authzExternalKeyID), test.ShouldBeNil)
	test.That(t, keyset.Add(pubKey), test.ShouldBeTrue)

	// API key auth: one key for the explicitly-listed user, another for a valid user
	// with no user_permissions entry (exercises the default user).
	listedKeyID, listedKey := uuid.NewString(), utils.RandomAlphaString(32)
	unlistedKeyID, unlistedKey := uuid.NewString(), utils.RandomAlphaString(32)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	options.FQDN = authzTestFQDN
	options.Auth.Handlers = []config.AuthHandlerConfig{
		{
			Type: rpc.CredentialsTypeAPIKey,
			Config: rutils.AttributeMap{
				listedKeyID:   listedKey,
				unlistedKeyID: unlistedKey,
				"keys":        []string{listedKeyID, unlistedKeyID},
			},
		},
	}
	options.Auth.ExternalAuthConfig = &config.ExternalAuthConfig{ValidatedKeySet: keyset}

	camPerms := func(camName string) []config.Permission {
		return []config.Permission{
			{
				Resources:      []string{camName},
				AllowedMethods: []string{authzGetImages, authzAddStream, authzRemoveStream},
			},
		}
	}
	options.Auth.UserPermissions = []config.UserPermission{
		{User: config.User{Type: config.UserTypeAPIKeyID, ID: listedKeyID}, Permissions: camPerms("listedCam")},
		{User: config.User{Type: config.UserTypeAppUserID, ID: authzTestAppUserID}, Permissions: camPerms("listedCam")},
		{User: config.User{Type: config.UserTypeDefault}, Permissions: camPerms("defaultCam")},
	}

	test.That(t, r.StartWeb(ctx, options), test.ShouldBeNil)

	appUserToken, err := signExternalTokenWithAppUserID(privKey, "someauthprovider/fusionauth-user-id", authzTestAppUserID, authzTestFQDN)
	test.That(t, err, test.ShouldBeNil)

	for _, tc := range []struct {
		name       string
		dialOpt    rpc.DialOption
		grantedCam string // the camera this user may access
		deniedCam  string // a camera this user may not access (another user's)
	}{
		{
			"api key ID user",
			rutils.WithEntityCredentials(listedKeyID,
				rpc.Credentials{Type: rpc.CredentialsTypeAPIKey, Payload: listedKey}),
			"listedCam", "defaultCam",
		},
		{
			"app user ID user",
			rpc.WithStaticAuthenticationMaterial(appUserToken),
			"listedCam", "defaultCam",
		},
		{
			"default user",
			rutils.WithEntityCredentials(unlistedKeyID,
				rpc.Credentials{Type: rpc.CredentialsTypeAPIKey, Payload: unlistedKey}),
			"defaultCam", "listedCam",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Force a WebRTC connection: AddStream only works over WebRTC.
			conn, err := rgrpc.Dial(ctx, addr, logger.Sublogger(tc.name),
				rpc.WithDisableDirectGRPC(),
				rpc.WithAllowInsecureWithCredentialsDowngrade(),
				tc.dialOpt,
			)
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				test.That(t, conn.Close(), test.ShouldBeNil)
			}()

			// granted camera: images work
			grantedCam, err := camera.NewClientFromConn(ctx, conn, "", camera.Named(tc.grantedCam), logger)
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				test.That(t, grantedCam.Close(ctx), test.ShouldBeNil)
			}()
			_, _, err = grantedCam.Images(ctx, nil, nil)
			test.That(t, err, test.ShouldBeNil)

			// another user's camera: images denied. This also proves the caller's identity
			// resolved to its own user_permissions entry rather than silently falling
			// through to another (e.g. the default) user.
			deniedCam, err := camera.NewClientFromConn(ctx, conn, "", camera.Named(tc.deniedCam), logger)
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				test.That(t, deniedCam.Close(ctx), test.ShouldBeNil)
			}()
			_, _, err = deniedCam.Images(ctx, nil, nil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, status.Code(err), test.ShouldEqual, codes.PermissionDenied)

			// granted camera: live stream works; the server renegotiates a video track
			// onto the peer connection
			streamClient := streampb.NewStreamServiceClient(conn)
			_, err = streamClient.AddStream(ctx, &streampb.AddStreamRequest{Name: tc.grantedCam})
			test.That(t, err, test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				test.That(tb, conn.PeerConn().CurrentLocalDescription().SDP, test.ShouldContainSubstring, "m=video")
			})

			// another user's camera: live stream denied
			_, err = streamClient.AddStream(ctx, &streampb.AddStreamRequest{Name: tc.deniedCam})
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, status.Code(err), test.ShouldEqual, codes.PermissionDenied)

			// clean up the granted stream so the next user starts fresh
			_, err = streamClient.RemoveStream(ctx, &streampb.RemoveStreamRequest{Name: tc.grantedCam})
			test.That(t, err, test.ShouldBeNil)
		})
	}
}
