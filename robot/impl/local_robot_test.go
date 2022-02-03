package robotimpl_test

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/metadata/service"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/services/web"
	"go.viam.com/rdk/spatialmath"
)

func TestConfig1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/cfgtest1.json")
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	c1, ok := r.CameraByName("c1")
	test.That(t, ok, test.ShouldBeTrue)
	pic, _, err := c1.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	bounds := pic.Bounds()

	test.That(t, bounds.Max.X, test.ShouldBeGreaterThanOrEqualTo, 32)

	test.That(t, cfg.Components[0].Attributes["bar"], test.ShouldEqual, fmt.Sprintf("a%sb%sc", os.Getenv("HOME"), os.Getenv("HOME")))
}

func TestConfigFake(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json")
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
}

func TestConfigRemote(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json")
	test.That(t, err, test.ShouldBeNil)

	metadataSvc, err := service.New()
	test.That(t, err, test.ShouldBeNil)
	ctx := service.ContextWithService(context.Background(), metadataSvc)

	r, err := robotimpl.New(ctx, cfg, logger, client.WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := web.NewOptions()
	options.Network.BindAddress = fmt.Sprintf("localhost:%d", port)
	svc, ok := r.ResourceByName(web.Name)
	test.That(t, ok, test.ShouldBeTrue)
	err = svc.(web.Service).Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	addr := fmt.Sprintf("localhost:%d", port)
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

	r2, err := robotimpl.New(context.Background(), remoteConfig, logger, client.WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)

	status, err := r2.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)

	expectedStatus := &pb.Status{
		Bases: map[string]bool{
			"foo": true,
		},
		Arms: map[string]*pb.ArmStatus{
			"pieceArm": {
				GridPosition: &pb.Pose{
					X: 0.0,
					Y: 0.0,
					Z: 0.0,
				},
				JointPositions: &pb.JointPositions{
					Degrees: []float64{0, 0, 0, 0, 0, 0},
				},
			},
			"foo.pieceArm": {
				GridPosition: &pb.Pose{
					X: 0.0,
					Y: 0.0,
					Z: 0.0,
				},
				JointPositions: &pb.JointPositions{
					Degrees: []float64{0, 0, 0, 0, 0, 0},
				},
			},
			"bar.pieceArm": {
				GridPosition: &pb.Pose{
					X: 0.0,
					Y: 0.0,
					Z: 0.0,
				},
				JointPositions: &pb.JointPositions{
					Degrees: []float64{0, 0, 0, 0, 0, 0},
				},
			},
		},
		Grippers: map[string]bool{
			"pieceGripper":     true,
			"foo.pieceGripper": true,
			"bar.pieceGripper": true,
		},
		Cameras: map[string]bool{
			"cameraOver":     true,
			"foo.cameraOver": true,
			"bar.cameraOver": true,
		},
		Sensors: nil,
		Functions: map[string]bool{
			"func1":     true,
			"foo.func1": true,
			"bar.func1": true,
			"func2":     true,
			"foo.func2": true,
			"bar.func2": true,
		},
		Services: map[string]bool{
			"rdk:service:frame_system": true,
			"rdk:service:web":          true,
		},
	}

	test.That(t, status, test.ShouldResemble, expectedStatus)

	cfg2, err := r2.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, 16, test.ShouldEqual, len(cfg2.Components))

	test.That(t, cfg2.FindComponent("pieceArm").Frame.Parent, test.ShouldEqual, "squee.world")
	test.That(t, cfg2.FindComponent("pieceArm").Frame.Translation, test.ShouldResemble, spatialmath.TranslationConfig{500, 500, 1000})
	test.That(t, cfg2.FindComponent("pieceArm").Frame.Orientation.AxisAngles(), test.ShouldResemble, &spatialmath.R4AA{0, 0, 0, 1})

	test.That(t, cfg2.FindComponent("foo.pieceArm").Frame.Parent, test.ShouldEqual, "foo.world")
	test.That(t, cfg2.FindComponent("foo.pieceArm").Frame.Translation, test.ShouldResemble, spatialmath.TranslationConfig{500, 500, 1000})
	test.That(t, cfg2.FindComponent("foo.pieceArm").Frame.Orientation.AxisAngles(), test.ShouldResemble, &spatialmath.R4AA{0, 0, 0, 1})

	test.That(t, cfg2.FindComponent("bar.pieceArm").Frame.Parent, test.ShouldEqual, "bar.world")
	test.That(t, cfg2.FindComponent("bar.pieceArm").Frame.Translation, test.ShouldResemble, spatialmath.TranslationConfig{500, 500, 1000})
	test.That(t, cfg2.FindComponent("bar.pieceArm").Frame.Orientation.AxisAngles(), test.ShouldResemble, &spatialmath.R4AA{0, 0, 0, 1})

	fs, err := r2.FrameSystem(context.Background(), "test", "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 29)
	t.Logf("frames: %v\n", fs.FrameNames())

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	test.That(t, r2.Close(context.Background()), test.ShouldBeNil)
}

