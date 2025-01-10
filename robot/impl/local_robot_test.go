package robotimpl

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/cloud"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/encoder"
	fakeencoder "go.viam.com/rdk/components/encoder/fake"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/motor"
	fakemotor "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	rgrpc "go.viam.com/rdk/grpc"
	internalcloud "go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/packages"
	putils "go.viam.com/rdk/robot/packages/testutils"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/builtin"
	genericservice "go.viam.com/rdk/services/generic"
	"go.viam.com/rdk/services/motion"
	motionBuiltin "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/navigation"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

var fakeModel = resource.DefaultModelFamily.WithModel("fake")

func TestConfig1(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/cfgtest1.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r := setupLocalRobot(t, context.Background(), cfg, logger)

	c1, err := camera.FromRobot(r, "c1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c1.Name(), test.ShouldResemble, camera.Named("c1"))

	pic, err := camera.DecodeImageFromCamera(context.Background(), rutils.MimeTypeJPEG, nil, c1)
	test.That(t, err, test.ShouldBeNil)

	bounds := pic.Bounds()

	test.That(t, bounds.Max.X, test.ShouldBeGreaterThanOrEqualTo, 32)
}

func TestConfigFake(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	setupLocalRobot(t, context.Background(), cfg, logger)
}

// this serves as a test for updateWeakDependents as the web service defines a weak
// dependency on all resources.
func TestConfigRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r := setupLocalRobot(t, ctx, cfg, logger.Sublogger("main_robot"))

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	o1 := &spatialmath.R4AA{
		Theta: math.Pi / 2.,
		RX:    0,
		RY:    0,
		RZ:    1,
	}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
	test.That(t, err, test.ShouldBeNil)

	remoteConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "foo",
				API:   base.API,
				Model: fakeModel,
				Frame: &referenceframe.LinkConfig{
					Parent: referenceframe.World,
				},
			},
			{
				Name:  "myParentIsRemote",
				API:   base.API,
				Model: fakeModel,
				Frame: &referenceframe.LinkConfig{
					Parent: "foo:cameraOver",
				},
			},
		},
		Services: []resource.Config{},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr,
				Frame: &referenceframe.LinkConfig{
					Parent:      "foo",
					Translation: r3.Vector{100, 200, 300},
					Orientation: o1Cfg,
				},
			},
			{
				Name:    "bar",
				Address: addr,
			},
			{
				Name:    "squee",
				Address: addr,
				Frame: &referenceframe.LinkConfig{
					Parent:      referenceframe.World,
					Translation: r3.Vector{100, 200, 300},
					Orientation: o1Cfg,
				},
			},
		},
	}

	ctx2 := context.Background()
	r2 := setupLocalRobot(t, ctx2, remoteConfig, logger.Sublogger("remote_robot"))

	expected := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		arm.Named("squee:pieceArm"),
		arm.Named("foo:pieceArm"),
		arm.Named("bar:pieceArm"),
		base.Named("foo"),
		base.Named("myParentIsRemote"),
		camera.Named("squee:cameraOver"),
		camera.Named("foo:cameraOver"),
		camera.Named("bar:cameraOver"),
		audioinput.Named("squee:mic1"),
		audioinput.Named("foo:mic1"),
		audioinput.Named("bar:mic1"),
		movementsensor.Named("squee:movement_sensor1"),
		movementsensor.Named("foo:movement_sensor1"),
		movementsensor.Named("bar:movement_sensor1"),
		movementsensor.Named("squee:movement_sensor2"),
		movementsensor.Named("foo:movement_sensor2"),
		movementsensor.Named("bar:movement_sensor2"),
		gripper.Named("squee:pieceGripper"),
		gripper.Named("foo:pieceGripper"),
		gripper.Named("bar:pieceGripper"),
		motion.Named("squee:builtin"),
		motion.Named("foo:builtin"),
		motion.Named("bar:builtin"),
	}

	resources2 := r2.ResourceNames()

	rtestutils.VerifySameResourceNames(t, resources2, expected)

	expectedRemotes := []string{"squee", "foo", "bar"}
	remotes2 := r2.RemoteNames()

	rtestutils.VerifySameElements(t, remotes2, expectedRemotes)

	arm1Name := arm.Named("bar:pieceArm")
	arm1, err := r2.ResourceByName(arm1Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1.Name(), test.ShouldResemble, arm1Name)
	pos1, err := arm1.(arm.Arm).EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	arm2, err := r2.ResourceByName(arm.Named("foo:pieceArm"))
	test.That(t, err, test.ShouldBeNil)
	pos2, err := arm2.(arm.Arm).EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincident(pos1, pos2), test.ShouldBeTrue)

	cfg2 := r2.Config()
	// Components should only include local components.
	test.That(t, len(cfg2.Components), test.ShouldEqual, 2)

	fsConfig, err := r2.FrameSystemConfig(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fsConfig.Parts, test.ShouldHaveLength, 12)
}

func TestConfigRemoteWithAuth(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

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
			ctx := context.Background()
			r := setupLocalRobot(t, ctx, cfg, logger)

			options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
			options.Managed = tc.Managed
			options.FQDN = tc.EntityName
			options.LocalFQDN = primitive.NewObjectID().Hex()
			apiKeyID := "sosecretID"
			apiKey := "sosecret"
			locationSecret := "locsosecret"

			options.Auth.Handlers = []config.AuthHandlerConfig{
				{
					Type: rpc.CredentialsTypeAPIKey,
					Config: rutils.AttributeMap{
						apiKeyID: apiKey,
						"keys":   []string{apiKeyID},
					},
				},
				{
					Type: rutils.CredentialsTypeRobotLocationSecret,
					Config: rutils.AttributeMap{
						"secret": locationSecret,
					},
				},
			}

			if tc.Managed {
				options.BakedAuthEntity = "blah"
				options.BakedAuthCreds = rpc.Credentials{Type: "blah"}
			}
			err = r.StartWeb(ctx, options)
			test.That(t, err, test.ShouldBeNil)

			entityName := tc.EntityName
			if entityName == "" {
				entityName = options.LocalFQDN
			}

			remoteConfig := &config.Config{
				Debug: true,
				Remotes: []config.Remote{
					{
						Name:    "foo",
						Address: addr,
						Auth: config.RemoteAuth{
							Managed: tc.Managed,
						},
					},
					{
						Name:    "bar",
						Address: addr,
						Auth: config.RemoteAuth{
							Managed: tc.Managed,
						},
					},
				},
			}

			setupLocalRobot(t, context.Background(), remoteConfig, logger)

			remoteConfig.Remotes[0].Auth.Credentials = &rpc.Credentials{
				Type:    rpc.CredentialsTypeAPIKey,
				Payload: apiKey,
			}
			remoteConfig.Remotes[1].Auth.Credentials = &rpc.Credentials{
				Type:    rutils.CredentialsTypeRobotLocationSecret,
				Payload: locationSecret,
			}

			var r2 robot.LocalRobot
			if tc.Managed {
				remoteConfig.Remotes[0].Auth.Entity = "wrong"
				setupLocalRobot(t, context.Background(), remoteConfig, logger)

				remoteConfig.AllowInsecureCreds = true

				r3 := setupLocalRobot(t, context.Background(), remoteConfig, logger)
				remoteBot, ok := r3.RemoteByName("foo")
				test.That(t, ok, test.ShouldBeFalse)
				test.That(t, remoteBot, test.ShouldBeNil)

				remoteConfig.Remotes[0].Auth.Entity = apiKeyID
				remoteConfig.Remotes[1].Auth.Entity = entityName
				test.That(t, setupLocalRobot(t, context.Background(), remoteConfig, logger).Close(context.Background()), test.ShouldBeNil)

				ctx2 := context.Background()
				remoteConfig.Remotes[0].Address = options.LocalFQDN
				if tc.EntityName != "" {
					remoteConfig.Remotes[1].Address = options.FQDN
				}
				r2 = setupLocalRobot(t, ctx2, remoteConfig, logger)
			} else {
				setupLocalRobot(t, context.Background(), remoteConfig, logger)

				remoteConfig.AllowInsecureCreds = true

				test.That(t, setupLocalRobot(t, context.Background(), remoteConfig, logger).Close(context.Background()), test.ShouldBeNil)

				remoteConfig.Remotes[0].Auth.Entity = apiKeyID

				ctx2 := context.Background()
				remoteConfig.Remotes[0].Address = options.LocalFQDN
				r2 = setupLocalRobot(t, ctx2, remoteConfig, logger)

				_, err = r2.ResourceByName(motion.Named(resource.DefaultServiceName))
				test.That(t, err, test.ShouldBeNil)
			}

			test.That(t, r2, test.ShouldNotBeNil)

			expected := []resource.Name{
				motion.Named(resource.DefaultServiceName),
				arm.Named("bar:pieceArm"),
				arm.Named("foo:pieceArm"),
				audioinput.Named("bar:mic1"),
				audioinput.Named("foo:mic1"),
				camera.Named("bar:cameraOver"),
				camera.Named("foo:cameraOver"),
				movementsensor.Named("bar:movement_sensor1"),
				movementsensor.Named("foo:movement_sensor1"),
				movementsensor.Named("bar:movement_sensor2"),
				movementsensor.Named("foo:movement_sensor2"),
				gripper.Named("bar:pieceGripper"),
				gripper.Named("foo:pieceGripper"),
				motion.Named("foo:builtin"),
				motion.Named("bar:builtin"),
			}

			resources2 := r2.ResourceNames()

			rtestutils.VerifySameResourceNames(t, resources2, expected)

			remotes2 := r2.RemoteNames()
			expectedRemotes := []string{"bar", "foo"}

			rtestutils.VerifySameElements(t, remotes2, expectedRemotes)
		})
	}
}

func TestConfigRemoteWithTLSAuth(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r := setupLocalRobot(t, ctx, cfg, logger)

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

	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	remoteTLSConfig := options.Network.TLSConfig.Clone()
	remoteTLSConfig.Certificates = nil
	remoteTLSConfig.ServerName = "somename"
	remoteConfig := &config.Config{
		Debug: true,
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr,
				Auth: config.RemoteAuth{
					Managed: true,
				},
			},
		},
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				TLSConfig: remoteTLSConfig,
			},
		},
	}

	setupLocalRobot(t, context.Background(), remoteConfig, logger)

	// use secret
	remoteConfig.Remotes[0].Auth.Credentials = &rpc.Credentials{
		Type:    rutils.CredentialsTypeRobotLocationSecret,
		Payload: locationSecret,
	}
	test.That(t, setupLocalRobot(t, context.Background(), remoteConfig, logger).Close(context.Background()), test.ShouldBeNil)

	// Create a clone such that the prior launched robot and the next robot can have their own tls
	// config object to safely read from.
	remoteConfig.Network.NetworkConfigData.TLSConfig = options.Network.TLSConfig.Clone()
	remoteTLSConfig = remoteConfig.Network.NetworkConfigData.TLSConfig
	// use cert
	remoteTLSConfig.Certificates = []tls.Certificate{cert}
	remoteTLSConfig.ServerName = "somename"
	test.That(t, setupLocalRobot(t, context.Background(), remoteConfig, logger).Close(context.Background()), test.ShouldBeNil)

	// use cert with mDNS
	remoteConfig.Remotes[0].Address = options.FQDN
	test.That(t, setupLocalRobot(t, context.Background(), remoteConfig, logger).Close(context.Background()), test.ShouldBeNil)

	// use signaling creds
	remoteConfig.Remotes[0].Address = addr
	remoteConfig.Remotes[0].Auth.Credentials = nil
	remoteConfig.Remotes[0].Auth.SignalingServerAddress = addr
	remoteConfig.Remotes[0].Auth.SignalingAuthEntity = options.FQDN
	remoteConfig.Remotes[0].Auth.SignalingCreds = &rpc.Credentials{
		Type:    rutils.CredentialsTypeRobotLocationSecret,
		Payload: locationSecret,
	}
	test.That(t, setupLocalRobot(t, context.Background(), remoteConfig, logger).Close(context.Background()), test.ShouldBeNil)

	// use cert with mDNS while signaling present
	ctx2 := context.Background()
	remoteConfig.Remotes[0].Auth.SignalingCreds = &rpc.Credentials{
		Type:    rutils.CredentialsTypeRobotLocationSecret,
		Payload: locationSecret + "bad",
	}
	remoteConfig.Remotes[0].Address = options.FQDN
	r2 := setupLocalRobot(t, ctx2, remoteConfig, logger)

	expected := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		arm.Named("foo:pieceArm"),
		audioinput.Named("foo:mic1"),
		camera.Named("foo:cameraOver"),
		movementsensor.Named("foo:movement_sensor1"),
		movementsensor.Named("foo:movement_sensor2"),
		gripper.Named("foo:pieceGripper"),
		motion.Named("foo:builtin"),
	}

	resources2 := r2.ResourceNames()

	rtestutils.VerifySameResourceNames(t, resources2, expected)

	remotes2 := r2.RemoteNames()
	expectedRemotes := []string{"foo"}

	rtestutils.VerifySameElements(t, remotes2, expectedRemotes)

	statuses, err := r2.MachineStatus(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses.Resources), test.ShouldEqual, 13)
	test.That(t, statuses, test.ShouldNotBeNil)
}

