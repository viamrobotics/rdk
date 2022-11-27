package main

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	v1 "go.viam.com/api/component/arm/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/utils"
	"go.viam.com/utils/rpc"
)

func main() {
	logger := golog.NewDevelopmentLogger("client")
	robot, err := client.New(
		context.Background(),
		"parent.tcz8zh8cf6.viam.cloud",
		logger,	
		client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
			Type:    utils.CredentialsTypeRobotLocationSecret,
			Payload: "ewvmwn3qs6wqcrbnewwe1g231nvzlx5k5r5g34c31n6f7hs8",
		})),
	)
	if err != nil {
		logger.Fatal(err)
	}
	defer robot.Close(context.Background())
	logger.Info("Resources:")
	logger.Info(robot.ResourceNames())

	sasha, err := arm.FromRobot(robot, "sasha")
	if err != nil {
		logger.Fatal(err)
	}
	err = sasha.MoveToJointPositions(context.Background(), &v1.JointPositions{Values: []float64{90, 0, 0, 0, 0, 0}}, nil)
	if err != nil {
		logger.Fatal(err)
	}
	fmt.Println(sasha.JointPositions(context.Background(), nil))
}