func TestConfigRemoteWithAuth(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json")
	test.That(t, err, test.ShouldBeNil)

	for _, tc := range []struct {
		Case        string
		Managed     bool
		EntityNames []string
	}{
		{Case: "unmanaged and default host"},
		{Case: "unmanaged and specific host", EntityNames: []string{"something-different", "something-really-different"}},
		{Case: "managed and default host", Managed: true},
		{Case: "managed and specific host", Managed: true, EntityNames: []string{"something-different", "something-really-different"}},
	} {
		t.Run(tc.Case, func(t *testing.T) {
			metadataSvc, err := service.New()
			test.That(t, err, test.ShouldBeNil)
			ctx := service.ContextWithService(context.Background(), metadataSvc)

			r, err := robotimpl.New(ctx, cfg, logger, client.WithDialOptions(rpc.WithInsecure()))
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				test.That(t, r.Close(context.Background()), test.ShouldBeNil)
			}()

			port, err := utils.TryReserveRandomPort()
			test.That(t, err, test.ShouldBeNil)
			options := web.NewOptions()
			options.Network.BindAddress = fmt.Sprintf("localhost:%d", port)
			options.Managed = tc.Managed
			options.FQDNs = tc.EntityNames
			apiKey := "sosecret"
			options.Auth.Handlers = []config.AuthHandlerConfig{
				{
					Type: rpc.CredentialsTypeAPIKey,
					Config: config.AttributeMap{
						"key": apiKey,
					},
				},
			}
			svc, ok := r.ResourceByName(web.Name)
			test.That(t, ok, test.ShouldBeTrue)
			err = svc.(web.Service).Start(ctx, options)
			test.That(t, err, test.ShouldBeNil)

			addr := fmt.Sprintf("localhost:%d", port)
			remoteConfig := &config.Config{
				Remotes: []config.Remote{
					{
						Name:    "foo",
						Address: addr,
					},
				},
			}

			_, err = robotimpl.New(context.Background(), remoteConfig, logger, client.WithDialOptions(rpc.WithInsecure()))
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

			entityNames := tc.EntityNames
			if len(entityNames) == 0 {
				entityNames = []string{web.DefaultFQDN}
			}
			for _, entityName := range entityNames {
				t.Run(fmt.Sprintf("auth required for %s", entityName), func(t *testing.T) {
					_, err = robotimpl.New(context.Background(), remoteConfig, logger, client.WithDialOptions(
						rpc.WithInsecure(),
						rpc.WithEntityCredentials(entityName, rpc.Credentials{
							Type:    rpc.CredentialsTypeAPIKey,
							Payload: apiKey,
						}),
					))
					test.That(t, err, test.ShouldNotBeNil)
					test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")
				})
			}

			remoteConfig.Remotes[0].Auth.Credentials = &rpc.Credentials{
				Type:    rpc.CredentialsTypeAPIKey,
				Payload: apiKey,
			}

			var r2 robot.LocalRobot
			if tc.Managed {
				_, err = robotimpl.New(context.Background(), remoteConfig, logger, client.WithDialOptions(rpc.WithInsecure()))
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

				for idx, entityName := range entityNames {
					t.Run(fmt.Sprintf("good auth for %s", entityName), func(t *testing.T) {
						remoteConfig.Remotes[0].Auth.Entity = entityName
						r2, err = robotimpl.New(context.Background(), remoteConfig, logger, client.WithDialOptions(rpc.WithInsecure()))
						test.That(t, err, test.ShouldBeNil)
						if idx != len(entityNames)-1 {
							test.That(t, r2.Close(context.Background()), test.ShouldBeNil)
						}
					})
				}
			} else {
				r2, err = robotimpl.New(context.Background(), remoteConfig, logger, client.WithDialOptions(rpc.WithInsecure()))
				test.That(t, err, test.ShouldBeNil)
			}

			status, err := r2.Status(context.Background())
			test.That(t, err, test.ShouldBeNil)

			expectedStatus := &pb.Status{
				Arms: map[string]*pb.ArmStatus{
					"pieceArm": {
						GridPosition: &pb.Pose{
							X: 0.0,
							Y: 0.0,
							Z: 0.0,
						},
						JointPositions: &pb.JointPositions{
							Degrees: []float64{0, 0, 0, 0, 0, 0},
						},
					},
				},
				Grippers: map[string]bool{
					"pieceGripper": true,
				},
				Cameras: map[string]bool{
					"cameraOver": true,
				},
				Sensors: nil,
				Functions: map[string]bool{
					"func1": true,
					"func2": true,
				},
				Services: map[string]bool{
					"rdk:service:web": true,
				},
			}

			test.That(t, status, test.ShouldResemble, expectedStatus)

			test.That(t, r.Close(context.Background()), test.ShouldBeNil)
			test.That(t, r2.Close(context.Background()), test.ShouldBeNil)
		})
	}
}

