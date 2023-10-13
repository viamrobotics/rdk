//go:build !no_media

package web_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang-jwt/jwt/v4"
	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/viamrobotics/gostream/codec/x264"
	streampb "github.com/viamrobotics/gostream/proto/stream/v1"
	"go.mongodb.org/mongo-driver/bson/primitive"
	echopb "go.viam.com/api/component/testecho/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	gizmopb "go.viam.com/rdk/examples/customresources/apis/proto/api/component/gizmo/v1"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

const arm1String = "arm1"

var resources = []resource.Name{arm.Named(arm1String)}

var pos = spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})

func TestWebStart(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx(t)

	svc := web.New(injectRobot, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)

	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	conn, err := rgrpc.Dial(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	arm1, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err := arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	err = svc.Start(context.Background(), weboptions.New())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already started")

	err = svc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestModule(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx(t)

	svc := web.New(injectRobot, logger)

	err := svc.StartModule(ctx)
	test.That(t, err, test.ShouldBeNil)

	conn1, err := rgrpc.Dial(context.Background(), "unix://"+svc.ModuleAddress(), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1, err := arm.NewClientFromConn(context.Background(), conn1, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)
	arm1Position, err := arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	err = svc.StartModule(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already started")

	options, _, _ := robottestutils.CreateBaseOptionsAndListener(t)

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	conn2, err := rgrpc.Dial(context.Background(), svc.Address(), logger)
	test.That(t, err, test.ShouldBeNil)
	arm2, err := arm.NewClientFromConn(context.Background(), conn2, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)

	arm2Position, err := arm2.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm2Position, test.ShouldResemble, pos)

	svc.Stop()
	time.Sleep(time.Second)

	_, err = arm2.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldNotBeNil)

	_, err = arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	err = svc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	_, err = arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldNotBeNil)

	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

func TestWebStartOptions(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx(t)

	svc := web.New(injectRobot, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)

	options.Network.BindAddress = "woop"
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "only set one of")
	options.Network.BindAddress = ""

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	conn, err := rgrpc.Dial(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	arm1, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err := arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	test.That(t, conn.Close(), test.ShouldBeNil)
	err = svc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestWebWithAuth(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx(t)

	for _, tc := range []struct {
		Case       string
		Managed    bool
		EntityName string
	}{
		{Case: "unmanaged and default host"},
		{Case: "unmanaged and specific host", EntityName: "something-different"},
		{Case: "managed and default host", Managed: true},
		{Case: "managed and specific host", Managed: true, EntityName: "something-different"},
	} {
		t.Run(tc.Case, func(t *testing.T) {
			svc := web.New(injectRobot, logger)

			keyset := jwk.NewSet()
			privKeyForWebAuth, err := rsa.GenerateKey(rand.Reader, 4096)
			test.That(t, err, test.ShouldBeNil)
			publicKeyForWebAuth, err := jwk.New(privKeyForWebAuth.PublicKey)
			test.That(t, err, test.ShouldBeNil)
			publicKeyForWebAuth.Set("alg", "RS256")
			publicKeyForWebAuth.Set(jwk.KeyIDKey, "key-id-1")
			test.That(t, keyset.Add(publicKeyForWebAuth), test.ShouldBeTrue)

			options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
			options.Managed = tc.Managed
			options.FQDN = tc.EntityName
			options.LocalFQDN = primitive.NewObjectID().Hex()
			apiKey := "sosecret"
			locationSecrets := []string{"locsosecret", "locsec2"}
			options.Auth.Handlers = []config.AuthHandlerConfig{
				{
					Type: rpc.CredentialsTypeAPIKey,
					Config: rutils.AttributeMap{
						"key": apiKey,
					},
				},
				{
					Type: rutils.CredentialsTypeRobotLocationSecret,
					Config: rutils.AttributeMap{
						"secrets": locationSecrets,
					},
				},
			}
			options.Auth.ExternalAuthConfig = &config.ExternalAuthConfig{
				ValidatedKeySet: keyset,
			}
			if tc.Managed {
				options.BakedAuthEntity = "blah"
				options.BakedAuthCreds = rpc.Credentials{Type: "blah"}
			}

			err = svc.Start(ctx, options)
			test.That(t, err, test.ShouldBeNil)

			_, err = rgrpc.Dial(context.Background(), addr, logger)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

			if tc.Managed {
				_, err = rgrpc.Dial(context.Background(), addr, logger, rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials("wrong", rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey,
					}))
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

				_, err = rgrpc.Dial(context.Background(), addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials("wrong", rpc.Credentials{
						Type:    rutils.CredentialsTypeRobotLocationSecret,
						Payload: locationSecrets[0],
					}),
				)
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

				entityName := tc.EntityName
				if entityName == "" {
					entityName = options.LocalFQDN
				}

				// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
				// WebRTC connections across unix sockets can create deadlock in CI.
				conn, err := rgrpc.Dial(context.Background(), addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials(entityName, rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey,
					}),
					rpc.WithForceDirectGRPC(),
				)
				test.That(t, err, test.ShouldBeNil)
				arm1, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
				test.That(t, err, test.ShouldBeNil)

				arm1Position, err := arm1.EndPosition(ctx, nil)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, arm1Position, test.ShouldResemble, pos)

				test.That(t, arm1.Close(context.Background()), test.ShouldBeNil)
				test.That(t, conn.Close(), test.ShouldBeNil)

				// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
				// WebRTC connections across unix sockets can create deadlock in CI.
				conn, err = rgrpc.Dial(context.Background(), addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials(entityName, rpc.Credentials{
						Type:    rutils.CredentialsTypeRobotLocationSecret,
						Payload: locationSecrets[0],
					}),
					rpc.WithForceDirectGRPC(),
				)
				test.That(t, err, test.ShouldBeNil)
				arm1, err = arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
				test.That(t, err, test.ShouldBeNil)

				arm1Position, err = arm1.EndPosition(ctx, nil)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, arm1Position, test.ShouldResemble, pos)

				test.That(t, arm1.Close(context.Background()), test.ShouldBeNil)
				test.That(t, conn.Close(), test.ShouldBeNil)

				// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
				// WebRTC connections across unix sockets can create deadlock in CI.
				conn, err = rgrpc.Dial(context.Background(), addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials(entityName, rpc.Credentials{
						Type:    rutils.CredentialsTypeRobotLocationSecret,
						Payload: locationSecrets[1],
					}),
					rpc.WithForceDirectGRPC(),
				)
				test.That(t, err, test.ShouldBeNil)
				arm1, err = arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
				test.That(t, err, test.ShouldBeNil)

				arm1Position, err = arm1.EndPosition(ctx, nil)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, arm1Position, test.ShouldResemble, pos)

				test.That(t, arm1.Close(context.Background()), test.ShouldBeNil)
				test.That(t, conn.Close(), test.ShouldBeNil)

				if tc.EntityName != "" {
					t.Run("can connect with external auth", func(t *testing.T) {
						accessToken, err := signJWKBasedExternalAccessToken(
							privKeyForWebAuth,
							entityName,
							options.FQDN,
							"iss",
							"key-id-1",
						)
						test.That(t, err, test.ShouldBeNil)
						// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
						// WebRTC connections across unix sockets can create deadlock in CI.
						conn, err = rgrpc.Dial(context.Background(), addr, logger,
							rpc.WithAllowInsecureWithCredentialsDowngrade(),
							rpc.WithStaticAuthenticationMaterial(accessToken),
							rpc.WithForceDirectGRPC(),
						)
						test.That(t, err, test.ShouldBeNil)
						test.That(t, conn.Close(), test.ShouldBeNil)
					})
				}
			} else {
				// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
				// WebRTC connections across unix sockets can create deadlock in CI.
				conn, err := rgrpc.Dial(context.Background(), addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithCredentials(rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey,
					}),
					rpc.WithForceDirectGRPC(),
				)
				test.That(t, err, test.ShouldBeNil)

				arm1, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
				test.That(t, err, test.ShouldBeNil)

				arm1Position, err := arm1.EndPosition(ctx, nil)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, arm1Position, test.ShouldResemble, pos)

				test.That(t, arm1.Close(context.Background()), test.ShouldBeNil)
				test.That(t, conn.Close(), test.ShouldBeNil)

				// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
				// WebRTC connections across unix sockets can create deadlock in CI.
				conn, err = rgrpc.Dial(context.Background(), addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithCredentials(rpc.Credentials{
						Type:    rutils.CredentialsTypeRobotLocationSecret,
						Payload: locationSecrets[0],
					}),
					rpc.WithForceDirectGRPC(),
				)
				test.That(t, err, test.ShouldBeNil)

				arm1, err = arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
				test.That(t, err, test.ShouldBeNil)

				arm1Position, err = arm1.EndPosition(ctx, nil)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, arm1Position, test.ShouldResemble, pos)

				test.That(t, arm1.Close(context.Background()), test.ShouldBeNil)
				test.That(t, conn.Close(), test.ShouldBeNil)
			}

			err = svc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)
		})
	}
}

