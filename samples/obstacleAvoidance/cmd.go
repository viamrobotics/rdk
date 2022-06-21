package main

import (
	"context"
	"flag"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/arm/fake"
	"go.viam.com/rdk/component/arm/xarm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	pb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	math "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/visualization"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

var logger = golog.NewDevelopmentLogger("client")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	// parse command line input
	simulation := flag.Bool("simulation", false, "choose to run in simulation")
	flag.Parse()

	// setup planning problem - the idea is to move from one position to the other while avoiding obstalces
	position1 := r3.Vector{0, -600, 200}
	//position2 := r3.Vector{0, -600, 400}
	position2 := r3.Vector{-600, -300, 200}
	box, _ := math.NewBox(math.NewPoseFromPoint(r3.Vector{-400, -550, 150}), r3.Vector{300, 300, 300})
	table, _ := math.NewBox(math.NewPoseFromPoint(r3.Vector{0, 0, -25}), r3.Vector{1500, 1500, 50})

	// construct world state message
	obstacles := make(map[string]math.Geometry)
	obstacles["box"] = box
	obstacles["table"] = table
	worldState := &pb.WorldState{
		Obstacles: []*pb.GeometriesInFrame{frame.GeometriesInFrameToProtobuf(frame.NewGeometriesInFrame(frame.World, obstacles))},
	}

	// get the arm to plan with - either use hardware or a fake arm
	var xArm arm.Arm
	var err error
	if *simulation {
		xArm, err = fake.NewArm(config.Component{Name: "xarm6"})
	} else {
		robotClient := connect()
		defer robotClient.Close(ctx)
		xArm, err = arm.FromRobot(robotClient, "xarm6")
	}
	if err != nil {
		logger.Fatal(err)
	}

	// determine which position to assign the start and which the goal
	currentPose, err := xArm.GetEndPosition(ctx)
	if err != nil {
		logger.Fatal(err)
	}
	eePosition := math.NewPoseFromProtobuf(currentPose).Point()
	delta1 := eePosition.Sub(position1).Norm()
	delta2 := eePosition.Sub(position2).Norm()
	start := math.PoseToProtobuf(math.NewPoseFromPoint(position1))
	goal := math.PoseToProtobuf(math.NewPoseFromPoint(position2))
	if delta1 > delta2 {
		start, goal = goal, start
	}

	// ensure that the arm starts in the correct position
	move(ctx, xArm, start, worldState)

	// visualize plan to move it to the goal
	joints, err := xArm.GetJointPositions(ctx)
	if err != nil {
		logger.Fatal(err)
	}
	model, err := xarm.XArmModel(6)
	if err != nil {
		logger.Fatal(err)
	}
	visualization.VisualizePlan(ctx, logger, model, joints, goal, worldState)

	// move it to the goal
	move(ctx, xArm, goal, worldState)
	return nil
}

func move(ctx context.Context, arm arm.Arm, toPose *pb.Pose, worldState *pb.WorldState) {
	if err := arm.MoveToPosition(ctx, toPose, worldState); err != nil {
		logger.Fatal(err)
	}
	time.Sleep(10 * time.Second) // TODO remove this when XArm move calls are blocking by default
}

func connect() robot.Robot {
	robot, err := client.New(
		context.Background(),
		"ray-pi-main.tcz8zh8cf6.viam.cloud",
		logger,
		client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
			Type:    rdkutils.CredentialsTypeRobotLocationSecret,
			Payload: "ewvmwn3qs6wqcrbnewwe1g231nvzlx5k5r5g34c31n6f7hs8",
		})),
	)
	if err != nil {
		logger.Fatal(err)
	}
	return robot
}
