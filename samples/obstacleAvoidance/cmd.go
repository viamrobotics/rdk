package main

import (
	"context"
	"flag"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/arm/fake"
	"go.viam.com/rdk/component/arm/wrapper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/motionplan/visualization"
	pb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	math "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

var logger = golog.NewDevelopmentLogger("client")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	// parse command line input
	simulation := flag.Bool("simulation", false, "choose to run in simulation")
	visualize := flag.Bool("visualize", false, "choose to display visualization")
	flag.Parse()

	// connect to the robot and get arm
	robotClient, xArm, err := connect(ctx, *simulation)
	if err != nil {
		return err
	}

	// setup planning problem - the idea is to move from one position to the other while avoiding obstalces
	position1 := r3.Vector{0, -600, 100}
	position2 := r3.Vector{0, -600, 300}
	// position2 := r3.Vector{-600, -300, 100}
	box, _ := math.NewBox(math.NewPoseFromPoint(r3.Vector{-400, -550, 150}), r3.Vector{300, 300, 300})
	table, _ := math.NewBox(math.NewPoseFromPoint(r3.Vector{0, 0, 0}), r3.Vector{1500, 1500, 50})
	ws, _ := math.NewBox(math.NewPoseFromPoint(r3.Vector{0, 0, 0}), r3.Vector{1500, 1500, 1000})

	// construct world state message
	obstacles := make(map[string]math.Geometry)
	obstacles["box"] = box
	obstacles["table"] = table
	workspace := make(map[string]math.Geometry)
	workspace["workspace"] = ws
	worldState := &pb.WorldState{
		Obstacles:         []*pb.GeometriesInFrame{frame.GeometriesInFrameToProtobuf(frame.NewGeometriesInFrame(frame.World, obstacles))},
		InteractionSpaces: []*pb.GeometriesInFrame{frame.GeometriesInFrameToProtobuf(frame.NewGeometriesInFrame(frame.World, workspace))},
	}

	// determine which position to assign the start and which the goal
	currentPose, err := xArm.GetEndPosition(ctx, nil)
	if err != nil {
		return err
	}
	eePosition := math.NewPoseFromProtobuf(currentPose).Point()
	delta1 := eePosition.Sub(position1).Norm()
	delta2 := eePosition.Sub(position2).Norm()
	start := math.PoseToProtobuf(math.NewPoseFromPoint(position1))
	goal := math.PoseToProtobuf(math.NewPoseFromPoint(position2))
	start.OZ = -1
	goal.OZ = -1
	if delta1 > delta2 {
		start, goal = goal, start
	}

	// ensure that the arm starts in the correct position
	inputs, err := xArm.GetJointPositions(ctx, nil)
	if err != nil {
		return err
	}
	visualization.VisualizeStep(xArm.ModelFrame(), worldState, xArm.ModelFrame().InputFromProtobuf(inputs))
	if err := xArm.MoveToPosition(ctx, start, worldState, nil); err != nil {
		return err
	}

	// move it to the goal
	solution, err := arm.Plan(ctx, robotClient, xArm, goal, nil)
	if err != nil {
		return err
	}
	if *visualize {
		// visualize if specified by flag
		if err := visualization.VisualizePlan(ctx, solution, xArm.ModelFrame(), worldState); err != nil {
			return err
		}
	}
	arm.GoToWaypoints(ctx, xArm, solution)
	return nil
}

func connect(ctx context.Context, simulation bool) (robotClient robot.Robot, xArm arm.Arm, err error) {
	armName := arm.Named("xarm6")
	if simulation {
		registry.RegisterComponent(arm.Subtype, "fake_ik", registry.Component{
			Constructor: func(
				ctx context.Context,
				deps registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				fakeArm, err := fake.NewArmIK(ctx, config, logger)
				if err != nil {
					return nil, err
				}
				return fakeArm, nil
			},
		})
		robotClient, err = robotimpl.RobotFromResources(ctx, map[resource.Name]interface{}{armName: xArm}, logger)
		if err != nil {
			return nil, nil, err
		}
		defer robotClient.Close(ctx)
		xArm, err = wrapper.NewWrapperArm(
			config.Component{
				ConvertedAttributes: wrapper.AttrConfig{ModelPath: rdkutils.ResolveFile("component/arm/xarm/xarm6_kinematics.json"), ArmName: "xarm6"},
			},
			robotClient,
			logger,
		)
		if err != nil {
			return nil, nil, err
		}
	} else {
		robotClient, err := client.New(
			context.Background(),
			"ray-pi-main.tcz8zh8cf6.viam.cloud",
			logger,
			client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
				Type:    rdkutils.CredentialsTypeRobotLocationSecret,
				Payload: "ewvmwn3qs6wqcrbnewwe1g231nvzlx5k5r5g34c31n6f7hs8",
			})),
		)
		if err != nil {
			return nil, nil, err
		}
		defer robotClient.Close(ctx)
		xArm, err = arm.FromRobot(robotClient, "xarm6")
		if err != nil {
			return nil, nil, err
		}
	}
	return robotClient, xArm, err
}
