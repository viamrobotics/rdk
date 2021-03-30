package server_test

import (
	"context"
	"strings"
	"testing"

	"go.viam.com/robotcore/api"
	apiserver "go.viam.com/robotcore/api/server"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestServer(t *testing.T) {
	logger := golog.NewTestLogger(t)

	cfg, err := api.ReadConfig(utils.ResolveFile("./robots/configs/fake.json"))
	test.That(t, err, test.ShouldBeNil)

	myRobot, err := robot.NewRobot(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	server := apiserver.New(myRobot)
	statusResp, err := server.Status(context.Background(), &pb.StatusRequest{})
	test.That(t, err, test.ShouldBeNil)

	// TODO(erd): more exhaustive test with errors too
	test.That(t, strings.ReplaceAll(statusResp.GetStatus().String(), " ", ""), test.ShouldResemble, `arms:{key:"arm1"value:{grid_position:{}joint_positions:{degrees:0degrees:0degrees:0degrees:0degrees:0degrees:0}}}bases:{key:"base1"value:true}grippers:{key:"gripper1"value:true}boards:{key:"board1"value:{motors:{key:"g"value:{}}servos:{key:"servo1"value:{}}analogs:{key:"analog1"value:{}}digital_interrupts:{key:"encoder"value:{}}}}`)

	// TODO(erd): test server.StatusStream
}