func TestStopAll(t *testing.T) {
	logger := logging.NewTestLogger(t)
	channel := make(chan struct{})

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))

	var (
		stopCount1 int
		stopCount2 int

		extraOptions1 map[string]interface{}
		extraOptions2 map[string]interface{}
	)
	dummyArm1 := &inject.Arm{
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			stopCount1++
			extraOptions1 = extra
			return nil
		},
		DoFunc: func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
			close(channel)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	dummyArm2 := &inject.Arm{
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			stopCount2++
			extraOptions2 = extra
			return nil
		},
		DoFunc: func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
			close(channel)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	resource.RegisterComponent(
		arm.API,
		model,
		resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (arm.Arm, error) {
			if conf.Name == "arm1" {
				return dummyArm1, nil
			}
			return dummyArm2, nil
		}})

	armConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%[1]s",
				"name": "arm1",
				"type": "arm"
			},
			{
				"model": "%[1]s",
				"name": "arm2",
				"type": "arm"
			}
		]
	}
	`, model.String())
	defer func() {
		resource.Deregister(arm.API, model)
	}()

	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(armConfig), logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r := setupLocalRobot(t, ctx, cfg, logger)

	test.That(t, stopCount1, test.ShouldEqual, 0)
	test.That(t, stopCount2, test.ShouldEqual, 0)

	test.That(t, extraOptions1, test.ShouldBeNil)
	test.That(t, extraOptions2, test.ShouldBeNil)

	err = r.StopAll(ctx, map[resource.Name]map[string]interface{}{arm.Named("arm2"): {"foo": "bar"}})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, stopCount1, test.ShouldEqual, 1)
	test.That(t, stopCount2, test.ShouldEqual, 1)

	test.That(t, extraOptions1, test.ShouldBeNil)
	test.That(t, extraOptions2, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

	// Test OPID cancellation
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	conn, err := rgrpc.Dial(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)
	arm1, err := arm.NewClientFromConn(ctx, conn, "somerem", arm.Named("arm1"), logger)
	test.That(t, err, test.ShouldBeNil)

	foundOPID := false
	stopAllErrCh := make(chan error, 1)
	go func() {
		<-channel
		for _, opid := range r.OperationManager().All() {
			if opid.Method == "/viam.component.arm.v1.ArmService/DoCommand" {
				foundOPID = true
				stopAllErrCh <- r.StopAll(ctx, nil)
			}
		}
	}()
	_, err = arm1.DoCommand(ctx, map[string]interface{}{})
	s, isGRPCErr := status.FromError(err)
	test.That(t, isGRPCErr, test.ShouldBeTrue)
	test.That(t, s.Code(), test.ShouldEqual, codes.Canceled)

	stopAllErr := <-stopAllErrCh
	test.That(t, foundOPID, test.ShouldBeTrue)
	test.That(t, stopAllErr, test.ShouldBeNil)
}

func TestNewTeardown(t *testing.T) {
	logger := logging.NewTestLogger(t)

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))

	var closeCount int
	resource.RegisterComponent(
		board.API,
		model,
		resource.Registration[board.Board, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (board.Board, error) {
			return &inject.Board{
				CloseFunc: func(ctx context.Context) error {
					closeCount++
					return nil
				},
			}, nil
		}})
	resource.RegisterComponent(
		gripper.API,
		model,
		resource.Registration[gripper.Gripper, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (gripper.Gripper, error) {
			return nil, errors.New("whoops")
		}})

	defer func() {
		resource.Deregister(board.API, model)
		resource.Deregister(gripper.API, model)
	}()

	failingConfig := fmt.Sprintf(`{
    "components": [
        {
            "model": "%[1]s",
            "name": "board1",
            "type": "board"
        },
        {
            "model": "%[1]s",
            "name": "gripper1",
            "type": "gripper",
            "depends_on": ["board1"]
        }
    ]
}
`, model)
	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(failingConfig), logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r := setupLocalRobot(t, ctx, cfg, logger)
	test.That(t, r.Close(ctx), test.ShouldBeNil)
	test.That(t, closeCount, test.ShouldEqual, 1)
}

func TestMetadataUpdate(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r := setupLocalRobot(t, ctx, cfg, logger)

	resources := r.ResourceNames()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(resources), test.ShouldEqual, 7)
	test.That(t, err, test.ShouldBeNil)

	// 5 declared resources + default motion
	resourceNames := []resource.Name{
		arm.Named("pieceArm"),
		audioinput.Named("mic1"),
		camera.Named("cameraOver"),
		gripper.Named("pieceGripper"),
		movementsensor.Named("movement_sensor1"),
		movementsensor.Named("movement_sensor2"),
		motion.Named(resource.DefaultServiceName),
	}

	resources = r.ResourceNames()
	test.That(t, len(resources), test.ShouldEqual, len(resourceNames))
	rtestutils.VerifySameResourceNames(t, resources, resourceNames)

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	resources = r.ResourceNames()
	test.That(t, resources, test.ShouldBeEmpty)
}

func TestGetRemoteResourceAndGrandFather(t *testing.T) {
	// set up remotes
	options, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)

	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	remoteRemoteConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				API:   arm.API,
				Model: fakeModel,
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
			},
			{
				Name:  "arm2",
				API:   arm.API,
				Model: fakeModel,
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
			},
			{
				Name:  "pieceArm",
				API:   arm.API,
				Model: fakeModel,
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
			},
		},
		Services: []resource.Config{},
		Remotes:  []config.Remote{},
	}

	r0 := setupLocalRobot(t, ctx, remoteRemoteConfig, logger)

	err := r0.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	r0arm1, err := r0.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	r0Arm, ok := r0arm1.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	err = r0Arm.MoveToJointPositions(context.Background(), []referenceframe.Input{{math.Pi}}, nil)
	test.That(t, err, test.ShouldBeNil)
	p0Arm1, err := r0Arm.JointPositions(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)

	options, _, addr2 := robottestutils.CreateBaseOptionsAndListener(t)
	remoteConfig := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "remote",
				Address: addr2,
			},
		},
	}

	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)
	cfg.Remotes = append(cfg.Remotes, config.Remote{
		Name:    "foo",
		Address: addr1,
	})
	r1 := setupLocalRobot(t, ctx, cfg, logger)
	err = r1.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	r := setupLocalRobot(t, ctx, remoteConfig, logger)

	rtestutils.VerifySameResourceNames(
		t,
		r.ResourceNames(),
		[]resource.Name{
			motion.Named(resource.DefaultServiceName),
			arm.Named("remote:foo:arm1"), arm.Named("remote:foo:arm2"),
			arm.Named("remote:pieceArm"),
			arm.Named("remote:foo:pieceArm"),
			audioinput.Named("remote:mic1"),
			camera.Named("remote:cameraOver"),
			movementsensor.Named("remote:movement_sensor1"),
			movementsensor.Named("remote:movement_sensor2"),
			gripper.Named("remote:pieceGripper"),
			motion.Named("remote:builtin"),
			motion.Named("remote:foo:builtin"),
		},
	)
	arm1, err := r.ResourceByName(arm.Named("remote:foo:arm1"))
	test.That(t, err, test.ShouldBeNil)
	rrArm1, ok := arm1.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	pos, err := rrArm1.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldResemble, p0Arm1)

	arm1, err = r.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	rrArm1, ok = arm1.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	pos, err = rrArm1.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldResemble, p0Arm1)

	_, err = r.ResourceByName(arm.Named("remote:foo:pieceArm"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(arm.Named("remote:pieceArm"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(arm.Named("pieceArm"))
	test.That(t, err, test.ShouldBeError, "more than one remote resources with name \"pieceArm\" exists")
}

type someConfig struct {
	Thing string
}

func (someConfig) Validate(path string) ([]string, error) {
	return nil, errors.New("fail")
}

func TestValidationErrorOnReconfigure(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	badConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:                "test",
				API:                 base.API,
				Model:               resource.DefaultModelFamily.WithModel("random"),
				ConvertedAttributes: someConfig{},
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 navigation.API,
				ConvertedAttributes: someConfig{},
			},
		},
		Remotes: []config.Remote{{
			Name:     "remote",
			Insecure: true,
			Address:  "",
		}},
		Cloud: &config.Cloud{},
	}
	r := setupLocalRobot(t, ctx, badConfig, logger)

	// Test Component Error
	name := base.Named("test")
	noBase, err := r.ResourceByName(name)
	test.That(
		t,
		err,
		test.ShouldBeError,
		resource.NewNotAvailableError(name, errors.New("resource config validation error: fail")),
	)
	test.That(t, noBase, test.ShouldBeNil)
	// Test Service Error
	s, err := r.ResourceByName(navigation.Named("fake1"))
	test.That(t, s, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "resource \"rdk:service:navigation/fake1\" not available")
	// Test Remote Error
	rem, ok := r.RemoteByName("remote")
	test.That(t, rem, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestConfigStartsInvalidReconfiguresValid(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	badConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:                "test",
				API:                 base.API,
				Model:               fakeModel,
				ConvertedAttributes: someConfig{},
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 datamanager.API,
				ConvertedAttributes: someConfig{},
			},
		},
		Remotes: []config.Remote{{
			Name:     "remote",
			Insecure: true,
			Address:  "",
		}},
	}
	test.That(t, badConfig.Ensure(false, logger), test.ShouldBeNil)
	r := setupLocalRobot(t, ctx, badConfig, logger)

	options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err := r.StartWeb(context.Background(), options1)
	test.That(t, err, test.ShouldBeNil)

	goodConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "test",
				API:   base.API,
				Model: fakeModel,
				// Added to force a component reconfigure.
				Attributes: rutils.AttributeMap{"version": 1},
			},
		},
		Services: []resource.Config{
			{
				Name:  "fake1",
				API:   datamanager.API,
				Model: resource.DefaultServiceModel,
				// Added to force a service reconfigure.
				Attributes:          rutils.AttributeMap{"version": 1},
				ConvertedAttributes: &builtin.Config{},
			},
		},
		Remotes: []config.Remote{{
			Name:     "remote",
			Insecure: true,
			Address:  addr1,
		}},
	}
	test.That(t, goodConfig.Ensure(false, logger), test.ShouldBeNil)

	// Test Component Error
	name := base.Named("test")
	noBase, err := base.FromRobot(r, "test")
	test.That(
		t,
		err,
		test.ShouldBeError,
		resource.NewNotAvailableError(name, errors.New("resource config validation error: fail")),
	)
	test.That(t, noBase, test.ShouldBeNil)
	// Test Service Error
	s, err := r.ResourceByName(datamanager.Named("fake1"))
	test.That(t, s, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "resource \"rdk:service:data_manager/fake1\" not available")
	// Test Remote Error
	rem, ok := r.RemoteByName("remote")
	test.That(t, rem, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	r.Reconfigure(ctx, goodConfig)
	// Test Component Valid
	noBase, err = base.FromRobot(r, "test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, noBase, test.ShouldNotBeNil)
	// Test Service Valid
	s, err = r.ResourceByName(datamanager.Named("fake1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)
	// Test Remote Valid
	rem, ok = r.RemoteByName("remote")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, rem, test.ShouldNotBeNil)
}

func TestConfigStartsValidReconfiguresInvalid(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	armConfig := resource.Config{
		Name:  "arm1",
		API:   arm.API,
		Model: fakeModel,
		ConvertedAttributes: &fake.Config{
			ModelFilePath: "../../components/arm/fake/fake_model.json",
		},
	}
	cfg := config.Config{
		Components: []resource.Config{armConfig},
	}

	robotRemote := setupLocalRobot(t, ctx, &cfg, logger)
	options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err := robotRemote.StartWeb(context.Background(), options1)
	test.That(t, err, test.ShouldBeNil)

	goodConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "test",
				API:   base.API,
				Model: fakeModel,
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 datamanager.API,
				Model:               resource.DefaultServiceModel,
				ConvertedAttributes: &builtin.Config{},
			},
		},
		Remotes: []config.Remote{{
			Name:     "remote",
			Insecure: true,
			Address:  addr1,
		}},
	}
	test.That(t, goodConfig.Ensure(false, logger), test.ShouldBeNil)
	r := setupLocalRobot(t, ctx, goodConfig, logger)

	badConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "test",
				API:   base.API,
				Model: fakeModel,
				// Added to force a component reconfigure.
				Attributes:          rutils.AttributeMap{"version": 1},
				ConvertedAttributes: someConfig{},
			},
		},
		Services: []resource.Config{
			{
				Name: "fake1",
				API:  datamanager.API,
				// Added to force a service reconfigure.
				Attributes:          rutils.AttributeMap{"version": 1},
				ConvertedAttributes: someConfig{},
			},
		},
		Remotes: []config.Remote{{
			Name:     "remote",
			Insecure: true,
			Address:  "",
		}},
	}
	test.That(t, badConfig.Ensure(false, logger), test.ShouldBeNil)
	// Test Component Valid
	noBase, err := base.FromRobot(r, "test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, noBase, test.ShouldNotBeNil)
	// Test Service Valid
	s, err := r.ResourceByName(datamanager.Named("fake1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)
	// Test Remote Valid
	rem, ok := r.RemoteByName("remote")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, rem, test.ShouldNotBeNil)

	r.Reconfigure(ctx, badConfig)
	// Test Component Error
	name := base.Named("test")
	noBase, err = base.FromRobot(r, "test")
	test.That(
		t,
		err,
		test.ShouldBeError,
		resource.NewNotAvailableError(name, errors.New("resource config validation error: fail")),
	)
	test.That(t, noBase, test.ShouldBeNil)
	// Test Service Error
	s, err = r.ResourceByName(datamanager.Named("fake1"))
	test.That(t, s, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "resource \"rdk:service:data_manager/fake1\" not available")
	// Test Remote Error
	rem, ok = r.RemoteByName("remote")
	test.That(t, rem, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestResourceStartsOnReconfigure(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	badConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "fake0",
				API:   base.API,
				Model: resource.DefaultModelFamily.WithModel("random"),
			},
		},
		Services: []resource.Config{
			{
				Name: "fake1",
			},
		},
	}
	test.That(t, badConfig.Ensure(false, logger), test.ShouldBeNil)

	goodConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "fake0",
				API:   base.API,
				Model: fakeModel,
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 datamanager.API,
				Model:               resource.DefaultServiceModel,
				ConvertedAttributes: &builtin.Config{},
			},
		},
	}
	test.That(t, goodConfig.Ensure(false, logger), test.ShouldBeNil)
	r := setupLocalRobot(t, ctx, badConfig, logger)

	noBase, err := r.ResourceByName(base.Named("fake0"))
	test.That(
		t,
		err,
		test.ShouldBeError,
		resource.NewNotAvailableError(
			base.Named("fake0"),
			errors.New(`resource build error: unknown resource type: API "rdk:component:base" with model "rdk:builtin:random" not registered`),
		),
	)
	test.That(t, noBase, test.ShouldBeNil)

	noSvc, err := r.ResourceByName(datamanager.Named("fake1"))
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(datamanager.Named("fake1")))
	test.That(t, noSvc, test.ShouldBeNil)

	r.Reconfigure(ctx, goodConfig)

	yesBase, err := r.ResourceByName(base.Named("fake0"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, yesBase, test.ShouldNotBeNil)

	yesSvc, err := r.ResourceByName(datamanager.Named("fake1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, yesSvc, test.ShouldNotBeNil)
}

func TestConfigProcess(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	r := setupLocalRobot(t, context.Background(), &config.Config{
		Processes: []pexec.ProcessConfig{
			{
				ID:      "1",
				Name:    "bash",
				Args:    []string{"-c", "echo heythere"},
				Log:     true,
				OneShot: true,
			},
		},
	}, logger)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	test.That(t, logs.FilterField(zap.String("output", "heythere\n")).Len(), test.ShouldEqual, 1)
}

func TestConfigPackages(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	fakePackageServer, err := putils.NewFakePackageServer(ctx, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(fakePackageServer.Shutdown)

	packageDir := t.TempDir()

	robotConfig := &config.Config{
		Packages: []config.PackageConfig{
			{
				Name:    "some-name-1",
				Package: "package-1",
				Version: "v1",
			},
		},
		Cloud: &config.Cloud{
			AppAddress: fmt.Sprintf("http://%s", fakePackageServer.Addr().String()),
		},
		PackagePath: packageDir,
	}

	r := setupLocalRobot(t, ctx, robotConfig, logger)

	_, err = r.PackageManager().PackagePath("some-name-1")
	test.That(t, err, test.ShouldEqual, packages.ErrPackageMissing)

	robotConfig2 := &config.Config{
		Packages: []config.PackageConfig{
			{
				Name:    "some-name-1",
				Package: "package-1",
				Version: "v1",
				Type:    "ml_model",
			},
			{
				Name:    "some-name-2",
				Package: "package-2",
				Version: "v2",
				Type:    "ml_model",
			},
		},
		Cloud: &config.Cloud{
			AppAddress: fmt.Sprintf("http://%s", fakePackageServer.Addr().String()),
		},
		PackagePath: packageDir,
	}

	fakePackageServer.StorePackage(robotConfig2.Packages...)
	r.Reconfigure(ctx, robotConfig2)

	path1, err := r.PackageManager().PackagePath("some-name-1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path1, test.ShouldEqual, path.Join(packageDir, "data", "ml_model", "package-1-v1"))

	path2, err := r.PackageManager().PackagePath("some-name-2")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path2, test.ShouldEqual, path.Join(packageDir, "data", "ml_model", "package-2-v2"))
}

// removeDefaultServices removes default services and returns the removed
// services for testing purposes.
func removeDefaultServices(cfg *config.Config) []resource.Config {
	if cfg == nil {
		return nil
	}

	// Make a set of registered default services.
	registeredDefaultSvcs := make(map[resource.Name]bool)
	for _, name := range resource.DefaultServices() {
		registeredDefaultSvcs[name] = true
	}

	var defaultSvcs, nonDefaultSvcs []resource.Config
	for _, svc := range cfg.Services {
		if registeredDefaultSvcs[svc.ResourceName()] {
			defaultSvcs = append(defaultSvcs, svc)
			continue
		}
		nonDefaultSvcs = append(nonDefaultSvcs, svc)
	}

	cfg.Services = nonDefaultSvcs
	return defaultSvcs
}

func TestConfigMethod(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Precompile complex module to avoid timeout issues when building takes too long.
	complexPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")

	r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)

	// Assert that Config method returns the default motion service.
	actualCfg := r.Config()
	defaultSvcs := removeDefaultServices(actualCfg)
	test.That(t, len(defaultSvcs), test.ShouldEqual, 1)
	for _, svc := range defaultSvcs {
		test.That(t, svc.API.SubtypeName, test.ShouldEqual,
			motion.API.SubtypeName)
	}
	test.That(t, actualCfg, test.ShouldResemble, &config.Config{})

	// Use a remote with components and services to ensure none of its resources
	// will be returned by Config.
	remoteCfg, err := config.Read(context.Background(), "data/remote_fake.json", logger)
	test.That(t, err, test.ShouldBeNil)
	remoteRobot := setupLocalRobot(t, ctx, remoteCfg, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = remoteRobot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Manually define mybase model, as importing it can cause double registration.
	myBaseModel := resource.NewModel("acme", "demo", "mybase")

	cfg := &config.Config{
		Cloud: &config.Cloud{},
		Modules: []config.Module{
			{
				Name:     "mod",
				ExePath:  complexPath,
				LogLevel: "info",
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr,
			},
		},
		Components: []resource.Config{
			{
				Name:  "myBase",
				API:   base.API,
				Model: myBaseModel,
				Attributes: rutils.AttributeMap{
					"motorL": "motor1",
					"motorR": "motor2",
				},
			},
			{
				Name:                "motor1",
				API:                 motor.API,
				Model:               fakeModel,
				ConvertedAttributes: &fakemotor.Config{},
				ImplicitDependsOn:   []string{"builtin:sensors"},
			},
			{
				Name:                "motor2",
				API:                 motor.API,
				Model:               fakeModel,
				ConvertedAttributes: &fakemotor.Config{},
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:      "1",
				Name:    "bash",
				Args:    []string{"-c", "echo heythere"},
				Log:     true,
				OneShot: true,
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 datamanager.API,
				Model:               resource.DefaultServiceModel,
				ConvertedAttributes: &builtin.Config{},
				ImplicitDependsOn:   []string{"foo:builtin:data_manager"},
			},
			{
				Name:  "builtin",
				API:   navigation.API,
				Model: resource.DefaultServiceModel,
			},
		},
		Packages: []config.PackageConfig{
			{
				Name:    "some-name-1",
				Package: "package-1",
				Version: "v1",
			},
		},
		Network:             config.NetworkConfig{},
		Auth:                config.AuthConfig{},
		Debug:               true,
		DisablePartialStart: true,
	}

	// Create copy of expectedCfg since Reconfigure modifies cfg.
	expectedCfg := *cfg
	r.Reconfigure(ctx, cfg)

	// Assert that Config method returns expected value.
	actualCfg = r.Config()

	// Assert that default motion and sensor services are still present, but data
	// manager default service has been replaced by the "fake1" data manager service.
	defaultSvcs = removeDefaultServices(actualCfg)
	test.That(t, len(defaultSvcs), test.ShouldEqual, 1)
	for _, svc := range defaultSvcs {
		test.That(t, svc.API.SubtypeName, test.ShouldResemble, motion.API.SubtypeName)
	}

	// Manually inspect remaining service resources as ordering of config is
	// non-deterministic within slices.
	test.That(t, len(actualCfg.Services), test.ShouldEqual, 2)
	for _, svc := range actualCfg.Services {
		isFake1DM := svc.Equals(expectedCfg.Services[0])
		isBuiltinNav := svc.Equals(expectedCfg.Services[1])
		test.That(t, isFake1DM || isBuiltinNav, test.ShouldBeTrue)
	}
	actualCfg.Services = nil
	expectedCfg.Services = nil

	// Manually inspect component resources as ordering of config is
	// non-deterministic within slices
	test.That(t, len(actualCfg.Components), test.ShouldEqual, 3)
	for _, comp := range actualCfg.Components {
		isMyBase := comp.Equals(expectedCfg.Components[0])
		isMotor1 := comp.Equals(expectedCfg.Components[1])
		isMotor2 := comp.Equals(expectedCfg.Components[2])
		test.That(t, isMyBase || isMotor1 || isMotor2, test.ShouldBeTrue)
	}
	actualCfg.Components = nil
	expectedCfg.Components = nil

	// Manually inspect remote resources, modules, and processes as Equals should be used
	// (alreadyValidated will have been set to true).
	test.That(t, len(actualCfg.Remotes), test.ShouldEqual, 1)
	test.That(t, actualCfg.Remotes[0].Equals(expectedCfg.Remotes[0]), test.ShouldBeTrue)
	actualCfg.Remotes = nil
	expectedCfg.Remotes = nil
	test.That(t, len(actualCfg.Processes), test.ShouldEqual, 1)
	test.That(t, actualCfg.Processes[0].Equals(expectedCfg.Processes[0]), test.ShouldBeTrue)
	actualCfg.Processes = nil
	expectedCfg.Processes = nil
	test.That(t, len(actualCfg.Modules), test.ShouldEqual, 1)
	test.That(t, actualCfg.Modules[0].Equals(expectedCfg.Modules[0]), test.ShouldBeTrue)
	actualCfg.Modules = nil
	expectedCfg.Modules = nil

	test.That(t, actualCfg, test.ShouldResemble, &expectedCfg)
}

func TestCheckMaxInstanceValid(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg := &config.Config{
		Services: []resource.Config{
			{
				Name:                "fake1",
				Model:               resource.DefaultServiceModel,
				API:                 motion.API,
				DependsOn:           []string{framesystem.InternalServiceName.String()},
				ConvertedAttributes: &motionBuiltin.Config{},
			},
			{
				Name:                "fake2",
				Model:               resource.DefaultServiceModel,
				API:                 motion.API,
				DependsOn:           []string{framesystem.InternalServiceName.String()},
				ConvertedAttributes: &motionBuiltin.Config{},
			},
		},
		Components: []resource.Config{
			{
				Name:                "fake2",
				Model:               fake.Model,
				API:                 arm.API,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	r := setupLocalRobot(t, context.Background(), cfg, logger)
	res, err := r.ResourceByName(motion.Named("fake1"))
	test.That(t, res, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	res, err = r.ResourceByName(motion.Named("fake2"))
	test.That(t, res, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	res, err = r.ResourceByName(arm.Named("fake2"))
	test.That(t, res, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
}

// The max allowed datamanager services is 1 so only one of the datamanager services
// from this config should build.
func TestCheckMaxInstanceInvalid(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg := &config.Config{
		Services: []resource.Config{
			{
				Name:                "fake1",
				Model:               resource.DefaultServiceModel,
				API:                 datamanager.API,
				ConvertedAttributes: &builtin.Config{},
				DependsOn:           []string{internalcloud.InternalServiceName.String()},
			},
			{
				Name:                "fake2",
				Model:               resource.DefaultServiceModel,
				API:                 datamanager.API,
				ConvertedAttributes: &builtin.Config{},
				DependsOn:           []string{internalcloud.InternalServiceName.String()},
			},
			{
				Name:                "fake3",
				Model:               resource.DefaultServiceModel,
				API:                 datamanager.API,
				ConvertedAttributes: &builtin.Config{},
				DependsOn:           []string{internalcloud.InternalServiceName.String()},
			},
		},
		Components: []resource.Config{
			{
				Name:                "fake2",
				Model:               fake.Model,
				API:                 arm.API,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "fake3",
				Model:               fake.Model,
				API:                 arm.API,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	r := setupLocalRobot(t, context.Background(), cfg, logger)
	maxInstance := 0
	for _, name := range r.ResourceNames() {
		if name.API == datamanager.API {
			maxInstance++
		}
	}
	test.That(t, maxInstance, test.ShouldEqual, 1)
	numInstances := 0
	for _, name := range r.ResourceNames() {
		if name.API == arm.API {
			numInstances++
		}
	}
	test.That(t, numInstances, test.ShouldEqual, 2)
}

func TestCheckMaxInstanceSkipRemote(t *testing.T) {
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)

	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	remoteConfig := setupLocalRobot(t, ctx, &config.Config{
		Services: []resource.Config{
			{
				Name:                "fake1",
				Model:               resource.DefaultServiceModel,
				API:                 datamanager.API,
				ConvertedAttributes: &builtin.Config{},
				DependsOn:           []string{internalcloud.InternalServiceName.String()},
			},
		},
	}, logger)

	err := remoteConfig.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	otherConfig := &config.Config{
		Services: []resource.Config{
			{
				Name:                "fake2",
				Model:               resource.DefaultServiceModel,
				API:                 datamanager.API,
				ConvertedAttributes: &builtin.Config{},
				DependsOn:           []string{internalcloud.InternalServiceName.String()},
			},
		},
		Remotes: []config.Remote{
			{
				Name:                      "remote",
				Address:                   addr,
				AssociatedResourceConfigs: []resource.AssociatedResourceConfig{},
			},
		},
	}

	r := setupLocalRobot(t, ctx, otherConfig, logger)

	maxInstance := 0
	for _, name := range r.ResourceNames() {
		if name.API == datamanager.API {
			maxInstance++
		}
	}
	test.That(t, maxInstance, test.ShouldEqual, 2)

	_, err = r.ResourceByName(datamanager.Named("fake1"))
	test.That(t, err, test.ShouldBeNil)
}

func TestDependentResources(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "b",
				Model: fakeModel,
				API:   base.API,
			},
			{
				Name:                "m",
				Model:               fakeModel,
				API:                 motor.API,
				DependsOn:           []string{"b"},
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:                "m1",
				Model:               fakeModel,
				API:                 motor.API,
				DependsOn:           []string{"m"},
				ConvertedAttributes: &fakemotor.Config{},
			},
		},
		Services: []resource.Config{
			{
				Name:      "s",
				Model:     fakeModel,
				API:       slam.API,
				DependsOn: []string{"b"},
			},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// Assert that removing base 'b' removes motors 'm' and 'm1' and slam service 's'.
	cfg2 := &config.Config{
		Components: []resource.Config{
			{
				Name:                "m",
				Model:               fakeModel,
				API:                 motor.API,
				DependsOn:           []string{"b"},
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:                "m1",
				Model:               fakeModel,
				API:                 motor.API,
				DependsOn:           []string{"m"},
				ConvertedAttributes: &fakemotor.Config{},
			},
		},
		Services: []resource.Config{
			{
				Name:      "s",
				Model:     fakeModel,
				API:       slam.API,
				DependsOn: []string{"b"},
			},
		},
	}
	r.Reconfigure(ctx, cfg2)

	res, err := r.ResourceByName(base.Named("b"))
	test.That(t, err, test.ShouldBeError,
		resource.NewNotFoundError(base.Named("b")))
	test.That(t, res, test.ShouldBeNil)
	res, err = r.ResourceByName(motor.Named("m"))
	test.That(t, err, test.ShouldBeError,
		resource.NewNotFoundError(motor.Named("m")))
	test.That(t, res, test.ShouldBeNil)
	res, err = r.ResourceByName(motor.Named("m1"))
	test.That(t, err, test.ShouldBeError,
		resource.NewNotFoundError(motor.Named("m1")))
	test.That(t, res, test.ShouldBeNil)
	res, err = r.ResourceByName(slam.Named("s"))
	test.That(t, err, test.ShouldBeError,
		resource.NewNotFoundError(slam.Named("s")))
	test.That(t, res, test.ShouldBeNil)

	// Assert that adding base 'b' back re-adds 'm' and 'm1' and slam service 's'.
	r.Reconfigure(ctx, cfg)

	_, err = r.ResourceByName(base.Named("b"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(motor.Named("m"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(motor.Named("m1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(slam.Named("s"))
	test.That(t, err, test.ShouldBeNil)
}

func TestOrphanedResources(t *testing.T) {
	ctx := context.Background()
	logger, logs := logging.NewObservedTestLogger(t)

	// Precompile modules to avoid timeout issues when building takes too long.
	complexPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")
	simplePath := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	// Manually define models, as importing them can cause double registration.
	gizmoModel := resource.NewModel("acme", "demo", "mygizmo")
	summationModel := resource.NewModel("acme", "demo", "mysum")
	gizmoAPI := resource.APINamespace("acme").WithComponentType("gizmo")
	summationAPI := resource.APINamespace("acme").WithServiceType("summation")
	helperModel := resource.NewModel("rdk", "test", "helper")

	r := setupLocalRobot(t, ctx, &config.Config{}, logger)

	t.Run("manual reconfiguration", func(t *testing.T) {
		cfg := &config.Config{
			Modules: []config.Module{
				{
					Name:    "mod",
					ExePath: complexPath,
				},
			},
			Components: []resource.Config{
				{
					Name:  "g",
					Model: gizmoModel,
					API:   gizmoAPI,
					Attributes: rutils.AttributeMap{
						"arg1": "foo",
					},
				},
			},
			Services: []resource.Config{
				{
					Name:  "s",
					Model: summationModel,
					API:   summationAPI,
				},
			},
		}
		r.Reconfigure(ctx, cfg)

		// Assert that reconfiguring module 'mod' to a new module that does not
		// handle old resources removes modular component 'g' and modular service
		// 's'.
		cfg2 := &config.Config{
			Modules: []config.Module{
				{
					Name:    "mod",
					ExePath: simplePath,
				},
			},
			Components: []resource.Config{
				{
					Name:  "g",
					Model: gizmoModel,
					API:   gizmoAPI,
					Attributes: rutils.AttributeMap{
						"arg1": "foo",
					},
				},
			},
			Services: []resource.Config{
				{
					Name:  "s",
					Model: summationModel,
					API:   summationAPI,
				},
			},
		}
		r.Reconfigure(ctx, cfg2)

		res, err := r.ResourceByName(gizmoapi.Named("g"))
		test.That(t, err, test.ShouldBeError,
			resource.NewNotFoundError(gizmoapi.Named("g")))
		test.That(t, res, test.ShouldBeNil)
		res, err = r.ResourceByName(summationapi.Named("s"))
		test.That(t, err, test.ShouldBeError,
			resource.NewNotFoundError(summationapi.Named("s")))
		test.That(t, res, test.ShouldBeNil)

		// Remove module entirely.
		cfg3 := &config.Config{
			Components: []resource.Config{
				{
					Name:  "g",
					Model: gizmoModel,
					API:   gizmoAPI,
					Attributes: rutils.AttributeMap{
						"arg1": "foo",
					},
				},
			},
			Services: []resource.Config{
				{
					Name:  "s",
					Model: summationModel,
					API:   summationAPI,
				},
			},
		}
		r.Reconfigure(ctx, cfg3)

		// Assert that adding module 'mod' back with original executable path re-adds
		// modular component 'g' and modular service 's'.
		r.Reconfigure(ctx, cfg)

		_, err = r.ResourceByName(gizmoapi.Named("g"))
		test.That(t, err, test.ShouldBeNil)
		_, err = r.ResourceByName(summationapi.Named("s"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("automatic reconfiguration", func(t *testing.T) {
		cfg := &config.Config{
			Modules: []config.Module{
				{
					Name:    "mod",
					ExePath: testPath,
				},
			},
			Components: []resource.Config{
				{
					Name:  "h",
					Model: helperModel,
					API:   generic.API,
				},
			},
		}
		r.Reconfigure(ctx, cfg)

		h, err := r.ResourceByName(generic.Named("h"))
		test.That(t, err, test.ShouldBeNil)

		// Assert that removing testmodule binary and killing testmodule orphans
		// helper 'h' a couple seconds after third restart attempt.
		err = os.Rename(testPath, testPath+".disabled")
		test.That(t, err, test.ShouldBeNil)
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "kill_module"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

		// Wait for 3 restart attempts in logs.
		testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, logs.FilterFieldKey("restart attempt").Len(),
				test.ShouldEqual, 3)
		})
		time.Sleep(2 * time.Second)

		_, err = r.ResourceByName(generic.Named("h"))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError,
			resource.NewNotFoundError(generic.Named("h")))

		// Assert that restoring testmodule, removing testmodule from config and
		// adding it back re-adds 'h'.
		err = os.Rename(testPath+".disabled", testPath)
		test.That(t, err, test.ShouldBeNil)
		cfg2 := &config.Config{
			Components: []resource.Config{
				{
					Name:  "h",
					Model: helperModel,
					API:   generic.API,
				},
			},
		}
		r.Reconfigure(ctx, cfg2)
		r.Reconfigure(ctx, cfg)

		h, err = r.ResourceByName(generic.Named("h"))
		test.That(t, err, test.ShouldBeNil)

		// Assert that replacing testmodule binary with disguised simplemodule
		// binary and killing testmodule orphans helper 'h' (not reachable), as
		// simplemodule binary cannot manage helper 'h'.
		tmpPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
		err = os.Rename(tmpPath, testPath)
		test.That(t, err, test.ShouldBeNil)
		_, err = h.DoCommand(ctx, map[string]interface{}{"command": "kill_module"})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

		// Wait for 3 restart attempts in logs.
		testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, logs.FilterFieldKey("restart attempt").Len(),
				test.ShouldEqual, 3)
		})
		time.Sleep(2 * time.Second)

		_, err = r.ResourceByName(generic.Named("h"))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError,
			resource.NewNotFoundError(generic.Named("h")))

		// Also assert that testmodule's resources were deregistered.
		_, ok := resource.LookupRegistration(generic.API, helperModel)
		test.That(t, ok, test.ShouldBeFalse)
		testMotorModel := resource.NewModel("rdk", "test", "motor")
		_, ok = resource.LookupRegistration(motor.API, testMotorModel)
		test.That(t, ok, test.ShouldBeFalse)
	})
}

var (
	doodadModel = resource.DefaultModelFamily.WithModel("mydoodad")
	doodadAPI   = resource.APINamespaceRDK.WithComponentType("doodad")
)

// doodad is an RDK-built component that depends on a modular gizmo.
type doodad struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	gizmo gizmoapi.Gizmo
}

// doThroughGizmo calls the underlying gizmo's DoCommand.
func (d *doodad) doThroughGizmo(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	return d.gizmo.DoCommand(ctx, cmd)
}

func TestDependentAndOrphanedResources(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Precompile modules to avoid timeout issues when building takes too long.
	complexPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")
	simplePath := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")

	// Manually define gizmo model, as importing it from mygizmo can cause double
	// registration.
	gizmoModel := resource.NewModel("acme", "demo", "mygizmo")

	// Register a doodad constructor and defer its deregistration.
	resource.RegisterComponent(doodadAPI, doodadModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			newDoodad := &doodad{
				Named: conf.ResourceName().AsNamed(),
			}
			for rName, res := range deps {
				if rName.API == gizmoapi.API {
					gizmo, ok := res.(gizmoapi.Gizmo)
					if !ok {
						return nil, fmt.Errorf("resource %s is not a gizmo", rName.Name)
					}
					newDoodad.gizmo = gizmo
				}
			}
			if newDoodad.gizmo == nil {
				return nil, fmt.Errorf("doodad %s must depend on a gizmo", conf.Name)
			}
			return newDoodad, nil
		},
	})
	defer func() {
		resource.Deregister(doodadAPI, doodadModel)
	}()

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: complexPath,
			},
		},
		Components: []resource.Config{
			{
				Name:      "g",
				API:       resource.APINamespace("acme").WithComponentType("gizmo"),
				Model:     gizmoModel,
				DependsOn: []string{"m"},
				Attributes: rutils.AttributeMap{
					"arg1": "foo",
				},
			},
			{
				Name:                "m",
				Model:               fakeModel,
				API:                 motor.API,
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:      "d",
				API:       resource.APINamespaceRDK.WithComponentType("doodad"),
				Model:     doodadModel,
				DependsOn: []string{"g"},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	r := setupLocalRobot(t, ctx, cfg, logger)

	// Assert that reconfiguring module 'mod' to a new module that does not handle
	// 'g' removes modular component 'g' and its dependent 'd' and leaves 'm' as-is.
	cfg2 := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: simplePath,
			},
		},
		Components: []resource.Config{
			{
				Name:      "g",
				API:       resource.APINamespace("acme").WithComponentType("gizmo"),
				Model:     gizmoModel,
				DependsOn: []string{"m"},
				Attributes: rutils.AttributeMap{
					"arg1": "foo",
				},
			},
			{
				Name:                "m",
				Model:               fakeModel,
				API:                 motor.API,
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:      "d",
				API:       resource.APINamespaceRDK.WithComponentType("doodad"),
				Model:     doodadModel,
				DependsOn: []string{"g"},
			},
		},
	}
	r.Reconfigure(ctx, cfg2)

	res, err := r.ResourceByName(gizmoapi.Named("g"))
	test.That(t, err, test.ShouldBeError,
		resource.NewNotFoundError(gizmoapi.Named("g")))
	test.That(t, res, test.ShouldBeNil)
	res, err = r.ResourceByName(resource.NewName(doodadAPI, "d"))
	test.That(t, err, test.ShouldBeError,
		resource.NewNotFoundError(resource.NewName(doodadAPI, "d")))
	test.That(t, res, test.ShouldBeNil)
	_, err = r.ResourceByName(motor.Named("m"))
	test.That(t, err, test.ShouldBeNil)

	// Remove module entirely.
	cfg3 := &config.Config{
		Components: []resource.Config{
			{
				Name:      "g",
				API:       resource.APINamespace("acme").WithComponentType("gizmo"),
				Model:     gizmoModel,
				DependsOn: []string{"m"},
				Attributes: rutils.AttributeMap{
					"arg1": "foo",
				},
			},
			{
				Name:                "m",
				Model:               fakeModel,
				API:                 motor.API,
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:      "d",
				API:       resource.APINamespaceRDK.WithComponentType("doodad"),
				Model:     doodadModel,
				DependsOn: []string{"g"},
			},
		},
	}
	r.Reconfigure(ctx, cfg3)

	// Assert that adding module 'mod' back with original executable path re-adds
	// modular component 'd' and its dependent 'd', and that 'm' is still present.
	r.Reconfigure(ctx, cfg)

	_, err = r.ResourceByName(gizmoapi.Named("g"))
	test.That(t, err, test.ShouldBeNil)
	d, err := r.ResourceByName(resource.NewName(doodadAPI, "d"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(motor.Named("m"))
	test.That(t, err, test.ShouldBeNil)

	// Assert that doodad 'd' can make gRPC calls through underlying 'g'.
	doodadD, ok := d.(*doodad)
	test.That(t, ok, test.ShouldBeTrue)
	cmd := map[string]interface{}{"foo": "bar"}
	resp, err := doodadD.doThroughGizmo(ctx, cmd)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp, test.ShouldResemble, cmd)
}

func TestModuleDebugReconfigure(t *testing.T) {
	ctx := context.Background()
	// We must use an Info level observed test logger to avoid testmodule
	// inheriting debug mode from the module manager.
	logger, logs := rtestutils.NewInfoObservedTestLogger(t)

	// Precompile module to avoid timeout issues when building takes too long.
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	// Create robot with testmodule with LogLevel unset and assert that after two
	// seconds, "debug mode enabled" debug log is not output by testmodule.
	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	time.Sleep(2 * time.Second)
	test.That(t, logs.FilterMessageSnippet("debug mode enabled").Len(),
		test.ShouldEqual, 0)

	// Reconfigure testmodule to have a "debug" LogLevel and assert that "debug
	// mode enabled" debug log is eventually output by testmodule.
	cfg2 := &config.Config{
		Modules: []config.Module{
			{
				Name:     "mod",
				ExePath:  testPath,
				LogLevel: "debug",
			},
		},
	}
	r.Reconfigure(ctx, cfg2)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, logs.FilterMessageSnippet("debug mode enabled").Len(),
			test.ShouldEqual, 1)
	})
}

func TestResourcelessModuleRemove(t *testing.T) {
	ctx := context.Background()
	logger, logs := logging.NewObservedTestLogger(t)

	// Precompile module to avoid timeout issues when building takes too long.
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// Reconfigure to an empty config and assert that the testmodule process
	// is stopped.
	r.Reconfigure(ctx, &config.Config{})

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, logs.FilterMessageSnippet("Shutting down gracefully").Len(),
			test.ShouldEqual, 1)
	})
}

func TestKill(t *testing.T) {
	// RSDK-9722: this test will not pass in CI as the managed process's manage goroutine
	// will not return from Wait() and thus fail the goroutine leak detection.
	t.Skip()
	ctx := context.Background()
	logger, logs := logging.NewObservedTestLogger(t)

	// Precompile module to avoid timeout issues when building takes too long.
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	r.Kill()

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, logs.FilterMessageSnippet("Killing module").Len(),
			test.ShouldEqual, 1)
	})
}

func TestCrashedModuleReconfigure(t *testing.T) {
	ctx := context.Background()
	logger, logs := logging.NewObservedTestLogger(t)

	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	// Manually define model, as importing it can cause double registration.
	helperModel := resource.NewModel("rdk", "test", "helper")

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
		Components: []resource.Config{
			{
				Name:  "h",
				Model: helperModel,
				API:   generic.API,
			},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	_, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)

	t.Run("reconfiguration timeout", func(t *testing.T) {
		// Lower timeouts to avoid waiting for 60 seconds for reconfig and module.
		defer func() {
			test.That(t, os.Unsetenv(rutils.ResourceConfigurationTimeoutEnvVar),
				test.ShouldBeNil)
			test.That(t, os.Unsetenv(rutils.ModuleStartupTimeoutEnvVar),
				test.ShouldBeNil)
		}()
		t.Setenv(rutils.ResourceConfigurationTimeoutEnvVar, "500ms")
		t.Setenv(rutils.ModuleStartupTimeoutEnvVar, "500ms")

		// Reconfigure module to a malformed module (does not start listening).
		// Assert that "h" is removed after reconfiguration error.
		cfg.Modules[0].ExePath = rutils.ResolveFile("module/testmodule/fakemodule.sh")
		r.Reconfigure(ctx, cfg)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			test.That(t, logs.FilterMessage("error reconfiguring module").Len(), test.ShouldEqual, 1)
		})

		_, err = r.ResourceByName(generic.Named("h"))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(generic.Named("h")))
	})

	// Reconfigure module back to testmodule. Assert that 'h' is eventually
	// added back to the resource manager (the module recovers).
	cfg.Modules[0].ExePath = testPath
	r.Reconfigure(ctx, cfg)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		_, err = r.ResourceByName(generic.Named("h"))
		test.That(tb, err, test.ShouldBeNil)
	})
}

func TestModularResourceReconfigurationCount(t *testing.T) {
	ctx := context.Background()
	logger, logs := logging.NewObservedTestLogger(t)

	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	// Manually define models, as importing them can cause double registration.
	helperModel := resource.NewModel("rdk", "test", "helper")
	otherModel := resource.NewModel("rdk", "test", "other")

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
		Components: []resource.Config{
			{
				Name:  "h",
				Model: helperModel,
				API:   generic.API,
			},
		},
		Services: []resource.Config{
			{
				Name:  "o",
				Model: otherModel,
				API:   genericservice.API,
			},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// Assert that helper and other have not yet `Reconfigure`d (only constructed).
	h, err := r.ResourceByName(generic.Named("h"))
	test.That(t, err, test.ShouldBeNil)
	resp, err := h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp["num_reconfigurations"], test.ShouldEqual, 0)
	o, err := r.ResourceByName(genericservice.Named("o"))
	test.That(t, err, test.ShouldBeNil)
	resp, err = o.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp["num_reconfigurations"], test.ShouldEqual, 0)

	cfg2 := &config.Config{
		Modules: []config.Module{
			{
				Name:     "mod",
				ExePath:  testPath,
				LogLevel: "debug",
			},
		},
		Components: []resource.Config{
			{
				Name:  "h",
				Model: helperModel,
				API:   generic.API,
			},
		},
		Services: []resource.Config{
			{
				Name:  "o",
				Model: otherModel,
				API:   genericservice.API,
			},
		},
	}
	r.Reconfigure(ctx, cfg2)

	// Assert that helper and other have still not `Reconfigure`d after their
	// module did (only constructed in the restarted module).
	resp, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp["num_reconfigurations"], test.ShouldEqual, 0)
	resp, err = o.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp["num_reconfigurations"], test.ShouldEqual, 0)

	cfg3 := &config.Config{
		Modules: []config.Module{
			{
				Name:     "mod",
				ExePath:  testPath,
				LogLevel: "debug",
			},
		},
		Components: []resource.Config{
			{
				Name:  "h",
				Model: helperModel,
				API:   generic.API,
				Attributes: rutils.AttributeMap{
					"foo": "bar",
				},
			},
		},
		Services: []resource.Config{
			{
				Name:  "o",
				Model: otherModel,
				API:   genericservice.API,
				Attributes: rutils.AttributeMap{
					"foo": "bar",
				},
			},
		},
	}
	r.Reconfigure(ctx, cfg3)

	// Assert that helper and other `Reconfigure` once when their attributes are
	// changed.
	resp, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp["num_reconfigurations"], test.ShouldEqual, 1)
	resp, err = o.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp["num_reconfigurations"], test.ShouldEqual, 1)

	cfg4 := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
		Components: []resource.Config{
			{
				Name:  "h",
				Model: helperModel,
				API:   generic.API,
				Attributes: rutils.AttributeMap{
					"bar": "baz",
				},
			},
		},
		Services: []resource.Config{
			{
				Name:  "o",
				Model: otherModel,
				API:   genericservice.API,
				Attributes: rutils.AttributeMap{
					"bar": "baz",
				},
			},
		},
	}
	r.Reconfigure(ctx, cfg4)

	// Assert that if module is reconfigured (`LogLevel` removed), _and_ helper
	// and other are reconfigured (attributes changed), helper and other are only
	// constructed in new module process and not `Reconfigure`d.
	resp, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp["num_reconfigurations"], test.ShouldEqual, 0)
	resp, err = o.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp["num_reconfigurations"], test.ShouldEqual, 0)

	// Assert that helper and other are only constructed after module
	// crash/successful restart and not `Reconfigure`d.
	_, err = h.DoCommand(ctx, map[string]any{"command": "kill_module"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "rpc error")

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessageSnippet("Module resources successfully re-added after module restart").Len(), test.ShouldEqual, 1)
	})

	resp, err = h.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp["num_reconfigurations"], test.ShouldEqual, 0)
	resp, err = o.DoCommand(ctx, map[string]any{"command": "get_num_reconfigurations"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)
	test.That(t, resp["num_reconfigurations"], test.ShouldEqual, 0)
}

func TestImplicitDepsAcrossModules(t *testing.T) {
	ctx := context.Background()
	logger, _ := logging.NewObservedTestLogger(t)

	// Precompile modules to avoid timeout issues when building takes too long.
	complexPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	// Manually define models, as importing them can cause double registration.
	myBaseModel := resource.NewModel("acme", "demo", "mybase")
	testMotorModel := resource.NewModel("rdk", "test", "motor")

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "complex-module",
				ExePath: complexPath,
			},
			{
				Name:    "test-module",
				ExePath: testPath,
			},
		},
		Components: []resource.Config{
			{
				Name:  "b",
				Model: myBaseModel,
				API:   base.API,
				Attributes: rutils.AttributeMap{
					"motorL": "m1",
					"motorR": "m2",
				},
			},
			{
				Name:  "m1",
				Model: testMotorModel,
				API:   motor.API,
			},
			{
				Name:  "m2",
				Model: testMotorModel,
				API:   motor.API,
			},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	_, err := r.ResourceByName(base.Named("b"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(motor.Named("m1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(motor.Named("m2"))
	test.That(t, err, test.ShouldBeNil)
}

func TestResourceByNameAcrossRemotes(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Setup a robot1 -> robot2 -> robot3 -> robot4 remote chain. Ensure that if
	// robot4 has an encoder "e", all robots in the chain can retrieve it by
	// simple name "e" or short name "[remote-prefix]:e". Also ensure that a
	// motor "m1" on robot1 can depend on "robot2:robot3:robot4:e" and a motor
	// "m2" on robot2 can depend on "e".

	startWeb := func(r robot.LocalRobot) string {
		var boundAddress string
		for i := 0; i < 10; i++ {
			port, err := utils.TryReserveRandomPort()
			test.That(t, err, test.ShouldBeNil)

			options := weboptions.New()
			boundAddress = fmt.Sprintf("localhost:%v", port)
			options.Network.BindAddress = boundAddress
			if err := r.StartWeb(ctx, options); err != nil {
				r.StopWeb()
				if strings.Contains(err.Error(), "address already in use") {
					logger.Infow("port in use; restarting on new port", "port", port, "err", err)
					continue
				}
				t.Fatalf("StartWeb error: %v", err)
			}
			break
		}
		return boundAddress
	}

	cfg4 := &config.Config{
		Components: []resource.Config{
			{
				Name:                "e",
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				API:                 encoder.API,
				ConvertedAttributes: &fakeencoder.Config{},
			},
		},
	}
	robot4 := setupLocalRobot(t, ctx, cfg4, logger)
	addr4 := startWeb(robot4)
	test.That(t, addr4, test.ShouldNotBeBlank)

	cfg3 := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "robot4",
				Address: addr4,
			},
		},
	}
	robot3 := setupLocalRobot(t, ctx, cfg3, logger)
	addr3 := startWeb(robot3)
	test.That(t, addr3, test.ShouldNotBeBlank)

	cfg2 := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "robot3",
				Address: addr3,
			},
		},
		Components: []resource.Config{
			{
				Name:                "m2",
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				API:                 motor.API,
				ConvertedAttributes: &fakemotor.Config{},
				// ensure DependsOn works with simple name (implicit remotes)
				DependsOn: []string{"e"},
			},
		},
	}
	robot2 := setupLocalRobot(t, ctx, cfg2, logger)
	addr2 := startWeb(robot2)
	test.That(t, addr2, test.ShouldNotBeBlank)

	cfg1 := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "robot2",
				Address: addr2,
			},
		},
		Components: []resource.Config{
			{
				Name:                "m1",
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				API:                 motor.API,
				ConvertedAttributes: &fakemotor.Config{},
				// ensure DependsOn works with short name (explicit remotes)
				DependsOn: []string{"robot2:robot3:robot4:e"},
			},
		},
	}
	robot1 := setupLocalRobot(t, ctx, cfg1, logger)

	// Ensure that "e" can be retrieved by short and simple names from all
	// robots. Also ensure "m1" and "m2" can be retrieved from robot1 and robot2
	// (they built properly).

	_, err := robot4.ResourceByName(encoder.Named("e"))
	test.That(t, err, test.ShouldBeNil)

	_, err = robot3.ResourceByName(encoder.Named("e"))
	test.That(t, err, test.ShouldBeNil)
	_, err = robot3.ResourceByName(encoder.Named("robot4:e"))
	test.That(t, err, test.ShouldBeNil)

	_, err = robot2.ResourceByName(encoder.Named("e"))
	test.That(t, err, test.ShouldBeNil)
	_, err = robot2.ResourceByName(encoder.Named("robot3:robot4:e"))
	test.That(t, err, test.ShouldBeNil)
	_, err = robot2.ResourceByName(motor.Named("m2"))
	test.That(t, err, test.ShouldBeNil)

	_, err = robot1.ResourceByName(encoder.Named("e"))
	test.That(t, err, test.ShouldBeNil)
	_, err = robot1.ResourceByName(encoder.Named("robot2:robot3:robot4:e"))
	test.That(t, err, test.ShouldBeNil)
	_, err = robot1.ResourceByName(motor.Named("m1"))
	test.That(t, err, test.ShouldBeNil)
}

func TestCloudMetadata(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	t.Run("no cloud data", func(t *testing.T) {
		cfg := &config.Config{}
		robot := setupLocalRobot(t, ctx, cfg, logger)
		_, err := robot.CloudMetadata(ctx)
		test.That(t, err, test.ShouldBeError, errors.New("cloud metadata not available"))
	})
	t.Run("with cloud data", func(t *testing.T) {
		cfg := &config.Config{
			Cloud: &config.Cloud{
				ID:           "the-robot-part",
				LocationID:   "the-location",
				PrimaryOrgID: "the-primary-org",
				MachineID:    "the-machine",
			},
		}
		robot := setupLocalRobot(t, ctx, cfg, logger)
		md, err := robot.CloudMetadata(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, md, test.ShouldResemble, cloud.Metadata{
			PrimaryOrgID:  "the-primary-org",
			LocationID:    "the-location",
			MachineID:     "the-machine",
			MachinePartID: "the-robot-part",
		})
	})
}

func TestReconfigureOnModuleRename(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Precompile complex module to avoid timeout issues when building takes too long.
	complexPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")

	// Manually define mybase model, as importing it can cause double registration.
	myBaseModel := resource.NewModel("acme", "demo", "mybase")

	// Create config with at least one module and a component from that module
	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:     "mod",
				ExePath:  complexPath,
				LogLevel: "info",
			},
		},
		Components: []resource.Config{
			{
				Name:  "myBase",
				API:   base.API,
				Model: myBaseModel,
				Attributes: rutils.AttributeMap{
					"motorL": "motor1",
					"motorR": "motor2",
				},
			},
			{
				Name:                "motor1",
				API:                 motor.API,
				Model:               fakeModel,
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:                "motor2",
				API:                 motor.API,
				Model:               fakeModel,
				ConvertedAttributes: &fakemotor.Config{},
			},
		},
	}

	// Create copy of cfg since Reconfigure (called when setting up a robot) modifies cfg.
	cfgCopy := *cfg
	r := setupLocalRobot(t, ctx, cfg, logger)

	// Use copy to modify config by renaming module
	cfgCopy.Modules[0].Name = "mod-renamed"

	r.Reconfigure(ctx, &cfgCopy)

	// Verify resources
	robotResources := []resource.Name{
		cfg.Components[0].ResourceName(),
		cfg.Components[1].ResourceName(),
		cfg.Components[2].ResourceName(),
	}
	expectedResources := rtestutils.ConcatResourceNames(
		robotResources,
		resource.DefaultServices(),
	)
	rtestutils.VerifySameResourceNames(t, r.ResourceNames(), expectedResources)
}

func TestCustomResourceBuildsOnModuleAddition(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Precompile complex module to avoid timeout issues when building takes too long.
	complexPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")

	// Manually define mygizmo model, as importing it can cause double registration, and its API
	gizmoModel := resource.NewModel("acme", "demo", "mygizmo")

	// Create config with a modular component from a custom API without its module
	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:       "myGizmo",
				API:        resource.NewAPI("acme", "component", "gizmo"),
				Model:      gizmoModel,
				Attributes: rutils.AttributeMap{"arg1": "arg1"},
			},
			{
				Name:                "motor1",
				API:                 motor.API,
				Model:               fakeModel,
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:                "motor2",
				API:                 motor.API,
				Model:               fakeModel,
				ConvertedAttributes: &fakemotor.Config{},
			},
		},
	}

	// Create copy of cfg since Reconfigure (called when setting up a robot) modifies cfg.
	cfgCopy := *cfg
	r := setupLocalRobot(t, ctx, cfg, logger)

	// Verify resources to ensure myGizmo does not build without module
	builtResources := []resource.Name{
		cfg.Components[1].ResourceName(),
		cfg.Components[2].ResourceName(),
	}
	expectedResources := rtestutils.ConcatResourceNames(
		builtResources,
		resource.DefaultServices(),
	)
	rtestutils.VerifySameResourceNames(t, r.ResourceNames(), expectedResources)

	// Modify config so that it now has a module that supports myGizmo
	mod := []config.Module{
		{
			Name:     "mod",
			ExePath:  complexPath,
			LogLevel: "info",
		},
	}
	cfgCopy.Modules = mod

	r.Reconfigure(ctx, &cfgCopy)

	// Verify resources to ensure myGizmo does build
	expectedResources = rtestutils.ConcatResourceNames(
		[]resource.Name{cfg.Components[0].ResourceName()},
		expectedResources,
	)
	rtestutils.VerifySameResourceNames(t, r.ResourceNames(), expectedResources)
}

func TestSendTriggerConfig(t *testing.T) {
	// Tests that the triggerConfig channel buffers exactly 1 message and that
	// sendTriggerConfig does not block whether or not the channel has
	// capacity.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Set up local robot normally so that the triggerConfig channel is set up normally
	r := setupLocalRobot(t, ctx, &config.Config{}, logger, withDisableCompleteConfigWorker())
	actualR := r.(*localRobot)

	// This pattern fails the test faster on deadlocks instead of having to wait for the full
	// test timeout.
	sentCh := make(chan bool)
	go func() {
		for i := 0; i < 5; i++ {
			actualR.sendTriggerConfig("remote1")
		}
		sentCh <- true
	}()
	select {
	case sent := <-sentCh:
		test.That(t, sent, test.ShouldBeTrue)
	case <-time.After(time.Second * 20):
		t.Fatal("took too long to send messages to triggerConfig channel, might be a deadlock")
	}
	test.That(t, len(actualR.triggerConfig), test.ShouldEqual, 1)
}

func assertContents(t *testing.T, path, expectedContents string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, contents, test.ShouldResemble, []byte(expectedContents))
}

func TestRestartModule(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("isRunning=false", func(t *testing.T) {
		tmp := t.TempDir()
		badExePath := filepath.Join(tmp, "/nosuchexe")
		const bash = `#!/usr/bin/env bash
		echo STARTED > result.txt
		echo exiting right away
		`
		os.WriteFile(badExePath, []byte(bash), 0o700)
		mod := &config.Module{Name: "restartSingleModule-test", ExePath: badExePath, Type: config.ModuleTypeLocal}
		r := setupLocalRobot(t, ctx, &config.Config{Modules: []config.Module{*mod}}, logger)
		test.That(t, mod.LocalVersion, test.ShouldBeEmpty)

		// make sure this started + failed
		outputPath := filepath.Join(tmp, "result.txt")
		assertContents(t, outputPath, "STARTED\n")
		// clear this so the restart attempt writes it again
		test.That(t, os.Remove(outputPath), test.ShouldBeNil)

		// confirm that we don't error
		err := r.RestartModule(ctx, robot.RestartModuleRequest{ModuleName: mod.Name})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, r.(*localRobot).localModuleVersions[mod.Name].String(), test.ShouldResemble, "0.0.1")
		// make sure it really ran again
		assertContents(t, outputPath, "STARTED\n")
	})

	t.Run("isRunning=true", func(t *testing.T) {
		logger, _ := logging.NewObservedTestLogger(t)
		simplePath := rtestutils.BuildTempModule(t, "examples/customresources/demos/simplemodule")
		mod := &config.Module{Name: "restartSingleModule-test", ExePath: simplePath, Type: config.ModuleTypeLocal}
		r := setupLocalRobot(t, ctx, &config.Config{Modules: []config.Module{*mod}}, logger)

		// test restart. note: we're not testing that the PID rolls over because we don't have access to
		// that state. 'no error' + 'version incremented' is a cheap proxy for that.
		err := r.RestartModule(ctx, robot.RestartModuleRequest{ModuleName: mod.Name})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, r.(*localRobot).localModuleVersions[mod.Name].String(), test.ShouldResemble, "0.0.1")
	})
}

var mockModel = resource.DefaultModelFamily.WithModel("mockmodel")

type mockResource struct {
	resource.Named
	resource.TriviallyCloseable
	name  string
	value int
}

type mockConfig struct {
	Value int    `json:"value"`
	Fail  bool   `json:"fail"`
	Sleep string `json:"sleep"`
}

//nolint:unparam // the resource name is currently always "m" but this could easily change
func newMockConfig(name string, val int, fail bool, sleep string) resource.Config {
	return resource.Config{
		Name:  name,
		Model: mockModel,
		API:   mockAPI,
		// We need to specify both `Attributes` and `ConvertedAttributes`.
		// The former triggers a reconfiguration and the former is actually
		// used to reconfigure the component.
		Attributes:          rutils.AttributeMap{"value": val, "fail": fail, "sleep": sleep},
		ConvertedAttributes: &mockConfig{Value: val, Fail: fail, Sleep: sleep},
	}
}

var errMockValidation = errors.New("whoops")

func (cfg *mockConfig) Validate(path string) ([]string, error) {
	if cfg.Fail {
		return nil, errMockValidation
	}
	return []string{}, nil
}

func newMock(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (resource.Resource, error) {
	m := &mockResource{name: conf.Name}
	if err := m.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *mockResource) Name() resource.Name {
	return mockNamed(m.name)
}

func (m *mockResource) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	mConf, err := resource.NativeConfig[*mockConfig](conf)
	if err != nil {
		return err
	}
	if mConf.Sleep != "" {
		if d, err := time.ParseDuration(mConf.Sleep); err == nil {
			log.Printf("sleeping for %s\n", d)
			time.Sleep(d)
		}
	}
	m.value = mConf.Value
	return nil
}

// getExpectedDefaultStatuses returns a slice of default [resource.Status] with a given
// revision and cloud metadata.
func getExpectedDefaultStatuses(revision string, md cloud.Metadata) []resource.Status {
	return []resource.Status{
		{
			NodeStatus: resource.NodeStatus{
				Name: resource.Name{
					API:  resource.APINamespaceRDKInternal.WithServiceType("frame_system"),
					Name: "builtin",
				},
				State: resource.NodeStateReady,
			},
			CloudMetadata: md,
		},
		{
			NodeStatus: resource.NodeStatus{
				Name: resource.Name{
					API:  resource.APINamespaceRDKInternal.WithServiceType("cloud_connection"),
					Name: "builtin",
				},
				State: resource.NodeStateReady,
			},
			CloudMetadata: md,
		},
		{
			NodeStatus: resource.NodeStatus{
				Name: resource.Name{
					API:  resource.APINamespaceRDKInternal.WithServiceType("packagemanager"),
					Name: "builtin",
				},
				State: resource.NodeStateReady,
			},
			CloudMetadata: md,
		},
		{
			NodeStatus: resource.NodeStatus{
				Name: resource.Name{
					API:  resource.APINamespaceRDKInternal.WithServiceType("web"),
					Name: "builtin",
				},
				State: resource.NodeStateReady,
			},
			CloudMetadata: md,
		},
		{
			NodeStatus: resource.NodeStatus{
				Name: resource.Name{
					API:  resource.APINamespaceRDK.WithServiceType("motion"),
					Name: "builtin",
				},
				State:    resource.NodeStateReady,
				Revision: revision,
			},
			CloudMetadata: md,
		},
	}
}

func TestMachineStatus(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	resource.RegisterComponent(
		mockAPI,
		mockModel,
		resource.Registration[resource.Resource, *mockConfig]{Constructor: newMock},
	)
	defer resource.Deregister(mockAPI, mockModel)

	t.Run("default resources", func(t *testing.T) {
		rev1 := "rev1"
		lr := setupLocalRobot(t, ctx, &config.Config{Revision: rev1}, logger)

		mStatus, err := lr.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, rev1)

		expectedStatuses := getExpectedDefaultStatuses(rev1, cloud.Metadata{})
		rtestutils.VerifySameResourceStatuses(t, mStatus.Resources, expectedStatuses)
	})

	t.Run("default resources with cloud metadata", func(t *testing.T) {
		rev1 := "rev1"
		partID := "the-robot-part"
		locID := "the-location"
		orgID := "the-org"
		machineID := "the-machine"
		cfg := &config.Config{
			Cloud: &config.Cloud{
				ID:           partID,
				LocationID:   locID,
				PrimaryOrgID: orgID,
				MachineID:    machineID,
			},
			Revision: rev1,
		}
		lr := setupLocalRobot(t, ctx, cfg, logger)

		mStatus, err := lr.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, rev1)

		md := cloud.Metadata{
			PrimaryOrgID:  orgID,
			LocationID:    locID,
			MachineID:     machineID,
			MachinePartID: partID,
		}
		expectedStatuses := getExpectedDefaultStatuses(rev1, md)
		rtestutils.VerifySameResourceStatuses(t, mStatus.Resources, expectedStatuses)
	})

	t.Run("poll after working and failing reconfigures", func(t *testing.T) {
		lr := setupLocalRobot(t, ctx, &config.Config{Revision: "rev1"}, logger)

		// Add a fake resource to the robot.
		rev2 := "rev2"
		lr.Reconfigure(ctx, &config.Config{
			Revision:   rev2,
			Components: []resource.Config{newMockConfig("m", 0, false, "")},
		})
		mStatus, err := lr.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, rev2)
		expectedStatuses := rtestutils.ConcatResourceStatuses(
			getExpectedDefaultStatuses(rev2, cloud.Metadata{}),
			[]resource.Status{
				{
					NodeStatus: resource.NodeStatus{
						Name:     mockNamed("m"),
						State:    resource.NodeStateReady,
						Revision: rev2,
					},
				},
			},
		)
		rtestutils.VerifySameResourceStatuses(t, mStatus.Resources, expectedStatuses)

		// Update resource config to cause reconfiguration to fail.
		rev3 := "rev3"
		lr.Reconfigure(ctx, &config.Config{
			Revision:   rev3,
			Components: []resource.Config{newMockConfig("m", 0, true, "")},
		})
		mStatus, err = lr.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, rev3)

		expectedConfigError := fmt.Errorf("resource config validation error: %w", errMockValidation)
		expectedStatuses = rtestutils.ConcatResourceStatuses(
			getExpectedDefaultStatuses(rev3, cloud.Metadata{}),
			[]resource.Status{
				{
					NodeStatus: resource.NodeStatus{
						Name:     mockNamed("m"),
						State:    resource.NodeStateUnhealthy,
						Revision: rev2,
						Error:    expectedConfigError,
					},
				},
			},
		)
		rtestutils.VerifySameResourceStatuses(t, mStatus.Resources, expectedStatuses)

		// Update resource with a working config.
		rev4 := "rev4"
		lr.Reconfigure(ctx, &config.Config{
			Revision:   rev4,
			Components: []resource.Config{newMockConfig("m", 200, false, "")},
		})
		mStatus, err = lr.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, rev4)
		expectedStatuses = rtestutils.ConcatResourceStatuses(
			getExpectedDefaultStatuses(rev4, cloud.Metadata{}),
			[]resource.Status{
				{
					NodeStatus: resource.NodeStatus{
						Name:     mockNamed("m"),
						State:    resource.NodeStateReady,
						Revision: rev4,
					},
				},
			},
		)
		rtestutils.VerifySameResourceStatuses(t, mStatus.Resources, expectedStatuses)
	})

	t.Run("poll during reconfiguration", func(t *testing.T) {
		rev1 := "rev1"
		lr := setupLocalRobot(t, ctx, &config.Config{
			Revision:   rev1,
			Components: []resource.Config{newMockConfig("m", 200, false, "")},
		}, logger)

		// update resource with a working config that is slow to reconfigure.
		rev2 := "rev2"
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			lr.Reconfigure(ctx, &config.Config{
				Revision:   rev2,
				Components: []resource.Config{newMockConfig("m", 300, false, "1s")},
			})
		}()
		// sleep for a short amount of time to allow the machine to receive a new
		// revision. this sleep should be shorter than the resource update duration
		// defined above so that updated resource is still in a "configuring" state.
		time.Sleep(time.Millisecond * 100)

		// get status while reconfiguring
		mStatus, err := lr.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, rev2)

		// the component whose config changed should be the only component in a
		// "configuring" state and associated with the original revision.
		filterConfiguring := rtestutils.FilterByStatus(t, mStatus.Resources, resource.NodeStateConfiguring)
		expectedConfiguring := []resource.Status{
			{
				NodeStatus: resource.NodeStatus{
					Name:     mockNamed("m"),
					State:    resource.NodeStateConfiguring,
					Revision: rev1,
				},
			},
		}
		rtestutils.VerifySameResourceStatuses(t, filterConfiguring, expectedConfiguring)

		// all other components should be in the "ready" state and associated with the
		// new revision.
		filterReady := rtestutils.FilterByStatus(t, mStatus.Resources, resource.NodeStateReady)
		expectedReady := getExpectedDefaultStatuses(rev2, cloud.Metadata{})
		rtestutils.VerifySameResourceStatuses(t, filterReady, expectedReady)

		wg.Wait()

		// get status after reconfigure finishes
		mStatus, err = lr.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, rev2)

		// now all components, including the one whose config changed, should all be in
		// the "ready" state and associated with the new revision.
		expectedStatuses := rtestutils.ConcatResourceStatuses(
			getExpectedDefaultStatuses(rev2, cloud.Metadata{}),
			[]resource.Status{
				{
					NodeStatus: resource.NodeStatus{
						Name:     mockNamed("m"),
						State:    resource.NodeStateReady,
						Revision: rev2,
					},
				},
			},
		)
		rtestutils.VerifySameResourceStatuses(t, mStatus.Resources, expectedStatuses)
	})
}

func TestMachineStatusWithRemotes(t *testing.T) {
	logger := logging.NewTestLogger(t)

	testCases := []struct {
		name          string
		localCloudMd  bool
		remoteCloudMd bool
	}{
		{
			name:          "local cloud metadata and remote cloud metadata",
			localCloudMd:  true,
			remoteCloudMd: true,
		},
		{
			name:          "local cloud metadata and no remote cloud metadata",
			localCloudMd:  true,
			remoteCloudMd: false,
		},
		{
			name:          "no local cloud metadata and remote cloud metadata",
			localCloudMd:  false,
			remoteCloudMd: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			resName1 := resource.NewName(resource.APINamespace("acme").WithComponentType("huwat"), "thing1")
			injectRemoteRobot := &inject.Robot{
				LoggerFunc:          func() logging.Logger { return logger },
				ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
				ResourceNamesFunc:   func() []resource.Name { return []resource.Name{resName1} },
			}
			remoteName := "remote1"
			injectRemoteRobot.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
				for _, rName := range injectRemoteRobot.ResourceNames() {
					if rName == name {
						return rgrpc.NewForeignResource(rName, nil), nil
					}
				}
				return nil, resource.NewNotFoundError(name)
			}

			remoteMd := cloud.Metadata{
				PrimaryOrgID:  "the-remote-org",
				LocationID:    "the-remote-location",
				MachineID:     "the-remote-machine",
				MachinePartID: "the-remote-part",
			}
			injectRemoteRobot.CloudMetadataFunc = func(context.Context) (cloud.Metadata, error) {
				if !tc.remoteCloudMd {
					return cloud.Metadata{}, errNoCloudMetadata
				}
				return remoteMd, nil
			}
			injectRemoteRobot.MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
				// check that a timeout is passed down from the caller.
				if _, ok := ctx.Deadline(); !ok {
					return robot.MachineStatus{}, errors.New("no timeout detected")
				}
				md := cloud.Metadata{}
				if tc.remoteCloudMd {
					md = remoteMd
				}
				return robot.MachineStatus{
					Resources: []resource.Status{
						{
							NodeStatus: resource.NodeStatus{
								Name: resName1, State: resource.NodeStateUnconfigured, Revision: "rev2",
							},
							CloudMetadata: md,
						},
					},
				}, nil
			}
			dRobot := newDummyRobot(t, injectRemoteRobot)
			md := cloud.Metadata{
				PrimaryOrgID:  "the-org",
				LocationID:    "the-location",
				MachineID:     "the-machine",
				MachinePartID: "the-robot-part",
			}
			rev1 := "rev1"
			cfg := &config.Config{}
			if tc.localCloudMd {
				cfg = &config.Config{
					Cloud: &config.Cloud{
						ID:           md.MachinePartID,
						LocationID:   md.LocationID,
						PrimaryOrgID: md.PrimaryOrgID,
						MachineID:    md.MachineID,
					},
					Revision: rev1,
				}
			}
			lr := setupLocalRobot(t, ctx, cfg, logger, withDisableCompleteConfigWorker())
			lr.(*localRobot).manager.addRemote(
				context.Background(),
				dRobot,
				nil,
				config.Remote{Name: remoteName},
			)

			mStatus, err := lr.MachineStatus(ctx)
			test.That(t, err, test.ShouldBeNil)

			expectedRev := ""
			expectedMd := cloud.Metadata{}
			if tc.localCloudMd {
				expectedRev = rev1
				expectedMd = md
			}

			expectedRemoteMd := cloud.Metadata{}
			if tc.remoteCloudMd {
				expectedRemoteMd = remoteMd
			}
			test.That(t, mStatus.Config.Revision, test.ShouldEqual, expectedRev)

			expectedStatuses := rtestutils.ConcatResourceStatuses(
				getExpectedDefaultStatuses(expectedRev, expectedMd),
				[]resource.Status{
					{
						NodeStatus: resource.NodeStatus{
							Name:  resource.NewName(client.RemoteAPI, remoteName),
							State: resource.NodeStateReady,
						},
						CloudMetadata: expectedRemoteMd,
					},
					{
						NodeStatus: resource.NodeStatus{
							Name:  resName1.PrependRemote(remoteName),
							State: resource.NodeStateReady,
						},
						CloudMetadata: expectedRemoteMd,
					},
				},
			)
			rtestutils.VerifySameResourceStatuses(t, mStatus.Resources, expectedStatuses)
		})
	}
}

func TestMachineStatusWithTwoRemotes(t *testing.T) {
	// test that if one remote returns an error, MachineStatus returns correctly.
	// ResourceStatuses for the erroring remote should not have CloudMetadata while
	// ResourceStatuses for the non-erroring remote should have CloudMetadata.
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	resName1 := resource.NewName(resource.APINamespace("acme").WithComponentType("huwat"), "thing1")
	injectRemoteRobot1 := &inject.Robot{
		LoggerFunc:          func() logging.Logger { return logger },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		ResourceNamesFunc:   func() []resource.Name { return []resource.Name{resName1} },
	}
	remoteName1 := "remote1"
	injectRemoteRobot1.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		for _, rName := range injectRemoteRobot1.ResourceNames() {
			if rName == name {
				return rgrpc.NewForeignResource(rName, nil), nil
			}
		}
		return nil, resource.NewNotFoundError(name)
	}
	injectRemoteRobot1.CloudMetadataFunc = func(context.Context) (cloud.Metadata, error) {
		return cloud.Metadata{}, errNoCloudMetadata
	}
	injectRemoteRobot1.MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
		// check that a timeout is passed down from the caller.
		if _, ok := ctx.Deadline(); !ok {
			return robot.MachineStatus{}, errors.New("no timeout detected")
		}
		return robot.MachineStatus{
			Resources: []resource.Status{
				{
					NodeStatus: resource.NodeStatus{
						Name: resName1, State: resource.NodeStateUnconfigured, Revision: "rev2",
					},
					CloudMetadata: cloud.Metadata{},
				},
			},
		}, nil
	}
	dRobot1 := newDummyRobot(t, injectRemoteRobot1)
	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, withDisableCompleteConfigWorker())
	lr.(*localRobot).manager.addRemote(
		context.Background(),
		dRobot1,
		nil,
		config.Remote{Name: remoteName1},
	)

	resName2 := resource.NewName(resource.APINamespace("acme").WithComponentType("huwat"), "thing2")
	injectRemoteRobot2 := &inject.Robot{
		LoggerFunc:          func() logging.Logger { return logger },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		ResourceNamesFunc:   func() []resource.Name { return []resource.Name{resName2} },
	}
	remoteName2 := "remote2"
	injectRemoteRobot2.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		for _, rName := range injectRemoteRobot2.ResourceNames() {
			if rName == name {
				return rgrpc.NewForeignResource(rName, nil), nil
			}
		}
		return nil, resource.NewNotFoundError(name)
	}

	remoteMd := cloud.Metadata{
		PrimaryOrgID:  "the-remote-org",
		LocationID:    "the-remote-location",
		MachineID:     "the-remote-machine",
		MachinePartID: "the-remote-part",
	}
	injectRemoteRobot2.CloudMetadataFunc = func(context.Context) (cloud.Metadata, error) {
		return remoteMd, nil
	}
	injectRemoteRobot2.MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
		// check that a timeout is passed down from the caller.
		if _, ok := ctx.Deadline(); !ok {
			return robot.MachineStatus{}, errors.New("no timeout detected")
		}
		return robot.MachineStatus{
			Resources: []resource.Status{
				{
					NodeStatus: resource.NodeStatus{
						Name: resName2, State: resource.NodeStateUnconfigured, Revision: "rev2",
					},
					CloudMetadata: remoteMd,
				},
			},
		}, nil
	}
	dRobot2 := newDummyRobot(t, injectRemoteRobot2)
	lr.(*localRobot).manager.addRemote(
		context.Background(),
		dRobot2,
		nil,
		config.Remote{Name: remoteName2},
	)

	mStatus, err := lr.MachineStatus(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, mStatus.Config.Revision, test.ShouldEqual, "")
	expectedStatuses := rtestutils.ConcatResourceStatuses(
		getExpectedDefaultStatuses("", cloud.Metadata{}),
		[]resource.Status{
			{
				NodeStatus: resource.NodeStatus{
					Name:  resource.NewName(client.RemoteAPI, remoteName1),
					State: resource.NodeStateReady,
				},
			},
			{
				NodeStatus: resource.NodeStatus{
					Name:  resName1.PrependRemote(remoteName1),
					State: resource.NodeStateReady,
				},
			},
			{
				NodeStatus: resource.NodeStatus{
					Name:  resource.NewName(client.RemoteAPI, remoteName2),
					State: resource.NodeStateReady,
				},
				CloudMetadata: remoteMd,
			},
			{
				NodeStatus: resource.NodeStatus{
					Name:  resName2.PrependRemote(remoteName2),
					State: resource.NodeStateReady,
				},
				CloudMetadata: remoteMd,
			},
		},
	)
	rtestutils.VerifySameResourceStatuses(t, mStatus.Resources, expectedStatuses)
}

func TestMachineStatusWithRemoteChain(t *testing.T) {
	// test that if MachineStatus returns correctly if the remote has a remote
	// The topography is local -> remote1 -> remote2
	// remote2 will always have cloud metadata, but this test will test behavior
	// depending on whether remote1 is online and has cloud metadata
	logger := logging.NewTestLogger(t)

	testCases := []struct {
		name          string
		remoteOffline bool
		remoteCloudMd bool
	}{
		{
			name:          "remote1 is online and has cloud metadata",
			remoteOffline: false,
			remoteCloudMd: true,
		},
		{
			name:          "remote1 is offline and has cloud metadata",
			remoteOffline: true,
			remoteCloudMd: true,
		},
		{
			name:          "remote1 is online and does not have cloud metadata",
			remoteOffline: false,
			remoteCloudMd: false,
		},
		{
			name:          "remote1 is offline and does not have cloud metadata",
			remoteOffline: true,
			remoteCloudMd: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// setup remote2
			resName1 := resource.NewName(resource.APINamespace("acme").WithComponentType("huwat"), "thing1")
			remote2 := &inject.Robot{
				LoggerFunc:          func() logging.Logger { return logger },
				ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
				ResourceNamesFunc:   func() []resource.Name { return []resource.Name{resName1} },
			}
			remoteName2 := "remote1"
			remote2.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
				for _, rName := range remote2.ResourceNames() {
					if rName == name {
						return rgrpc.NewForeignResource(rName, nil), nil
					}
				}
				return nil, resource.NewNotFoundError(name)
			}

			remote2Md := cloud.Metadata{
				PrimaryOrgID:  "the-remote-org",
				LocationID:    "the-remote-location",
				MachineID:     "the-remote-machine",
				MachinePartID: "the-remote-part",
			}
			remote2.CloudMetadataFunc = func(context.Context) (cloud.Metadata, error) {
				return remote2Md, nil
			}
			remote2.MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
				// check that a timeout is passed down from the caller.
				if _, ok := ctx.Deadline(); !ok {
					return robot.MachineStatus{}, errors.New("no timeout detected")
				}
				return robot.MachineStatus{
					Resources: []resource.Status{
						{
							NodeStatus: resource.NodeStatus{
								Name: resName1, State: resource.NodeStateUnconfigured, Revision: "rev1",
							},
							CloudMetadata: remote2Md,
						},
					},
				}, nil
			}
			remote2Dummy := newDummyRobot(t, remote2)

			// setup remote1
			cfg := &config.Config{}
			md := cloud.Metadata{
				PrimaryOrgID:  "the-org",
				LocationID:    "the-location",
				MachineID:     "the-machine",
				MachinePartID: "the-robot-part",
			}
			if tc.remoteCloudMd {
				cfg = &config.Config{
					Cloud: &config.Cloud{
						ID:           md.MachinePartID,
						LocationID:   md.LocationID,
						PrimaryOrgID: md.PrimaryOrgID,
						MachineID:    md.MachineID,
					},
				}
			}
			remote1 := setupLocalRobot(t, ctx, cfg, logger, withDisableCompleteConfigWorker())
			remote1.(*localRobot).manager.addRemote(
				context.Background(),
				remote2Dummy,
				nil,
				config.Remote{Name: remoteName2},
			)

			// setup local
			remoteName1 := "remote1"
			remote1Dummy := newDummyRobot(t, remote1)
			lRobot := setupLocalRobot(t, ctx, &config.Config{}, logger, withDisableCompleteConfigWorker())
			lRobot.(*localRobot).manager.addRemote(
				context.Background(),
				remote1Dummy,
				nil,
				config.Remote{Name: remoteName1},
			)

			remote1Dummy.SetOffline(tc.remoteOffline)
			// even though the remote1 is now offline, resources will be kept in the resource graph
			// but marked unreachable
			anythingChanged := lRobot.(*localRobot).manager.updateRemotesResourceNames(ctx)
			test.That(t, anythingChanged, test.ShouldBeFalse)

			mStatus, err := lRobot.MachineStatus(ctx)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, mStatus.Config.Revision, test.ShouldEqual, "")

			expectedMd := cloud.Metadata{}
			expectedRemote2Md := cloud.Metadata{}
			if !tc.remoteOffline {
				if tc.remoteCloudMd {
					expectedMd = md
				}
				expectedRemote2Md = remote2Md
			}
			expectedStatuses := rtestutils.ConcatResourceStatuses(
				getExpectedDefaultStatuses("", cloud.Metadata{}),
				[]resource.Status{
					{
						NodeStatus: resource.NodeStatus{
							Name:  resource.NewName(client.RemoteAPI, remoteName1),
							State: resource.NodeStateReady,
						},
						CloudMetadata: expectedMd,
					},
					{
						NodeStatus: resource.NodeStatus{
							Name:  motion.Named("builtin").PrependRemote(remoteName1),
							State: resource.NodeStateReady,
						},
						CloudMetadata: expectedMd,
					},
					{
						NodeStatus: resource.NodeStatus{
							Name:  resName1.PrependRemote(remoteName2).PrependRemote(remoteName1),
							State: resource.NodeStateReady,
						},
						CloudMetadata: expectedRemote2Md,
					},
				},
			)
			rtestutils.VerifySameResourceStatuses(t, mStatus.Resources, expectedStatuses)
		})
	}
}

// assertDialFails reconnects an existing `RobotClient` with a small timeout value to keep tests
// fast.
func assertDialFails(t *testing.T, client *client.RobotClient) {
	t.Helper()
	ctx, done := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer done()
	err := client.Connect(ctx)
	test.That(t, err, test.ShouldNotBeNil)
}

// TestStickyWebRTCConnection ensures that once a RobotClient object makes a WebRTC connection, it
// will only make future WebRTC connections.
func TestStickyWebRTCConnection(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	// Start a robot and stand up its "web".
	robot := setupLocalRobot(t, ctx, &config.Config{}, logger.Sublogger("robot"))
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err := robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)
	defer robot.StopWeb()

	// Connect to the robot with a client. This should be a WebRTC connection, but we do not assert
	// that.
	robotClient, connectionErr := client.New(ctx, addr, logger.Sublogger("client"))
	test.That(t, connectionErr, test.ShouldBeNil)
	defer robotClient.Close(ctx)

	// Stop the "web".
	robot.StopWeb()
	// Explicitly reconnect the RobotClient. The "web" is down therefore this client will time out
	// and error.
	assertDialFails(t, robotClient)

	// Massage the options to restart the "web" on the same port as before. Note: this can result in
	// a test bug/failure as another test may have picked up the same port in the meantime.
	options.Network.BindAddress = addr
	options.Network.Listener = nil
	err = robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Explicitly reconnect with the robot client. Reconnecting will succeed, and this should be a
	// WebRTC connection. Because this reconnect should succeed, we do not pass in a timeout.
	connectionErr = robotClient.Connect(ctx)
	test.That(t, connectionErr, test.ShouldBeNil)

	// Stop the "web" again. Assert we can no longer connect.
	robot.StopWeb()
	assertDialFails(t, robotClient)

	// Restart the "web" but only accept direct gRPC connections.
	options.DisallowWebRTC = true
	err = robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Explicitly reconnect with the RobotClient. The RobotClient should only try creating a WebRTC
	// connection and thus fail this attempt.
	assertDialFails(t, robotClient)

	// However, a new RobotClient should happily create a direct gRPC connection.
	cleanClient, err := client.New(ctx, addr, logger.Sublogger("clean_client"))
	test.That(t, err, test.ShouldBeNil)
	cleanClient.Close(ctx)
}

var (
	fooModel = resource.DefaultModelFamily.WithModel("foo")
	barModel = resource.DefaultModelFamily.WithModel("bar")
)

// fooComponent is an RDK-built component that can output logs.
type fooComponent struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	logger logging.Logger
}

// DoCommand accepts a "log" command.
func (fc *fooComponent) DoCommand(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	return doCommand(ctx, req, fc.logger)
}

// barService is an RDK-built service that can output logs.
type barService struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	logger logging.Logger
}

// DoCommand accepts a "log" command.
func (bs *barService) DoCommand(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	return doCommand(ctx, req, bs.logger)
}

func doCommand(ctx context.Context, req map[string]interface{}, logger logging.Logger) (map[string]interface{}, error) {
	cmd, ok := req["command"]
	if !ok {
		return nil, errors.New("missing 'command' string")
	}

	switch req["command"] {
	case "log":
		level, err := logging.LevelFromString(req["level"].(string))
		if err != nil {
			return nil, err
		}

		msg := req["msg"].(string)
		switch level {
		case logging.DEBUG:
			logger.CDebugw(ctx, msg)
		case logging.INFO:
			logger.CInfow(ctx, msg)
		case logging.WARN:
			logger.CWarnw(ctx, msg)
		case logging.ERROR:
			logger.CErrorw(ctx, msg)
		}

		return map[string]any{}, nil
	default:
		return nil, fmt.Errorf("unknown command string %s", cmd)
	}
}

func TestModuleLogging(t *testing.T) {
	// Similar to, but slightly different from `TestLogPropagation` below. We
	// want to ensure that the RDK trusts modules to handle `log_configuration`
	// fields themselves. Even if the RDK is configured at INFO level, assert
	// that modular resources can log at their own, configured levels.

	ctx := context.Background()
	logger, observer, registry := logging.NewObservedTestLoggerWithRegistry(t, "rdk")
	logger.SetLevel(logging.INFO)
	helperModel := resource.NewModel("rdk", "test", "helper")
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
		Components: []resource.Config{
			{
				Name:  "helper",
				API:   generic.API,
				Model: helperModel,
				LogConfiguration: &resource.LogConfig{
					Level: logging.DEBUG,
				},
			},
		},
	}

	config.UpdateLoggerRegistryFromConfig(registry, cfg, logger)
	lr := setupLocalRobot(t, ctx, cfg, logger)

	startsAtDebugRes, err := lr.ResourceByName(generic.Named("helper"))
	test.That(t, err, test.ShouldBeNil)
	_, err = startsAtDebugRes.DoCommand(ctx,
		map[string]interface{}{"command": "log", "msg": "debug log line", "level": "DEBUG"})
	test.That(t, err, test.ShouldBeNil)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(t, observer.FilterMessageSnippet("debug log line").Len(), test.ShouldEqual, 1)
	})
}

func TestLogPropagation(t *testing.T) {
	// Mimic what entrypoint code does: use a config to update a registry, and
	// assert log levels are propagated to a robot. This means we call
	// `UpdateLoggerRegistryFromConfig` and _then_ `setupLocalRobot` or
	// `Reconfigure`.

	ctx := context.Background()
	logger, observer, registry := logging.NewObservedTestLoggerWithRegistry(t, "rdk")
	helperModel := resource.NewModel("rdk", "test", "helper")
	otherModel := resource.NewModel("rdk", "test", "other")
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	// Register a foo component and defer its deregistration.
	resource.RegisterComponent(generic.API, fooModel, resource.Registration[resource.Resource,
		resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			return &fooComponent{
				Named:  conf.ResourceName().AsNamed(),
				logger: logger,
			}, nil
		},
	})
	defer func() {
		resource.Deregister(generic.API, fooModel)
	}()

	// Register a bar service and defer its deregistration.
	resource.RegisterService(genericservice.API, barModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			return &barService{
				Named:  conf.ResourceName().AsNamed(),
				logger: logger,
			}, nil
		},
	})
	defer func() {
		resource.Deregister(genericservice.API, barModel)
	}()

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
		Components: []resource.Config{
			{
				Name:  "foo", // built-in component
				API:   generic.API,
				Model: fooModel,
			},
			{
				Name:  "helper", // modular component
				API:   generic.API,
				Model: helperModel,
			},
		},
		Services: []resource.Config{
			{
				Name:  "bar", // built-in service
				API:   genericservice.API,
				Model: barModel,
			},
			{
				Name:  "other", // modular service
				API:   genericservice.API,
				Model: otherModel,
			},
		},
		LogConfig: []logging.LoggerPatternConfig{
			{
				Pattern: "rdk.resource_manager.rdk:component:generic/foo",
				Level:   "INFO",
			},
			{
				Pattern: "rdk.TestModule.rdk:component:generic/helper",
				Level:   "INFO",
			},
			{
				Pattern: "rdk.resource_manager.rdk:service:generic/bar",
				Level:   "INFO",
			},
			{
				Pattern: "rdk.TestModule.rdk:service:generic/other",
				Level:   "INFO",
			},
		},
	}

	config.UpdateLoggerRegistryFromConfig(registry, cfg, logger)
	lr := setupLocalRobot(t, ctx, cfg, logger)

	resourceNames := []resource.Name{
		generic.Named("foo"),
		generic.Named("helper"),
		genericservice.Named("bar"),
		genericservice.Named("other"),
	}

	// Assert that all resources' loggers are configured at INFO level
	// (INFO level logs appear but DEBUG level logs do not) due to `LogConfig`
	// patterns.
	for i, name := range resourceNames {
		// Use index to differentiate logs from different resources.
		infoLogLine := fmt.Sprintf("INFO level log line %d", i)
		infoLogCmd := map[string]interface{}{"command": "log", "msg": infoLogLine, "level": "INFO"}
		debugLogLine := fmt.Sprintf("debug-level log line %d", i)
		debugLogCmd := map[string]interface{}{"command": "log", "msg": debugLogLine, "level": "DEBUG"}

		res, err := lr.ResourceByName(name)
		test.That(t, err, test.ShouldBeNil)
		_, err = res.DoCommand(ctx, infoLogCmd)
		test.That(t, err, test.ShouldBeNil)
		_, err = res.DoCommand(ctx, debugLogCmd)
		test.That(t, err, test.ShouldBeNil)

		// Each should output one INFO level log and no DEBUG level logs.
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, observer.FilterMessageSnippet(infoLogLine).Len(), test.ShouldEqual, 1)
		})
		test.That(t, observer.FilterMessage(debugLogLine+"\n").Len(), test.ShouldEqual, 0)
	}

	// Mutate config to set `LogConfiguration` fields to DEBUG for all resources.
	debugLogConfiguration := &resource.LogConfig{Level: logging.DEBUG}
	cfg.Components[0].LogConfiguration = debugLogConfiguration
	cfg.Components[1].LogConfiguration = debugLogConfiguration
	cfg.Services[0].LogConfiguration = debugLogConfiguration
	cfg.Services[1].LogConfiguration = debugLogConfiguration

	config.UpdateLoggerRegistryFromConfig(registry, cfg, logger)
	lr.Reconfigure(ctx, cfg)

	// Assert that all resources' loggers are now configured at DEBUG level (INFO
	// level and DEBUG level logs appear). `LogConfiguration` fields should be
	// honored above `LogConfig` fields.
	//nolint:dupl
	for i, name := range resourceNames {
		// Use index (offset by 4 to account for previous logs) to differentiate
		// logs from different resources.
		infoLogLine := fmt.Sprintf("info-level log line %d", i+4)
		infoLogCmd := map[string]interface{}{"command": "log", "msg": infoLogLine, "level": "info"}
		debugLogLine := fmt.Sprintf("debug-level log line %d", i+4)
		debugLogCmd := map[string]interface{}{"command": "log", "msg": debugLogLine, "level": "debug"}

		res, err := lr.ResourceByName(name)
		test.That(t, err, test.ShouldBeNil)
		_, err = res.DoCommand(ctx, infoLogCmd)
		test.That(t, err, test.ShouldBeNil)
		_, err = res.DoCommand(ctx, debugLogCmd)
		test.That(t, err, test.ShouldBeNil)

		// Each should output both an INFO level log and a DEBUG level log.
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, observer.FilterMessageSnippet(infoLogLine).Len(), test.ShouldEqual, 1)
			test.That(t, observer.FilterMessageSnippet(debugLogLine).Len(), test.ShouldEqual, 1)
		})
	}

	// Mutate config to remove all log configurations.
	cfg.Components[0].LogConfiguration = nil
	cfg.Components[1].LogConfiguration = nil
	cfg.Services[0].LogConfiguration = nil
	cfg.Services[1].LogConfiguration = nil
	cfg.LogConfig = nil

	config.UpdateLoggerRegistryFromConfig(registry, cfg, logger)
	lr.Reconfigure(ctx, cfg)

	// Assert that all resources' loggers are still configured at DEBUG level
	// (INFO level and DEBUG level logs appear). In the absence of any log
	// configuration, log levels should fall back to level of top-level logger
	// (DEBUG in this case, due to using a test logger).
	//nolint:dupl
	for i, name := range resourceNames {
		// Use index (offset by 8 to account for previous logs) to differentiate logs from different resources.
		infoLogLine := fmt.Sprintf("info-level log line %d", i+8)
		infoLogCmd := map[string]interface{}{"command": "log", "msg": infoLogLine, "level": "info"}
		debugLogLine := fmt.Sprintf("debug-level log line %d", i+8)
		debugLogCmd := map[string]interface{}{"command": "log", "msg": debugLogLine, "level": "debug"}

		res, err := lr.ResourceByName(name)
		test.That(t, err, test.ShouldBeNil)
		_, err = res.DoCommand(ctx, infoLogCmd)
		test.That(t, err, test.ShouldBeNil)
		_, err = res.DoCommand(ctx, debugLogCmd)
		test.That(t, err, test.ShouldBeNil)

		// Each should output both an INFO level log and a DEBUG level log.
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(tb, observer.FilterMessageSnippet(infoLogLine).Len(), test.ShouldEqual, 1)
			test.That(t, observer.FilterMessageSnippet(debugLogLine).Len(), test.ShouldEqual, 1)
		})
	}
}

func TestCheckMaintenanceSensorReadings(t *testing.T) {
	logger := logging.NewTestLogger(t)
	t.Run("Sensor reading errors out", func(t *testing.T) {
		r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)
		localRobot := r.(*localRobot)
		canReconfigure, err := localRobot.checkMaintenanceSensorReadings(context.Background(), "", newErrorSensor())

		test.That(t, canReconfigure, test.ShouldEqual, false)
		test.That(t, err.Error(), test.ShouldEqual, "error reading maintenance sensor readings. Wallet not found")
	})
	t.Run("maintenanceAllowedKey does not exist", func(t *testing.T) {
		r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)
		localRobot := r.(*localRobot)
		canReconfigure, err := localRobot.checkMaintenanceSensorReadings(context.Background(), "keyDoesNotExist", newValidSensor())

		test.That(t, canReconfigure, test.ShouldEqual, false)
		test.That(t, err.Error(), test.ShouldEqual, "error getting maintenance_allowed_key keyDoesNotExist from sensor reading")
	})
	t.Run("maintenanceAllowedKey is a number not a boolean", func(t *testing.T) {
		r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)
		localRobot := r.(*localRobot)
		canReconfigure, err := localRobot.checkMaintenanceSensorReadings(context.Background(), "ThatIsNotAWallet", newValidSensor())

		test.That(t, canReconfigure, test.ShouldEqual, false)
		test.That(t, err.Error(), test.ShouldEqual, "maintenance_allowed_key ThatIsNotAWallet is not a bool value")
	})
	t.Run("maintenanceAllowedKey is one not a boolean", func(t *testing.T) {
		r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)
		localRobot := r.(*localRobot)
		canReconfigure, err := localRobot.checkMaintenanceSensorReadings(context.Background(), "OneIsNotTrue", newValidSensor())

		test.That(t, canReconfigure, test.ShouldEqual, false)
		test.That(t, err.Error(), test.ShouldEqual, "maintenance_allowed_key OneIsNotTrue is not a bool value")
	})
	t.Run("maintenanceAllowedKey is string true not a boolean", func(t *testing.T) {
		r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)
		localRobot := r.(*localRobot)
		canReconfigure, err := localRobot.checkMaintenanceSensorReadings(context.Background(), "TrueIsNotTrue", newValidSensor())

		test.That(t, canReconfigure, test.ShouldEqual, false)
		test.That(t, err.Error(), test.ShouldEqual, "maintenance_allowed_key TrueIsNotTrue is not a bool value")
	})
}

func TestCheckMaintenanceSensorReadingsSuccess(t *testing.T) {
	logger := logging.NewTestLogger(t)
	testsValid := []struct {
		testName              string
		canReconfigure        bool
		maintenanceAllowedKey string
		sensor                resource.Sensor
	}{
		{
			testName:              "Sensor returns reading false",
			canReconfigure:        false,
			maintenanceAllowedKey: "ThatsMyWallet",
			sensor:                newValidSensor(),
		},
		{
			testName:              "Sensor returns reading true",
			canReconfigure:        true,
			maintenanceAllowedKey: "ThatsNotMyWallet",
			sensor:                newValidSensor(),
		},
	}
	for _, tc := range testsValid {
		t.Run("", func(t *testing.T) {
			r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)
			localRobot := r.(*localRobot)
			canReconfigure, err := localRobot.checkMaintenanceSensorReadings(context.Background(), tc.maintenanceAllowedKey, tc.sensor)

			test.That(t, canReconfigure, test.ShouldEqual, tc.canReconfigure)
			test.That(t, err, test.ShouldBeNil)
		})
	}
}

func newValidSensor() sensor.Sensor {
	s := &inject.Sensor{}
	s.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		// We want to ensure that we get the same readings after convering to proto and back to go
		readings := map[string]any{
			"ThatsMyWallet": false, "ThatsNotMyWallet": true,
			"ThatIsNotAWallet": 5, "TrueIsNotTrue": "true", "OneIsNotTrue": 1,
		}
		readingsProto, _ := protoutils.ReadingGoToProto(readings)
		retReadings, _ := protoutils.ReadingProtoToGo(readingsProto)
		return retReadings, nil
	}
	s.CloseFunc = func(ctx context.Context) error { return nil }
	return s
}

func newErrorSensor() sensor.Sensor {
	s := &inject.Sensor{}
	s.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return nil, errors.New("Wallet not found")
	}
	return s
}

func newInvalidSensor() sensor.Sensor {
	s := &inject.Sensor{}
	s.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return map[string]any{"ThatsMyWallet": true, "ThatsNotMyWallet": false}, nil
	}
	return s
}

func TestMaintenanceConfig(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	modelErrorSensor := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	resource.RegisterComponent(
		sensor.API,
		model,
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (sensor.Sensor, error) {
			return newValidSensor(), nil
		}})
	resource.RegisterComponent(
		sensor.API,
		modelErrorSensor,
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (sensor.Sensor, error) {
			return newInvalidSensor(), nil
		}})
	defer func() {
		resource.Deregister(sensor.API, model)
		resource.Deregister(sensor.API, modelErrorSensor)
	}()
	remoteCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "sensor",
				Model: model,
				API:   sensor.API,
			},
		},
	}
	sensor1 := []resource.Config{
		{
			Name:  "sensor",
			Model: model,
			API:   sensor.API,
		},
	}
	sensor2 := []resource.Config{
		{
			Name:  "sensor2",
			API:   sensor.API,
			Model: model,
		},
	}
	// This needs to share a name with sensor so name colisions can be tested
	errorSensor := []resource.Config{
		{
			Name:  "sensor",
			API:   sensor.API,
			Model: modelErrorSensor,
		},
	}
	cfgBlocked := &config.Config{
		MaintenanceConfig: &config.MaintenanceConfig{SensorName: "rdk:component:sensor/sensor", MaintenanceAllowedKey: "ThatsMyWallet"},
		Components:        sensor2,
	}

	t.Run("maintenanceConfig sensor blocks reconfigure, reconfigure reenabled when maintenanceConfig removed", func(t *testing.T) {
		cfg := &config.Config{
			MaintenanceConfig: &config.MaintenanceConfig{SensorName: "rdk:component:sensor/sensor", MaintenanceAllowedKey: "ThatsMyWallet"},
			Components:        sensor1,
		}
		cfgUnblock := &config.Config{
			Components: sensor2,
		}

		r := setupLocalRobot(t, context.Background(), cfg, logger)
		sensorResource, err := r.ResourceByName(sensor.Named("sensor"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sensorResource, test.ShouldNotBeNil)

		// Maintenance sensor will block reconfig so sensor2 should not be added
		r.Reconfigure(ctx, cfgBlocked)
		sensorBlocked, err := r.ResourceByName(sensor.Named("sensor2"))
		test.That(t, sensorBlocked, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "resource \"rdk:component:sensor/sensor2\" not found")

		// removing maintenance config unblocks reconfig and allows sensor to be added
		r.Reconfigure(ctx, cfgUnblock)
		sensorBlocked, err = r.ResourceByName(sensor.Named("sensor2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sensorBlocked, test.ShouldNotBeNil)
	})

	t.Run("remote sensor successfully blocks reconfigure when remote name is specified and not specified", func(t *testing.T) {
		ctx := context.Background()
		// Setup remote with maintenance sensor
		remote := setupLocalRobot(t, context.Background(), remoteCfg, logger)
		options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
		err := remote.StartWeb(ctx, options)
		test.That(t, err, test.ShouldBeNil)
		cfg := &config.Config{
			MaintenanceConfig: &config.MaintenanceConfig{SensorName: "rdk:component:sensor/sensor", MaintenanceAllowedKey: "ThatsMyWallet"},
			Remotes: []config.Remote{
				{
					Name:     "remote",
					Insecure: true,
					Address:  addr,
				},
			},
		}
		cfgBlocked := &config.Config{
			MaintenanceConfig: &config.MaintenanceConfig{SensorName: "rdk:component:sensor/sensor", MaintenanceAllowedKey: "ThatsMyWallet"},
			Components:        sensor2,
		}
		cfgBlockedWithRemoteSpecified := &config.Config{
			MaintenanceConfig: &config.MaintenanceConfig{SensorName: "rdk:component:sensor/remote:sensor", MaintenanceAllowedKey: "ThatsMyWallet"},
			Components:        sensor2,
		}

		// Setup robot pointing maintenanceConfig at the remote sensor
		r := setupLocalRobot(t, context.Background(), cfg, logger)

		// reconfig should be blocked ensure new resource is not added
		r.Reconfigure(ctx, cfgBlocked)
		sensorBlocked, err := r.ResourceByName(sensor.Named("sensor2"))
		test.That(t, sensorBlocked, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "resource \"rdk:component:sensor/sensor2\" not found")

		// Attempt to reconfig again using remote:sensor name
		// Reconfig should still be blocked
		r.Reconfigure(ctx, cfgBlockedWithRemoteSpecified)
		sensorBlocked, err = r.ResourceByName(sensor.Named("sensor2"))
		test.That(t, sensorBlocked, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "resource \"rdk:component:sensor/sensor2\" not found")
	})

	t.Run("conflicting remote and main sensor names default to main", func(t *testing.T) {
		ctx := context.Background()
		// Setup remote with error maintenance sensor, if sensor is ever called it will error and reconfigure normally
		remoteErrConfig := &config.Config{
			Components: errorSensor,
		}
		remote := setupLocalRobot(t, context.Background(), remoteErrConfig, logger)
		options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
		err := remote.StartWeb(ctx, options)
		test.That(t, err, test.ShouldBeNil)
		cfg := &config.Config{
			MaintenanceConfig: &config.MaintenanceConfig{SensorName: "rdk:component:sensor/sensor", MaintenanceAllowedKey: "ThatsMyWallet"},
			Remotes: []config.Remote{
				{
					Name:     "remote",
					Insecure: true,
					Address:  addr,
				},
			},
			Components: sensor1,
		}
		cfgRemoteUnblocked := &config.Config{
			MaintenanceConfig: &config.MaintenanceConfig{SensorName: "rdk:component:sensor/remote:sensor", MaintenanceAllowedKey: "ThatsMyWallet"},
			Components:        sensor2,
		}

		// Setup robot pointing maintenanceConfig with conflicting sensors
		r := setupLocalRobot(t, context.Background(), cfg, logger)

		// reconfig should be blocked since the sensor on main if chosen instead of the remote
		r.Reconfigure(ctx, cfgBlocked)
		sensorBlocked, err := r.ResourceByName(sensor.Named("sensor2"))
		test.That(t, sensorBlocked, test.ShouldBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "resource \"rdk:component:sensor/sensor2\" not found")

		// robot should reconfigure since remote will return an error
		r.Reconfigure(ctx, cfgRemoteUnblocked)
		sensorBlocked, err = r.ResourceByName(sensor.Named("sensor2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sensorBlocked, test.ShouldNotBeNil)
	})
	t.Run("multiple remotes with conflicting names errors out", func(t *testing.T) {
		ctx := context.Background()
		//  setup two identical remotes
		remote := setupLocalRobot(t, context.Background(), remoteCfg, logger)
		options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
		err := remote.StartWeb(ctx, options)
		test.That(t, err, test.ShouldBeNil)
		remote2 := setupLocalRobot(t, context.Background(), remoteCfg, logger)
		options2, _, addr2 := robottestutils.CreateBaseOptionsAndListener(t)
		err = remote2.StartWeb(ctx, options2)
		test.That(t, err, test.ShouldBeNil)
		cfg := &config.Config{
			MaintenanceConfig: &config.MaintenanceConfig{SensorName: "rdk:component:sensor/sensor", MaintenanceAllowedKey: "ThatsMyWallet"},
			Remotes: []config.Remote{
				{
					Name:     "remote",
					Insecure: true,
					Address:  addr,
				},
				{
					Name:     "remote2",
					Insecure: true,
					Address:  addr2,
				},
			},
		}

		cfgUnblocked := &config.Config{
			MaintenanceConfig: &config.MaintenanceConfig{SensorName: "rdk:component:sensor/sensor", MaintenanceAllowedKey: "ThatsMyWallet"},
			Components:        sensor2,
		}

		// Setup robot pointing maintenanceConfig with conflicting remote sensors
		r := setupLocalRobot(t, context.Background(), cfg, logger)

		// reconfig should not be blocked because the two remotes will have conflicting resources resulting in reconfiguring
		r.Reconfigure(ctx, cfgUnblocked)
		sensorUnBlocked, err := r.ResourceByName(sensor.Named("sensor2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sensorUnBlocked, test.ShouldNotBeNil)
	})
}

func TestRemovingOfflineRemote(t *testing.T) {
	logger, _ := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	motorName := "remoteMotorFoo"
	motorResourceName := resource.NewName(motor.API, motorName)

	remoteCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:                motorName,
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				API:                 motor.API,
				ConvertedAttributes: &fakemotor.Config{},
			},
		},
	}

	remoteRobot := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("remote"))
	remoteOptions, _, remoteAddr := robottestutils.CreateBaseOptionsAndListener(t)
	err := remoteRobot.StartWeb(ctx, remoteOptions)
	test.That(t, err, test.ShouldBeNil)

	// Set up a local main robot which is connected to the remote.
	mainRobotCfg := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "remote",
				Address: remoteAddr,
				// These values dictate how quickly we'll observe the remote going offline. And how
				// quickly we'll observe it coming back online.
				ConnectionCheckInterval: 10 * time.Millisecond,
				ReconnectInterval:       10 * time.Millisecond,
			},
		},
	}
	mainRobotI := setupLocalRobot(t, ctx, mainRobotCfg, logger.Sublogger("main"))
	// We'll manually access the resource manager to move the test forward.
	mainRobot := mainRobotI.(*localRobot)

	// Grab the RobotClient resource graph node from the main robot that is connected to the
	// remote. We'll use this to know when the main robot observes the remote has gone offline.
	mainToRemoteClientRes, _ := mainRobot.RemoteByName("remote")
	test.That(t, mainToRemoteClientRes, test.ShouldNotBeNil)
	mainToRemoteClient := mainToRemoteClientRes.(*client.RobotClient)
	test.That(t, mainToRemoteClient.Connected(), test.ShouldBeTrue)

	// Stop the remote's web server. Wait for the main robot to observe there's a connection problem.
	logger.Info("Stopping web")
	remoteRobot.StopWeb()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, mainToRemoteClient.Connected(), test.ShouldBeFalse)
	})

	// Reconfigure the main robot with the offline remote removed
	mainRobot.Reconfigure(ctx, &config.Config{})

	// Ensure that the remote has been removed correctly
	findRemote, ok := mainRobot.RemoteByName("remote")
	test.That(t, findRemote, test.ShouldBeEmpty)
	test.That(t, ok, test.ShouldBeFalse)

	// Ensure that motor is removed
	names := mainRobot.ResourceNames()
	test.That(t, names, test.ShouldNotContain, motorResourceName)
}

// TestRemovingOfflineRemotes tests a case where a robot's reconfigure loop is modifying
// a resource graph node at the same time as the complete config loop. In this case the remote is
// marked to be removed by the reconfig loop and then changed to [NodeStateUnhealthy] by the complete config
// loop. This caused the remote that should have been removed to be stay on the robot
// and continue to try and reconnect. We recreate that scenario and ensure that our fix
// prevents that behavior and removes the remote correctly.
func TestRemovingOfflineRemotes(t *testing.T) {
	// Close the robot to stop the background workers from processing any messages to triggerConfig
	r := setupLocalRobot(t, context.Background(), &config.Config{}, logging.NewTestLogger(t), withDisableCompleteConfigWorker())
	localRobot := r.(*localRobot)

	// Create a context that we can cancel to similuate the remote connection timeout
	ctxCompleteConfig, cancelCompleteConfig := context.WithCancel(context.Background())
	defer cancelCompleteConfig()

	// This cancel is used to ensure the goroutine is cleaned up properly after the test
	ctxReconfig, cancelReconfig := context.WithCancel(context.Background())
	defer cancelReconfig()

	// Create a remote graph node and manually add it to the graph
	// This is to avoid calling reconfigure and blocking on trying to connect to the remote
	remoteName := fromRemoteNameToRemoteNodeName("remoteOffline")
	configRemote := config.Remote{
		Name:    "remoteOffline",
		Address: "123.123.123.123",
	}
	configRemote.Validate("")
	node := resource.NewConfiguredGraphNode(
		resource.Config{
			ConvertedAttributes: &configRemote,
		}, nil, builtinModel)
	// Set node to [NodeStateUnhealthy]
	node.LogAndSetLastError(errors.New("Its so bad plz help"))
	localRobot.manager.resources.AddNode(remoteName, node)

	// Set up a wait group to ensure go routines do not leak after test ends
	var wg sync.WaitGroup
	// Spin up the two competing go routines
	wg.Add(1)
	go func() {
		defer wg.Done()
		// manually grab the lock as completeConfig doesn't grab a lock
		localRobot.reconfigurationLock.Lock()
		defer localRobot.reconfigurationLock.Unlock()
		localRobot.manager.completeConfig(ctxCompleteConfig, localRobot, false)
	}()

	// Ensure that complete config grabs the lock
	time.Sleep(1 * time.Second)
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.Reconfigure(ctxReconfig, &config.Config{})
	}()

	// Sleep needed to ensure reconfig is waiting on complete cofig to release the lock
	// and that complete config is hanging on trying to dial the remote
	time.Sleep(2 * time.Second)

	// Ensure that the remote is not marked for removal while trying to connect to the remote
	remote, ok := localRobot.manager.resources.Node(remoteName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, remote.MarkedForRemoval(), test.ShouldBeFalse)

	// Simulate a timeout by canceling the context while trying to connect to the remote
	cancelCompleteConfig()

	// Ensure that the remote is removed from the robot
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		remote, ok := r.RemoteByName("remoteOffline")
		test.That(tb, ok, test.ShouldBeFalse)
		test.That(tb, remote, test.ShouldBeNil)
	})

	// Wait for both goroutines to complete before finishing test
	cancelReconfig()
	wg.Wait()
}
