package robotimpl

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"net"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	// registers all components.
	commonpb "go.viam.com/api/common/v1"
	armpb "go.viam.com/api/component/arm/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
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
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	rgrpc "go.viam.com/rdk/grpc"
	internalcloud "go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/packages"
	putils "go.viam.com/rdk/robot/packages/testutils"
	"go.viam.com/rdk/robot/server"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/builtin"
	genericservice "go.viam.com/rdk/services/generic"
	"go.viam.com/rdk/services/motion"
	motionBuiltin "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/navigation"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/sensors"
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
	pic, _, err := camera.ReadImage(context.Background(), c1)
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

	r := setupLocalRobot(t, ctx, cfg, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	o1 := &spatialmath.R4AA{math.Pi / 2., 0, 0, 1}
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
	r2 := setupLocalRobot(t, ctx2, remoteConfig, logger)

	expected := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
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
		sensors.Named("squee:builtin"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
	}

	resources2 := r2.ResourceNames()

	rtestutils.VerifySameResourceNames(t, resources2, expected)

	expectedRemotes := []string{"squee", "foo", "bar"}
	remotes2 := r2.RemoteNames()

	test.That(
		t, utils.NewStringSet(remotes2...),
		test.ShouldResemble,
		utils.NewStringSet(expectedRemotes...),
	)

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

	statuses, err := r2.Status(
		context.Background(),
		[]resource.Name{
			movementsensor.Named("squee:movement_sensor1"),
			movementsensor.Named("foo:movement_sensor1"),
			movementsensor.Named("bar:movement_sensor1"),
		},
	)
	test.That(t, err, test.ShouldBeNil)

	expectedStatusLength := 3
	test.That(t, len(statuses), test.ShouldEqual, expectedStatusLength)

	for idx := 0; idx < expectedStatusLength; idx++ {
		test.That(t, statuses[idx].Status, test.ShouldResemble, map[string]interface{}{})
		// Assert that last reconfigured values are within last hour (remote
		// recently configured all three resources).
		lr := statuses[idx].LastReconfigured
		test.That(t, lr, test.ShouldHappenBetween,
			time.Now().Add(-1*time.Hour), time.Now())
	}

	statuses, err = r2.Status(
		context.Background(),
		[]resource.Name{arm.Named("squee:pieceArm"), arm.Named("foo:pieceArm"), arm.Named("bar:pieceArm")},
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 3)

	armStatus := &armpb.Status{
		EndPosition:    &commonpb.Pose{X: 500, Z: 300, OZ: 1},
		JointPositions: &armpb.JointPositions{Values: []float64{0.0}},
	}
	convMap := &armpb.Status{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(statuses[0].Status)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, armStatus)

	convMap = &armpb.Status{}
	decoder, err = mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(statuses[1].Status)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, armStatus)

	convMap = &armpb.Status{}
	decoder, err = mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(statuses[2].Status)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, armStatus)

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
			apiKey := "sosecret"
			locationSecret := "locsosecret"

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

				remoteConfig.Remotes[0].Auth.Entity = entityName
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

				ctx2 := context.Background()
				remoteConfig.Remotes[0].Address = options.LocalFQDN
				r2 = setupLocalRobot(t, ctx2, remoteConfig, logger)

				_, err = r2.ResourceByName(motion.Named(resource.DefaultServiceName))
				test.That(t, err, test.ShouldBeNil)
			}

			test.That(t, r2, test.ShouldNotBeNil)

			expected := []resource.Name{
				motion.Named(resource.DefaultServiceName),
				sensors.Named(resource.DefaultServiceName),
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
				sensors.Named("foo:builtin"),
				motion.Named("bar:builtin"),
				sensors.Named("bar:builtin"),
			}

			resources2 := r2.ResourceNames()

			rtestutils.VerifySameResourceNames(t, resources2, expected)

			remotes2 := r2.RemoteNames()
			expectedRemotes := []string{"bar", "foo"}

			test.That(
				t, utils.NewStringSet(remotes2...),
				test.ShouldResemble,
				utils.NewStringSet(expectedRemotes...),
			)

			statuses, err := r2.Status(
				context.Background(), []resource.Name{movementsensor.Named("bar:movement_sensor1"), movementsensor.Named("foo:movement_sensor1")},
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(statuses), test.ShouldEqual, 2)
			test.That(t, statuses[0].Status, test.ShouldResemble, map[string]interface{}{})
			test.That(t, statuses[1].Status, test.ShouldResemble, map[string]interface{}{})

			statuses, err = r2.Status(
				context.Background(), []resource.Name{arm.Named("bar:pieceArm"), arm.Named("foo:pieceArm")},
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(statuses), test.ShouldEqual, 2)

			armStatus := &armpb.Status{
				EndPosition:    &commonpb.Pose{X: 500, Z: 300, OZ: 1},
				JointPositions: &armpb.JointPositions{Values: []float64{0.0}},
			}
			convMap := &armpb.Status{}
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
			test.That(t, err, test.ShouldBeNil)
			err = decoder.Decode(statuses[0].Status)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, convMap, test.ShouldResemble, armStatus)

			convMap = &armpb.Status{}
			decoder, err = mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
			test.That(t, err, test.ShouldBeNil)
			err = decoder.Decode(statuses[1].Status)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, convMap, test.ShouldResemble, armStatus)
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

	// use cert
	remoteTLSConfig.Certificates = []tls.Certificate{cert}
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
		sensors.Named(resource.DefaultServiceName),
		arm.Named("foo:pieceArm"),
		audioinput.Named("foo:mic1"),
		camera.Named("foo:cameraOver"),
		movementsensor.Named("foo:movement_sensor1"),
		movementsensor.Named("foo:movement_sensor2"),
		gripper.Named("foo:pieceGripper"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
	}

	resources2 := r2.ResourceNames()

	rtestutils.VerifySameResourceNames(t, resources2, expected)

	remotes2 := r2.RemoteNames()
	expectedRemotes := []string{"foo"}

	test.That(
		t, utils.NewStringSet(remotes2...),
		test.ShouldResemble,
		utils.NewStringSet(expectedRemotes...),
	)

	statuses, err := r2.Status(context.Background(), []resource.Name{movementsensor.Named("foo:movement_sensor1")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 1)
	test.That(t, statuses[0].Status, test.ShouldResemble, map[string]interface{}{})

	statuses, err = r2.Status(context.Background(), []resource.Name{arm.Named("foo:pieceArm")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 1)

	armStatus := &armpb.Status{
		EndPosition:    &commonpb.Pose{X: 500, Z: 300, OZ: 1},
		JointPositions: &armpb.JointPositions{Values: []float64{0.0}},
	}
	convMap := &armpb.Status{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(statuses[0].Status)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, armStatus)
}

type dummyArm struct {
	arm.Arm
	stopCount int
	extra     map[string]interface{}
	channel   chan struct{}
}

func (da *dummyArm) Name() resource.Name {
	return arm.Named("bad")
}

func (da *dummyArm) MoveToPosition(
	ctx context.Context,
	pose spatialmath.Pose,
	extra map[string]interface{},
) error {
	return nil
}

func (da *dummyArm) MoveToJointPositions(ctx context.Context, positionDegs *armpb.JointPositions, extra map[string]interface{}) error {
	return nil
}

func (da *dummyArm) JointPositions(ctx context.Context, extra map[string]interface{}) (*armpb.JointPositions, error) {
	return nil, errors.New("fake error")
}

func (da *dummyArm) Stop(ctx context.Context, extra map[string]interface{}) error {
	da.stopCount++
	da.extra = extra
	return nil
}

func (da *dummyArm) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	close(da.channel)
	<-ctx.Done()
	return nil, ctx.Err()
}

