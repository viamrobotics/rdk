package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

func prettyString(p spatialmath.Pose) string {
	o := p.Orientation().OrientationVectorDegrees()
	f := ""
	for _, v := range []string{"X", "Y", "Z", "OX", "OY", "OZ", "Theta"} {
		f += v + ": %7.2f "
	}
	return fmt.Sprintf(f, p.Point().X, p.Point().Y, p.Point().Z, o.OX, o.OY, o.OZ, o.Theta)
}

type motionPrintArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string
}

func motionPrintConfigAction(c *cli.Context, args motionPrintArgs) error {
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

func motionPrintStatusAction(c *cli.Context, args motionPrintArgs) error {
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

	myMotion, err := motion.FromProvider(robotClient, "builtin")
	if err != nil || myMotion == nil {
		return fmt.Errorf("no motion: %w", err)
	}

	for _, p := range frameSystem.Parts {
		n := p.FrameConfig.Name()

		pif, err := myMotion.GetPose(ctx, n, "world", nil, nil)
		if err != nil {
			return err
		}

		printf(c.App.Writer, "%20s : %v", n, prettyString(pif.Pose()))
	}

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

	myMotion, err := motion.FromProvider(robotClient, "builtin")
	if err != nil || myMotion == nil {
		return fmt.Errorf("no motion: %w", err)
	}

	pif, err := myMotion.GetPose(ctx, args.Component, "world", nil, nil)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "%v", prettyString(pif.Pose()))

	return nil
}

type motionSetPoseArgs struct {
	Organization string
	Location     string
	Machine      string
	Part         string

	Component string

	X, Y, Z, Ox, Oy, Oz, Theta []float64
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

	myMotion, err := motion.FromProvider(robotClient, "builtin")
	if err != nil || myMotion == nil {
		return fmt.Errorf("no motion: %w", err)
	}

	pose, err := myMotion.GetPose(ctx, args.Component, "world", nil, nil)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "start pose %v", pose)

	pt := pose.Pose().Point()
	o := pose.Pose().Orientation().OrientationVectorDegrees()

	if len(args.X) > 0 {
		pt.X = args.X[0]
	}
	if len(args.Y) > 0 {
		pt.Y = args.Y[0]
	}
	if len(args.Z) > 0 {
		pt.Z = args.Z[0]
	}

	if len(args.Ox) > 0 {
		o.OX = args.Ox[0]
	}
	if len(args.Oy) > 0 {
		o.OY = args.Oy[0]
	}
	if len(args.Oz) > 0 {
		o.OZ = args.Oz[0]
	}
	if len(args.Theta) > 0 {
		o.Theta = args.Theta[0]
	}

	pose = referenceframe.NewPoseInFrame(pose.Parent(), spatialmath.NewPose(pt, o))

	printf(c.App.Writer, "going to pose %v", pose)

	req := motion.MoveReq{
		ComponentName: args.Component,
		Destination:   pose,
	}
	_, err = myMotion.Move(ctx, req)
	if err != nil {
		return err
	}

	return nil
}
