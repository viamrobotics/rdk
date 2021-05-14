package robotimpl_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/go-errors/errors"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/utils"
	"go.viam.com/core/web"

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

	pic, _, err := r.CameraByName("c1").Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	bounds := pic.Bounds()

	test.That(t, bounds.Max.X, test.ShouldBeGreaterThanOrEqualTo, 100)

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

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := web.NewOptions()
	options.Port = port

	webDone := make(chan struct{})
	go func() {
		web.RunWeb(cancelCtx, r, options, logger)
		close(webDone)
	}()

	addr := fmt.Sprintf("localhost:%d", port)
	remoteConfig := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr,
				Prefix:  true,
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
					X:  0.0,
					Y:  0.0,
					Z:  0.0,
					RX: 0.0,
					RY: 0.0,
					RZ: 0.0,
				},
				JointPositions: &pb.JointPositions{
					Degrees: []float64{0, 0, 0, 0, 0, 0},
				},
			},
			"foo.pieceArm": {
				GridPosition: &pb.ArmPosition{
					X:  0.0,
					Y:  0.0,
					Z:  0.0,
					RX: 0.0,
					RY: 0.0,
					RZ: 0.0,
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
	}

	test.That(t, status, test.ShouldResemble, expectedStatus)

	cancel()
	<-webDone

	test.That(t, r.Close(), test.ShouldBeNil)
	test.That(t, r2.Close(), test.ShouldBeNil)
}

type dummyBoard struct {
	board.Board
	closeCount int
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
	registry.RegisterGripper(modelName, func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
		return nil, errors.New("whoops")
	})

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
