package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

type motionPrintArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
}

func motionPrintAction(c *cli.Context, args motionPrintArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}

	ctx, fqdn, rpcOpts, err := client.prepareDial(args.Organization, args.Location, args.Machine, args.Part, globalArgs.Debug)
	if err != nil {
		return err
	}

	logger := globalArgs.createLogger()

	robotClient, err := client.connectToRobot(ctx, fqdn, rpcOpts, globalArgs.Debug, logger)
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(robotClient.Close(ctx))
	}()

	frameSystem, err := robotClient.FrameSystemConfig(ctx)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "%v", frameSystem)

	return nil
}

type motionGetPoseArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string

	Component string
}

func motionGetPoseAction(c *cli.Context, args motionGetPoseArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}

	ctx, fqdn, rpcOpts, err := client.prepareDial(args.Organization, args.Location, args.Machine, args.Part, globalArgs.Debug)
	if err != nil {
		return err
	}

	logger := globalArgs.createLogger()

	robotClient, err := client.connectToRobot(ctx, fqdn, rpcOpts, globalArgs.Debug, logger)
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(robotClient.Close(ctx))
	}()

	theComponent, err := robot.ResourceByName(robotClient, args.Component)
	if err != nil {
		return err
	}

	myMotion, err := motion.FromRobot(robotClient, "builtin")
	if err != nil || myMotion == nil {
		return fmt.Errorf("no motion: %w", err)
	}

	pose, err := myMotion.GetPose(ctx, theComponent.Name(), "world", nil, nil)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "%v", pose)

	return nil
}

type motionSetPoseArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string

	Component string

	X, Y, Z []float64
}
func motionSetPoseAction(c *cli.Context, args motionSetPoseArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return err
	}

	ctx, fqdn, rpcOpts, err := client.prepareDial(args.Organization, args.Location, args.Machine, args.Part, globalArgs.Debug)
	if err != nil {
		return err
	}

	logger := globalArgs.createLogger()

	robotClient, err := client.connectToRobot(ctx, fqdn, rpcOpts, globalArgs.Debug, logger)
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(robotClient.Close(ctx))
	}()

	theComponent, err := robot.ResourceByName(robotClient, args.Component)
	if err != nil {
		return err
	}

	myMotion, err := motion.FromRobot(robotClient, "builtin")
	if err != nil || myMotion == nil {
		return fmt.Errorf("no motion: %w", err)
	}

	pose, err := myMotion.GetPose(ctx, theComponent.Name(), "world", nil, nil)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "start pose %v", pose)

	pt := pose.Pose().Point()

	if len(args.X) > 0 {
		pt.X = args.X[0]
	}
	if len(args.Y) > 0 {
		pt.Y = args.Y[0]
	}
	if len(args.Z) > 0 {
		pt.Z = args.Z[0]
	}

	pose = referenceframe.NewPoseInFrame(pose.Parent(), spatialmath.NewPose(pt, pose.Pose().Orientation()))

	printf(c.App.Writer, "going to pose %v", pose)

	req := motion.MoveReq{
		ComponentName: theComponent.Name(),
		Destination:   pose,
	}
	_, err = myMotion.Move(ctx, req)
	if err != nil {
		return err
	}

	return nil
}