func TestWebWithTLSAuth(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx(t)

	svc := web.New(injectRobot, logger)

	altName := primitive.NewObjectID().Hex()
	cert, certFile, keyFile, certPool, err := testutils.GenerateSelfSignedCertificate("somename", altName)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		os.Remove(certFile)
		os.Remove(keyFile)
	})

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	options.Network.TLSConfig = &tls.Config{
		RootCAs:      certPool,
		ClientCAs:    certPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.VerifyClientCertIfGiven,
	}
	options.Auth.TLSAuthEntities = leaf.DNSNames
	options.Managed = true
	options.FQDN = altName
	options.LocalFQDN = "localhost" // this will allow authentication to work in unmanaged, default host
	locationSecret := "locsosecret"
	options.Auth.Handlers = []config.AuthHandlerConfig{
		{
			Type: rutils.CredentialsTypeRobotLocationSecret,
			Config: rutils.AttributeMap{
				"secret": locationSecret,
			},
		},
	}
	options.BakedAuthEntity = "blah"
	options.BakedAuthCreds = rpc.Credentials{Type: "blah"}

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	clientTLSConfig := options.Network.TLSConfig.Clone()
	clientTLSConfig.Certificates = nil
	clientTLSConfig.ServerName = "somename"

	_, err = rgrpc.Dial(context.Background(), addr, logger,
		rpc.WithTLSConfig(clientTLSConfig),
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

	_, err = rgrpc.Dial(context.Background(), addr, logger,
		rpc.WithTLSConfig(clientTLSConfig),
		rpc.WithEntityCredentials("wrong", rpc.Credentials{
			Type:    rutils.CredentialsTypeRobotLocationSecret,
			Payload: locationSecret,
		}),
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

	// use secret
	conn, err := rgrpc.Dial(context.Background(), addr, logger,
		rpc.WithTLSConfig(clientTLSConfig),
		rpc.WithEntityCredentials(options.FQDN, rpc.Credentials{
			Type:    rutils.CredentialsTypeRobotLocationSecret,
			Payload: locationSecret,
		}),
	)
	test.That(t, err, test.ShouldBeNil)

	arm1, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err := arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)
	test.That(t, conn.Close(), test.ShouldBeNil)

	// use cert
	clientTLSConfig.Certificates = []tls.Certificate{cert}
	conn, err = rgrpc.Dial(context.Background(), addr, logger,
		rpc.WithTLSConfig(clientTLSConfig),
	)
	test.That(t, err, test.ShouldBeNil)

	arm1, err = arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err = arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)
	test.That(t, conn.Close(), test.ShouldBeNil)

	// use cert with mDNS
	conn, err = rgrpc.Dial(context.Background(), options.FQDN, logger,
		rpc.WithDialDebug(),
		rpc.WithTLSConfig(clientTLSConfig),
	)
	test.That(t, err, test.ShouldBeNil)

	arm1, err = arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err = arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)
	test.That(t, conn.Close(), test.ShouldBeNil)

	// use signaling creds
	conn, err = rgrpc.Dial(context.Background(), addr, logger,
		rpc.WithDialDebug(),
		rpc.WithTLSConfig(clientTLSConfig),
		rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
			SignalingServerAddress: addr,
			SignalingAuthEntity:    options.FQDN,
			SignalingCreds: rpc.Credentials{
				Type:    rutils.CredentialsTypeRobotLocationSecret,
				Payload: locationSecret,
			},
		}),
	)
	test.That(t, err, test.ShouldBeNil)

	arm1, err = arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)
	arm1Position, err = arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)
	test.That(t, conn.Close(), test.ShouldBeNil)

	// use cert with mDNS while signaling present
	conn, err = rgrpc.Dial(context.Background(), options.FQDN, logger,
		rpc.WithDialDebug(),
		rpc.WithTLSConfig(clientTLSConfig),
		rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
			SignalingServerAddress: addr,
			SignalingAuthEntity:    options.FQDN,
			SignalingCreds: rpc.Credentials{
				Type:    rutils.CredentialsTypeRobotLocationSecret,
				Payload: locationSecret + "bad",
			},
		}),
		rpc.WithDialMulticastDNSOptions(rpc.DialMulticastDNSOptions{
			RemoveAuthCredentials: true,
		}),
	)
	test.That(t, err, test.ShouldBeNil)

	arm1, err = arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err = arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	err = svc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestWebWithBadAuthHandlers(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx(t)

	svc := web.New(injectRobot, logger)

	options, _, _ := robottestutils.CreateBaseOptionsAndListener(t)
	options.Auth.Handlers = []config.AuthHandlerConfig{
		{
			Type: "unknown",
		},
	}

	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown")
	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)

	svc = web.New(injectRobot, logger)

	options, _, _ = robottestutils.CreateBaseOptionsAndListener(t)
	options.Auth.Handlers = []config.AuthHandlerConfig{
		{
			Type: rpc.CredentialsTypeAPIKey,
		},
	}

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "non-empty")
	test.That(t, err.Error(), test.ShouldContainSubstring, "api-key")
	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)
}