type dummyBoard struct {
	board.LocalBoard
	closeCount int
}

func (db *dummyBoard) MotorNames() []string {
	return nil
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
            "name": "gripper1",
            "type": "gripper",
            "depends_on": ["board1"]
        },
        {
            "model": "%[1]s",
            "name": "board1",
            "type": "board"
        }
    ]
}
`, modelName)
	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(failingConfig))
	test.That(t, err, test.ShouldBeNil)

	_, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	test.That(t, dummyBoard1.closeCount, test.ShouldEqual, 1)
}

func TestMetadataUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json")
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	svc, err := service.New()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(svc.All()), test.ShouldEqual, 1)
	ctx = service.ContextWithService(ctx, svc)

	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(context.Background()), test.ShouldBeNil)

	// 8 declared resources + default web and metadata service
	test.That(t, len(svc.All()), test.ShouldEqual, 10)

	resources := map[resource.Name]struct{}{
		{
			UUID: "00db7188-edaa-5ea9-b573-80ce7d2cee61",
			Subtype: resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceRDK,
					ResourceType: resource.ResourceTypeService,
				},
				ResourceSubtype: service.SubtypeName,
			},
			Name: "",
		}: {},
		{
			UUID:    "a2521aec-dd23-5bd4-bfe6-21d9887c917f",
			Subtype: arm.Subtype,
			Name:    "pieceArm",
		}: {},
		{
			UUID:    "f926189a-1206-5af1-8cc6-cc934c2a6d59",
			Subtype: camera.Subtype,
			Name:    "cameraOver",
		}: {},
		{
			UUID: "8882dd3c-3b80-50e4-bcc3-8f47ada67f85",
			Subtype: resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceRDK,
					ResourceType: resource.ResourceTypeFunction,
				},
				ResourceSubtype: resource.ResourceSubtypeFunction,
			},
			Name: "func1",
		}: {},
		{
			UUID: "9ba51a01-26a3-5e12-8b83-219076150c74",
			Subtype: resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceRDK,
					ResourceType: resource.ResourceTypeFunction,
				},
				ResourceSubtype: resource.ResourceSubtypeFunction,
			},
			Name: "func2",
		}: {},
		{
			UUID:    "6e1135a7-4ce9-54bc-b9e4-1c50aa9b5ce8",
			Subtype: gripper.Subtype,
			Name:    "pieceGripper",
		}: {},
		{
			UUID: "07c9cc8d-f36d-5f7d-a114-5a38b96a148c",
			Subtype: resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceRDK,
					ResourceType: resource.ResourceTypeComponent,
				},
				ResourceSubtype: gps.SubtypeName,
			},
			Name: "gps1",
		}: {},
		{
			UUID: "d89112b0-8f1c-51ea-a4ab-87b9129ae671",
			Subtype: resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceRDK,
					ResourceType: resource.ResourceTypeComponent,
				},
				ResourceSubtype: gps.SubtypeName,
			},
			Name: "gps2",
		}: {},
		{
			UUID:    "e1c00c06-16ca-5069-be52-30084eb40d4f",
			Subtype: framesystem.Subtype,
			Name:    "",
		}: {},
		{
			UUID:    "6b2d25f5-81ee-5386-8db8-42a0c5a29df3",
			Subtype: web.Subtype,
			Name:    "",
		}: {},
	}
	svcResources := svc.All()
	svcResourcesSet := make(map[resource.Name]struct{})
	for _, r := range svcResources {
		svcResourcesSet[r] = struct{}{}
	}
	test.That(t, svcResourcesSet, test.ShouldResemble, resources)
}