func (da *dummyArm) Close(ctx context.Context) error {
	return nil
}

func TestStopAll(t *testing.T) {
	logger := logging.NewTestLogger(t)
	channel := make(chan struct{})

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	dummyArm1 := dummyArm{channel: channel}
	dummyArm2 := dummyArm{channel: channel}
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
				return &dummyArm1, nil
			}
			return &dummyArm2, nil
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

	test.That(t, dummyArm1.stopCount, test.ShouldEqual, 0)
	test.That(t, dummyArm2.stopCount, test.ShouldEqual, 0)

	test.That(t, dummyArm1.extra, test.ShouldBeNil)
	test.That(t, dummyArm2.extra, test.ShouldBeNil)

	err = r.StopAll(ctx, map[resource.Name]map[string]interface{}{arm.Named("arm2"): {"foo": "bar"}})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, dummyArm1.stopCount, test.ShouldEqual, 1)
	test.That(t, dummyArm2.stopCount, test.ShouldEqual, 1)

	test.That(t, dummyArm1.extra, test.ShouldBeNil)
	test.That(t, dummyArm2.extra, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

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

type dummyBoard struct {
	board.Board
	closeCount int
}

func (db *dummyBoard) Name() resource.Name {
	return board.Named("bad")
}

func (db *dummyBoard) AnalogNames() []string {
	return nil
}

func (db *dummyBoard) DigitalInterruptNames() []string {
	return nil
}

func (db *dummyBoard) Close(ctx context.Context) error {
	db.closeCount++
	return nil
}

func TestNewTeardown(t *testing.T) {
	logger := logging.NewTestLogger(t)

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	var dummyBoard1 dummyBoard
	resource.RegisterComponent(
		board.API,
		model,
		resource.Registration[board.Board, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (board.Board, error) {
			return &dummyBoard1, nil
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
	test.That(t, dummyBoard1.closeCount, test.ShouldEqual, 1)
}

func TestMetadataUpdate(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r := setupLocalRobot(t, ctx, cfg, logger)

	resources := r.ResourceNames()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(resources), test.ShouldEqual, 8)
	test.That(t, err, test.ShouldBeNil)

	// 5 declared resources + default sensors
	resourceNames := []resource.Name{
		arm.Named("pieceArm"),
		audioinput.Named("mic1"),
		camera.Named("cameraOver"),
		gripper.Named("pieceGripper"),
		movementsensor.Named("movement_sensor1"),
		movementsensor.Named("movement_sensor2"),
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
	}

	resources = r.ResourceNames()
	test.That(t, len(resources), test.ShouldEqual, len(resourceNames))
	rtestutils.VerifySameResourceNames(t, resources, resourceNames)

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	resources = r.ResourceNames()
	test.That(t, resources, test.ShouldBeEmpty)
}

func TestSensorsService(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r := setupLocalRobot(t, context.Background(), cfg, logger)

	svc, err := sensors.FromRobot(r, resource.DefaultServiceName)
	test.That(t, err, test.ShouldBeNil)

	sensorNames := []resource.Name{movementsensor.Named("movement_sensor1"), movementsensor.Named("movement_sensor2")}
	foundSensors, err := svc.Sensors(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	rtestutils.VerifySameResourceNames(t, foundSensors, sensorNames)

	readings, err := svc.Readings(context.Background(), []resource.Name{movementsensor.Named("movement_sensor1")}, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(readings), test.ShouldEqual, 1)
	test.That(t, readings[0].Name, test.ShouldResemble, movementsensor.Named("movement_sensor1"))
	test.That(t, len(readings[0].Readings), test.ShouldBeGreaterThan, 3)

	readings, err = svc.Readings(context.Background(), sensorNames, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(readings), test.ShouldEqual, 2)
}

func TestStatusService(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r := setupLocalRobot(t, context.Background(), cfg, logger)

	resourceNames := []resource.Name{arm.Named("pieceArm"), movementsensor.Named("movement_sensor1")}
	rArm, err := arm.FromRobot(r, "pieceArm")
	test.That(t, err, test.ShouldBeNil)
	armStatus, err := arm.CreateStatus(context.Background(), rArm)
	test.That(t, err, test.ShouldBeNil)
	expected := map[resource.Name]interface{}{
		arm.Named("pieceArm"):                    armStatus,
		movementsensor.Named("movement_sensor1"): map[string]interface{}{},
	}

	statuses, err := r.Status(context.Background(), []resource.Name{movementsensor.Named("movement_sensor1")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 1)
	test.That(t, statuses[0].Name, test.ShouldResemble, movementsensor.Named("movement_sensor1"))
	test.That(t, statuses[0].Status, test.ShouldResemble, expected[statuses[0].Name])

	statuses, err = r.Status(context.Background(), resourceNames)
	test.That(t, err, test.ShouldBeNil)

	expectedStatusLength := 2
	test.That(t, len(statuses), test.ShouldEqual, expectedStatusLength)

	for idx := 0; idx < expectedStatusLength; idx++ {
		test.That(t, statuses[idx].Status, test.ShouldResemble, expected[statuses[idx].Name])
	}
}

func TestStatus(t *testing.T) {
	buttonAPI := resource.APINamespace("acme").WithComponentType("button")
	button1 := resource.NewName(buttonAPI, "button1")
	button2 := resource.NewName(buttonAPI, "button2")

	workingAPI := resource.APINamespace("acme").WithComponentType("working")
	working1 := resource.NewName(workingAPI, "working1")

	failAPI := resource.APINamespace("acme").WithComponentType("fail")
	fail1 := resource.NewName(failAPI, "fail1")

	workingStatus := map[string]interface{}{"position": "up"}
	errFailed := errors.New("can't get status")

	resource.RegisterAPI(
		workingAPI,
		resource.APIRegistration[resource.Resource]{
			Status: func(ctx context.Context, res resource.Resource) (interface{}, error) { return workingStatus, nil },
		},
	)

	resource.RegisterAPI(
		failAPI,
		resource.APIRegistration[resource.Resource]{
			Status: func(ctx context.Context, res resource.Resource) (interface{}, error) { return nil, errFailed },
		},
	)
	defer func() {
		resource.DeregisterAPI(workingAPI)
		resource.DeregisterAPI(failAPI)
	}()

	expectedRobotStatus := robot.Status{Name: button1, Status: map[string]interface{}{}}
	logger := logging.NewTestLogger(t)
	resourceNames := []resource.Name{working1, button1, fail1}
	resourceMap := map[resource.Name]resource.Resource{
		working1: rtestutils.NewUnimplementedResource(working1),
		button1:  rtestutils.NewUnimplementedResource(button1),
		fail1:    rtestutils.NewUnimplementedResource(fail1),
	}

	t.Run("not found", func(t *testing.T) {
		r, err := RobotFromResources(context.Background(), resourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()

		test.That(t, err, test.ShouldBeNil)

		_, err = r.Status(context.Background(), []resource.Name{button2})
		test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(button2))
	})

	t.Run("no CreateStatus", func(t *testing.T) {
		r, err := RobotFromResources(context.Background(), resourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, err, test.ShouldBeNil)

		resp, err := r.Status(context.Background(), []resource.Name{button1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp[0].Name, test.ShouldResemble, expectedRobotStatus.Name)
		test.That(t, resp[0].Status, test.ShouldResemble, expectedRobotStatus.Status)
		test.That(t, resp[0].LastReconfigured, test.ShouldHappenBetween,
			time.Now().Add(-1*time.Hour), time.Now())
	})

	t.Run("failing resource", func(t *testing.T) {
		r, err := RobotFromResources(context.Background(), resourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, err, test.ShouldBeNil)

		_, err = r.Status(context.Background(), []resource.Name{fail1})
		test.That(t, err, test.ShouldBeError, errors.Wrapf(errFailed, "failed to get status from %q", fail1))
	})

	t.Run("many status", func(t *testing.T) {
		expected := map[resource.Name]interface{}{
			working1: workingStatus,
			button1:  map[string]interface{}{},
		}
		r, err := RobotFromResources(context.Background(), resourceMap, logger)
		test.That(t, err, test.ShouldBeNil)

		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()
		_, err = r.Status(context.Background(), []resource.Name{button2})
		test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(button2))

		resp, err := r.Status(context.Background(), []resource.Name{working1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		status := resp[0]
		test.That(t, status.Name, test.ShouldResemble, working1)
		test.That(t, status.Status, test.ShouldResemble, workingStatus)

		resp, err = r.Status(context.Background(), []resource.Name{working1, working1, working1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		status = resp[0]
		test.That(t, status.Name, test.ShouldResemble, working1)
		test.That(t, status.Status, test.ShouldResemble, workingStatus)

		resp, err = r.Status(context.Background(), []resource.Name{working1, button1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)
		test.That(t, resp[0].Status, test.ShouldResemble, expected[resp[0].Name])
		test.That(t, resp[1].Status, test.ShouldResemble, expected[resp[1].Name])

		_, err = r.Status(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeError, errors.Wrapf(errFailed, "failed to get status from %q", fail1))
	})

	t.Run("get all status", func(t *testing.T) {
		workingResourceMap := map[resource.Name]resource.Resource{
			working1: rtestutils.NewUnimplementedResource(working1),
			button1:  rtestutils.NewUnimplementedResource(button1),
		}
		expected := map[resource.Name]interface{}{
			working1: workingStatus,
			button1:  map[string]interface{}{},
		}
		r, err := RobotFromResources(context.Background(), workingResourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, err, test.ShouldBeNil)

		resp, err := r.Status(context.Background(), []resource.Name{})
		test.That(t, err, test.ShouldBeNil)
		// 5 because the 3 default services are always added to a local_robot. We only care
		// about the first two (working1 and button1) however.
		test.That(t, len(resp), test.ShouldEqual, 4)

		// although the response is length 5, the only thing we actually care about for testing
		// is consistency with the expected values in the workingResourceMap. So we eliminate
		// the values that aren't in the workingResourceMap.
		actual := []robot.Status{}
		for _, status := range resp {
			if _, ok := workingResourceMap[status.Name]; ok {
				actual = append(actual, status)
			}
		}
		test.That(t, len(actual), test.ShouldEqual, 2)
		test.That(t, actual[0].Status, test.ShouldResemble, expected[actual[0].Name])
		test.That(t, actual[1].Status, test.ShouldResemble, expected[actual[1].Name])
	})
}

func TestStatusRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)
	// set up remotes
	listener1 := testutils.ReserveRandomListener(t)
	addr1 := listener1.Addr().String()

	listener2 := testutils.ReserveRandomListener(t)
	addr2 := listener2.Addr().String()

	gServer1 := grpc.NewServer()
	gServer2 := grpc.NewServer()
	resourcesFunc := func() []resource.Name { return []resource.Name{arm.Named("arm1"), arm.Named("arm2")} }
	statusCallCount := 0

	// TODO: RSDK-882 will update this so that this is not necessary
	frameSystemConfigFunc := func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: []*referenceframe.FrameSystemPart{
			{
				FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, nil, "arm1", nil),
				ModelFrame:  referenceframe.NewSimpleModel("arm1"),
			},
			{
				FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, nil, "arm2", nil),
				ModelFrame:  referenceframe.NewSimpleModel("arm2"),
			},
		}}, nil
	}

	injectRobot1 := &inject.Robot{
		FrameSystemConfigFunc: frameSystemConfigFunc,
		ResourceNamesFunc:     resourcesFunc,
		ResourceRPCAPIsFunc:   func() []resource.RPCAPI { return nil },
	}
	armStatus := &armpb.Status{
		EndPosition:    &commonpb.Pose{},
		JointPositions: &armpb.JointPositions{Values: []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0}},
	}

	lastReconfigured, err := time.Parse("2006-01-02 15:04:05", "2011-11-11 00:00:00")
	test.That(t, err, test.ShouldBeNil)

	injectRobot1.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
		statusCallCount++
		statuses := make([]robot.Status, 0, len(resourceNames))
		for _, n := range resourceNames {
			statuses = append(statuses, robot.Status{
				Name:             n,
				LastReconfigured: lastReconfigured,
				Status:           armStatus,
			})
		}
		return statuses, nil
	}
	injectRobot2 := &inject.Robot{
		FrameSystemConfigFunc: frameSystemConfigFunc,
		ResourceNamesFunc:     resourcesFunc,
		ResourceRPCAPIsFunc:   func() []resource.RPCAPI { return nil },
	}
	injectRobot2.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
		statusCallCount++
		statuses := make([]robot.Status, 0, len(resourceNames))
		for _, n := range resourceNames {
			statuses = append(statuses, robot.Status{
				Name:             n,
				LastReconfigured: lastReconfigured,
				Status:           armStatus,
			})
		}
		return statuses, nil
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot2))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	remoteConfig := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
			{
				Name:    "bar",
				Address: addr2,
			},
		},
	}
	test.That(t, remoteConfig.Ensure(false, logger), test.ShouldBeNil)
	ctx := context.Background()
	r := setupLocalRobot(t, ctx, remoteConfig, logger)

	rtestutils.VerifySameResourceNames(
		t,
		r.ResourceNames(),
		[]resource.Name{
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			arm.Named("foo:arm1"),
			arm.Named("foo:arm2"),
			arm.Named("bar:arm1"),
			arm.Named("bar:arm2"),
		},
	)
	statuses, err := r.Status(
		ctx, []resource.Name{arm.Named("foo:arm1"), arm.Named("foo:arm2"), arm.Named("bar:arm1"), arm.Named("bar:arm2")},
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 4)
	test.That(t, statusCallCount, test.ShouldEqual, 2)

	for _, status := range statuses {
		convMap := &armpb.Status{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
		test.That(t, err, test.ShouldBeNil)
		err = decoder.Decode(status.Status)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, convMap, test.ShouldResemble, armStatus)

		// Test that LastReconfigured values are from remotes, and not set based on
		// when local resource graph nodes were added for the remote resources.
		test.That(t, status.LastReconfigured, test.ShouldEqual, lastReconfigured)
	}
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
	tPos := referenceframe.JointPositionsFromRadians([]float64{math.Pi})
	err = r0Arm.MoveToJointPositions(context.Background(), tPos, nil)
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
			sensors.Named(resource.DefaultServiceName),
			arm.Named("remote:foo:arm1"), arm.Named("remote:foo:arm2"),
			arm.Named("remote:pieceArm"),
			arm.Named("remote:foo:pieceArm"),
			audioinput.Named("remote:mic1"),
			camera.Named("remote:cameraOver"),
			movementsensor.Named("remote:movement_sensor1"),
			movementsensor.Named("remote:movement_sensor2"),
			gripper.Named("remote:pieceGripper"),
			motion.Named("remote:builtin"),
			sensors.Named("remote:builtin"),
			motion.Named("remote:foo:builtin"),
			sensors.Named("remote:foo:builtin"),
		},
	)
	arm1, err := r.ResourceByName(arm.Named("remote:foo:arm1"))
	test.That(t, err, test.ShouldBeNil)
	rrArm1, ok := arm1.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	pos, err := rrArm1.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.Values, test.ShouldResemble, p0Arm1.Values)

	arm1, err = r.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	rrArm1, ok = arm1.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	pos, err = rrArm1.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.Values, test.ShouldResemble, p0Arm1.Values)

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

	// Assert that Config method returns the two default services: motion and sensors.
	actualCfg := r.Config()
	defaultSvcs := removeDefaultServices(actualCfg)
	test.That(t, len(defaultSvcs), test.ShouldEqual, 2)
	for _, svc := range defaultSvcs {
		test.That(t, svc.API.SubtypeName, test.ShouldBeIn,
			motion.API.SubtypeName, sensors.API.SubtypeName)
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
	test.That(t, len(defaultSvcs), test.ShouldEqual, 2)
	for _, svc := range defaultSvcs {
		test.That(t, svc.API.SubtypeName, test.ShouldBeIn, motion.API.SubtypeName,
			sensors.API.SubtypeName)
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

func TestReconnectRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	// start the first robot
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

	robot := setupLocalRobot(t, ctx, &cfg, logger)
	err := robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// start the second robot
	ctx1 := context.Background()
	options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)

	remoteConf := config.Remote{
		Name:     "remote",
		Insecure: true,
		Address:  addr,
	}

	cfg1 := config.Config{
		Remotes: []config.Remote{remoteConf},
	}

	robot1 := setupLocalRobot(t, ctx, &cfg1, logger)

	err = robot1.StartWeb(ctx1, options1)
	test.That(t, err, test.ShouldBeNil)

	robotClient := robottestutils.NewRobotClient(t, logger, addr1, time.Second)
	defer func() {
		test.That(t, robotClient.Close(context.Background()), test.ShouldBeNil)
	}()

	a1, err := arm.FromRobot(robot1, "arm1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, a1, test.ShouldNotBeNil)

	remoteRobot, ok := robot1.RemoteByName("remote")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, remoteRobot, test.ShouldNotBeNil)
	remoteRobotClient, ok := remoteRobot.(*client.RobotClient)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, remoteRobotClient, test.ShouldNotBeNil)

	a, err := robotClient.ResourceByName(arm.Named("remote:arm1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, a, test.ShouldNotBeNil)
	anArm, ok := a.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	_, err = anArm.EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	// close/disconnect the robot
	robot.StopWeb()
	test.That(t, <-remoteRobotClient.Changed(), test.ShouldBeTrue)
	test.That(t, len(remoteRobotClient.ResourceNames()), test.ShouldEqual, 0)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, len(robotClient.ResourceNames()), test.ShouldEqual, 2)
	})
	test.That(t, len(robot1.ResourceNames()), test.ShouldEqual, 2)
	_, err = anArm.EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeError)

	// reconnect the first robot
	ctx2 := context.Background()
	listener, err := net.Listen("tcp", addr)
	test.That(t, err, test.ShouldBeNil)

	options.Network.Listener = listener
	err = robot.StartWeb(ctx2, options)
	test.That(t, err, test.ShouldBeNil)

	// check if the original arm can still be called
	test.That(t, <-remoteRobotClient.Changed(), test.ShouldBeTrue)
	test.That(t, remoteRobotClient.Connected(), test.ShouldBeTrue)
	test.That(t, len(remoteRobotClient.ResourceNames()), test.ShouldEqual, 3)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, len(robotClient.ResourceNames()), test.ShouldEqual, 5)
	})
	test.That(t, len(robot1.ResourceNames()), test.ShouldEqual, 5)
	_, err = remoteRobotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)

	_, err = robotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = anArm.EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
}

func TestReconnectRemoteChangeConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// start the first robot
	ctx := context.Background()
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
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

	robot := setupLocalRobot(t, ctx, &cfg, logger)
	err := robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// start the second robot
	ctx1 := context.Background()
	options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	remoteConf := config.Remote{
		Name:     "remote",
		Insecure: true,
		Address:  addr,
	}

	cfg1 := config.Config{
		Remotes: []config.Remote{remoteConf},
	}

	robot1 := setupLocalRobot(t, ctx, &cfg1, logger)

	err = robot1.StartWeb(ctx1, options1)
	test.That(t, err, test.ShouldBeNil)

	robotClient := robottestutils.NewRobotClient(t, logger, addr1, time.Second)
	defer func() {
		test.That(t, robotClient.Close(context.Background()), test.ShouldBeNil)
	}()

	a1, err := arm.FromRobot(robot1, "arm1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, a1, test.ShouldNotBeNil)

	remoteRobot, ok := robot1.RemoteByName("remote")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, remoteRobot, test.ShouldNotBeNil)
	remoteRobotClient, ok := remoteRobot.(*client.RobotClient)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, remoteRobotClient, test.ShouldNotBeNil)

	a, err := robotClient.ResourceByName(arm.Named("remote:arm1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, a, test.ShouldNotBeNil)
	anArm, ok := a.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	_, err = anArm.EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	// close/disconnect the robot
	test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
	test.That(t, <-remoteRobotClient.Changed(), test.ShouldBeTrue)
	test.That(t, len(remoteRobotClient.ResourceNames()), test.ShouldEqual, 0)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, len(robotClient.ResourceNames()), test.ShouldEqual, 2)
	})
	test.That(t, len(robot1.ResourceNames()), test.ShouldEqual, 2)
	_, err = anArm.EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeError)

	// reconnect the first robot
	ctx2 := context.Background()
	listener, err := net.Listen("tcp", addr)
	test.That(t, err, test.ShouldBeNil)
	baseConfig := resource.Config{
		Name:  "base1",
		API:   base.API,
		Model: fakeModel,
	}
	cfg = config.Config{
		Components: []resource.Config{baseConfig},
	}

	options = weboptions.New()
	options.Network.BindAddress = ""
	options.Network.Listener = listener
	robot = setupLocalRobot(t, ctx, &cfg, logger)
	err = robot.StartWeb(ctx2, options)
	test.That(t, err, test.ShouldBeNil)

	// check if the original arm can't be called anymore
	test.That(t, <-remoteRobotClient.Changed(), test.ShouldBeTrue)
	test.That(t, remoteRobotClient.Connected(), test.ShouldBeTrue)
	test.That(t, len(remoteRobotClient.ResourceNames()), test.ShouldEqual, 3)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, len(robotClient.ResourceNames()), test.ShouldEqual, 5)
	})
	test.That(t, len(robot1.ResourceNames()), test.ShouldEqual, 5)
	_, err = anArm.EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeError)

	// check that base is now instantiated
	_, err = remoteRobotClient.ResourceByName(base.Named("base1"))
	test.That(t, err, test.ShouldBeNil)

	b, err := robotClient.ResourceByName(base.Named("remote:base1"))
	test.That(t, err, test.ShouldBeNil)
	aBase, ok := b.(base.Base)
	test.That(t, ok, test.ShouldBeTrue)

	err = aBase.Stop(ctx, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(robotClient.ResourceNames()), test.ShouldEqual, 5)
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
						return nil, errors.Errorf("resource %s is not a gizmo", rName.Name)
					}
					newDoodad.gizmo = gizmo
				}
			}
			if newDoodad.gizmo == nil {
				return nil, errors.Errorf("doodad %s must depend on a gizmo", conf.Name)
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
