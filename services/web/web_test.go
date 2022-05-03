package web_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"

	streampb "github.com/edaniels/gostream/proto/stream/v1"
	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	rgrpc "go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/web"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const arm1String = "arm1"

var resources = []resource.Name{arm.Named(arm1String)}

var pos = &commonpb.Pose{X: 1, Y: 2, Z: 3}

func TestWebStart(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx()

	svc, err := web.New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	err = svc.Start(ctx, web.NewOptions())
	test.That(t, err, test.ShouldBeNil)

	arm1, err := arm.NewClient(context.Background(), arm1String, "localhost:8080", logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err := arm1.GetEndPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	err = svc.Start(context.Background(), web.NewOptions())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already started")

	err = utils.TryClose(context.Background(), svc)
	test.That(t, err, test.ShouldBeNil)
}

func TestWebStartOptions(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx()

	svc, err := web.New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := web.NewOptions()
	addr := fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	arm1, err := arm.NewClient(context.Background(), arm1String, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err := arm1.GetEndPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	err = utils.TryClose(context.Background(), svc)
	test.That(t, err, test.ShouldBeNil)
}

func TestWebWithAuth(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx()

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
			svc, err := web.New(ctx, injectRobot, config.Service{}, logger)
			test.That(t, err, test.ShouldBeNil)

			port, err := utils.TryReserveRandomPort()
			test.That(t, err, test.ShouldBeNil)
			options := web.NewOptions()
			addr := fmt.Sprintf("localhost:%d", port)
			options.Network.BindAddress = addr
			options.Managed = tc.Managed
			options.FQDN = tc.EntityName
			options.LocalFQDN = primitive.NewObjectID().Hex()
			apiKey := "sosecret"
			locationSecret := "locsosecret"
			options.Auth.Handlers = []config.AuthHandlerConfig{
				{
					Type: rpc.CredentialsTypeAPIKey,
					Config: config.AttributeMap{
						"key": apiKey,
					},
				},
				{
					Type: rutils.CredentialsTypeRobotLocationSecret,
					Config: config.AttributeMap{
						"secret": locationSecret,
					},
				},
			}
			if tc.Managed {
				options.BakedAuthEntity = "blah"
				options.BakedAuthCreds = rpc.Credentials{Type: "blah"}
			}

			err = svc.Start(ctx, options)
			test.That(t, err, test.ShouldBeNil)

			_, err = arm.NewClient(context.Background(), arm1String, addr, logger)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

			if tc.Managed {
				_, err = arm.NewClient(context.Background(), arm1String, addr, logger, rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials("wrong", rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey,
					}))
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

				_, err = arm.NewClient(context.Background(), arm1String, addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials("wrong", rpc.Credentials{
						Type:    rutils.CredentialsTypeRobotLocationSecret,
						Payload: locationSecret,
					}),
				)
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

				entityName := tc.EntityName
				if entityName == "" {
					entityName = options.LocalFQDN
				}
				arm1, err := arm.NewClient(context.Background(), arm1String, addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials(entityName, rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey,
					}),
				)
				test.That(t, err, test.ShouldBeNil)

				arm1Position, err := arm1.GetEndPosition(ctx)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, arm1Position, test.ShouldResemble, pos)

				test.That(t, utils.TryClose(context.Background(), arm1), test.ShouldBeNil)

				arm1, err = arm.NewClient(context.Background(), arm1String, addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithEntityCredentials(entityName, rpc.Credentials{
						Type:    rutils.CredentialsTypeRobotLocationSecret,
						Payload: locationSecret,
					}),
				)
				test.That(t, err, test.ShouldBeNil)

				arm1Position, err = arm1.GetEndPosition(ctx)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, arm1Position, test.ShouldResemble, pos)

				test.That(t, utils.TryClose(context.Background(), arm1), test.ShouldBeNil)
			} else {
				arm1, err := arm.NewClient(context.Background(), arm1String, addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithCredentials(rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey,
					}),
				)
				test.That(t, err, test.ShouldBeNil)

				arm1Position, err := arm1.GetEndPosition(ctx)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, arm1Position, test.ShouldResemble, pos)

				test.That(t, utils.TryClose(context.Background(), arm1), test.ShouldBeNil)

				arm1, err = arm.NewClient(context.Background(), arm1String, addr, logger,
					rpc.WithAllowInsecureWithCredentialsDowngrade(),
					rpc.WithCredentials(rpc.Credentials{
						Type:    rutils.CredentialsTypeRobotLocationSecret,
						Payload: locationSecret,
					}),
				)
				test.That(t, err, test.ShouldBeNil)

				arm1Position, err = arm1.GetEndPosition(ctx)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, arm1Position, test.ShouldResemble, pos)

				test.That(t, utils.TryClose(context.Background(), arm1), test.ShouldBeNil)
			}

			err = utils.TryClose(context.Background(), svc)
			test.That(t, err, test.ShouldBeNil)
		})
	}
}

