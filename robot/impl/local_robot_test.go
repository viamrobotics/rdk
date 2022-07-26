package robotimpl_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/gripper"
	// registers all components.
	_ "go.viam.com/rdk/component/register"
	"go.viam.com/rdk/config"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/grpc/server"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	armpb "go.viam.com/rdk/proto/api/component/arm/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
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
	pic, _, err := c1.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	bounds := pic.Bounds()

	test.That(t, bounds.Max.X, test.ShouldBeGreaterThanOrEqualTo, 32)

	test.That(t, cfg.Components[0].Attributes["bar"], test.ShouldEqual, fmt.Sprintf("a%sb%sc", os.Getenv("HOME"), os.Getenv("HOME")))
}

func TestConfigFake(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

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

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr := fmt.Sprintf("localhost:%d", port)
	options := weboptions.New()
	options.Network.BindAddress = addr
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	remoteConfig := &config.Config{
		Components: []config.Component{
			{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      base.SubtypeName,
				Model:     "fake",
				Frame: &config.Frame{
					Parent: referenceframe.World,
				},
			},
			{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "myParentIsRemote",
				Type:      base.SubtypeName,
				Model:     "fake",
				Frame: &config.Frame{
					Parent: "cameraOver",
				},
			},
		},
		Services: []config.Service{},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr,
				Prefix:  true,
				Frame: &config.Frame{
					Parent:      "foo",
					Translation: spatialmath.TranslationConfig{100, 200, 300},
					Orientation: &spatialmath.R4AA{math.Pi / 2., 0, 0, 1},
				},
			},
			{
				Name:    "bar",
				Address: addr,
				Prefix:  true,
			},
			{
				Name:    "squee",
				Prefix:  false,
				Address: addr,
				Frame: &config.Frame{
					Parent:      referenceframe.World,
					Translation: spatialmath.TranslationConfig{100, 200, 300},
					Orientation: &spatialmath.R4AA{math.Pi / 2., 0, 0, 1},
				},
			},
		},
	}

	ctx2 := context.Background()
	r2, err := robotimpl.New(ctx2, remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)

	expected := []resource.Name{
		vision.Name,
		sensors.Name,
		datamanager.Name,
		arm.Named("squee:pieceArm"),
		arm.Named("foo:pieceArm"),
		arm.Named("bar:pieceArm"),
		base.Named("foo"),
		base.Named("myParentIsRemote"),
		camera.Named("squee:cameraOver"),
		camera.Named("foo:cameraOver"),
		camera.Named("bar:cameraOver"),
		gps.Named("squee:gps1"),
		gps.Named("foo:gps1"),
		gps.Named("bar:gps1"),
		gps.Named("squee:gps2"),
		gps.Named("foo:gps2"),
		gps.Named("bar:gps2"),
		gripper.Named("squee:pieceGripper"),
		gripper.Named("foo:pieceGripper"),
		gripper.Named("bar:pieceGripper"),
		vision.Named("squee:"),
		sensors.Named("squee:"),
		datamanager.Named("squee:"),
		vision.Named("foo:"),
		sensors.Named("foo:"),
		datamanager.Named("foo:"),
		vision.Named("bar:"),
		sensors.Named("bar:"),
		datamanager.Named("bar:"),
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

	arm1, err := r2.ResourceByName(arm.Named("bar:pieceArm"))
	test.That(t, err, test.ShouldBeNil)
	pos1, err := arm1.(arm.Arm).GetEndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	arm2, err := r2.ResourceByName(arm.Named("foo:pieceArm"))
	test.That(t, err, test.ShouldBeNil)
	pos2, err := arm2.(arm.Arm).GetEndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm.PositionGridDiff(pos1, pos2), test.ShouldAlmostEqual, 0)

	statuses, err := r2.GetStatus(
		context.Background(),
		[]resource.Name{gps.Named("squee:gps1"), gps.Named("foo:gps1"), gps.Named("bar:gps1")},
	)
	test.That(t, err, test.ShouldBeNil)

	expectedStatusLength := 3
	test.That(t, len(statuses), test.ShouldEqual, expectedStatusLength)

	for idx := 0; idx < expectedStatusLength; idx++ {
		test.That(t, statuses[idx].Status, test.ShouldResemble, map[string]interface{}{})
	}

	statuses, err = r2.GetStatus(
		context.Background(),
		[]resource.Name{arm.Named("squee:pieceArm"), arm.Named("foo:pieceArm"), arm.Named("bar:pieceArm")},
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 3)

	armStatus := &armpb.Status{
		EndPosition:    &commonpb.Pose{},
		JointPositions: &armpb.JointPositions{Values: []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}},
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

			port, err := utils.TryReserveRandomPort()
			test.That(t, err, test.ShouldBeNil)
			options := weboptions.New()
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
						Prefix: true,
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

			_r, err := robotimpl.New(context.Background(), remoteConfig, logger)
			defer func() {
				test.That(t, _r.Close(context.Background()), test.ShouldBeNil)
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
				_r, err := robotimpl.New(context.Background(), remoteConfig, logger)
				defer func() {
					test.That(t, _r.Close(context.Background()), test.ShouldBeNil)
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
				_r, err := robotimpl.New(context.Background(), remoteConfig, logger)
				test.That(t, err, test.ShouldBeNil)
				defer func() {
					test.That(t, _r.Close(context.Background()), test.ShouldBeNil)
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
			}

			test.That(t, r2, test.ShouldNotBeNil)

			expected := []resource.Name{
				vision.Name,
				sensors.Name,
				datamanager.Name,
				arm.Named("bar:pieceArm"),
				arm.Named("foo:pieceArm"),
				camera.Named("bar:cameraOver"),
				camera.Named("foo:cameraOver"),
				gps.Named("bar:gps1"),
				gps.Named("foo:gps1"),
				gps.Named("bar:gps2"),
				gps.Named("foo:gps2"),
				gripper.Named("bar:pieceGripper"),
				gripper.Named("foo:pieceGripper"),
				vision.Named("foo:"),
				sensors.Named("foo:"),
				datamanager.Named("foo:"),
				vision.Named("bar:"),
				sensors.Named("bar:"),
				datamanager.Named("bar:"),
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

			statuses, err := r2.GetStatus(
				context.Background(), []resource.Name{gps.Named("bar:gps1"), gps.Named("foo:gps1")},
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(statuses), test.ShouldEqual, 2)
			test.That(t, statuses[0].Status, test.ShouldResemble, map[string]interface{}{})
			test.That(t, statuses[1].Status, test.ShouldResemble, map[string]interface{}{})

			statuses, err = r2.GetStatus(
				context.Background(), []resource.Name{arm.Named("bar:pieceArm"), arm.Named("foo:pieceArm")},
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(statuses), test.ShouldEqual, 2)

			armStatus := &armpb.Status{
				EndPosition:    &commonpb.Pose{},
				JointPositions: &armpb.JointPositions{Values: []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}},
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

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := weboptions.New()
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
		vision.Name,
		sensors.Name,
		datamanager.Name,
		arm.Named("foo:pieceArm"),
		camera.Named("foo:cameraOver"),
		gps.Named("foo:gps1"),
		gps.Named("foo:gps2"),
		gripper.Named("foo:pieceGripper"),
		vision.Named("foo:"),
		sensors.Named("foo:"),
		datamanager.Named("foo:"),
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

	statuses, err := r2.GetStatus(context.Background(), []resource.Name{gps.Named("foo:gps1")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 1)
	test.That(t, statuses[0].Status, test.ShouldResemble, map[string]interface{}{})

	statuses, err = r2.GetStatus(context.Background(), []resource.Name{arm.Named("foo:pieceArm")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 1)

	armStatus := &armpb.Status{
		EndPosition:    &commonpb.Pose{},
		JointPositions: &armpb.JointPositions{Values: []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}},
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
	arm.LocalArm
	stopCount int
	extra     map[string]interface{}
	channel   chan struct{}
}

func (da *dummyArm) GetEndPosition(ctx context.Context, extra map[string]interface{}) (*commonpb.Pose, error) {
	return nil, errors.New("fake error")
}

func (da *dummyArm) MoveToPosition(
	ctx context.Context,
	pose *commonpb.Pose,
	worldState *commonpb.WorldState,
	extra map[string]interface{},
) error {
	return nil
}

func (da *dummyArm) MoveToJointPositions(ctx context.Context, positionDegs *armpb.JointPositions, extra map[string]interface{}) error {
	return nil
}

func (da *dummyArm) GetJointPositions(ctx context.Context, extra map[string]interface{}) (*armpb.JointPositions, error) {
	return nil, errors.New("fake error")
}

func (da *dummyArm) Stop(ctx context.Context, extra map[string]interface{}) error {
	da.stopCount++
	da.extra = extra
	return nil
}

func (da *dummyArm) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	close(da.channel)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestStopAll(t *testing.T) {
	logger := golog.NewTestLogger(t)
	channel := make(chan struct{})

	modelName := utils.RandomAlphaString(8)
	dummyArm1 := dummyArm{channel: channel}
	dummyArm2 := dummyArm{channel: channel}
	registry.RegisterComponent(
		arm.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			if config.Name == "arm1" {
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
	`, modelName)

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
	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr := fmt.Sprintf("localhost:%d", port)
	options := weboptions.New()
	options.Network.BindAddress = addr
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	conn, err := rgrpc.Dial(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)
	arm1 := arm.NewClientFromConn(ctx, conn, "arm1", logger)

	foundOPID := false
	stopAllErrCh := make(chan error, 1)
	go func() {
		<-channel
		for _, opid := range r.OperationManager().All() {
			if opid.Method == "/proto.api.component.generic.v1.GenericService/Do" {
				foundOPID = true
				stopAllErrCh <- r.StopAll(ctx, nil)
			}
		}
	}()
	_, err = arm1.Do(ctx, map[string]interface{}{})
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

func (db *dummyBoard) Close() {
	db.closeCount++
}

func TestNewTeardown(t *testing.T) {
	logger := golog.NewTestLogger(t)

	modelName := utils.RandomAlphaString(8)
	var dummyBoard1 dummyBoard
	registry.RegisterComponent(
		board.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return &dummyBoard1, nil
		}})
	registry.RegisterComponent(
		gripper.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
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

	test.That(t, len(resources), test.ShouldEqual, 8)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)

	// 5 declared resources + default sensors
	resourceNames := []resource.Name{
		arm.Named("pieceArm"),
		camera.Named("cameraOver"),
		gripper.Named("pieceGripper"),
		gps.Named("gps1"),
		gps.Named("gps2"),
		vision.Name,
		sensors.Name,
		datamanager.Name,
	}

	resources = r.ResourceNames()
	test.That(t, len(resources), test.ShouldEqual, len(resourceNames))

	test.That(t, rtestutils.NewResourceNameSet(resources...), test.ShouldResemble, rtestutils.NewResourceNameSet(resourceNames...))
}

func TestSensorsService(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	svc, err := sensors.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)

	sensorNames := []resource.Name{gps.Named("gps1"), gps.Named("gps2")}
	foundSensors, err := svc.GetSensors(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rtestutils.NewResourceNameSet(foundSensors...), test.ShouldResemble, rtestutils.NewResourceNameSet(sensorNames...))

	readings1 := []interface{}{0.0, 0.0, 0.0, 0.0, 0, 0, 0.0, 0.0, false}
	expected := map[resource.Name]interface{}{
		gps.Named("gps1"): readings1,
		gps.Named("gps2"): readings1,
	}

	readings, err := svc.GetReadings(context.Background(), []resource.Name{gps.Named("gps1")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(readings), test.ShouldEqual, 1)
	test.That(t, readings[0].Name, test.ShouldResemble, gps.Named("gps1"))
	test.That(t, readings[0].Readings, test.ShouldResemble, expected[readings[0].Name])

	readings, err = svc.GetReadings(context.Background(), sensorNames)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(readings), test.ShouldEqual, 2)

	test.That(t, readings[0].Readings, test.ShouldResemble, expected[readings[0].Name])
	test.That(t, readings[1].Readings, test.ShouldResemble, expected[readings[1].Name])

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestStatusService(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	resourceNames := []resource.Name{arm.Named("pieceArm"), gps.Named("gps1")}
	rArm, err := arm.FromRobot(r, "pieceArm")
	test.That(t, err, test.ShouldBeNil)
	armStatus, err := arm.CreateStatus(context.Background(), rArm)
	test.That(t, err, test.ShouldBeNil)
	expected := map[resource.Name]interface{}{
		arm.Named("pieceArm"): armStatus,
		gps.Named("gps1"):     map[string]interface{}{},
	}

	statuses, err := r.GetStatus(context.Background(), []resource.Name{gps.Named("gps1")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 1)
	test.That(t, statuses[0].Name, test.ShouldResemble, gps.Named("gps1"))
	test.That(t, statuses[0].Status, test.ShouldResemble, expected[statuses[0].Name])

	statuses, err = r.GetStatus(context.Background(), resourceNames)
	test.That(t, err, test.ShouldBeNil)

	expectedStatusLength := 2
	test.That(t, len(statuses), test.ShouldEqual, expectedStatusLength)

	for idx := 0; idx < expectedStatusLength; idx++ {
		test.That(t, statuses[idx].Status, test.ShouldResemble, expected[statuses[idx].Name])
	}
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestGetStatus(t *testing.T) {
	buttonSubtype := resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("button"))
	button1 := resource.NameFromSubtype(buttonSubtype, "button1")
	button2 := resource.NameFromSubtype(buttonSubtype, "button2")

	workingSubtype := resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("working"))
	working1 := resource.NameFromSubtype(workingSubtype, "working1")

	failSubtype := resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("fail"))
	fail1 := resource.NameFromSubtype(failSubtype, "fail1")

	workingStatus := map[string]interface{}{"position": "up"}
	errFailed := errors.New("can't get status")

	registry.RegisterResourceSubtype(
		workingSubtype,
		registry.ResourceSubtype{
			Status: func(ctx context.Context, resource interface{}) (interface{}, error) { return workingStatus, nil },
		},
	)

	registry.RegisterResourceSubtype(
		failSubtype,
		registry.ResourceSubtype{
			Status: func(ctx context.Context, resource interface{}) (interface{}, error) { return nil, errFailed },
		},
	)

	statuses := []robot.Status{{Name: button1, Status: map[string]interface{}{}}}
	logger := golog.NewTestLogger(t)
	resourceNames := []resource.Name{working1, button1, fail1}
	resourceMap := map[resource.Name]interface{}{working1: "resource", button1: "resource", fail1: "resource"}

	t.Run("not found", func(t *testing.T) {
		r, err := robotimpl.RobotFromResources(context.Background(), resourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()

		test.That(t, err, test.ShouldBeNil)

		_, err = r.GetStatus(context.Background(), []resource.Name{button2})
		test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(button2))
	})

	t.Run("no CreateStatus", func(t *testing.T) {
		r, err := robotimpl.RobotFromResources(context.Background(), resourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, err, test.ShouldBeNil)

		resp, err := r.GetStatus(context.Background(), []resource.Name{button1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, statuses)
	})

	t.Run("failing resource", func(t *testing.T) {
		r, err := robotimpl.RobotFromResources(context.Background(), resourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, err, test.ShouldBeNil)

		_, err = r.GetStatus(context.Background(), []resource.Name{fail1})
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
		_, err = r.GetStatus(context.Background(), []resource.Name{button2})
		test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(button2))

		resp, err := r.GetStatus(context.Background(), []resource.Name{working1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		status := resp[0]
		test.That(t, status.Name, test.ShouldResemble, working1)
		test.That(t, status.Status, test.ShouldResemble, workingStatus)

		resp, err = r.GetStatus(context.Background(), []resource.Name{working1, working1, working1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		status = resp[0]
		test.That(t, status.Name, test.ShouldResemble, working1)
		test.That(t, status.Status, test.ShouldResemble, workingStatus)

		resp, err = r.GetStatus(context.Background(), []resource.Name{working1, button1})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)
		test.That(t, resp[0].Status, test.ShouldResemble, expected[resp[0].Name])
		test.That(t, resp[1].Status, test.ShouldResemble, expected[resp[1].Name])

		_, err = r.GetStatus(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeError, errors.Wrapf(errFailed, "failed to get status from %q", fail1))
	})

	t.Run("get all status", func(t *testing.T) {
		workingResourceMap := map[resource.Name]interface{}{working1: "resource", button1: "resource"}
		expected := map[resource.Name]interface{}{
			working1: workingStatus,
			button1:  map[string]interface{}{},
		}
		r, err := robotimpl.RobotFromResources(context.Background(), workingResourceMap, logger)
		defer func() {
			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, err, test.ShouldBeNil)

		resp, err := r.GetStatus(context.Background(), []resource.Name{})
		test.That(t, err, test.ShouldBeNil)
		// 5 because the 3 default services are always added to a local_robot. We only care
		// about the first two (working1 and button1) however.
		test.That(t, len(resp), test.ShouldEqual, 5)

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

func TestGetStatusRemote(t *testing.T) {
	// set up remotes
	port1, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr1 := fmt.Sprintf("localhost:%d", port1)
	listener1, err := net.Listen("tcp", addr1)
	test.That(t, err, test.ShouldBeNil)
	port2, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr2 := fmt.Sprintf("localhost:%d", port2)
	listener2, err := net.Listen("tcp", addr2)
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	gServer2 := grpc.NewServer()
	resourcesFunc := func() []resource.Name { return []resource.Name{arm.Named("arm1"), arm.Named("arm2")} }
	statusCallCount := 0

	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:       resourcesFunc,
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}
	armStatus := &armpb.Status{
		EndPosition:    &commonpb.Pose{},
		JointPositions: &armpb.JointPositions{Values: []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0}},
	}
	injectRobot1.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
		statusCallCount++
		statuses := make([]robot.Status, 0, len(resourceNames))
		for _, n := range resourceNames {
			statuses = append(statuses, robot.Status{Name: n, Status: armStatus})
		}
		return statuses, nil
	}
	injectRobot2 := &inject.Robot{
		ResourceNamesFunc:       resourcesFunc,
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}
	injectRobot2.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
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
				Prefix:  false,
			},
			{
				Name:    "bar",
				Address: addr2,
				Prefix:  true,
			},
		},
	}
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	r, err := robotimpl.New(ctx, remoteConfig, logger)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), r), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		rtestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rtestutils.NewResourceNameSet(
			vision.Name, sensors.Name, datamanager.Name,
			arm.Named("foo:arm1"), arm.Named("foo:arm2"), arm.Named("bar:arm1"), arm.Named("bar:arm2"),
		),
	)
	statuses, err := r.GetStatus(
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
	port1, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr1 := fmt.Sprintf("localhost:%d", port1)
	test.That(t, err, test.ShouldBeNil)
	port2, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr2 := fmt.Sprintf("localhost:%d", port2)

	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	remoteRemoteConfig := &config.Config{
		Components: []config.Component{
			{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "arm1",
				Type:      arm.SubtypeName,
				Model:     "fake",
			},
			{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "arm2",
				Type:      arm.SubtypeName,
				Model:     "fake",
			},
			{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "pieceArm",
				Type:      arm.SubtypeName,
				Model:     "fake",
			},
		},
		Services: []config.Service{},
		Remotes:  []config.Remote{},
	}

	r0, err := robotimpl.New(ctx, remoteRemoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r0.Close(context.Background()), test.ShouldBeNil)
	}()
	options := weboptions.New()
	options.Network.BindAddress = addr1
	err = r0.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	r0arm1, err := r0.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	r0Arm, ok := r0arm1.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	tPos := referenceframe.JointPositionsFromRadians([]float64{10.0})
	err = r0Arm.MoveToJointPositions(context.Background(), tPos, nil)
	test.That(t, err, test.ShouldBeNil)
	p0Arm1, err := r0Arm.GetJointPositions(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)

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
	options = weboptions.New()
	options.Network.BindAddress = addr2
	err = r1.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(ctx, remoteConfig, logger)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), r), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)

	test.That(
		t,
		rtestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rtestutils.NewResourceNameSet(
			vision.Name, sensors.Name, datamanager.Name,
			arm.Named("remote:foo:arm1"), arm.Named("remote:foo:arm2"),
			arm.Named("remote:pieceArm"),
			arm.Named("remote:foo:pieceArm"),
			camera.Named("remote:cameraOver"),
			gps.Named("remote:gps1"),
			gps.Named("remote:gps2"),
			gripper.Named("remote:pieceGripper"),
			vision.Named("remote:"),
			sensors.Named("remote:"),
			datamanager.Named("remote:"),
			vision.Named("remote:foo:"),
			sensors.Named("remote:foo:"),
			datamanager.Named("remote:foo:"),
		),
	)
	arm1, err := r.ResourceByName(arm.Named("remote:foo:arm1"))
	test.That(t, err, test.ShouldBeNil)
	rrArm1, ok := arm1.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	pos, err := rrArm1.GetJointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.Values, test.ShouldResemble, p0Arm1.Values)

	arm1, err = r.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	rrArm1, ok = arm1.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	pos, err = rrArm1.GetJointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.Values, test.ShouldResemble, p0Arm1.Values)

	_, err = r.ResourceByName(arm.Named("remote:foo:pieceArm"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(arm.Named("remote:pieceArm"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(arm.Named("pieceArm"))
	test.That(t, err, test.ShouldBeError, "more that one remote resources with name \"pieceArm\" exists")
}

func TestResourceStartsOnReconfigure(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	badConfig := &config.Config{
		Components: []config.Component{
			{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "fake0",
				Type:      base.SubtypeName,
				Model:     "random",
			},
		},
		Services: []config.Service{
			{
				Name: "fake1",
				Type: "no",
			},
		},
		Cloud: &config.Cloud{},
	}

	goodConfig := &config.Config{
		Components: []config.Component{
			{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "fake0",
				Type:      base.SubtypeName,
				Model:     "fake",
			},
		},
		Services: []config.Service{
			{
				Namespace:           resource.ResourceNamespaceRDK,
				Name:                "fake1",
				Type:                config.ServiceType(datamanager.SubtypeName),
				ConvertedAttributes: &datamanager.Config{},
			},
		},
		Cloud: &config.Cloud{},
	}
	r, err := robotimpl.New(ctx, badConfig, logger)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r, test.ShouldNotBeNil)

	noBase, err := r.ResourceByName(base.Named("fake0"))
	test.That(t, err, test.ShouldBeError)
	test.That(t, err.Error(), test.ShouldResemble, rutils.NewResourceNotFoundError(base.Named("fake0")).Error())
	test.That(t, noBase, test.ShouldBeNil)

	r.Reconfigure(ctx, goodConfig)

	yesBase, err := r.ResourceByName(base.Named("fake0"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, yesBase, test.ShouldNotBeNil)

	yesSvc, err := r.ResourceByName(datamanager.Name)
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
