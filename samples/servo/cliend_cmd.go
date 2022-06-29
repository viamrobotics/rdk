package main

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/utils"
)

func balh() {
	logger := golog.NewDevelopmentLogger("client")
	robot, err := client.New(
		context.Background(),
		"ChaosCore-main.x3ysqs64b0.viam.cloud",
		logger,
		client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
			Type:    utils.CredentialsTypeRobotLocationSecret,
			Payload: "c0x4zf0px5447cia5hk8lnz7s3o2fbu1ocubpjbo75arwsyz",
		})),
	)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("Resources:")
	logger.Info(robot.ResourceNames())
}
