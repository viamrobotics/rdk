package robot_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"go.viam.com/robotcore/api"
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
	if err := r.Close(context.Background()); err != nil {
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
	remoteConfig := api.Config{
		Remotes: []api.Remote{
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
	}

	test.That(t, status, test.ShouldResemble, expectedStatus)

	cancel()
	<-webDone

	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := r2.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
