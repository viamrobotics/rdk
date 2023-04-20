package robotimpl_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"net"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
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

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/motor"
	fakemotor "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/packages"
	putils "go.viam.com/rdk/robot/packages/testutils"
	"go.viam.com/rdk/robot/server"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/builtin"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/navigation"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

var fakeModel = resource.NewDefaultModel("fake")

func TestConfig1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/cfgtest1.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	c1, err := camera.FromRobot(r, "c1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c1.Name(), test.ShouldResemble, camera.Named("c1"))
	pic, _, err := camera.ReadImage(context.Background(), c1)
	test.That(t, err, test.ShouldBeNil)

	bounds := pic.Bounds()

	test.That(t, bounds.Max.X, test.ShouldBeGreaterThanOrEqualTo, 32)
}

func TestConfigFake(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

// this serves as a test for updateWeakDependents as the web service defines a weak
// dependency on all resources.
func TestConfigRemote(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

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
				API:   base.Subtype,
				Model: fakeModel,
				Frame: &referenceframe.LinkConfig{
					Parent: referenceframe.World,
				},
			},
			{
				Name:  "myParentIsRemote",
				API:   base.Subtype,
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
	r2, err := robotimpl.New(ctx2, remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)

	expected := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		vision.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		datamanager.Named(resource.DefaultServiceName),
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
		vision.Named("squee:builtin"),
		sensors.Named("squee:builtin"),
		datamanager.Named("squee:builtin"),
		motion.Named("foo:builtin"),
		vision.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		datamanager.Named("foo:builtin"),
		motion.Named("bar:builtin"),
		vision.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
		datamanager.Named("bar:builtin"),
	}

	resources2 := r2.ResourceNames()

	test.That(
		t,
		rtestutils.NewResourceNameSet(resources2...),
		test.ShouldResemble,
		rtestutils.NewResourceNameSet(expected...),
	)

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

	cfg2, err := r2.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cfg2.Components), test.ShouldEqual, 2)

	fsConfig, err := r2.FrameSystemConfig(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fsConfig, test.ShouldHaveLength, 12)

	test.That(t, r2.Close(context.Background()), test.ShouldBeNil)
}

func TestConfigRemoteWithAuth(t *testing.T) {
	logger := golog.NewTestLogger(t)
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
			r, err := robotimpl.New(ctx, cfg, logger)
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				test.That(t, r.Close(context.Background()), test.ShouldBeNil)
			}()

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

			remoteRobot, err := robotimpl.New(context.Background(), remoteConfig, logger)
			defer func() {
				test.That(t, remoteRobot.Close(context.Background()), test.ShouldBeNil)
			}()
			test.That(t, err, test.ShouldBeNil)

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
				managedRemote, err := robotimpl.New(context.Background(), remoteConfig, logger)
				defer func() {
					test.That(t, managedRemote.Close(context.Background()), test.ShouldBeNil)
				}()
				test.That(t, err, test.ShouldBeNil)

				remoteConfig.AllowInsecureCreds = true

				r3, err := robotimpl.New(context.Background(), remoteConfig, logger)
				defer func() {
					test.That(t, r3.Close(context.Background()), test.ShouldBeNil)
				}()
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r3, test.ShouldNotBeNil)
				remoteBot, ok := r3.RemoteByName("foo")
				test.That(t, ok, test.ShouldBeFalse)
				test.That(t, remoteBot, test.ShouldBeNil)

				remoteConfig.Remotes[0].Auth.Entity = entityName
				remoteConfig.Remotes[1].Auth.Entity = entityName
				r2, err = robotimpl.New(context.Background(), remoteConfig, logger)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r2.Close(context.Background()), test.ShouldBeNil)

				test.That(t, err, test.ShouldBeNil)
				ctx2 := context.Background()
				remoteConfig.Remotes[0].Address = options.LocalFQDN
				if tc.EntityName != "" {
					remoteConfig.Remotes[1].Address = options.FQDN
				}
				r2, err = robotimpl.New(ctx2, remoteConfig, logger)
				test.That(t, err, test.ShouldBeNil)
			} else {
				unmanagedRobot, err := robotimpl.New(context.Background(), remoteConfig, logger)
				test.That(t, err, test.ShouldBeNil)
				defer func() {
					test.That(t, unmanagedRobot.Close(context.Background()), test.ShouldBeNil)
				}()

				remoteConfig.AllowInsecureCreds = true

				r2, err = robotimpl.New(context.Background(), remoteConfig, logger)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r2.Close(context.Background()), test.ShouldBeNil)

				test.That(t, err, test.ShouldBeNil)
				ctx2 := context.Background()
				remoteConfig.Remotes[0].Address = options.LocalFQDN
				r2, err = robotimpl.New(ctx2, remoteConfig, logger)
				test.That(t, err, test.ShouldBeNil)

				_, err = r2.ResourceByName(motion.Named(resource.DefaultServiceName))
				test.That(t, err, test.ShouldBeNil)
			}

			test.That(t, r2, test.ShouldNotBeNil)

			expected := []resource.Name{
				motion.Named(resource.DefaultServiceName),
				vision.Named(resource.DefaultServiceName),
				sensors.Named(resource.DefaultServiceName),
				datamanager.Named(resource.DefaultServiceName),
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
				vision.Named("foo:builtin"),
				sensors.Named("foo:builtin"),
				datamanager.Named("foo:builtin"),
				motion.Named("bar:builtin"),
				vision.Named("bar:builtin"),
				sensors.Named("bar:builtin"),
				datamanager.Named("bar:builtin"),
			}

			resources2 := r2.ResourceNames()

			test.That(
				t,
				rtestutils.NewResourceNameSet(resources2...),
				test.ShouldResemble,
				rtestutils.NewResourceNameSet(expected...),
			)

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

			test.That(t, r2.Close(context.Background()), test.ShouldBeNil)
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		})
	}
}

