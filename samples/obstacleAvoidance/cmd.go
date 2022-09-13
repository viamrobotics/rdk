// Package main is an obstacle avoidance utility.
package main

import (
	"context"
	"flag"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/viamrobotics/visualization"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/arm/wrapper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	pb "go.viam.com/rdk/proto/api/common/v1"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	robotimpl "go.viam.com/rdk/robot/impl"
	spatial "go.viam.com/rdk/spatialmath"
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
	// position2 := r3.Vector{0, -600, 300}
	position2 := r3.Vector{-600, -300, 100}
	box, _ := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{-400, -550, 150}), r3.Vector{300, 300, 300})
	table, _ := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{0, 0, 0}), r3.Vector{1500, 1500, 50})
	ws, _ := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{0, 0, 0}), r3.Vector{1500, 1500, 1500})

	// construct world state message
	obstacles := make(map[string]spatial.Geometry)
	obstacles["box"] = box
	obstacles["table"] = table
	workspace := make(map[string]spatial.Geometry)
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
	eePosition := spatial.NewPoseFromProtobuf(currentPose).Point()
	delta1 := eePosition.Sub(position1).Norm()
	delta2 := eePosition.Sub(position2).Norm()
	start := spatial.PoseToProtobuf(spatial.NewPoseFromPoint(position1))
	goal := spatial.PoseToProtobuf(spatial.NewPoseFromPoint(position2))
	start.OZ = -1
	goal.OZ = -1
	if delta1 > delta2 {
		start, goal = goal, start
	}

	// move it to the start
	if err := arm.Move(ctx, robotClient, xArm, start, nil); err != nil {
		return err
	}

	// setup planner options
	opt := motionplan.NewBasicPlannerOptions()
	opt.AddConstraint("collision", motionplan.NewCollisionConstraint(xArm.ModelFrame(), obstacles, workspace))
	// opt.AddConstraint("collision", motionplan.NewCollisionConstraint(xArm.ModelFrame(), obstacles, workspace))

	// move it to the goal
	inputs, err := xArm.CurrentInputs(ctx)
	if err != nil {
		return err
	}
	planner, err := motionplan.NewRRTStarConnectMotionPlanner(xArm.ModelFrame(), 1, logger)
	if err != nil {
		return err
	}
	solution, err := planner.Plan(ctx, goal, inputs, opt)
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
	armName := "xarm6"
	if simulation {
		fakeName := "fake"
		fakeArm, err := fake.NewArmIK(ctx, config.Component{Name: fakeName}, logger)
		if err != nil {
			return nil, nil, err
		}
		robotClient, err = robotimpl.RobotFromResources(ctx, map[resource.Name]interface{}{
			arm.Named(armName):  xArm,
			arm.Named(fakeName): fakeArm,
		}, logger)
		if err != nil {
			return nil, nil, err
		}
		defer robotClient.Close(ctx)
		names := robotClient.ResourceNames()
		_ = names
		xArm, err = wrapper.NewWrapperArm(
			config.Component{
				ConvertedAttributes: &wrapper.AttrConfig{
					ModelPath: rdkutils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"),
					ArmName:   "fake",
				},
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
