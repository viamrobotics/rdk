package web_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/lestrrat-go/jwx/jwk"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	echopb "go.viam.com/api/component/testecho/v1"
	robotpb "go.viam.com/api/robot/v1"
	streampb "go.viam.com/api/stream/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	gizmopb "go.viam.com/rdk/examples/customresources/apis/proto/api/component/gizmo/v1"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/x264"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	rclient "go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	genericservice "go.viam.com/rdk/services/generic"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

const arm1String = "arm1"

var resources = []resource.Name{arm.Named(arm1String)}

var pos = spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})

func TestWebStart(t *testing.T) {
	logger := logging.NewTestLogger(t)
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
	logger := logging.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx(t)

	svc := web.New(injectRobot, logger)

	err := svc.StartModule(ctx)
	test.That(t, err, test.ShouldBeNil)

	conn1, err := rgrpc.Dial(context.Background(), "unix://"+svc.ModuleAddresses().UnixAddr, logger)
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
	logger := logging.NewTestLogger(t)
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
	logger := logging.NewTestLogger(t)
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
			apiKeyID1 := uuid.New().String()
			apiKey1 := utils.RandomAlphaString(32)
			apiKeyID2 := uuid.New().String()
			apiKey2 := utils.RandomAlphaString(32)
			locationSecrets := []string{"locsosecret", "locsec2"}
			options.Auth.Handlers = []config.AuthHandlerConfig{
				{
					Type: rpc.CredentialsTypeAPIKey,
					Config: rutils.AttributeMap{
						apiKeyID1: apiKey1,
						apiKeyID2: apiKey2,
						"keys":    []string{apiKeyID1, apiKeyID2},
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
					rutils.WithEntityCredentials("wrong", rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey1,
					}))
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

				_, err = rgrpc.Dial(context.Background(), addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rutils.WithEntityCredentials("wrong", rpc.Credentials{
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
					rpc.WithEntityCredentials(apiKeyID1, rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey1,
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

				_, err = rgrpc.Dial(context.Background(), addr, logger, rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials(apiKeyID1, rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey2,
					}))
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

				_, err = rgrpc.Dial(context.Background(), addr, logger, rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials(entityName, rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey1,
					}))
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

				conn, err = rgrpc.Dial(context.Background(), addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials(apiKeyID1, rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey1,
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

				conn, err = rgrpc.Dial(context.Background(), addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials(apiKeyID2, rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey2,
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
					rpc.WithEntityCredentials(apiKeyID1, rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey1,
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
	logger := logging.NewTestLogger(t)
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
		rutils.WithEntityCredentials("wrong", rpc.Credentials{
			Type:    rutils.CredentialsTypeRobotLocationSecret,
			Payload: locationSecret,
		}),
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

	// use secret
	conn, err := rgrpc.Dial(context.Background(), addr, logger,
		rpc.WithTLSConfig(clientTLSConfig),
		rutils.WithEntityCredentials(options.FQDN, rpc.Credentials{
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
	logger := logging.NewTestLogger(t)
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

func TestWebWithOnlyNewAPIKeyAuthHandlers(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx(t)

	svc := web.New(injectRobot, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	apiKeyID1 := uuid.New().String()
	apiKey1 := utils.RandomAlphaString(32)
	apiKeyID2 := uuid.New().String()
	apiKey2 := utils.RandomAlphaString(32)
	options.Auth.Handlers = []config.AuthHandlerConfig{
		{
			Type: rpc.CredentialsTypeAPIKey,
			Config: rutils.AttributeMap{
				apiKeyID1: apiKey1,
				apiKeyID2: apiKey2,
				"keys":    []string{apiKeyID1, apiKeyID2},
			},
		},
	}

	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	_, err = rgrpc.Dial(context.Background(), addr, logger, rpc.WithAllowInsecureWithCredentialsDowngrade(),
		rpc.WithEntityCredentials(apiKeyID1, rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: apiKey2,
		}))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

	_, err = rgrpc.Dial(context.Background(), addr, logger, rpc.WithAllowInsecureWithCredentialsDowngrade(),
		rpc.WithEntityCredentials("something-different", rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: apiKey1,
		}))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

	conn, err := rgrpc.Dial(context.Background(), addr, logger,
		rpc.WithAllowInsecureWithCredentialsDowngrade(),
		rpc.WithEntityCredentials(apiKeyID1, rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: apiKey1,
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

	conn, err = rgrpc.Dial(context.Background(), addr, logger,
		rpc.WithAllowInsecureWithCredentialsDowngrade(),
		rpc.WithEntityCredentials(apiKeyID2, rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: apiKey2,
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

	test.That(t, svc.Close(context.Background()), test.ShouldBeNil)
}

func TestWebWithStreams(t *testing.T) {
	const (
		camera1Key = "camera1"
		camera2Key = "camera2"
		audioKey   = "audio"
	)

	// Start a robot with a camera
	robot := &inject.Robot{}
	cam1 := inject.NewCamera(camera1Key)
	cam1.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	rs := map[resource.Name]resource.Resource{cam1.Name(): cam1}
	robot.MockResourcesFromMap(rs)

	ctx, cancel := context.WithCancel(context.Background())

	// Start service
	logger := logging.NewTestLogger(t)
	robot.LoggerFunc = func() logging.Logger { return logger }
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	svc := web.New(robot, logger, web.WithStreamConfig(gostream.StreamConfig{
		VideoEncoderFactory: x264.NewEncoderFactory(),
	}))
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
	cam2 := inject.NewCamera(camera2Key)
	cam2.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	robot.Mu.Lock()
	rs[cam2.Name()] = cam2
	robot.Mu.Unlock()
	robot.MockResourcesFromMap(rs)
	err = svc.Reconfigure(context.Background(), rs, resource.Config{})
	test.That(t, err, test.ShouldBeNil)

	// Test that new streams are available
	resp, err = streamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Names, test.ShouldContain, camera1Key)
	test.That(t, resp.Names, test.ShouldContain, camera2Key)
	test.That(t, resp.Names, test.ShouldHaveLength, 2)

	// We need to cancel otherwise we are stuck waiting for WebRTC to start streaming.
	cancel()
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
	<-ctx.Done()
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
	logger := logging.NewTestLogger(t)
	robot.LoggerFunc = func() logging.Logger { return logger }
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
	cam1 := inject.NewCamera(camera1Key)
	cam1.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	robot.Mu.Lock()
	rs[cam1.Name()] = cam1
	robot.Mu.Unlock()
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
	<-ctx.Done()
}

func TestWebStreamImmediateClose(t *testing.T) {
	// Primarily a regression test for RSDK-2418

	// Start a robot with a camera
	robot := &inject.Robot{}
	cam1 := &inject.Camera{
		PropertiesFunc: func(ctx context.Context) (camera.Properties, error) {
			return camera.Properties{}, nil
		},
	}
	rs := map[resource.Name]resource.Resource{camera.Named("camera1"): cam1}
	robot.MockResourcesFromMap(rs)

	ctx, cancel := context.WithCancel(context.Background())

	// Start service
	logger := logging.NewTestLogger(t)
	robot.LoggerFunc = func() logging.Logger { return logger }
	options, _, _ := robottestutils.CreateBaseOptionsAndListener(t)
	svc := web.New(robot, logger, web.WithStreamConfig(x264.DefaultStreamConfig))
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Immediately Close service.
	cancel()
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
	<-ctx.Done()
}

type (
	setupRobotOption  func(*setupRobotOptions) *setupRobotOptions
	setupRobotOptions struct {
		armEndPosition armEndPositionFunc
		machineStatus  machineStatusFunc
	}
	armEndPositionFunc func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error)
	machineStatusFunc  func(ctx context.Context) (robot.MachineStatus, error)
)

func withArmEndPosition(f armEndPositionFunc) setupRobotOption {
	return func(sro *setupRobotOptions) *setupRobotOptions {
		sro.armEndPosition = f
		return sro
	}
}

func withMachineStatus(f machineStatusFunc) setupRobotOption {
	return func(sro *setupRobotOptions) *setupRobotOptions {
		sro.machineStatus = f
		return sro
	}
}

func setupRobotCtx(t *testing.T, opts ...setupRobotOption) (context.Context, robot.LocalRobot) {
	t.Helper()

	options := &setupRobotOptions{
		armEndPosition: func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
			return pos, nil
		},
	}
	for _, o := range opts {
		options = o(options)
	}

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = options.armEndPosition
	injectRobot := &inject.Robot{}
	injectRobot.ConfigFunc = func() *config.Config { return &config.Config{} }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return resources }
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return injectArm, nil
	}
	injectRobot.FindBySimpleNameAndAPIFunc = func(string, resource.API) (resource.Resource, error) {
		return injectArm, nil
	}
	injectRobot.LoggerFunc = func() logging.Logger { return logging.NewTestLogger(t) }
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	if options.machineStatus != nil {
		injectRobot.MachineStatusFunc = options.machineStatus
	}

	return context.Background(), injectRobot
}

func TestForeignResource(t *testing.T) {
	logger := logging.NewTestLogger(t)
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
	injectRobot.LoggerFunc = func() logging.Logger { return logger }
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
		RPCServiceServerConstructor: func(apiResColl resource.APIResourceGetter[resource.Resource], logger logging.Logger) interface{} {
			return &echoServer{}
		},
		RPCServiceHandler: echopb.RegisterTestEchoServiceHandlerFromEndpoint,
		RPCServiceDesc:    &echopb.TestEchoService_ServiceDesc,
	})
	defer resource.DeregisterAPI(echoAPI)

	logger := logging.NewTestLogger(t)
	ctx, iRobot := setupRobotCtx(t)

	svc := web.New(iRobot, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	iRobot.(*inject.Robot).MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{}, nil
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
	_, err = client.GetMachineStatus(ctx, &robotpb.GetMachineStatusRequest{}, grpc.Header(&hdr))
	test.That(t, err, test.ShouldBeNil)
	checkOpID(hdr, true)

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
	md, err := echoStreamClient.Header()
	test.That(t, err, test.ShouldBeNil)
	checkOpID(md, true) // EchoMultiple is NOT filtered, so should have an opID
	test.That(t, conn.Close(), test.ShouldBeNil)

	test.That(t, svc.Close(ctx), test.ShouldBeNil)
}

func TestUnaryRequestCounter(t *testing.T) {
	echoAPI := resource.NewAPI("rdk", "component", "echo")
	resource.RegisterAPI(echoAPI, resource.APIRegistration[resource.Resource]{
		RPCServiceServerConstructor: func(apiResColl resource.APIResourceGetter[resource.Resource], logger logging.Logger) interface{} {
			return &echoServer{}
		},
		RPCServiceHandler: echopb.RegisterTestEchoServiceHandlerFromEndpoint,
		RPCServiceDesc:    &echopb.TestEchoService_ServiceDesc,
	})
	defer resource.DeregisterAPI(echoAPI)

	logger := logging.NewTestLogger(t)
	ctx, iRobot := setupRobotCtx(t)

	svc := web.New(iRobot, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	iRobot.(*inject.Robot).MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{}, nil
	}
	iRobot.(*inject.Robot).FindBySimpleNameAndAPIFunc = func(s string, a resource.API) (resource.Resource, error) {
		return nil, resource.NewNotFoundError(resource.NewName(a, s))
	}

	conn, err := rgrpc.Dial(context.Background(), addr, logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
	test.That(t, err, test.ShouldBeNil)

	// test un-targeted (no name field) counts
	client := robotpb.NewRobotServiceClient(conn)

	_, ok := svc.RequestCounter().Stats().(map[string]int64)["RobotService/GetMachineStatus"]
	test.That(t, ok, test.ShouldBeFalse)

	_, err = client.GetMachineStatus(ctx, &robotpb.GetMachineStatusRequest{})
	test.That(t, err, test.ShouldBeNil)
	count := svc.RequestCounter().Stats().(map[string]int64)["RobotService/GetMachineStatus"]
	test.That(t, count, test.ShouldEqual, 1)

	_, err = client.GetMachineStatus(ctx, &robotpb.GetMachineStatusRequest{})
	test.That(t, err, test.ShouldBeNil)
	count = svc.RequestCounter().Stats().(map[string]int64)["RobotService/GetMachineStatus"]
	test.That(t, count, test.ShouldEqual, 2)

	// test targeted (with name field) counts
	echoclient := echopb.NewTestEchoServiceClient(conn)

	_, ok = svc.RequestCounter().Stats().(map[string]int64)["test1.TestEchoService/Echo"]
	test.That(t, ok, test.ShouldBeFalse)

	_, err = echoclient.Echo(ctx, &echopb.EchoRequest{Name: "test1"})
	test.That(t, err, test.ShouldBeNil)
	count = svc.RequestCounter().Stats().(map[string]int64)["test1.TestEchoService/Echo"]
	test.That(t, count, test.ShouldEqual, 1)

	_, err = echoclient.Echo(ctx, &echopb.EchoRequest{Name: "test1"})
	test.That(t, err, test.ShouldBeNil)
	count = svc.RequestCounter().Stats().(map[string]int64)["test1.TestEchoService/Echo"]
	test.That(t, count, test.ShouldEqual, 2)

	_, err = echoclient.Echo(ctx, &echopb.EchoRequest{Name: "test2"})
	test.That(t, err, test.ShouldBeNil)
	count = svc.RequestCounter().Stats().(map[string]int64)["test2.TestEchoService/Echo"]
	test.That(t, count, test.ShouldEqual, 1)

	// test service with a name field
	genericclient, err := genericservice.NewClientFromConn(ctx, conn, "", genericservice.Named("generictest"), logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = genericclient.DoCommand(ctx, nil)
	// errors here but RC still counts the request.
	test.That(t, err.Error(), test.ShouldEqual,
		"rpc error: code = Unknown desc = resource rdk:service:generic/generictest not found")

	count = svc.RequestCounter().Stats().(map[string]int64)["generictest.GenericService/DoCommand"]
	test.That(t, count, test.ShouldEqual, 1)

	test.That(t, conn.Close(), test.ShouldBeNil)
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
}

func TestStreamingRequestCounter(t *testing.T) {
	echoAPI := resource.NewAPI("rdk", "component", "echo")
	resource.RegisterAPI(echoAPI, resource.APIRegistration[resource.Resource]{
		RPCServiceServerConstructor: func(apiResColl resource.APIResourceGetter[resource.Resource], logger logging.Logger) interface{} {
			return &echoServer{}
		},
		RPCServiceHandler: echopb.RegisterTestEchoServiceHandlerFromEndpoint,
		RPCServiceDesc:    &echopb.TestEchoService_ServiceDesc,
	})
	defer resource.DeregisterAPI(echoAPI)

	logger := logging.NewTestLogger(t)
	ctx, iRobot := setupRobotCtx(t)

	svc := web.New(iRobot, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	conn, err := rgrpc.Dial(context.Background(), addr, logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
	test.That(t, err, test.ShouldBeNil)
	echoclient := echopb.NewTestEchoServiceClient(conn)

	// test counting streaming service with name
	_, ok := svc.RequestCounter().Stats().(map[string]int64)["test1.TestEchoService/EchoMultiple"]
	test.That(t, ok, test.ShouldBeFalse)
	s, err := echoclient.EchoMultiple(ctx, &echopb.EchoMultipleRequest{Name: "test1", Message: ""})
	test.That(t, err, test.ShouldBeNil)
	_, err = s.Recv()

	test.That(t, err, test.ShouldBeNil)
	count := svc.RequestCounter().Stats().(map[string]int64)["test1.TestEchoService/EchoMultiple"]
	test.That(t, count, test.ShouldEqual, 1)

	s, err = echoclient.EchoMultiple(ctx, &echopb.EchoMultipleRequest{Name: "test1", Message: ""})
	test.That(t, err, test.ShouldBeNil)
	_, err = s.Recv()
	test.That(t, err, test.ShouldBeNil)
	count = svc.RequestCounter().Stats().(map[string]int64)["test1.TestEchoService/EchoMultiple"]
	test.That(t, count, test.ShouldEqual, 2)

	// test named bidirectional stream (client sends multiple messages, but RC only increments once)
	client, err := echoclient.EchoBiDi(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = client.Send(&echopb.EchoBiDiRequest{Name: "qwerty", Message: "asdfg"})
	test.That(t, err, test.ShouldBeNil)
	ch, err := client.Recv()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ch.GetMessage(), test.ShouldEqual, "a")
	stats := svc.RequestCounter().Stats().(map[string]int64)
	test.That(t, stats["qwerty.TestEchoService/EchoBiDi"], test.ShouldEqual, 1)
	test.That(t, stats, test.ShouldContainKey, "qwerty.TestEchoService/EchoBiDi.dataSentBytes")
	test.That(t, stats["qwerty.TestEchoService/EchoBiDi.dataSentBytes"], test.ShouldBeGreaterThan, 0)

	err = client.Send(&echopb.EchoBiDiRequest{Name: "qwerty", Message: "zxcvb"})
	test.That(t, err, test.ShouldBeNil)
	err = client.CloseSend()
	test.That(t, err, test.ShouldBeNil)
	count = svc.RequestCounter().Stats().(map[string]int64)["qwerty.TestEchoService/EchoBiDi"]
	test.That(t, count, test.ShouldEqual, 1)

	// EchoBiDi echoes back all received msgs one character at a time.
	// 10 in total for this test & the first one is checked separately above.
	for range 9 {
		ch, err := client.Recv()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ch.GetMessage()), test.ShouldEqual, 1)
	}
	count = svc.RequestCounter().Stats().(map[string]int64)["qwerty.TestEchoService/EchoBiDi"]
	test.That(t, count, test.ShouldEqual, 1)

	test.That(t, conn.Close(), test.ShouldBeNil)
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
}

func TestInboundMethodTimeout(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx, iRobot := setupRobotCtx(t)

	t.Run("web start", func(t *testing.T) {
		t.Run("default timeout", func(t *testing.T) {
			svc := web.New(iRobot, logger)
			options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)

			err := svc.Start(ctx, options)
			test.That(t, err, test.ShouldBeNil)

			// Use an injected status function to check that the default deadline was added
			// to the context.
			iRobot.(*inject.Robot).MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
				deadline, deadlineSet := ctx.Deadline()
				test.That(t, deadlineSet, test.ShouldBeTrue)
				// Assert that deadline is between 9 and 10 minutes from now (some time will
				// have elapsed).
				test.That(t, deadline, test.ShouldHappenBetween,
					time.Now().Add(time.Minute*9), time.Now().Add(time.Minute*10))

				return robot.MachineStatus{}, nil
			}

			conn, err := rgrpc.Dial(context.Background(), addr, logger,
				rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
			test.That(t, err, test.ShouldBeNil)
			client := robotpb.NewRobotServiceClient(conn)

			// Use GetMachineStatus to call injected status function.
			_, err = client.GetMachineStatus(ctx, &robotpb.GetMachineStatusRequest{})
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
			// added to the context, and the deadline passed to GetMachineStatus was used instead.
			iRobot.(*inject.Robot).MachineStatusFunc = func(ctx context.Context,
			) (robot.MachineStatus, error) {
				deadline, deadlineSet := ctx.Deadline()
				test.That(t, deadlineSet, test.ShouldBeTrue)
				// Assert that deadline is between 4 and 5 minutes from now (some time will
				// have elapsed).
				test.That(t, deadline, test.ShouldHappenBetween,
					time.Now().Add(time.Minute*4), time.Now().Add(time.Minute*5))
				return robot.MachineStatus{}, nil
			}

			conn, err := rgrpc.Dial(context.Background(), addr, logger,
				rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
			test.That(t, err, test.ShouldBeNil)
			client := robotpb.NewRobotServiceClient(conn)

			// Use GetMachineStatus and a context with a deadline to call injected status function.
			overrideCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			_, err = client.GetMachineStatus(overrideCtx, &robotpb.GetMachineStatusRequest{})
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
			iRobot.(*inject.Robot).MachineStatusFunc = func(ctx context.Context,
			) (robot.MachineStatus, error) {
				deadline, deadlineSet := ctx.Deadline()
				test.That(t, deadlineSet, test.ShouldBeTrue)
				// Assert that deadline is between 9 and 10 minutes from now (some time will
				// have elapsed).
				test.That(t, deadline, test.ShouldHappenBetween,
					time.Now().Add(time.Minute*9), time.Now().Add(time.Minute*10))

				return robot.MachineStatus{}, nil
			}

			conn, err := rgrpc.Dial(context.Background(), "unix://"+svc.ModuleAddresses().UnixAddr,
				logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
			test.That(t, err, test.ShouldBeNil)
			client := robotpb.NewRobotServiceClient(conn)

			// Use GetMachineStatus to call injected status function.
			_, err = client.GetMachineStatus(ctx, &robotpb.GetMachineStatusRequest{})
			test.That(t, err, test.ShouldBeNil)

			test.That(t, conn.Close(), test.ShouldBeNil)
			test.That(t, svc.Close(ctx), test.ShouldBeNil)
		})
		t.Run("overridden timeout", func(t *testing.T) {
			svc := web.New(iRobot, logger)

			err := svc.StartModule(ctx)
			test.That(t, err, test.ShouldBeNil)

			// Use an injected status function to check that the default deadline was not
			// added to the context, and the deadline passed to GetMachineStatus was used instead.
			iRobot.(*inject.Robot).MachineStatusFunc = func(ctx context.Context,
			) (robot.MachineStatus, error) {
				deadline, deadlineSet := ctx.Deadline()
				test.That(t, deadlineSet, test.ShouldBeTrue)
				// Assert that deadline is between 4 and 5 minutes from now (some time will
				// have elapsed).
				test.That(t, deadline, test.ShouldHappenBetween,
					time.Now().Add(time.Minute*4), time.Now().Add(time.Minute*5))
				return robot.MachineStatus{}, nil
			}

			conn, err := rgrpc.Dial(context.Background(), "unix://"+svc.ModuleAddresses().UnixAddr,
				logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
			test.That(t, err, test.ShouldBeNil)
			client := robotpb.NewRobotServiceClient(conn)

			// Use GetMachineStatus and a context with a deadline to call injected status function.
			overrideCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			_, err = client.GetMachineStatus(overrideCtx, &robotpb.GetMachineStatusRequest{})
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

// EchoBiDi responds to incoming Message(s) by echoing back one character at a time.
func (srv *echoServer) EchoBiDi(stream echopb.TestEchoService_EchoBiDiServer) error {
	for {
		in, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, ch := range in.GetMessage() {
			if err := stream.Send(&echopb.EchoBiDiResponse{Message: fmt.Sprintf("%c", ch)}); err != nil {
				return err
			}
		}
	}
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

func TestPerRequestFTDC(t *testing.T) {
	// This test creates a robot with a resource running with a web service. It will then assert
	// that making gRPC requests will increment counters output by the `RequestCounter`s `Stats`
	// call.
	logger := logging.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx(t)
	defer injectRobot.Close(ctx)

	svc := web.New(injectRobot, logger)
	defer svc.Stop()
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)

	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Dial to the robot and create a gRPC client object to the "arm" specifically.
	conn, err := rgrpc.Dial(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(conn.Close)
	armClient, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
	//nolint
	defer armClient.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	// Making a gRPC `EndPosition` call with the default inject method returns a success.
	_, err = armClient.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	// We can assert that there are 5 counters in our stats: 4 for the api and 1
	// for the resource. The fact that `GetEndPosition` was called once and we
	// hence spent (negligible) time in that RPC call.
	stats := svc.RequestCounter().Stats().(map[string]int64)
	test.That(t, len(stats), test.ShouldEqual, 5)
	test.That(t, stats["arm1.ArmService/GetEndPosition"], test.ShouldEqual, 1)
	test.That(t, stats, test.ShouldContainKey, "arm1.ArmService/GetEndPosition.timeSpent")
	test.That(t, stats, test.ShouldContainKey, "arm1.ArmService/GetEndPosition.dataSentBytes")
	test.That(t, stats["arm1.ArmService/GetEndPosition.dataSentBytes"], test.ShouldBeGreaterThan, 0)
	test.That(t, stats, test.ShouldContainKey, "arm1.ArmService/GetEndPosition.errorCnt")
	test.That(t, stats, test.ShouldContainKey, "arm1.viam.component.arm.v1.ArmService.inFlightRequests")

	// Get a handle on the inject arm resource.
	injectArmRes, err := injectRobot.ResourceByName(arm.Named(arm1String))
	test.That(t, err, test.ShouldBeNil)
	injectArm := injectArmRes.(*inject.Arm)
	// Mutate the arm to have its `EndPosition` RPC call return an error.
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return nil, errors.New("error")
	}

	// Try calling `EndPosition` again. Assert it returned an error.
	_, err = armClient.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldNotBeNil)

	// Now observe that we called `GetEndPosition` a second time. And one of the responses returned
	// an error.
	stats = svc.RequestCounter().Stats().(map[string]int64)
	test.That(t, len(stats), test.ShouldEqual, 5)
	test.That(t, stats["arm1.ArmService/GetEndPosition"], test.ShouldEqual, 2)
	test.That(t, stats, test.ShouldContainKey, "arm1.ArmService/GetEndPosition.timeSpent")
	test.That(t, stats["arm1.ArmService/GetEndPosition.errorCnt"], test.ShouldEqual, 1)
	test.That(t, stats, test.ShouldContainKey, "arm1.ArmService/GetEndPosition.dataSentBytes")
}

type clientCall = func(context.Context) error

func testResourceLimitsAndFTDC(
	t *testing.T,
	keyPrefix string,
	method string,
	setupBlock func(onEnter, wait func()) setupRobotOption,
	createCall func(string, logging.Logger) clientCall,
) {
	logger, logs := logging.NewObservedTestLogger(t)

	blockCall := make(chan struct{})
	callBlocking := make(chan struct{})
	opt := setupBlock(
		func() {
			close(callBlocking)
		},
		func() {
			<-blockCall
		},
	)
	ctx, injectRobot := setupRobotCtx(t, opt)
	defer injectRobot.Close(ctx)

	originalRequestLimit := os.Getenv(rutils.ViamResourceRequestsLimitEnvVar)
	os.Setenv(rutils.ViamResourceRequestsLimitEnvVar, "1")
	t.Cleanup(func() {
		os.Setenv(rutils.ViamResourceRequestsLimitEnvVar, originalRequestLimit)
	})

	svc := web.New(injectRobot, logger)
	defer svc.Stop()
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)

	err := svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Create a caller to invoke the gRPC method used for testing
	call := createCall(addr, logger)

	// Check that the in-flight request counter is zero
	statsKey := keyPrefix + ".inFlightRequests"
	stats := svc.RequestCounter().Stats().(map[string]int64)
	test.That(t, stats[statsKey], test.ShouldEqual, 0)

	// Make a gRPC `EndPosition` call that hangs until we close the channel
	clientCallsWg := sync.WaitGroup{}
	clientCallsWg.Add(1)
	go func() {
		defer clientCallsWg.Done()
		err := call(ctx)
		test.That(t, err, test.ShouldBeNil)
	}()

	// Wait for the gRPC call to reach our function so we know the request
	// counters have been updated.
	<-callBlocking

	// Check that the in-flight request counter has increased to 1
	stats = svc.RequestCounter().Stats().(map[string]int64)
	test.That(t, stats[statsKey], test.ShouldEqual, 1)

	// Make a second request that should return an error due to the limit.
	err = call(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, status.Convert(err).Code(), test.ShouldEqual, codes.ResourceExhausted)
	test.That(t, err.Error(), test.ShouldEndWith,
		fmt.Sprintf(
			"exceeded request limit 1 on resource %v (your client is responsible for 1). "+
				"See %v for troubleshooting steps",
			keyPrefix,
			web.ReqLimitExceededURL,
		),
	)

	// Assert that an appropriate warning log was output by the server.
	reqLimitExceededLogs := logs.FilterMessageSnippet("Request limit exceeded").All()
	test.That(t, reqLimitExceededLogs, test.ShouldHaveLength, 1)
	test.That(t, reqLimitExceededLogs[0].Level, test.ShouldEqual, zapcore.WarnLevel)
	expectedMsg := fmt.Sprintf("Request limit exceeded for resource. See %v for troubleshooting steps. ", web.ReqLimitExceededURL)
	test.That(t, reqLimitExceededLogs[0].Message, test.ShouldStartWith, expectedMsg)

	var fields map[string]any
	err = json.Unmarshal(
		[]byte(strings.TrimPrefix(reqLimitExceededLogs[0].Message, expectedMsg)),
		&fields,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fields["method"], test.ShouldEqual, method)
	test.That(t, fields["resource"], test.ShouldEqual, keyPrefix)
	{
		offendingClientInformation := fields["offending_client_information"]
		offendingClientInformationM, ok := offendingClientInformation.(map[string]any)
		test.That(t, ok, test.ShouldBeTrue)
		// Assert exact values on inflight_requests and rejected_requests and assert the rest
		// of the fields simply exist.
		inFlightRequests := offendingClientInformationM["inflight_requests"]
		inFlightRequestsM, ok := inFlightRequests.(map[string]any)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, inFlightRequestsM[keyPrefix], test.ShouldEqual, 1)
		rejectedRequests := offendingClientInformationM["rejected_requests"]
		rejectedRequestsM, ok := rejectedRequests.(map[string]any)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, rejectedRequestsM[keyPrefix], test.ShouldEqual, 1)
		test.That(t, offendingClientInformationM["client_metadata"], test.ShouldNotBeNil)
		test.That(t, offendingClientInformationM["connection_id"], test.ShouldNotBeNil)
		test.That(t, offendingClientInformationM["connect_time"], test.ShouldNotBeNil)
		test.That(t, offendingClientInformationM["time_since_connect"], test.ShouldNotBeNil)
		test.That(t, offendingClientInformationM["server_ip"], test.ShouldNotBeNil)
		test.That(t, offendingClientInformationM["client_ip"], test.ShouldNotBeNil)
	}
	test.That(t, fields["all_other_client_information"], test.ShouldResemble, []any{})

	// In-flight requests counter should still only be 1
	stats = svc.RequestCounter().Stats().(map[string]int64)
	test.That(t, stats[statsKey], test.ShouldEqual, 1)

	// Release the original call and wait for it to complete
	close(blockCall)
	clientCallsWg.Wait()

	// In-flight requests counter should be back to 0
	stats = svc.RequestCounter().Stats().(map[string]int64)
	test.That(t, stats[statsKey], test.ShouldEqual, 0)
}

func TestPerResourceLimitsAndFTDC(t *testing.T) {
	t.Run("arm resource", func(t *testing.T) {
		testResourceLimitsAndFTDC(
			t,
			"arm1.viam.component.arm.v1.ArmService",
			"/viam.component.arm.v1.ArmService/GetEndPosition",
			func(onEnter, wait func()) setupRobotOption {
				return withArmEndPosition(func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
					onEnter()
					wait()
					return pos, nil
				})
			},
			func(addr string, logger logging.Logger) clientCall {
				conn, err := rgrpc.Dial(context.Background(), addr, logger)
				test.That(t, err, test.ShouldBeNil)
				t.Cleanup(func() {
					utils.UncheckedErrorFunc(conn.Close)
				})
				armClient, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
				test.That(t, err, test.ShouldBeNil)
				t.Cleanup(func() {
					armClient.Close(context.Background())
				})
				return func(ctx context.Context) error {
					_, err := armClient.EndPosition(ctx, nil)
					return err
				}
			},
		)
	})
	t.Run("robot service", func(t *testing.T) {
		testResourceLimitsAndFTDC(
			t,
			"viam.robot.v1.RobotService",
			"/viam.robot.v1.RobotService/GetMachineStatus",
			func(onEnter, wait func()) setupRobotOption {
				return withMachineStatus(func(ctx context.Context) (robot.MachineStatus, error) {
					onEnter()
					wait()
					return robot.MachineStatus{}, nil
				})
			},
			func(addr string, logger logging.Logger) clientCall {
				// The robot client implicitly calls MachineStatus by default when run
				// in a test. Disable that behavior since we're going to block the
				// first call to that method.
				originalDoNotWaitForRunning := rclient.DoNotWaitForRunning.Load()
				rclient.DoNotWaitForRunning.Store(true)
				t.Cleanup(func() {
					rclient.DoNotWaitForRunning.Store(originalDoNotWaitForRunning)
				})
				robotClient, err := rclient.New(context.Background(), addr, logger)
				test.That(t, err, test.ShouldBeNil)
				t.Cleanup(func() {
					robotClient.Close(context.Background())
				})
				return func(ctx context.Context) error {
					_, err := robotClient.MachineStatus(ctx)
					return err
				}
			},
		)
	})
	t.Run("request counter map pruning", func(t *testing.T) {
		// Primarily a regression test for RSDK-12896. Ensures that a client that has
		// connected, invoked a gRPC method on a resource, and disconnected before the request
		// limit exceeded is actually hit is not included in all_other_client_information.

		logger, logs := logging.NewObservedTestLogger(t)

		// Lower resource requests limit for test.
		originalRequestLimit := os.Getenv(rutils.ViamResourceRequestsLimitEnvVar)
		os.Setenv(rutils.ViamResourceRequestsLimitEnvVar, "1")
		t.Cleanup(func() {
			os.Setenv(rutils.ViamResourceRequestsLimitEnvVar, originalRequestLimit)
		})

		// Create and start web service of injected robot.
		callBlocking := make(chan struct{})
		blockCall := make(chan struct{})
		var numTimesEndPositionInvoked int
		ctx, injectRobot := setupRobotCtx(
			t,
			withArmEndPosition(func(ctx context.Context, extra map[string]any) (spatialmath.Pose, error) {
				// Do NOT block on first call to EndPosition.
				if numTimesEndPositionInvoked++; numTimesEndPositionInvoked < 2 {
					return pos, nil
				}
				close(callBlocking)
				<-blockCall
				return pos, nil
			}))
		defer injectRobot.Close(ctx)
		svc := web.New(injectRobot, logger)
		defer svc.Stop()
		options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
		err := svc.Start(ctx, options)
		test.That(t, err, test.ShouldBeNil)

		// Create a client, invoke EndPosition on the arm resource, and then close the client.
		// This EndPosition call should not block but should add the pc to rc.requestsPerPC
		// and rc.pcToClientMetadata.
		conn, err := rgrpc.Dial(context.Background(), addr, logger)
		test.That(t, err, test.ShouldBeNil)
		armClient, err := arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
		test.That(t, err, test.ShouldBeNil)
		_, err = armClient.EndPosition(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, armClient.Close(ctx), test.ShouldBeNil)
		utils.UncheckedErrorFunc(conn.Close)

		// Wait for two logs indicating the connection closure: one from the client-side
		// ("webrtc.client") and one from the server-side ("networking"):
		var observedCSChangedLogs *observer.ObservedLogs
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			observedCSChangedLogs = logs.FilterMessageSnippet("connection state changed")
			test.That(tb, observedCSChangedLogs.Len(), test.ShouldEqual, 2)
		})
		csChangedLogs := observedCSChangedLogs.All()
		test.That(t, strings.HasSuffix(csChangedLogs[0].LoggerName, "webrtc.client"), test.ShouldBeTrue)
		test.That(t, csChangedLogs[0].ContextMap()["conn_state"], test.ShouldEqual, "closed")
		test.That(t, strings.HasSuffix(csChangedLogs[1].LoggerName, "networking"), test.ShouldBeTrue)
		test.That(t, csChangedLogs[1].ContextMap()["conn_state"], test.ShouldEqual, "closed")

		// Create a new client, and invoke EndPosition in a goroutine on the arm resource.
		// This EndPosition call should block and should add to rc.requestsPerPC and
		// rc.pcToClientMetadata.
		conn, err = rgrpc.Dial(context.Background(), addr, logger)
		t.Cleanup(func() {
			utils.UncheckedErrorFunc(conn.Close)
		})
		test.That(t, err, test.ShouldBeNil)
		armClient, err = arm.NewClientFromConn(context.Background(), conn, "", arm.Named(arm1String), logger)
		test.That(t, err, test.ShouldBeNil)
		clientCallsWg := sync.WaitGroup{}
		clientCallsWg.Go(func() {
			_, err = armClient.EndPosition(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
		})

		// Wait for the gRPC call to reach our function so we know the request is "in-flight."
		<-callBlocking

		// Invoke EndPosition for a _third_ time. We should now error due to the limit.
		_, err = armClient.EndPosition(ctx, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, status.Convert(err).Code(), test.ShouldEqual, codes.ResourceExhausted)
		test.That(t, err.Error(), test.ShouldEndWith,
			fmt.Sprintf(
				"exceeded request limit 1 on resource arm1.viam.component.arm.v1.ArmService (your client is responsible for 1). "+
					"See %v for troubleshooting steps",
				web.ReqLimitExceededURL,
			),
		)

		// Assert that an appropriate warning log was output by the server. In generating this
		// warning log, the request counter should have pruned the first of the two clients
		// from the appropriate maps.
		reqLimitExceededLogs := logs.FilterMessageSnippet("Request limit exceeded").All()
		test.That(t, reqLimitExceededLogs, test.ShouldHaveLength, 1)
		test.That(t, reqLimitExceededLogs[0].Level, test.ShouldEqual, zapcore.WarnLevel)
		expectedMsg := fmt.Sprintf("Request limit exceeded for resource. See %v for troubleshooting steps. ", web.ReqLimitExceededURL)
		test.That(t, reqLimitExceededLogs[0].Message, test.ShouldStartWith, expectedMsg)
		var fields map[string]any
		err = json.Unmarshal(
			[]byte(strings.TrimPrefix(reqLimitExceededLogs[0].Message, expectedMsg)),
			&fields,
		)
		test.That(t, err, test.ShouldBeNil)
		// Assert that the first of the two clients is no longer included in
		// "all_other_client_information" now that it has disconnected. No need to assert on
		// any of the other fields, as the subtests above will do that.
		test.That(t, fields["all_other_client_information"], test.ShouldResemble, []any{})

		// Release the original call and wait for it to complete
		close(blockCall)
		clientCallsWg.Wait()
	})
}