func TestConfigRemoteWithTLSAuth(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	altName := primitive.NewObjectID().Hex()
	cert, _, _, certPool, err := testutils.GenerateSelfSignedCertificate("somename", altName)
	test.That(t, err, test.ShouldBeNil)

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

	_r, err := robotimpl.New(context.Background(), remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, _r.Close(context.Background()), test.ShouldBeNil)
	}()

	// use secret
	remoteConfig.Remotes[0].Auth.Credentials = &rpc.Credentials{
		Type:    rutils.CredentialsTypeRobotLocationSecret,
		Payload: locationSecret,
	}
	r2, err := robotimpl.New(context.Background(), remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r2.Close(context.Background()), test.ShouldBeNil)

	// use cert
	remoteTLSConfig.Certificates = []tls.Certificate{cert}
	r2, err = robotimpl.New(context.Background(), remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r2.Close(context.Background()), test.ShouldBeNil)

	// use cert with mDNS
	remoteConfig.Remotes[0].Address = options.FQDN
	r2, err = robotimpl.New(context.Background(), remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r2.Close(context.Background()), test.ShouldBeNil)

	// use signaling creds
	remoteConfig.Remotes[0].Address = addr
	remoteConfig.Remotes[0].Auth.Credentials = nil
	remoteConfig.Remotes[0].Auth.SignalingServerAddress = addr
	remoteConfig.Remotes[0].Auth.SignalingAuthEntity = options.FQDN
	remoteConfig.Remotes[0].Auth.SignalingCreds = &rpc.Credentials{
		Type:    rutils.CredentialsTypeRobotLocationSecret,
		Payload: locationSecret,
	}
	r2, err = robotimpl.New(context.Background(), remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r2.Close(context.Background()), test.ShouldBeNil)

	// use cert with mDNS while signaling present
	test.That(t, err, test.ShouldBeNil)
	ctx2 := context.Background()
	remoteConfig.Remotes[0].Auth.SignalingCreds = &rpc.Credentials{
		Type:    rutils.CredentialsTypeRobotLocationSecret,
		Payload: locationSecret + "bad",
	}
	remoteConfig.Remotes[0].Address = options.FQDN
	r2, err = robotimpl.New(ctx2, remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)

	expected := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		vision.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		datamanager.Named(resource.DefaultServiceName),
		arm.Named("foo:pieceArm"),
		audioinput.Named("foo:mic1"),
		camera.Named("foo:cameraOver"),
		movementsensor.Named("foo:movement_sensor1"),
		movementsensor.Named("foo:movement_sensor2"),
		gripper.Named("foo:pieceGripper"),
		motion.Named("foo:builtin"),
		vision.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		datamanager.Named("foo:builtin"),
	}

	resources2 := r2.ResourceNames()

	test.That(
		t,
		rtestutils.NewResourceNameSet(resources2...),
		test.ShouldResemble,
		rtestutils.NewResourceNameSet(expected...),
	)

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

	test.That(t, r2.Close(context.Background()), test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
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
	logger := golog.NewTestLogger(t)
	channel := make(chan struct{})

	modelName := resource.NewDefaultModel(resource.ModelName(utils.RandomAlphaString(8)))
	dummyArm1 := dummyArm{channel: channel}
	dummyArm2 := dummyArm{channel: channel}
	resource.RegisterComponent(
		arm.Subtype,
		modelName,
		resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
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
	`, modelName.String())
	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(armConfig), logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r, err := robotimpl.New(ctx, cfg, logger)
	defer func() {
		test.That(t, r.Close(ctx), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)

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
	arm1, err := arm.NewClientFromConn(ctx, conn, arm.Named("somerem:arm1"), logger)
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
	board.LocalBoard
	closeCount int
}

func (db *dummyBoard) Name() resource.Name {
	return board.Named("bad")
}

func (db *dummyBoard) SPINames() []string {
	return nil
}

func (db *dummyBoard) I2CNames() []string {
	return nil
}

func (db *dummyBoard) AnalogReaderNames() []string {
	return nil
}

func (db *dummyBoard) DigitalInterruptNames() []string {
	return nil
}

func (db *dummyBoard) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

func (db *dummyBoard) Close(ctx context.Context) error {
	db.closeCount++
	return nil
}

func TestNewTeardown(t *testing.T) {
	logger := golog.NewTestLogger(t)

	modelName := resource.NewDefaultModel(resource.ModelName(utils.RandomAlphaString(8)))
	var dummyBoard1 dummyBoard
	resource.RegisterComponent(
		board.Subtype,
		modelName,
		resource.Registration[board.Board, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (board.Board, error) {
			return &dummyBoard1, nil
		}})
	resource.RegisterComponent(
		gripper.Subtype,
		modelName,
		resource.Registration[gripper.Gripper, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (gripper.Gripper, error) {
			return nil, errors.New("whoops")
		}})

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
`, modelName)
	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(failingConfig), logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	err = r.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dummyBoard1.closeCount, test.ShouldEqual, 1)
}

func TestMetadataUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	resources := r.ResourceNames()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(resources), test.ShouldEqual, 10)
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
		vision.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		datamanager.Named(resource.DefaultServiceName),
	}

	resources = r.ResourceNames()
	test.That(t, len(resources), test.ShouldEqual, len(resourceNames))
	test.That(t, rtestutils.NewResourceNameSet(resources...), test.ShouldResemble, rtestutils.NewResourceNameSet(resourceNames...))

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	resources = r.ResourceNames()
	test.That(t, resources, test.ShouldBeEmpty)
}

func TestSensorsService(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	svc, err := sensors.FromRobot(r, resource.DefaultServiceName)
	test.That(t, err, test.ShouldBeNil)

	sensorNames := []resource.Name{movementsensor.Named("movement_sensor1"), movementsensor.Named("movement_sensor2")}
	foundSensors, err := svc.Sensors(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rtestutils.NewResourceNameSet(foundSensors...), test.ShouldResemble, rtestutils.NewResourceNameSet(sensorNames...))

	readings, err := svc.Readings(context.Background(), []resource.Name{movementsensor.Named("movement_sensor1")}, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(readings), test.ShouldEqual, 1)
	test.That(t, readings[0].Name, test.ShouldResemble, movementsensor.Named("movement_sensor1"))
	test.That(t, len(readings[0].Readings), test.ShouldBeGreaterThan, 3)

	readings, err = svc.Readings(context.Background(), sensorNames, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(readings), test.ShouldEqual, 2)

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestStatusService(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)

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
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestStatus(t *testing.T) {
	buttonSubtype := resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("button"))
	button1 := resource.NameFromSubtype(buttonSubtype, "button1")
	button2 := resource.NameFromSubtype(buttonSubtype, "button2")

	workingSubtype := resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("working"))
	working1 := resource.NameFromSubtype(workingSubtype, "working1")

	failSubtype := resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("fail"))
	fail1 := resource.NameFromSubtype(failSubtype, "fail1")

	workingStatus := map[string]interface{}{"position": "up"}
	errFailed := errors.New("can't get status")

	resource.RegisterSubtype(
		workingSubtype,
		resource.SubtypeRegistration[resource.Resource]{
			Status: func(ctx context.Context, res resource.Resource) (interface{}, error) { return workingStatus, nil },
		},
	)

	resource.RegisterSubtype(
		failSubtype,
		resource.SubtypeRegistration[resource.Resource]{
			Status: func(ctx context.Context, res resource.Resource) (interface{}, error) { return nil, errFailed },
		},
	)

	statuses := []robot.Status{{Name: button1, Status: map[string]interface{}{}}}
	logger := golog.NewTestLogger(t)
	resourceNames := []resource.Name{working1, button1, fail1}
	resourceMap := map[resource.Name]resource.Resource{
		working1: rtestutils.NewUnimplementedResource(working1),
		button1:  rtestutils.NewUnimplementedResource(button1),
		fail1:    rtestutils.NewUnimplementedResource(fail1),
	}

	t.Run("not found", func(t *testing.T) {
		r, err := robotimpl.RobotFromResources(context.Background(), resourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()

		test.That(t, err, test.ShouldBeNil)

		_, err = r.Status(context.Background(), []resource.Name{button2})
		test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(button2))
	})

	t.Run("no CreateStatus", func(t *testing.T) {
		r, err := robotimpl.RobotFromResources(context.Background(), resourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, err, test.ShouldBeNil)

		resp, err := r.Status(context.Background(), []resource.Name{button1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, statuses)
	})

	t.Run("failing resource", func(t *testing.T) {
		r, err := robotimpl.RobotFromResources(context.Background(), resourceMap, logger)
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
		r, err := robotimpl.RobotFromResources(context.Background(), resourceMap, logger)
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
		r, err := robotimpl.RobotFromResources(context.Background(), workingResourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, err, test.ShouldBeNil)

		resp, err := r.Status(context.Background(), []resource.Name{})
		test.That(t, err, test.ShouldBeNil)
		// 6 because the 4 default services are always added to a local_robot. We only care
		// about the first two (working1 and button1) however.
		test.That(t, len(resp), test.ShouldEqual, 6)

		// although the response is length 6, the only thing we actually care about for testing
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
	logger := golog.NewTestLogger(t)
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
	frameSystemConfigFunc := func(
		ctx context.Context,
		additionalTransforms []*referenceframe.LinkInFrame,
	) (framesystemparts.Parts, error) {
		return framesystemparts.Parts{
			&referenceframe.FrameSystemPart{
				FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, nil, "arm1", nil),
				ModelFrame:  referenceframe.NewSimpleModel("arm1"),
			},
			&referenceframe.FrameSystemPart{
				FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, nil, "arm2", nil),
				ModelFrame:  referenceframe.NewSimpleModel("arm2"),
			},
		}, nil
	}

	injectRobot1 := &inject.Robot{
		FrameSystemConfigFunc:   frameSystemConfigFunc,
		ResourceNamesFunc:       resourcesFunc,
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}
	armStatus := &armpb.Status{
		EndPosition:    &commonpb.Pose{},
		JointPositions: &armpb.JointPositions{Values: []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0}},
	}
	injectRobot1.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
		statusCallCount++
		statuses := make([]robot.Status, 0, len(resourceNames))
		for _, n := range resourceNames {
			statuses = append(statuses, robot.Status{Name: n, Status: armStatus})
		}
		return statuses, nil
	}
	injectRobot2 := &inject.Robot{
		FrameSystemConfigFunc:   frameSystemConfigFunc,
		ResourceNamesFunc:       resourcesFunc,
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}
	injectRobot2.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
		statusCallCount++
		statuses := make([]robot.Status, 0, len(resourceNames))
		for _, n := range resourceNames {
			statuses = append(statuses, robot.Status{Name: n, Status: armStatus})
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
	r, err := robotimpl.New(ctx, remoteConfig, logger)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)

	test.That(
		t,
		rtestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rtestutils.NewResourceNameSet(
			motion.Named(resource.DefaultServiceName),
			vision.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			datamanager.Named(resource.DefaultServiceName),
			arm.Named("foo:arm1"),
			arm.Named("foo:arm2"),
			arm.Named("bar:arm1"),
			arm.Named("bar:arm2"),
		),
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
	}
}

