//go:build !no_media

package robotimpl_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"math"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"go.mongodb.org/mongo-driver/bson/primitive"

	// registers all components.
	commonpb "go.viam.com/api/common/v1"
	armpb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/motion"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/spatialmath"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

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
	r2, err := robotimpl.New(ctx2, remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)

	expected := []resource.Name{
		motion.Named(resource.DefaultServiceName),
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
		sensors.Named("squee:builtin"),
		datamanager.Named("squee:builtin"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		datamanager.Named("foo:builtin"),
		motion.Named("bar:builtin"),
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

	cfg2 := r2.Config()
	// Components should only include local components.
	test.That(t, len(cfg2.Components), test.ShouldEqual, 2)

	fsConfig, err := r2.FrameSystemConfig(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fsConfig.Parts, test.ShouldHaveLength, 12)

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
				sensors.Named("foo:builtin"),
				datamanager.Named("foo:builtin"),
				motion.Named("bar:builtin"),
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
		sensors.Named(resource.DefaultServiceName),
		datamanager.Named(resource.DefaultServiceName),
		arm.Named("foo:pieceArm"),
		audioinput.Named("foo:mic1"),
		camera.Named("foo:cameraOver"),
		movementsensor.Named("foo:movement_sensor1"),
		movementsensor.Named("foo:movement_sensor2"),
		gripper.Named("foo:pieceGripper"),
		motion.Named("foo:builtin"),
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

func TestMetadataUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	resources := r.ResourceNames()
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(resources), test.ShouldEqual, 9)
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
		datamanager.Named(resource.DefaultServiceName),
	}

	resources = r.ResourceNames()
	test.That(t, len(resources), test.ShouldEqual, len(resourceNames))
	test.That(t, rtestutils.NewResourceNameSet(resources...), test.ShouldResemble, rtestutils.NewResourceNameSet(resourceNames...))

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	resources = r.ResourceNames()
	test.That(t, resources, test.ShouldBeEmpty)
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
			sensors.Named("remote:builtin"),
			datamanager.Named("remote:builtin"),
			motion.Named("remote:foo:builtin"),
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