func TestWebReconfigure(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, robot := setupRobotCtx(t)

	svc := web.New(robot, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
	// WebRTC connections across unix sockets can create deadlock in CI.
	conn, err := rgrpc.Dial(context.Background(), addr, logger, rpc.WithForceDirectGRPC())
	test.That(t, err, test.ShouldBeNil)

	arm1, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err := arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)
	test.That(t, conn.Close(), test.ShouldBeNil)

	// add arm to robot and then update
	injectArm := &inject.Arm{}
	newPos := spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 3, Z: 6})
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return newPos, nil
	}
	rs := map[resource.Name]resource.Resource{arm.Named(arm1String): injectArm}
	err = svc.Reconfigure(context.Background(), rs, resource.Config{})
	test.That(t, err, test.ShouldBeNil)

	// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
	// WebRTC connections across unix sockets can create deadlock in CI.
	conn, err = rgrpc.Dial(context.Background(), addr, logger, rpc.WithForceDirectGRPC())
	test.That(t, err, test.ShouldBeNil)
	aClient, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)
	position, err := aClient.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, newPos)

	test.That(t, arm1.Close(context.Background()), test.ShouldBeNil)
	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)
	test.That(t, aClient.Close(context.Background()), test.ShouldBeNil)

	// now start it with the arm already in it
	ctx, robot2 := setupRobotCtx(t)
	robot2.(*inject.Robot).ResourceNamesFunc = func() []resource.Name { return resources }
	robot2.(*inject.Robot).ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return injectArm, nil
	}

	svc2 := web.New(robot2, logger)

	listener := testutils.ReserveRandomListener(t)
	addr = listener.Addr().String()
	options.Network.Listener = listener

	err = svc2.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)

	// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
	// WebRTC connections across unix sockets can create deadlock in CI.
	conn, err = rgrpc.Dial(context.Background(), addr, logger, rpc.WithForceDirectGRPC())
	test.That(t, err, test.ShouldBeNil)

	arm1, err = arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err = arm1.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, newPos)
	test.That(t, conn.Close(), test.ShouldBeNil)

	// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
	// WebRTC connections across unix sockets can create deadlock in CI.
	conn, err = rgrpc.Dial(context.Background(), addr, logger, rpc.WithForceDirectGRPC())
	test.That(t, err, test.ShouldBeNil)
	aClient2, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	position, err = aClient2.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, newPos)

	// add a second arm
	arm2 := "arm2"
	injectArm2 := &inject.Arm{}
	pos2 := spatialmath.NewPoseFromPoint(r3.Vector{X: 2, Y: 3, Z: 4})
	injectArm2.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pos2, nil
	}
	rs[arm.Named(arm2)] = injectArm2
	err = svc2.Reconfigure(context.Background(), rs, resource.Config{})
	test.That(t, err, test.ShouldBeNil)

	position, err = aClient2.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, newPos)

	aClient3, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm2), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	position, err = aClient3.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, pos2)

	test.That(t, arm1.Close(context.Background()), test.ShouldBeNil)
	test.That(t, svc2.Close(context.Background()), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestWebWithStreams(t *testing.T) {
	const (
		camera1Key = "camera1"
		camera2Key = "camera2"
		audioKey   = "audio"
	)

	// Start a robot with a camera
	robot := &inject.Robot{}
	cam1 := &inject.Camera{}
	rs := map[resource.Name]resource.Resource{camera.Named(camera1Key): cam1}
	robot.MockResourcesFromMap(rs)

	ctx, cancel := context.WithCancel(context.Background())

	// Start service
	logger := golog.NewTestLogger(t)
	robot.LoggerFunc = func() golog.Logger { return logger }
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	svc := web.New(robot, logger, web.WithStreamConfig(x264.DefaultStreamConfig))
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Start a stream service client
	conn, err := rgrpc.Dial(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	streamClient := streampb.NewStreamServiceClient(conn)

	// Test that only one stream is available
	resp, err := streamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Names, test.ShouldContain, camera1Key)
	test.That(t, resp.Names, test.ShouldHaveLength, 1)

	// Add another camera and update
	cam2 := &inject.Camera{}
	rs[camera.Named(camera2Key)] = cam2
	robot.MockResourcesFromMap(rs)
	err = svc.Reconfigure(context.Background(), rs, resource.Config{})
	test.That(t, err, test.ShouldBeNil)

	// Add an audio stream
	audio := &inject.AudioInput{}
	rs[audioinput.Named(audioKey)] = audio
	robot.MockResourcesFromMap(rs)
	err = svc.Reconfigure(context.Background(), rs, resource.Config{})
	test.That(t, err, test.ShouldBeNil)

	// Test that new streams are available
	resp, err = streamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Names, test.ShouldContain, camera1Key)
	test.That(t, resp.Names, test.ShouldContain, camera2Key)
	test.That(t, resp.Names, test.ShouldHaveLength, 3)

	// We need to cancel otherwise we are stuck waiting for WebRTC to start streaming.
	cancel()
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestWebAddFirstStream(t *testing.T) {
	const (
		camera1Key = "camera1"
	)

	// Start a robot without a camera
	robot := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}
	robot.MockResourcesFromMap(rs)

	ctx, cancel := context.WithCancel(context.Background())

	// Start service
	logger := golog.NewTestLogger(t)
	robot.LoggerFunc = func() golog.Logger { return logger }
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	svc := web.New(robot, logger, web.WithStreamConfig(x264.DefaultStreamConfig))
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Start a stream service client
	conn, err := rgrpc.Dial(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	streamClient := streampb.NewStreamServiceClient(conn)

	// Test that there are no streams available
	resp, err := streamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Names, test.ShouldHaveLength, 0)

	// Add first camera and update
	cam1 := &inject.Camera{}
	rs[camera.Named(camera1Key)] = cam1
	robot.MockResourcesFromMap(rs)
	err = svc.Reconfigure(ctx, rs, resource.Config{})
	test.That(t, err, test.ShouldBeNil)

	// Test that new streams are available
	resp, err = streamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Names, test.ShouldContain, camera1Key)
	test.That(t, resp.Names, test.ShouldHaveLength, 1)

	// We need to cancel otherwise we are stuck waiting for WebRTC to start streaming.
	cancel()
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestWebStreamImmediateClose(t *testing.T) {
	// Primarily a regression test for RSDK-2418

	// Start a robot with a camera
	robot := &inject.Robot{}
	cam1 := &inject.Camera{}
	rs := map[resource.Name]resource.Resource{camera.Named("camera1"): cam1}
	robot.MockResourcesFromMap(rs)

	ctx, cancel := context.WithCancel(context.Background())

	// Start service
	logger := golog.NewTestLogger(t)
	robot.LoggerFunc = func() golog.Logger { return logger }
	options, _, _ := robottestutils.CreateBaseOptionsAndListener(t)
	svc := web.New(robot, logger, web.WithStreamConfig(x264.DefaultStreamConfig))
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Immediately Close service.
	cancel()
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
}