func TestWebWithTLSAuth(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx()

	svc, err := web.New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	altName := primitive.NewObjectID().Hex()
	cert, _, _, certPool, err := testutils.GenerateSelfSignedCertificate("somename", altName)
	test.That(t, err, test.ShouldBeNil)

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	test.That(t, err, test.ShouldBeNil)

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := web.NewOptions()
	addr := fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr
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
			Config: config.AttributeMap{
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

	_, err = arm.NewClient(context.Background(), arm1String, addr, logger,
		rpc.WithTLSConfig(clientTLSConfig),
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

	_, err = arm.NewClient(context.Background(), arm1String, addr, logger,
		rpc.WithTLSConfig(clientTLSConfig),
		rpc.WithEntityCredentials("wrong", rpc.Credentials{
			Type:    rutils.CredentialsTypeRobotLocationSecret,
			Payload: locationSecret,
		}),
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

	// use secret
	arm1, err := arm.NewClient(context.Background(), arm1String, addr, logger,
		rpc.WithTLSConfig(clientTLSConfig),
		rpc.WithEntityCredentials(options.FQDN, rpc.Credentials{
			Type:    rutils.CredentialsTypeRobotLocationSecret,
			Payload: locationSecret,
		}),
	)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err := arm1.GetEndPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	// use cert
	clientTLSConfig.Certificates = []tls.Certificate{cert}
	arm1, err = arm.NewClient(context.Background(), arm1String, addr, logger,
		rpc.WithTLSConfig(clientTLSConfig),
	)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err = arm1.GetEndPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	// use cert with mDNS
	arm1, err = arm.NewClient(context.Background(), arm1String, options.FQDN, logger,
		rpc.WithDialDebug(),
		rpc.WithTLSConfig(clientTLSConfig),
	)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err = arm1.GetEndPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	// use signaling creds
	arm1, err = arm.NewClient(context.Background(), arm1String, addr, logger,
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

	arm1Position, err = arm1.GetEndPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	// use cert with mDNS while signaling present
	arm1, err = arm.NewClient(context.Background(), arm1String, options.FQDN, logger,
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

	arm1Position, err = arm1.GetEndPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	err = utils.TryClose(context.Background(), svc)
	test.That(t, err, test.ShouldBeNil)
}

func TestWebWithBadAuthHandlers(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx()

	svc, err := web.New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := web.NewOptions()
	addr := fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr
	options.Auth.Handlers = []config.AuthHandlerConfig{
		{
			Type: "unknown",
		},
	}

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown")
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	svc, err = web.New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	port, err = utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options = web.NewOptions()
	addr = fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr
	options.Auth.Handlers = []config.AuthHandlerConfig{
		{
			Type: rpc.CredentialsTypeAPIKey,
		},
	}

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "non-empty")
	test.That(t, err.Error(), test.ShouldContainSubstring, "api-key")
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
}

func TestWebUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, robot := setupRobotCtx()

	svc, err := web.New(ctx, robot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := web.NewOptions()
	addr := fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr
	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	arm1, err := arm.NewClient(context.Background(), arm1String, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err := arm1.GetEndPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, pos)

	// add arm to robot and then update
	injectArm := &inject.Arm{}
	newPos := &commonpb.Pose{X: 1, Y: 3, Z: 6}
	injectArm.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return newPos, nil
	}
	rs := map[resource.Name]interface{}{arm.Named(arm1String): injectArm}
	updateable, ok := svc.(resource.Updateable)
	test.That(t, ok, test.ShouldBeTrue)
	err = updateable.Update(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)

	conn, err := rgrpc.Dial(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	aClient := arm.NewClientFromConn(context.Background(), conn, arm1String, logger)
	position, err := aClient.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, newPos)

	test.That(t, utils.TryClose(context.Background(), arm1), test.ShouldBeNil)
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	test.That(t, utils.TryClose(context.Background(), aClient), test.ShouldBeNil)

	// now start it with the arm already in it
	ctx, robot2 := setupRobotCtx()
	robot2.(*inject.Robot).ResourceNamesFunc = func() []resource.Name { return resources }
	robot2.(*inject.Robot).ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return injectArm, nil
	}

	svc2, err := web.New(ctx, robot2, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	err = svc2.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	arm1, err = arm.NewClient(context.Background(), arm1String, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	arm1Position, err = arm1.GetEndPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1Position, test.ShouldResemble, newPos)

	conn, err = rgrpc.Dial(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	aClient2 := arm.NewClientFromConn(context.Background(), conn, arm1String, logger)
	test.That(t, err, test.ShouldBeNil)
	position, err = aClient2.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, newPos)

	// add a second arm
	arm2 := "arm2"
	injectArm2 := &inject.Arm{}
	pos2 := &commonpb.Pose{X: 2, Y: 3, Z: 4}
	injectArm2.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos2, nil
	}
	rs[arm.Named(arm2)] = injectArm2
	updateable, ok = svc2.(resource.Updateable)
	test.That(t, ok, test.ShouldBeTrue)
	err = updateable.Update(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)

	position, err = aClient2.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, newPos)

	aClient3 := arm.NewClientFromConn(context.Background(), conn, arm2, logger)
	test.That(t, err, test.ShouldBeNil)
	position, err = aClient3.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, pos2)

	test.That(t, utils.TryClose(context.Background(), arm1), test.ShouldBeNil)
	test.That(t, utils.TryClose(context.Background(), svc2), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func TestWebWithStreams(t *testing.T) {
	const (
		camera1Key = "camera1"
		camera2Key = "camera2"
	)

	// Start a robot with a camera
	robot := &inject.Robot{}
	cam1 := &inject.Camera{}
	rs := map[resource.Name]interface{}{camera.Named(camera1Key): cam1}
	robot.MockResourcesFromMap(rs)

	ctx, cancel := context.WithCancel(context.Background())

	// Start service
	logger := golog.NewTestLogger(t)
	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := web.NewOptions()
	addr := fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr
	svc, err := web.New(ctx, robot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Start a stream service client
	conn, err := rgrpc.Dial(context.Background(), addr, logger)
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
	updateable, ok := svc.(resource.Updateable)
	test.That(t, ok, test.ShouldBeTrue)
	err = updateable.Update(ctx, rs)
	test.That(t, err, test.ShouldBeNil)

	// Test that new streams are available
	resp, err = streamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Names, test.ShouldContain, camera1Key)
	test.That(t, resp.Names, test.ShouldContain, camera2Key)
	test.That(t, resp.Names, test.ShouldHaveLength, 2)

	// We need to cancel otherwise we are stuck waiting for WebRTC to start streaming.
	cancel()
	test.That(t, utils.TryClose(ctx, streamClient), test.ShouldBeNil)
	test.That(t, utils.TryClose(ctx, svc), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

// TODO: we can never add stream servers unless there is least one camera on the
// robot when the service starts! Make it so this is not the case.
func TestWebAddFirstStream(t *testing.T) {
	const (
		camera1Key = "camera1"
	)

	// Start a robot without a camera
	robot := &inject.Robot{}
	rs := map[resource.Name]interface{}{}
	robot.MockResourcesFromMap(rs)

	ctx, cancel := context.WithCancel(context.Background())

	// Start service
	logger := golog.NewTestLogger(t)
	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := web.NewOptions()
	addr := fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr
	svc, err := web.New(ctx, robot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Start a stream service client
	conn, err := rgrpc.Dial(context.Background(), addr, logger)
	streamClient := streampb.NewStreamServiceClient(conn)

	// Test that there are no streams available
	resp, err := streamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Names, test.ShouldHaveLength, 0)

	// Add first camera and update
	cam1 := &inject.Camera{}
	rs[camera.Named(camera1Key)] = cam1
	robot.MockResourcesFromMap(rs)
	updateable, ok := svc.(resource.Updateable)
	test.That(t, ok, test.ShouldBeTrue)
	err = updateable.Update(ctx, rs)
	test.That(t, err, test.ShouldBeNil)

	// Test that new streams are available
	resp, err = streamClient.ListStreams(ctx, &streampb.ListStreamsRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Names, test.ShouldContain, camera1Key)
	test.That(t, resp.Names, test.ShouldHaveLength, 1)

	// We need to cancel otherwise we are stuck waiting for WebRTC to start streaming.
	cancel()
	test.That(t, utils.TryClose(ctx, streamClient), test.ShouldBeNil)
	test.That(t, utils.TryClose(ctx, svc), test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func setupRobotCtx() (context.Context, robot.Robot) {
	injectArm := &inject.Arm{}
	injectArm.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos, nil
	}
	injectRobot := &inject.Robot{}
	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) { return &config.Config{}, nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return resources }
	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return injectArm, nil
	}

	return context.Background(), injectRobot
}

func setupInjectRobot() (*inject.Robot, *mock) {
	web1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return web1, nil
	}
	return r, web1
}

func TestFromRobot(t *testing.T) {
	r, web1 := setupInjectRobot()

	rWeb, err := web.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rWeb, test.ShouldNotBeNil)

	err = rWeb.Start(context.Background(), web.NewOptions())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, web1.startCount, test.ShouldEqual, 1)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return "not web", nil
	}

	rWeb, err = web.FromRobot(r)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("web.Service", "string"))
	test.That(t, rWeb, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return nil, rutils.NewResourceNotFoundError(name)
	}

	rWeb, err = web.FromRobot(r)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(web.Name))
	test.That(t, rWeb, test.ShouldBeNil)
}

type mock struct {
	web.Service

	startCount int
}

func (m *mock) Start(context.Context, web.Options) error { m.startCount++; return nil }