func TestGetRemoteResourceAndGrandFather(t *testing.T) {
	// set up remotes
	options, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)

	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	remoteRemoteConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				API:   arm.Subtype,
				Model: fakeModel,
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
			},
			{
				Name:  "arm2",
				API:   arm.Subtype,
				Model: fakeModel,
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
			},
			{
				Name:  "pieceArm",
				API:   arm.Subtype,
				Model: fakeModel,
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
			},
		},
		Services: []resource.Config{},
		Remotes:  []config.Remote{},
	}

	r0, err := robotimpl.New(ctx, remoteRemoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r0.Close(context.Background()), test.ShouldBeNil)
	}()

	err = r0.StartWeb(ctx, options)
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
	r1, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r1.Close(context.Background()), test.ShouldBeNil)
	}()
	err = r1.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(ctx, remoteConfig, logger)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)

	test.That(
		t,
		rtestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rtestutils.NewResourceNameSet(
			motion.Named(resource.DefaultServiceName),
			vision.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			datamanager.Named(resource.DefaultServiceName),
			arm.Named("remote:foo:arm1"), arm.Named("remote:foo:arm2"),
			arm.Named("remote:pieceArm"),
			arm.Named("remote:foo:pieceArm"),
			audioinput.Named("remote:mic1"),
			camera.Named("remote:cameraOver"),
			movementsensor.Named("remote:movement_sensor1"),
			movementsensor.Named("remote:movement_sensor2"),
			gripper.Named("remote:pieceGripper"),
			motion.Named("remote:builtin"),
			vision.Named("remote:builtin"),
			sensors.Named("remote:builtin"),
			datamanager.Named("remote:builtin"),
			motion.Named("remote:foo:builtin"),
			vision.Named("remote:foo:builtin"),
			sensors.Named("remote:foo:builtin"),
			datamanager.Named("remote:foo:builtin"),
		),
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
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	badConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:                "test",
				API:                 base.Subtype,
				Model:               resource.NewDefaultModel("random"),
				ConvertedAttributes: someConfig{},
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 navigation.Subtype,
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
	r, err := robotimpl.New(ctx, badConfig, logger)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r, test.ShouldNotBeNil)
	// Test Component Error
	name := base.Named("test")
	noBase, err := r.ResourceByName(name)
	test.That(
		t,
		err,
		test.ShouldBeError,
		resource.NewNotAvailableError(name, errors.New("config validation error found in resource: rdk:component:base/test: fail")),
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
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	badConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:                "test",
				API:                 base.Subtype,
				Model:               fakeModel,
				ConvertedAttributes: someConfig{},
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 datamanager.Subtype,
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
	r, err := robotimpl.New(ctx, badConfig, logger)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r, test.ShouldNotBeNil)
	options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(context.Background(), options1)
	test.That(t, err, test.ShouldBeNil)

	goodConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "test",
				API:   base.Subtype,
				Model: fakeModel,
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 datamanager.Subtype,
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

	// Test Component Error
	name := base.Named("test")
	noBase, err := base.FromRobot(r, "test")
	test.That(
		t,
		err,
		test.ShouldBeError,
		resource.NewNotAvailableError(name, errors.New("config validation error found in resource: rdk:component:base/test: fail")),
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
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	armConfig := resource.Config{
		Name:  "arm1",
		API:   arm.Subtype,
		Model: fakeModel,
		ConvertedAttributes: &fake.Config{
			ModelFilePath: "../../components/arm/fake/fake_model.json",
		},
	}
	cfg := config.Config{
		Components: []resource.Config{armConfig},
	}

	robotRemote, err := robotimpl.New(ctx, &cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robotRemote, test.ShouldNotBeNil)
	defer func() {
		test.That(t, robotRemote.Close(context.Background()), test.ShouldBeNil)
	}()
	options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err = robotRemote.StartWeb(context.Background(), options1)
	test.That(t, err, test.ShouldBeNil)

	goodConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "test",
				API:   base.Subtype,
				Model: fakeModel,
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 datamanager.Subtype,
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
	r, err := robotimpl.New(ctx, goodConfig, logger)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r, test.ShouldNotBeNil)

	badConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:                "test",
				API:                 base.Subtype,
				Model:               fakeModel,
				ConvertedAttributes: someConfig{},
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 datamanager.Subtype,
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
		resource.NewNotAvailableError(name, errors.New("config validation error found in resource: rdk:component:base/test: fail")),
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
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	badConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "fake0",
				API:   base.Subtype,
				Model: resource.NewDefaultModel("random"),
			},
		},
		Services: []resource.Config{
			{
				Name:              "fake1",
				DeprecatedSubtype: "no",
			},
		},
	}
	test.That(t, badConfig.Ensure(false, logger), test.ShouldBeNil)

	goodConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "fake0",
				API:   base.Subtype,
				Model: fakeModel,
			},
		},
		Services: []resource.Config{
			{
				Name:                "fake1",
				API:                 datamanager.Subtype,
				Model:               resource.DefaultServiceModel,
				ConvertedAttributes: &builtin.Config{},
			},
		},
	}
	test.That(t, goodConfig.Ensure(false, logger), test.ShouldBeNil)
	r, err := robotimpl.New(ctx, badConfig, logger)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r, test.ShouldNotBeNil)

	noBase, err := r.ResourceByName(base.Named("fake0"))
	test.That(
		t,
		err,
		test.ShouldBeError,
		resource.NewNotAvailableError(
			base.Named("fake0"),
			errors.New("resource build error: unknown resource type: rdk:component:base and/or model: rdk:builtin:random"),
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
	logger, logs := golog.NewObservedTestLogger(t)

	r, err := robotimpl.New(context.Background(), &config.Config{
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
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	test.That(t, logs.FilterField(zap.String("output", "heythere\n")).Len(), test.ShouldEqual, 1)
}

func TestConfigPackages(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

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

	r, err := robotimpl.New(ctx, robotConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	_, err = r.PackageManager().PackagePath("some-name-1")
	test.That(t, err, test.ShouldEqual, packages.ErrPackageMissing)

	robotConfig2 := &config.Config{
		Packages: []config.PackageConfig{
			{
				Name:    "some-name-1",
				Package: "package-1",
				Version: "v1",
			},
			{
				Name:    "some-name-2",
				Package: "package-1",
				Version: "v2",
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
	test.That(t, path1, test.ShouldEqual, path.Join(packageDir, "some-name-1"))

	path2, err := r.PackageManager().PackagePath("some-name-2")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path2, test.ShouldEqual, path.Join(packageDir, "some-name-2"))
}

func TestConfigPackageReferenceReplacement(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

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
			{
				Name:    "some-name-2",
				Package: "package-2",
				Version: "latest",
			},
		},
		PackagePath: packageDir,
		Services: []resource.Config{
			{
				Name:  "my_ml_model_service",
				API:   mlmodel.Subtype,
				Model: resource.NewDefaultModel("tflite_cpu"),
				Attributes: rutils.AttributeMap{
					"model_path":  "${packages.package-1}/model.tflite",
					"label_path":  "${packages.package-1}/labels.txt",
					"num_threads": 1,
				},
			},
		},
	}

	fakePackageServer.StorePackage(robotConfig.Packages...)

	r, err := robotimpl.New(ctx, robotConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestReconnectRemote(t *testing.T) {
	logger := golog.NewTestLogger(t)
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	// start the first robot
	ctx := context.Background()
	armConfig := resource.Config{
		Name:  "arm1",
		API:   arm.Subtype,
		Model: fakeModel,
		ConvertedAttributes: &fake.Config{
			ModelFilePath: "../../components/arm/fake/fake_model.json",
		},
	}
	cfg := config.Config{
		Components: []resource.Config{armConfig},
	}

	robot, err := robotimpl.New(ctx, &cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	defer func() {
		test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
	}()
	err = robot.StartWeb(ctx, options)
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

	robot1, err := robotimpl.New(ctx, &cfg1, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	defer func() {
		test.That(t, robot1.Close(context.Background()), test.ShouldBeNil)
	}()

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
	test.That(t, robot.StopWeb(context.Background()), test.ShouldBeNil)
	test.That(t, <-remoteRobotClient.Changed(), test.ShouldBeTrue)
	test.That(t, len(remoteRobotClient.ResourceNames()), test.ShouldEqual, 0)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, len(robotClient.ResourceNames()), test.ShouldEqual, 4)
	})
	test.That(t, len(robot1.ResourceNames()), test.ShouldEqual, 4)
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
	test.That(t, len(remoteRobotClient.ResourceNames()), test.ShouldEqual, 5)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, len(robotClient.ResourceNames()), test.ShouldEqual, 9)
	})
	test.That(t, len(robot1.ResourceNames()), test.ShouldEqual, 9)
	_, err = remoteRobotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)

	_, err = robotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = anArm.EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
}

func TestReconnectRemoteChangeConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)

	// start the first robot
	ctx := context.Background()
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	armConfig := resource.Config{
		Name:  "arm1",
		API:   arm.Subtype,
		Model: fakeModel,
		ConvertedAttributes: &fake.Config{
			ModelFilePath: "../../components/arm/fake/fake_model.json",
		},
	}
	cfg := config.Config{
		Components: []resource.Config{armConfig},
	}

	robot, err := robotimpl.New(ctx, &cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	defer func() {
		test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
	}()
	err = robot.StartWeb(ctx, options)
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

	robot1, err := robotimpl.New(ctx, &cfg1, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	defer func() {
		test.That(t, robot1.Close(context.Background()), test.ShouldBeNil)
	}()

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
		test.That(tb, len(robotClient.ResourceNames()), test.ShouldEqual, 4)
	})
	test.That(t, len(robot1.ResourceNames()), test.ShouldEqual, 4)
	_, err = anArm.EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeError)

	// reconnect the first robot
	ctx2 := context.Background()
	listener, err := net.Listen("tcp", addr)
	test.That(t, err, test.ShouldBeNil)
	baseConfig := resource.Config{
		Name:  "base1",
		API:   base.Subtype,
		Model: fakeModel,
	}
	cfg = config.Config{
		Components: []resource.Config{baseConfig},
	}

	options = weboptions.New()
	options.Network.BindAddress = ""
	options.Network.Listener = listener
	robot, err = robotimpl.New(ctx, &cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	err = robot.StartWeb(ctx2, options)
	test.That(t, err, test.ShouldBeNil)

	// check if the original arm can't be called anymore
	test.That(t, <-remoteRobotClient.Changed(), test.ShouldBeTrue)
	test.That(t, remoteRobotClient.Connected(), test.ShouldBeTrue)
	test.That(t, len(remoteRobotClient.ResourceNames()), test.ShouldEqual, 5)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, len(robotClient.ResourceNames()), test.ShouldEqual, 9)
	})
	test.That(t, len(robot1.ResourceNames()), test.ShouldEqual, 9)
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

	test.That(t, len(robotClient.ResourceNames()), test.ShouldEqual, 9)
}

