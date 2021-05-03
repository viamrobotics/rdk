package robot_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
	"go.viam.com/robotcore/utils"

	_ "go.viam.com/robotcore/rimage/imagesource"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"github.com/stretchr/testify/assert"
)

func TestConfig1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := api.ReadConfig("data/cfgtest1.json")
	if err != nil {
		t.Fatal(err)
	}

	r, err := robot.NewRobot(context.Background(), cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	pic, _, err := r.CameraByName("c1").Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	bounds := pic.Bounds()

	if bounds.Max.X < 100 {
		t.Errorf("pictures seems wrong %d %d", bounds.Max.X, bounds.Max.Y)
	}

	assert.Equal(t, fmt.Sprintf("a%sb%sc", os.Getenv("HOME"), os.Getenv("HOME")), cfg.Components[0].Attributes["bar"])
}

func TestConfigFake(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := api.ReadConfig("data/fake.json")
	if err != nil {
		t.Fatal(err)
	}

	r, err := robot.NewRobot(context.Background(), cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigRemote(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := api.ReadConfig("data/fake.json")
	if err != nil {
		t.Fatal(err)
	}

	r, err := robot.NewRobot(context.Background(), cfg, logger)
	if err != nil {
		t.Fatal(err)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port, err := utils.TryReserveRandomPort()
	if err != nil {
		t.Fatal(err)
	}
	options := web.NewOptions()
	options.Port = port

	webDone := make(chan struct{})
	go func() {
		web.RunWeb(cancelCtx, r, options, logger)
		close(webDone)
	}()

	addr := fmt.Sprintf("localhost:%d", port)
	remoteConfig := &api.Config{
		Remotes: []api.RemoteConfig{
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

	r2, err := robot.NewRobot(context.Background(), remoteConfig, logger)
	if err != nil {
		t.Fatal(err)
	}

	status, err := r2.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}

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
		Bases:  map[string]bool{},
		Boards: map[string]*pb.BoardStatus{},
		Grippers: map[string]bool{
			"pieceGripper":     true,
			"foo.pieceGripper": true,
		},
		Cameras: map[string]bool{
			"cameraOver":     true,
			"foo.cameraOver": true,
		},
		LidarDevices: map[string]bool{
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

	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	if err := r2.Close(); err != nil {
		t.Fatal(err)
	}
}

type dummyBoard struct {
	board.Board
	closeCount int
}

func (db *dummyBoard) Close() error {
	db.closeCount++
	return nil
}

func TestNewRobotTeardown(t *testing.T) {
	logger := golog.NewTestLogger(t)

	modelName := utils.RandomAlphaString(8)
	var dummyBoard1 dummyBoard
	board.RegisterBoard(modelName, func(ctx context.Context, cfg board.Config, logger golog.Logger) (board.Board, error) {
		return &dummyBoard1, nil
	})
	api.RegisterGripper(modelName, func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (api.Gripper, error) {
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
	cfg, err := api.ReadConfigFromReader("", strings.NewReader(failingConfig))
	test.That(t, err, test.ShouldBeNil)

	_, err = robot.NewRobot(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	test.That(t, dummyBoard1.closeCount, test.ShouldEqual, 1)
}
