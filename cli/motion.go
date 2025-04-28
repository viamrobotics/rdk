package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"go.viam.com/utils"

	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/motion"
)

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

	myMotion, err := motion.FromRobot(robotClient, "builtin")
	if err != nil || myMotion == nil {
		return fmt.Errorf("no motion: %w", err)
	}

	theComponent, err := robot.ResourceByName(robotClient, args.Component)
	if err != nil {
		return err
	}

	pose, err := myMotion.GetPose(ctx, theComponent.Name(), "world", nil, nil)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "%v", pose)

	return nil
}
