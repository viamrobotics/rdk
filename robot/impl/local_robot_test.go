package robotimpl_test

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/pkg/errors"

	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/camera"
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
	"go.viam.com/rdk/services/web"
	"go.viam.com/rdk/spatialmath"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestConfig1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read("data/cfgtest1.json")
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(), test.ShouldBeNil)
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
	cfg, err := config.Read("data/fake.json")
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(), test.ShouldBeNil)
}

func TestConfigRemote(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read("data/fake.json")
	test.That(t, err, test.ShouldBeNil)

	metadataSvc, err := service.New()
	test.That(t, err, test.ShouldBeNil)
	ctx := service.ContextWithService(context.Background(), metadataSvc)

	r, err := robotimpl.New(ctx, cfg, logger, client.WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(), test.ShouldBeNil)
	}()

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := web.NewOptions()
	options.Network.BindAddress = fmt.Sprintf("localhost:%d", port)
	svc, ok := r.ServiceByName(robotimpl.WebSvcName)
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
				Name: "frame_system",
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
					Translation: spatialmath.Translation{100, 200, 300},
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
					Translation: spatialmath.Translation{100, 200, 300},
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
		Sensors: map[string]*pb.SensorStatus{
			"compass1": {
				Type: "compass",
			},
			"foo.compass1": {
				Type: "compass",
			},
			"bar.compass1": {
				Type: "compass",
			},
			"compass2": {
				Type: "relative_compass",
			},
			"foo.compass2": {
				Type: "relative_compass",
			},
			"bar.compass2": {
				Type: "relative_compass",
			},
		},
		Functions: map[string]bool{
			"func1":     true,
			"foo.func1": true,
			"bar.func1": true,
			"func2":     true,
			"foo.func2": true,
			"bar.func2": true,
		},
		Services: map[string]bool{
			"frame_system": true,
			"web1":         true,
		},
	}

	test.That(t, status, test.ShouldResemble, expectedStatus)

	cfg2, err := r2.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, 16, test.ShouldEqual, len(cfg2.Components))

	test.That(t, cfg2.FindComponent("pieceArm").Frame.Parent, test.ShouldEqual, "squee.world")
	test.That(t, cfg2.FindComponent("pieceArm").Frame.Translation, test.ShouldResemble, spatialmath.Translation{500, 500, 1000})
	test.That(t, cfg2.FindComponent("pieceArm").Frame.Orientation.AxisAngles(), test.ShouldResemble, &spatialmath.R4AA{0, 0, 0, 1})

	test.That(t, cfg2.FindComponent("foo.pieceArm").Frame.Parent, test.ShouldEqual, "foo.world")
	test.That(t, cfg2.FindComponent("foo.pieceArm").Frame.Translation, test.ShouldResemble, spatialmath.Translation{500, 500, 1000})
	test.That(t, cfg2.FindComponent("foo.pieceArm").Frame.Orientation.AxisAngles(), test.ShouldResemble, &spatialmath.R4AA{0, 0, 0, 1})

	test.That(t, cfg2.FindComponent("bar.pieceArm").Frame.Parent, test.ShouldEqual, "bar.world")
	test.That(t, cfg2.FindComponent("bar.pieceArm").Frame.Translation, test.ShouldResemble, spatialmath.Translation{500, 500, 1000})
	test.That(t, cfg2.FindComponent("bar.pieceArm").Frame.Orientation.AxisAngles(), test.ShouldResemble, &spatialmath.R4AA{0, 0, 0, 1})

	fs, err := r2.FrameSystem(context.Background(), "test", "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 29)
	t.Logf("frames: %v\n", fs.FrameNames())

	test.That(t, r.Close(), test.ShouldBeNil)
	test.That(t, r2.Close(), test.ShouldBeNil)

}

type dummyBoard struct {
	board.Board
	closeCount int
}

func (db *dummyBoard) MotorNames() []string {
	return nil
}

func (db *dummyBoard) ServoNames() []string {
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

func (db *dummyBoard) Close() error {
	db.closeCount++
	return nil
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

	var failingConfig = fmt.Sprintf(`{
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
	cfg, err := config.FromReader("", strings.NewReader(failingConfig))
	test.That(t, err, test.ShouldBeNil)

	_, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	test.That(t, dummyBoard1.closeCount, test.ShouldEqual, 1)
}

func TestMetadataUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read("data/fake.json")
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	svc, err := service.New()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(svc.All()), test.ShouldEqual, 1)
	ctx = service.ContextWithService(ctx, svc)

	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(), test.ShouldBeNil)

	test.That(t, len(svc.All()), test.ShouldEqual, 8)

	resources := map[resource.Name]struct{}{
		{
			UUID: "661c4dea-b6be-56bf-a839-cfb7f99b0a6b",
			Subtype: resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceCore,
					ResourceType: resource.ResourceTypeService,
				},
				ResourceSubtype: service.SubtypeName,
			},
			Name: "",
		}: {},
		{
			UUID:    "0ecee0a4-3d25-5bfa-ba5d-4c2f765cef6a",
			Subtype: arm.Subtype,
			Name:    "pieceArm",
		}: {},
		{
			UUID:    "06f7a658-e502-5a3b-a160-af023795b49a",
			Subtype: camera.Subtype,
			Name:    "cameraOver",
		}: {},
		{
			UUID: "064a7e85-c5d6-524c-a6c4-d050bca20da9",
			Subtype: resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceCore,
					ResourceType: resource.ResourceTypeService,
				},
				ResourceSubtype: resource.ResourceSubtypeFunction,
			},
			Name: "func1",
		}: {},
		{
			UUID: "405b6596-11ff-5a69-a3d2-1a945414a632",
			Subtype: resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceCore,
					ResourceType: resource.ResourceTypeService,
				},
				ResourceSubtype: resource.ResourceSubtypeFunction,
			},
			Name: "func2",
		}: {},
		{
			UUID:    "813681b8-d6af-5e1c-b22a-8960ccf204fb",
			Subtype: gripper.Subtype,
			Name:    "pieceGripper",
		}: {},
		{
			UUID: "d1587bf0-8655-5eb3-95af-e2f83d872ce8",
			Subtype: resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceCore,
					ResourceType: resource.ResourceTypeComponent,
				},
				ResourceSubtype: resource.ResourceSubtypeSensor,
			},
			Name: "compass1",
		}: {},
		{
			UUID: "595cfa62-fb18-59ac-9553-d257b3dcebc0",
			Subtype: resource.Subtype{
				Type: resource.Type{
					Namespace:    resource.ResourceNamespaceCore,
					ResourceType: resource.ResourceTypeComponent,
				},
				ResourceSubtype: resource.ResourceSubtypeSensor,
			},
			Name: "compass2",
		}: {},
	}
	svcResources := svc.All()
	svcResourcesSet := make(map[resource.Name]struct{})
	for _, r := range svcResources {
		svcResourcesSet[r] = struct{}{}
	}
	test.That(t, svcResourcesSet, test.ShouldResemble, resources)
}
