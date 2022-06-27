package main

import (
	"context"
	"flag"
	"strconv"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/utils"
	util "go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

var logger = golog.NewDevelopmentLogger("client")

func main() {
	util.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {

	flag.Parse()
	angle, err := strconv.Atoi(flag.Arg(0))
	if err != nil {
		return err
	}

	robot, err := client.New(
		context.Background(),
		"ChaosCore-main.x3ysqs64b0.viam.cloud",
		logger,
		client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
			Type:    utils.CredentialsTypeRobotLocationSecret,
			Payload: "c0x4zf0px5447cia5hk8lnz7s3o2fbu1ocubpjbo75arwsyz",
		})),
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err != nil {
		logger.Fatal(err)
		return err
	}

	logger.Info("Resources:")
	logger.Info(robot.ResourceNames())

	pan, err := servo.FromRobot(robot, "pan")
	if err != nil {
		logger.Error(err)
		return err
	}

	err = pan.Move(ctx, uint8(angle))

	return nil
}