func TestCheckMaxInstanceValid(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg := &config.Config{
		Services: []resource.Config{
			{
				Name:  "fake1",
				Model: resource.DefaultServiceModel,
				API:   motion.Subtype,
			},
			{
				Name:  "fake2",
				Model: resource.DefaultServiceModel,
				API:   motion.Subtype,
			},
		},
		Components: []resource.Config{
			{
				Name:                "fake2",
				Model:               fake.ModelName,
				API:                 arm.Subtype,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
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
	logger := golog.NewTestLogger(t)
	cfg := &config.Config{
		Services: []resource.Config{
			{
				Name:  "fake1",
				Model: resource.DefaultServiceModel,
				API:   datamanager.Subtype,
			},
			{
				Name:  "fake2",
				Model: resource.DefaultServiceModel,
				API:   datamanager.Subtype,
			},
			{
				Name:  "fake3",
				Model: resource.DefaultServiceModel,
				API:   datamanager.Subtype,
			},
		},
		Components: []resource.Config{
			{
				Name:                "fake2",
				Model:               fake.ModelName,
				API:                 arm.Subtype,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "fake3",
				Model:               fake.ModelName,
				API:                 arm.Subtype,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	maxInstance := 0
	for _, name := range r.ResourceNames() {
		if name.Subtype == datamanager.Subtype {
			maxInstance++
		}
	}
	test.That(t, maxInstance, test.ShouldEqual, 1)
	numInstances := 0
	for _, name := range r.ResourceNames() {
		if name.Subtype == arm.Subtype {
			numInstances++
		}
	}
	test.That(t, numInstances, test.ShouldEqual, 2)
}

func TestCheckMaxInstanceSkipRemote(t *testing.T) {
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)

	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	r0, err := robotimpl.New(ctx, &config.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r0.Close(context.Background()), test.ShouldBeNil)
	}()

	err = r0.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	remoteConfig := &config.Config{
		Services: []resource.Config{
			{
				Name:  "fake1",
				Model: resource.DefaultServiceModel,
				API:   datamanager.Subtype,
			},
			{
				Name:  "fake2",
				Model: resource.DefaultServiceModel,
				API:   datamanager.Subtype,
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "remote",
				Address: addr,
			},
		},
	}

	r, err := robotimpl.New(ctx, remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	maxInstance := 0
	for _, name := range r.ResourceNames() {
		if name.Subtype == datamanager.Subtype {
			maxInstance++
		}
	}
	test.That(t, maxInstance, test.ShouldEqual, 2)

	_, err = r.ResourceByName(datamanager.Named("remote:builtin"))
	test.That(t, err, test.ShouldBeNil)
}

func TestOrphanedResources(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "b",
				Model: fakeModel,
				API:   base.Subtype,
			},
			{
				Name:                "m",
				Model:               fakeModel,
				API:                 motor.Subtype,
				DependsOn:           []string{"b"},
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:                "m1",
				Model:               fakeModel,
				API:                 motor.Subtype,
				DependsOn:           []string{"m"},
				ConvertedAttributes: &fakemotor.Config{},
			},
		},
		Services: []resource.Config{
			{
				Name:      "s",
				Model:     fakeModel,
				API:       slam.Subtype,
				DependsOn: []string{"b"},
			},
		},
	}
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	// Assert that removing base 'b' removes motors 'm' and 'm1' and slam service 's'.
	cfg2 := &config.Config{
		Components: []resource.Config{
			{
				Name:                "m",
				Model:               fakeModel,
				API:                 motor.Subtype,
				DependsOn:           []string{"b"},
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:                "m1",
				Model:               fakeModel,
				API:                 motor.Subtype,
				DependsOn:           []string{"m"},
				ConvertedAttributes: &fakemotor.Config{},
			},
		},
		Services: []resource.Config{
			{
				Name:      "s",
				Model:     fakeModel,
				API:       slam.Subtype,
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

func TestModularOrphanedResources(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	// Precompile modules to avoid timeout issues when building takes too long.
	builder := exec.Command("go", "build", ".")
	builder.Dir = rutils.ResolveFile("examples/customresources/demos/complexmodule")
	out, err := builder.CombinedOutput()
	test.That(t, string(out), test.ShouldEqual, "")
	test.That(t, err, test.ShouldBeNil)
	builder = exec.Command("go", "build", ".")
	builder.Dir = rutils.ResolveFile("examples/customresources/demos/simplemodule")
	out, err = builder.CombinedOutput()
	test.That(t, string(out), test.ShouldEqual, "")
	test.That(t, err, test.ShouldBeNil)

	// Manually define models, as importing them can cause double registration.
	gizmoModel := resource.NewModel(resource.Namespace("acme"), resource.ModelFamilyName("demo"),
		resource.ModelName("mygizmo"))
	summationModel := resource.NewModel(resource.Namespace("acme"), resource.ModelFamilyName("demo"),
		resource.ModelName("mysum"))
	gizmoSubtype := resource.NewSubtype("acme", resource.ResourceTypeComponent, "gizmo")
	summationSubtype := resource.NewSubtype("acme", resource.ResourceTypeService, "summation")

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: rutils.ResolveFile("examples/customresources/demos/complexmodule/run.sh"),
			},
		},
		Components: []resource.Config{
			{
				Name:  "g",
				Model: gizmoModel,
				API:   gizmoSubtype,
			},
		},
		Services: []resource.Config{
			{
				Name:  "s",
				Model: summationModel,
				API:   summationSubtype,
			},
		},
	}
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	// Assert that reconfiguring module 'mod' to a new module that does not handle
	// old resources removes modular component 'g' and modular service 's'.
	cfg2 := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: rutils.ResolveFile("examples/customresources/demos/simplemodule/run.sh"),
			},
		},
		Components: []resource.Config{
			{
				Name:  "g",
				Model: gizmoModel,
				API:   gizmoSubtype,
			},
		},
		Services: []resource.Config{
			{
				Name:  "s",
				Model: summationModel,
				API:   summationSubtype,
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
				API:   gizmoSubtype,
			},
		},
		Services: []resource.Config{
			{
				Name:  "s",
				Model: summationModel,
				API:   summationSubtype,
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
}

var (
	doodadModel   = resource.NewDefaultModel("mydoodad")
	doodadSubtype = resource.NewDefaultSubtype("doodad", resource.ResourceTypeComponent)
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

func TestMixedOrphanedResources(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	// Precompile modules to avoid timeout issues when building takes too long.
	builder := exec.Command("go", "build", ".")
	builder.Dir = rutils.ResolveFile("examples/customresources/demos/complexmodule")
	out, err := builder.CombinedOutput()
	test.That(t, string(out), test.ShouldEqual, "")
	test.That(t, err, test.ShouldBeNil)
	builder = exec.Command("go", "build", ".")
	builder.Dir = rutils.ResolveFile("examples/customresources/demos/simplemodule")
	out, err = builder.CombinedOutput()
	test.That(t, string(out), test.ShouldEqual, "")
	test.That(t, err, test.ShouldBeNil)

	// Manually define gizmo model, as importing it from mygizmo can cause double
	// registration.
	gizmoModel := resource.NewModel(resource.Namespace("acme"), resource.ModelFamilyName("demo"),
		resource.ModelName("mygizmo"))

	// Register a doodad constructor and defer its deregistration.
	resource.RegisterComponent(doodadSubtype, doodadModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			newDoodad := &doodad{
				Named: conf.ResourceName().AsNamed(),
			}
			for rName, res := range deps {
				if rName.Subtype == gizmoapi.Subtype {
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
		resource.Deregister(doodadSubtype, doodadModel)
	}()

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: rutils.ResolveFile("examples/customresources/demos/complexmodule/run.sh"),
			},
		},
		Components: []resource.Config{
			{
				Name:                "g",
				DeprecatedNamespace: "acme",
				Model:               gizmoModel,
				DeprecatedSubtype:   resource.SubtypeName("gizmo"),
				DependsOn:           []string{"m"},
			},
			{
				Name:                "m",
				Model:               fakeModel,
				API:                 motor.Subtype,
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:              "d",
				Model:             doodadModel,
				DeprecatedSubtype: resource.SubtypeName("doodad"),
				DependsOn:         []string{"g"},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	// Assert that reconfiguring module 'mod' to a new module that does not handle
	// 'g' removes modular component 'g' and its dependent 'd' and leaves 'm' as-is.
	cfg2 := &config.Config{
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: rutils.ResolveFile("examples/customresources/demos/simplemodule/run.sh"),
			},
		},
		Components: []resource.Config{
			{
				Name:                "g",
				DeprecatedNamespace: "acme",
				Model:               gizmoModel,
				DeprecatedSubtype:   resource.SubtypeName("gizmo"),
				DependsOn:           []string{"m"},
			},
			{
				Name:                "m",
				Model:               fakeModel,
				API:                 motor.Subtype,
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:              "d",
				Model:             doodadModel,
				DeprecatedSubtype: resource.SubtypeName("doodad"),
				DependsOn:         []string{"g"},
			},
		},
	}
	r.Reconfigure(ctx, cfg2)

	res, err := r.ResourceByName(gizmoapi.Named("g"))
	test.That(t, err, test.ShouldBeError,
		resource.NewNotFoundError(gizmoapi.Named("g")))
	test.That(t, res, test.ShouldBeNil)
	res, err = r.ResourceByName(resource.NameFromSubtype(doodadSubtype, "d"))
	test.That(t, err, test.ShouldBeError,
		resource.NewNotFoundError(resource.NameFromSubtype(doodadSubtype, "d")))
	test.That(t, res, test.ShouldBeNil)
	_, err = r.ResourceByName(motor.Named("m"))
	test.That(t, err, test.ShouldBeNil)

	// Remove module entirely.
	cfg3 := &config.Config{
		Components: []resource.Config{
			{
				Name:                "g",
				DeprecatedNamespace: "acme",
				Model:               gizmoModel,
				DeprecatedSubtype:   resource.SubtypeName("gizmo"),
				DependsOn:           []string{"m"},
			},
			{
				Name:                "m",
				Model:               fakeModel,
				API:                 motor.Subtype,
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:              "d",
				Model:             doodadModel,
				DeprecatedSubtype: resource.SubtypeName("doodad"),
				DependsOn:         []string{"g"},
			},
		},
	}
	r.Reconfigure(ctx, cfg3)

	// Assert that adding module 'mod' back with original executable path re-adds
	// modular component 'd' and its dependent 'd', and that 'm' is still present.
	r.Reconfigure(ctx, cfg)

	_, err = r.ResourceByName(gizmoapi.Named("g"))
	test.That(t, err, test.ShouldBeNil)
	d, err := r.ResourceByName(resource.NameFromSubtype(doodadSubtype, "d"))
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
