# Go client
The Go client for the Viam RDK can function as an SDK to connect to a robot.

# Install

To install the Go SDK, run

	go get go.viam.com/rdk/robot/client

# Basic Usage

To connect to a robot as a client, you should instantiate a client.

	package main

	import (
		"context"

		"github.com/edaniels/golog"
		"go.viam.com/rdk/robot/client"
		"go.viam.com/rdk/utils"
		"go.viam.com/utils/rpc"
	)

	func main() {
		logger := golog.NewDevelopmentLogger("client")
		robot, err := client.New(
			context.Background(),
			"<address of robot>",
			logger,
			client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
				Type:    utils.CredentialsTypeRobotLocationSecret,
				Payload: "<robot secret>",
			})),
		)
		if err != nil {
			logger.Fatal(err)
		}
	}

If the robot is managed by Viam, you can also navigate to the robot page on app.viam.com,
select the CONNECT tab, and copy the boilerplate code from the section labeled Golang SDK.

You can then query resources and also grab a resource by its name.

	logger.Info("Resources:")
  	logger.Info(robot.ResourceNames())
	m1 := motor.FromRobot(robot, "motor1")
	position, err := m1.Position(context.Background())
	if err != nil {
		logger.Error(err)
	}
	logger.Info(position)

Remember to close the client at the end!

	robot.Close(context.Background())
