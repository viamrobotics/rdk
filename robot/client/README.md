# Go client
The Go client for the Viam RDK can function as an SDK to connect to a robot.

# Install

To install the Go SDK, run the following command from inside a Go module (use
`go mod init` to create a module if you do not have one already).

```
	go get go.viam.com/rdk/robot/client
```

After adding the code below, you may have to run `go mod tidy` to add all required
dependencies to your `go.mod` file.

# Basic Usage

To connect to a robot as a client, you should instantiate a client.

```
	package main

	import (
		"context"

		"go.viam.com/rdk/logging"
		"go.viam.com/rdk/robot/client"
		"go.viam.com/rdk/utils"
		"go.viam.com/utils/rpc"
	)

	func main() {
		logger := logging.NewDebugLogger("client")
		// this instantiates a robot client that is connected to the robot at <address>
		robot, err := client.New(
			context.Background(),
			"<address of robot>",
			logger,
			// credentials can be found in the Viam app if the robot is Viam managed
			// otherwise, credential keys can also be set through the config
			client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
				Type:    utils.CredentialsTypeRobotLocationSecret,
				Payload: "<robot secret>",
			})),
		)
		if err != nil {
			logger.Fatal(err)
		}
	}
```

If the robot is managed by Viam, you can also navigate to the robot page on app.viam.com,
select the CONNECT tab, and copy the boilerplate code from the section labeled Golang SDK.

You can then query resources and also grab a resource by its name.

```
	logger.Info("Resources:")
  	logger.Info(robot.ResourceNames())

	// grab a motor by its name and query for its position
	m1, err := motor.FromProvider(robot, "motor1")
	if err != nil {
		logger.Fatal(err)
	}
	position, err := m1.Position(context.Background(), map[string]interface{}{})
	if err != nil {
		logger.Error(err)
	}
```

Remember to close the client at the end!

```
	robot.Close(context.Background())
```