func setupRobotCtx(t *testing.T) (context.Context, robot.Robot) {
	t.Helper()

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pos, nil
	}
	injectRobot := &inject.Robot{}
	injectRobot.ConfigFunc = func() *config.Config { return &config.Config{} }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return resources }
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return injectArm, nil
	}
	injectRobot.LoggerFunc = func() golog.Logger { return golog.NewTestLogger(t) }
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	return context.Background(), injectRobot
}

func TestForeignResource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, robot := setupRobotCtx(t)

	svc := web.New(robot, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
	// WebRTC connections across unix sockets can create deadlock in CI.
	conn, err := rgrpc.Dial(context.Background(), addr, logger, rpc.WithForceDirectGRPC())
	test.That(t, err, test.ShouldBeNil)

	myCompClient := gizmopb.NewGizmoServiceClient(conn)
	_, err = myCompClient.DoOne(ctx, &gizmopb.DoOneRequest{Name: "thing1", Arg1: "hello"})
	test.That(t, err, test.ShouldNotBeNil)
	errStatus, ok := status.FromError(err)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, errStatus.Code(), test.ShouldEqual, codes.Unimplemented)

	test.That(t, svc.Close(ctx), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)

	remoteServer := grpc.NewServer()
	gizmopb.RegisterGizmoServiceServer(remoteServer, &myCompServer{})

	listenerR, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	go remoteServer.Serve(listenerR)
	defer remoteServer.Stop()

	// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
	// WebRTC connections across unix sockets can create deadlock in CI.
	remoteConn, err := rgrpc.Dial(context.Background(), listenerR.Addr().String(),
		logger, rpc.WithForceDirectGRPC())
	test.That(t, err, test.ShouldBeNil)

	resourceAPI := resource.NewAPI(
		"acme",
		"component",
		"mycomponent",
	)
	resName := resource.NewName(resourceAPI, "thing1")

	foreignRes := rgrpc.NewForeignResource(resName, remoteConn)

	svcDesc, err := grpcreflect.LoadServiceDescriptor(&gizmopb.GizmoService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	injectRobot := &inject.Robot{}
	injectRobot.LoggerFunc = func() golog.Logger { return logger }
	injectRobot.ConfigFunc = func() *config.Config { return &config.Config{} }
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{
			resource.NewName(resourceAPI, "thing1"),
		}
	}
	injectRobot.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return foreignRes, nil
	}

	listener := testutils.ReserveRandomListener(t)
	addr = listener.Addr().String()
	options.Network.Listener = listener
	svc = web.New(injectRobot, logger)
	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// TODO(RSDK-4473) Reenable WebRTC when we figure out why multiple
	// WebRTC connections across unix sockets can create deadlock in CI.
	conn, err = rgrpc.Dial(context.Background(), addr, logger, rpc.WithForceDirectGRPC())
	test.That(t, err, test.ShouldBeNil)

	myCompClient = gizmopb.NewGizmoServiceClient(conn)

	injectRobot.Mu.Lock()
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI {
		return nil
	}
	injectRobot.Mu.Unlock()

	_, err = myCompClient.DoOne(ctx, &gizmopb.DoOneRequest{Name: "thing1", Arg1: "hello"})
	test.That(t, err, test.ShouldNotBeNil)
	errStatus, ok = status.FromError(err)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, errStatus.Code(), test.ShouldEqual, codes.Unimplemented)

	injectRobot.Mu.Lock()
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI {
		return []resource.RPCAPI{
			{
				API:  resourceAPI,
				Desc: svcDesc,
			},
		}
	}
	injectRobot.Mu.Unlock()

	resp, err := myCompClient.DoOne(ctx, &gizmopb.DoOneRequest{Name: "thing1", Arg1: "hello"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Ret1, test.ShouldBeTrue)

	test.That(t, svc.Close(ctx), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
	test.That(t, remoteConn.Close(), test.ShouldBeNil)
}

type myCompServer struct {
	gizmopb.UnimplementedGizmoServiceServer
}

func (s *myCompServer) DoOne(ctx context.Context, req *gizmopb.DoOneRequest) (*gizmopb.DoOneResponse, error) {
	return &gizmopb.DoOneResponse{Ret1: req.Arg1 == "hello"}, nil
}

func TestRawClientOperation(t *testing.T) {
	// Need an unfiltered streaming call to test interceptors
	echoAPI := resource.NewAPI("rdk", "component", "echo")
	resource.RegisterAPI(echoAPI, resource.APIRegistration[resource.Resource]{
		RPCServiceServerConstructor: func(apiResColl resource.APIResourceCollection[resource.Resource]) interface{} { return &echoServer{} },
		RPCServiceHandler:           echopb.RegisterTestEchoServiceHandlerFromEndpoint,
		RPCServiceDesc:              &echopb.TestEchoService_ServiceDesc,
	})

	logger := golog.NewTestLogger(t)
	ctx, iRobot := setupRobotCtx(t)

	svc := web.New(iRobot, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	iRobot.(*inject.Robot).StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
		return []robot.Status{}, nil
	}

	checkOpID := func(md metadata.MD, expected bool) {
		t.Helper()
		if expected {
			test.That(t, md["opid"], test.ShouldHaveLength, 1)
			_, err = uuid.Parse(md["opid"][0])
			test.That(t, err, test.ShouldBeNil)
		} else {
			// StreamStatus is in operations' list of filtered methods, so expect no opID.
			test.That(t, md["opid"], test.ShouldHaveLength, 0)
		}
	}

	conn, err := rgrpc.Dial(context.Background(), addr, logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
	test.That(t, err, test.ShouldBeNil)
	client := robotpb.NewRobotServiceClient(conn)

	var hdr metadata.MD
	_, err = client.GetStatus(ctx, &robotpb.GetStatusRequest{}, grpc.Header(&hdr))
	test.That(t, err, test.ShouldBeNil)
	checkOpID(hdr, true)

	streamClient, err := client.StreamStatus(ctx, &robotpb.StreamStatusRequest{})
	test.That(t, err, test.ShouldBeNil)
	md, err := streamClient.Header()
	test.That(t, err, test.ShouldBeNil)
	checkOpID(md, false) // StreamStatus is in the filtered method list, so doesn't get an opID
	test.That(t, conn.Close(), test.ShouldBeNil)

	// test with a simple echo proto as well
	conn, err = rgrpc.Dial(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	echoclient := echopb.NewTestEchoServiceClient(conn)

	hdr = metadata.MD{}
	trailers := metadata.MD{} // won't do anything but helps test goutils
	_, err = echoclient.Echo(ctx, &echopb.EchoRequest{}, grpc.Header(&hdr), grpc.Trailer(&trailers))
	test.That(t, err, test.ShouldBeNil)
	checkOpID(hdr, true)

	echoStreamClient, err := echoclient.EchoMultiple(ctx, &echopb.EchoMultipleRequest{})
	test.That(t, err, test.ShouldBeNil)
	md, err = echoStreamClient.Header()
	test.That(t, err, test.ShouldBeNil)
	checkOpID(md, true) // EchoMultiple is NOT filtered, so should have an opID
	test.That(t, conn.Close(), test.ShouldBeNil)

	test.That(t, svc.Close(ctx), test.ShouldBeNil)
}

func TestInboundMethodTimeout(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, iRobot := setupRobotCtx(t)

	t.Run("web start", func(t *testing.T) {
		t.Run("default timeout", func(t *testing.T) {
			svc := web.New(iRobot, logger)
			options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)

			err := svc.Start(ctx, options)
			test.That(t, err, test.ShouldBeNil)

			// Use an injected status function to check that the default deadline was added
			// to the context.
			iRobot.(*inject.Robot).StatusFunc = func(ctx context.Context,
				resourceNames []resource.Name,
			) ([]robot.Status, error) {
				deadline, deadlineSet := ctx.Deadline()
				test.That(t, deadlineSet, test.ShouldBeTrue)
				// Assert that deadline is between 9 and 10 minutes from now (some time will
				// have elapsed).
				test.That(t, deadline, test.ShouldHappenBetween,
					time.Now().Add(time.Minute*9), time.Now().Add(time.Minute*10))

				return []robot.Status{}, nil
			}

			conn, err := rgrpc.Dial(context.Background(), addr, logger,
				rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
			test.That(t, err, test.ShouldBeNil)
			client := robotpb.NewRobotServiceClient(conn)

			// Use GetStatus to call injected status function.
			_, err = client.GetStatus(ctx, &robotpb.GetStatusRequest{})
			test.That(t, err, test.ShouldBeNil)

			test.That(t, conn.Close(), test.ShouldBeNil)
			test.That(t, svc.Close(ctx), test.ShouldBeNil)
		})
		t.Run("overridden timeout", func(t *testing.T) {
			svc := web.New(iRobot, logger)
			options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)

			err := svc.Start(ctx, options)
			test.That(t, err, test.ShouldBeNil)

			// Use an injected status function to check that the default deadline was not
			// added to the context, and the deadline passed to GetStatus was used instead.
			iRobot.(*inject.Robot).StatusFunc = func(ctx context.Context,
				resourceNames []resource.Name,
			) ([]robot.Status, error) {
				deadline, deadlineSet := ctx.Deadline()
				test.That(t, deadlineSet, test.ShouldBeTrue)
				// Assert that deadline is between 4 and 5 minutes from now (some time will
				// have elapsed).
				test.That(t, deadline, test.ShouldHappenBetween,
					time.Now().Add(time.Minute*4), time.Now().Add(time.Minute*5))
				return []robot.Status{}, nil
			}

			conn, err := rgrpc.Dial(context.Background(), addr, logger,
				rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
			test.That(t, err, test.ShouldBeNil)
			client := robotpb.NewRobotServiceClient(conn)

			// Use GetStatus and a context with a deadline to call injected status function.
			overrideCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			_, err = client.GetStatus(overrideCtx, &robotpb.GetStatusRequest{})
			test.That(t, err, test.ShouldBeNil)

			test.That(t, conn.Close(), test.ShouldBeNil)
			test.That(t, svc.Close(ctx), test.ShouldBeNil)
		})
	})
	t.Run("module start", func(t *testing.T) {
		t.Run("default timeout", func(t *testing.T) {
			svc := web.New(iRobot, logger)

			err := svc.StartModule(ctx)
			test.That(t, err, test.ShouldBeNil)

			// Use an injected status function to check that the default deadline was added
			// to the context.
			iRobot.(*inject.Robot).StatusFunc = func(ctx context.Context,
				resourceNames []resource.Name,
			) ([]robot.Status, error) {
				deadline, deadlineSet := ctx.Deadline()
				test.That(t, deadlineSet, test.ShouldBeTrue)
				// Assert that deadline is between 9 and 10 minutes from now (some time will
				// have elapsed).
				test.That(t, deadline, test.ShouldHappenBetween,
					time.Now().Add(time.Minute*9), time.Now().Add(time.Minute*10))

				return []robot.Status{}, nil
			}

			conn, err := rgrpc.Dial(context.Background(), "unix://"+svc.ModuleAddress(),
				logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
			test.That(t, err, test.ShouldBeNil)
			client := robotpb.NewRobotServiceClient(conn)

			// Use GetStatus to call injected status function.
			_, err = client.GetStatus(ctx, &robotpb.GetStatusRequest{})
			test.That(t, err, test.ShouldBeNil)

			test.That(t, conn.Close(), test.ShouldBeNil)
			test.That(t, svc.Close(ctx), test.ShouldBeNil)
		})
		t.Run("overridden timeout", func(t *testing.T) {
			svc := web.New(iRobot, logger)

			err := svc.StartModule(ctx)
			test.That(t, err, test.ShouldBeNil)

			// Use an injected status function to check that the default deadline was not
			// added to the context, and the deadline passed to GetStatus was used instead.
			iRobot.(*inject.Robot).StatusFunc = func(ctx context.Context,
				resourceNames []resource.Name,
			) ([]robot.Status, error) {
				deadline, deadlineSet := ctx.Deadline()
				test.That(t, deadlineSet, test.ShouldBeTrue)
				// Assert that deadline is between 4 and 5 minutes from now (some time will
				// have elapsed).
				test.That(t, deadline, test.ShouldHappenBetween,
					time.Now().Add(time.Minute*4), time.Now().Add(time.Minute*5))
				return []robot.Status{}, nil
			}

			conn, err := rgrpc.Dial(context.Background(), "unix://"+svc.ModuleAddress(),
				logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
			test.That(t, err, test.ShouldBeNil)
			client := robotpb.NewRobotServiceClient(conn)

			// Use GetStatus and a context with a deadline to call injected status function.
			overrideCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			_, err = client.GetStatus(overrideCtx, &robotpb.GetStatusRequest{})
			test.That(t, err, test.ShouldBeNil)

			test.That(t, conn.Close(), test.ShouldBeNil)
			test.That(t, svc.Close(ctx), test.ShouldBeNil)
		})
	})
}

type echoServer struct {
	echopb.UnimplementedTestEchoServiceServer
}

func (srv *echoServer) EchoMultiple(
	req *echopb.EchoMultipleRequest,
	server echopb.TestEchoService_EchoMultipleServer,
) error {
	return server.Send(&echopb.EchoMultipleResponse{})
}

func (srv *echoServer) Echo(context.Context, *echopb.EchoRequest) (*echopb.EchoResponse, error) {
	return &echopb.EchoResponse{}, nil
}

// signJWKBasedExternalAccessToken returns an access jwt access token typically returned by an OIDC provider.
func signJWKBasedExternalAccessToken(
	key *rsa.PrivateKey,
	entity, aud, iss, keyID string,
) (string, error) {
	token := &jwt.Token{
		Header: map[string]interface{}{
			"typ": "JWT",
			"alg": jwt.SigningMethodRS256.Alg(),
			"kid": keyID,
		},
		Claims: rpc.JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Audience: []string{aud},
				Issuer:   iss,
				Subject:  fmt.Sprintf("someauthprovider/%s", entity),
				IssuedAt: jwt.NewNumericDate(time.Now()),
			},
			AuthCredentialsType: rpc.CredentialsTypeExternal,
		},
		Method: jwt.SigningMethodRS256,
	}

	return token.SignedString(key)
}
