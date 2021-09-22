package robotimpl_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/metadata/service"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"

	_ "go.viam.com/core/rimage/imagesource"

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

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(), test.ShouldBeNil)
	}()

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := web.NewOptions()
	options.Port = port

	webDone := make(chan struct{})
	go func() {
		webserver.RunWeb(cancelCtx, r, options, logger)
		close(webDone)
	}()

	addr := fmt.Sprintf("localhost:%d", port)
	remoteConfig := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr,
				Prefix:  true,
				Parent:  "ppp",
			},
			{
				Address: addr,
			},
		},
	}

	r2, err := robotimpl.New(context.Background(), remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)

	status, err := r2.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)

	expectedStatus := &pb.Status{
		Arms: map[string]*pb.ArmStatus{
			"pieceArm": {
				GridPosition: &pb.ArmPosition{
					X: 0.0,
					Y: 0.0,
					Z: 0.0,
				},
				JointPositions: &pb.JointPositions{
					Degrees: []float64{0, 0, 0, 0, 0, 0},
				},
			},
			"foo.pieceArm": {
				GridPosition: &pb.ArmPosition{
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
		},
		Cameras: map[string]bool{
			"cameraOver":     true,
			"foo.cameraOver": true,
		},
		Lidars: map[string]bool{
			"lidar1":     true,
			"foo.lidar1": true,
		},
		Sensors: map[string]*pb.SensorStatus{
			"compass1": {
				Type: "compass",
			},
			"foo.compass1": {
				Type: "compass",
			},
			"compass2": {
				Type: "relative_compass",
			},
			"foo.compass2": {
				Type: "relative_compass",
			},
		},
		Functions: map[string]bool{
			"func1":     true,
			"foo.func1": true,
			"func2":     true,
			"foo.func2": true,
		},
	}

	test.That(t, status, test.ShouldResemble, expectedStatus)

	cfg2, err := r2.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, 12, test.ShouldEqual, len(cfg2.Components))
	test.That(t, cfg2.FindComponent("foo.pieceArm").Frame.Parent, test.ShouldEqual, "foo.world")

	cancel()
	<-webDone

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
	board.RegisterBoard(modelName, func(ctx context.Context, cfg board.Config, logger golog.Logger) (board.Board, error) {
		return &dummyBoard1, nil
	})
	registry.RegisterGripper(modelName, registry.Gripper{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
		return nil, errors.New("whoops")
	}})

	var failingConfig = fmt.Sprintf(`{
	"boards": [
		{
			"model": "%[1]s",
			"name": "board1"
		}
	],
    "components": [
        {
            "model": "%[1]s",
            "name": "gripper1",
            "type": "gripper"
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

	test.That(t, len(svc.All()), test.ShouldEqual, 9)

	resources := []resource.Name{
		{
			UUID:      "661c4dea-b6be-56bf-a839-cfb7f99b0a6b",
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeService,
			Subtype:   resource.ResourceSubtypeMetadata,
			Name:      "",
		},
		{
			UUID:      "0ecee0a4-3d25-5bfa-ba5d-4c2f765cef6a",
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeComponent,
			Subtype:   resource.ResourceSubtypeArm,
			Name:      "pieceArm",
		},
		{
			UUID:      "06f7a658-e502-5a3b-a160-af023795b49a",
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeComponent,
			Subtype:   resource.ResourceSubtypeCamera,
			Name:      "cameraOver",
		},
		{
			UUID:      "064a7e85-c5d6-524c-a6c4-d050bca20da9",
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeService,
			Subtype:   resource.ResourceSubtypeFunction,
			Name:      "func1",
		},
		{
			UUID:      "405b6596-11ff-5a69-a3d2-1a945414a632",
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeService,
			Subtype:   resource.ResourceSubtypeFunction,
			Name:      "func2",
		},
		{
			UUID:      "813681b8-d6af-5e1c-b22a-8960ccf204fb",
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeComponent,
			Subtype:   resource.ResourceSubtypeGripper,
			Name:      "pieceGripper",
		},
		{
			UUID:      "b0e1e671-fa92-5d84-b6c1-d50d17e5e2ac",
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeComponent,
			Subtype:   resource.ResourceSubtypeLidar,
			Name:      "lidar1",
		},
		{
			UUID:      "d1587bf0-8655-5eb3-95af-e2f83d872ce8",
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeComponent,
			Subtype:   resource.ResourceSubtypeSensor,
			Name:      "compass1",
		},
		{
			UUID:      "595cfa62-fb18-59ac-9553-d257b3dcebc0",
			Namespace: resource.ResourceNamespaceCore,
			Type:      resource.ResourceTypeComponent,
			Subtype:   resource.ResourceSubtypeSensor,
			Name:      "compass2",
		},
	}
	test.That(t, svc.All(), test.ShouldResemble, resources)
}
