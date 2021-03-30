package server_test

import (
	"context"
	"testing"

	"go.viam.com/robotcore/api"
	apiserver "go.viam.com/robotcore/api/server"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func newServer(t *testing.T) pb.RobotServiceServer {
	t.Helper()
	logger := golog.NewTestLogger(t)
	cfg, err := api.ReadConfig(utils.ResolveFile("./robots/configs/fake.json"))
	test.That(t, err, test.ShouldBeNil)
	myRobot, err := robot.NewRobot(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	return apiserver.New(myRobot)
}

var emptyStatus = &pb.StatusResponse{
	Status: &pb.Status{
		Arms: map[string]*pb.ArmStatus{
			"arm1": {
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
		Bases: map[string]bool{
			"base1": true,
		},
		Grippers: map[string]bool{
			"gripper1": true,
		},
		Boards: map[string]*pb.BoardStatus{
			"board1": {
				Motors: map[string]*pb.MotorStatus{
					"g": {},
				},
				Servos: map[string]*pb.ServoStatus{
					"servo1": {},
				},
				Analogs: map[string]*pb.AnalogStatus{
					"analog1": {},
				},
				DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{
					"encoder": {},
				},
			},
		},
	},
}

func TestServer(t *testing.T) {
	t.Run("Status", func(t *testing.T) {
		server := newServer(t)
		statusResp, err := server.Status(context.Background(), &pb.StatusRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, statusResp, test.ShouldResemble, emptyStatus)

		newPosition := &pb.ArmPosition{
			X:  1.0,
			Y:  2.0,
			Z:  3.0,
			RX: 4.0,
			RY: 5.0,
			RZ: 6.0,
		}
		_, err = server.MoveArmToPosition(context.Background(), &pb.MoveArmToPositionRequest{
			Name: "arm1",
			To:   newPosition,
		})
		test.That(t, err, test.ShouldBeNil)

		statusResp, err = server.Status(context.Background(), &pb.StatusRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, statusResp, test.ShouldNotResemble, emptyStatus)
		test.That(t, statusResp.Status.Arms["arm1"].GridPosition, test.ShouldResemble, newPosition)
	})
}
