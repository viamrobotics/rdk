package robotimpl_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	armpb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/services/metadata"
	"go.viam.com/rdk/services/objectsegmentation"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/services/status"
	"go.viam.com/rdk/services/web"
	"go.viam.com/rdk/spatialmath"
	rtestutils "go.viam.com/rdk/testutils"
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
	options := web.NewOptions()
	options.Network.BindAddress = addr
	svc, err := web.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	remoteConfig := &config.Config{
		Components: []config.Component{
			{
				Name:  "foo",
				Type:  config.ComponentTypeBase,
				Model: "fake",
				Frame: &config.Frame{
					Parent: referenceframe.World,
				},
			},
			{
				Name:  "myParentIsRemote",
				Type:  config.ComponentTypeBase,
				Model: "fake",
				Frame: &config.Frame{
					Parent: "cameraOver",
				},
			},
		},
		Services: []config.Service{
			{
				Type: "frame_system",
			},
		},
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

	test.That(t, err, test.ShouldBeNil)
	ctx2 := context.Background()
	r2, err := robotimpl.New(ctx2, remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	metadataSvc2, err := metadata.FromRobot(r2)
	test.That(t, err, test.ShouldBeNil)

	expected := []resource.Name{
		metadata.Name,
		framesystem.Name,
		objectsegmentation.Name,
		sensors.Name,
		status.Name,
		datamanager.Name,
		resource.NewName(resource.ResourceNamespaceRDK, resource.ResourceTypeComponent, resource.ResourceSubtypeRemote, "foo"),
		resource.NewName(resource.ResourceNamespaceRDK, resource.ResourceTypeComponent, resource.ResourceSubtypeRemote, "bar"),
		resource.NewName(resource.ResourceNamespaceRDK, resource.ResourceTypeComponent, resource.ResourceSubtypeRemote, "squee"),
		arm.Named("pieceArm"),
		arm.Named("foo.pieceArm"),
		arm.Named("bar.pieceArm"),
		base.Named("foo"),
		base.Named("myParentIsRemote"),
		camera.Named("cameraOver"),
		camera.Named("foo.cameraOver"),
		camera.Named("bar.cameraOver"),
		gps.Named("gps1"),
		gps.Named("foo.gps1"),
		gps.Named("bar.gps1"),
		gps.Named("gps2"),
		gps.Named("foo.gps2"),
		gps.Named("bar.gps2"),
		gripper.Named("pieceGripper"),
		gripper.Named("foo.pieceGripper"),
		gripper.Named("bar.pieceGripper"),
	}

	resources2, err := metadataSvc2.Resources(ctx2)
	test.That(t, err, test.ShouldBeNil)

	test.That(
		t,
		rtestutils.NewResourceNameSet(resources2...),
		test.ShouldResemble,
		rtestutils.NewResourceNameSet(expected...),
	)
	statusSvc, err := status.FromRobot(r2)
	test.That(t, err, test.ShouldBeNil)

	statuses, err := statusSvc.GetStatus(
		context.Background(),
		[]resource.Name{gps.Named("gps1"), gps.Named("foo.gps1"), gps.Named("bar.gps1")},
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 3)
	test.That(t, statuses[0].Status, test.ShouldResemble, struct{}{})
	test.That(t, statuses[1].Status, test.ShouldResemble, struct{}{})
	test.That(t, statuses[2].Status, test.ShouldResemble, struct{}{})

	statuses, err = statusSvc.GetStatus(
		context.Background(),
		[]resource.Name{arm.Named("pieceArm"), arm.Named("foo.pieceArm"), arm.Named("bar.pieceArm")},
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 3)
	test.That(
		t,
		statuses[0].Status,
		test.ShouldResemble,
		&armpb.Status{EndPosition: &commonpb.Pose{}, JointPositions: &armpb.JointPositions{Degrees: []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}}},
	)
	test.That(
		t,
		statuses[1].Status,
		test.ShouldResemble,
		&armpb.Status{EndPosition: &commonpb.Pose{}, JointPositions: &armpb.JointPositions{Degrees: []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}}},
	)
	test.That(
		t,
		statuses[2].Status,
		test.ShouldResemble,
		&armpb.Status{EndPosition: &commonpb.Pose{}, JointPositions: &armpb.JointPositions{Degrees: []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}}},
	)

	cfg2, err := r2.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(cfg2.Components), test.ShouldEqual, 2)

	frameService, err := framesystem.FromRobot(r2)
	test.That(t, err, test.ShouldBeNil)
	fsConfig, err := frameService.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fsConfig, test.ShouldHaveLength, 12)

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
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
			svc, err := web.FromRobot(r)
			test.That(t, err, test.ShouldBeNil)
			err = svc.Start(ctx, options)
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

			_, err = robotimpl.New(context.Background(), remoteConfig, logger)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

			remoteConfig.Remotes[0].Auth.Credentials = &rpc.Credentials{
				Type:    rpc.CredentialsTypeAPIKey,
				Payload: apiKey,
			}
			remoteConfig.Remotes[1].Auth.Credentials = &rpc.Credentials{
				Type:    rutils.CredentialsTypeRobotLocationSecret,
				Payload: locationSecret,
			}

			var r2 robot.LocalRobot
			var metadataSvc2 metadata.Service
			if tc.Managed {
				remoteConfig.Remotes[0].Auth.Entity = "wrong"
				_, err = robotimpl.New(context.Background(), remoteConfig, logger)
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "must use Config.AllowInsecureCreds")

				remoteConfig.AllowInsecureCreds = true

				_, err = robotimpl.New(context.Background(), remoteConfig, logger)
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

				remoteConfig.Remotes[0].Auth.Entity = entityName
				remoteConfig.Remotes[1].Auth.Entity = entityName
				r2, err = robotimpl.New(context.Background(), remoteConfig, logger)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r2.Close(context.Background()), test.ShouldBeNil)

				metadataSvc2, err = metadata.FromRobot(r2)
				test.That(t, err, test.ShouldBeNil)
				ctx2 := context.Background()
				remoteConfig.Remotes[0].Address = options.LocalFQDN
				if tc.EntityName != "" {
					remoteConfig.Remotes[1].Address = options.FQDN
				}
				r2, err = robotimpl.New(ctx2, remoteConfig, logger)
				test.That(t, err, test.ShouldBeNil)
			} else {
				_, err = robotimpl.New(context.Background(), remoteConfig, logger)
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "must use Config.AllowInsecureCreds")

				remoteConfig.AllowInsecureCreds = true

				r2, err = robotimpl.New(context.Background(), remoteConfig, logger)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, r2.Close(context.Background()), test.ShouldBeNil)

				metadataSvc2, err = metadata.FromRobot(r2)
				test.That(t, err, test.ShouldBeNil)
				ctx2 := context.Background()
				remoteConfig.Remotes[0].Address = options.LocalFQDN
				r2, err = robotimpl.New(ctx2, remoteConfig, logger)
				test.That(t, err, test.ShouldBeNil)
			}

			test.That(t, r2, test.ShouldNotBeNil)

			expected := []resource.Name{
				metadata.Name,
				framesystem.Name,
				objectsegmentation.Name,
				sensors.Name,
				status.Name,
				datamanager.Name,
				resource.NewName(resource.ResourceNamespaceRDK, resource.ResourceTypeComponent, resource.ResourceSubtypeRemote, "foo"),
				resource.NewName(resource.ResourceNamespaceRDK, resource.ResourceTypeComponent, resource.ResourceSubtypeRemote, "bar"),
				arm.Named("pieceArm"),
				arm.Named("foo.pieceArm"),
				camera.Named("cameraOver"),
				camera.Named("foo.cameraOver"),
				gps.Named("gps1"),
				gps.Named("foo.gps1"),
				gps.Named("gps2"),
				gps.Named("foo.gps2"),
				gripper.Named("pieceGripper"),
				gripper.Named("foo.pieceGripper"),
			}

			resources2, err := metadataSvc2.Resources(ctx)
			test.That(t, err, test.ShouldBeNil)

			test.That(
				t,
				rtestutils.NewResourceNameSet(resources2...),
				test.ShouldResemble,
				rtestutils.NewResourceNameSet(expected...),
			)
			statusSvc, err := status.FromRobot(r2)
			test.That(t, err, test.ShouldBeNil)

			statuses, err := statusSvc.GetStatus(
				context.Background(), []resource.Name{gps.Named("gps1"), gps.Named("foo.gps1")},
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(statuses), test.ShouldEqual, 2)
			test.That(t, statuses[0].Status, test.ShouldResemble, struct{}{})
			test.That(t, statuses[1].Status, test.ShouldResemble, struct{}{})

			statuses, err = statusSvc.GetStatus(
				context.Background(), []resource.Name{arm.Named("pieceArm"), arm.Named("foo.pieceArm")},
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(statuses), test.ShouldEqual, 2)
			test.That(
				t,
				statuses[0].Status,
				test.ShouldResemble,
				&armpb.Status{EndPosition: &commonpb.Pose{}, JointPositions: &armpb.JointPositions{Degrees: []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}}},
			)
			test.That(
				t,
				statuses[1].Status,
				test.ShouldResemble,
				&armpb.Status{EndPosition: &commonpb.Pose{}, JointPositions: &armpb.JointPositions{Degrees: []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}}},
			)
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

	svc, err := r.ResourceByName(web.Name)
	test.That(t, err, test.ShouldBeNil)
	err = svc.(web.Service).Start(ctx, options)
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

	_, err = robotimpl.New(context.Background(), remoteConfig, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

	remoteConfig.Remotes[0].Auth.Entity = "wrong"
	_, err = robotimpl.New(context.Background(), remoteConfig, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

	remoteConfig.Remotes[0].Auth.Entity = options.FQDN
	_, err = robotimpl.New(context.Background(), remoteConfig, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

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
	metadataSvc2, err := metadata.FromRobot(r2)
	test.That(t, err, test.ShouldBeNil)

	expected := []resource.Name{
		metadata.Name,
		framesystem.Name,
		objectsegmentation.Name,
		sensors.Name,
		status.Name,
		datamanager.Name,
		resource.NewName(resource.ResourceNamespaceRDK, resource.ResourceTypeComponent, resource.ResourceSubtypeRemote, "foo"),
		arm.Named("pieceArm"),
		camera.Named("cameraOver"),
		gps.Named("gps1"),
		gps.Named("gps2"),
		gripper.Named("pieceGripper"),
	}

	resources2, err := metadataSvc2.Resources(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(
		t,
		rtestutils.NewResourceNameSet(resources2...),
		test.ShouldResemble,
		rtestutils.NewResourceNameSet(expected...),
	)
	statusSvc, err := status.FromRobot(r2)
	test.That(t, err, test.ShouldBeNil)

	statuses, err := statusSvc.GetStatus(context.Background(), []resource.Name{gps.Named("gps1")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 1)
	test.That(t, statuses[0].Status, test.ShouldResemble, struct{}{})

	statuses, err = statusSvc.GetStatus(context.Background(), []resource.Name{arm.Named("pieceArm")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 1)
	test.That(
		t,
		statuses[0].Status,
		test.ShouldResemble,
		&armpb.Status{EndPosition: &commonpb.Pose{}, JointPositions: &armpb.JointPositions{Degrees: []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}}},
	)

	test.That(t, r2.Close(context.Background()), test.ShouldBeNil)

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
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
			r robot.Robot,
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
			r robot.Robot,
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

	_, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	test.That(t, dummyBoard1.closeCount, test.ShouldEqual, 1)
}

func TestMetadataUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	svc, err := metadata.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)

	resources, err := svc.Resources(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(resources), test.ShouldEqual, 11)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)

	// 11 declared resources + default web, sensors, status, and metadata service
	resourceNames := []resource.Name{
		arm.Named("pieceArm"),
		camera.Named("cameraOver"),
		gripper.Named("pieceGripper"),
		gps.Named("gps1"),
		gps.Named("gps2"),
		framesystem.Name,
		objectsegmentation.Name,
		sensors.Name,
		status.Name,
		datamanager.Name,
		metadata.Name,
	}

	svcResources, err := svc.Resources(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(svcResources), test.ShouldEqual, len(resourceNames))

	test.That(t, rtestutils.NewResourceNameSet(svcResources...), test.ShouldResemble, rtestutils.NewResourceNameSet(resourceNames...))
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

	svc, err := status.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)

	resourceNames := []resource.Name{arm.Named("pieceArm"), gps.Named("gps1"), status.Name, web.Name}
	rArm, err := arm.FromRobot(r, "pieceArm")
	test.That(t, err, test.ShouldBeNil)
	armStatus, err := arm.CreateStatus(context.Background(), rArm)
	test.That(t, err, test.ShouldBeNil)
	expected := map[resource.Name]interface{}{
		arm.Named("pieceArm"): armStatus,
		gps.Named("gps1"):     struct{}{},
		status.Name:           struct{}{},
		web.Name:              struct{}{},
	}

	statuses, err := svc.GetStatus(context.Background(), []resource.Name{gps.Named("gps1")})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 1)
	test.That(t, statuses[0].Name, test.ShouldResemble, gps.Named("gps1"))
	test.That(t, statuses[0].Status, test.ShouldResemble, expected[statuses[0].Name])

	statuses, err = svc.GetStatus(context.Background(), resourceNames)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(statuses), test.ShouldEqual, 4)

	test.That(t, statuses[0].Status, test.ShouldResemble, expected[statuses[0].Name])
	test.That(t, statuses[1].Status, test.ShouldResemble, expected[statuses[1].Name])
	test.That(t, statuses[2].Status, test.ShouldResemble, expected[statuses[2].Name])
	test.That(t, statuses[3].Status, test.ShouldResemble, expected[statuses[3].Name])

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}
