package main

import (
	"context"
	"fmt"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/grpc/client"
)

func main() {
	logger := golog.NewDevelopmentLogger("client")
	robot, err := client.New(
		context.Background(),
		"localhost:8081",
		logger,
		client.WithCheckConnectedEvery(time.Second),
		client.WithReconnectEvery(time.Second),
	)
	if err != nil {
		logger.Fatal(err)
	}
	defer robot.Close(context.Background())

	res, err := arm.FromRobot(robot, "arm1")
	if err != nil {
		logger.Fatal(err)
	}
	for {
		fmt.Printf("robot.ResourceNames(): %v\n", robot.ResourceNames())
		fmt.Println(res.GetEndPosition(context.Background(), map[string]interface{}{}))
		time.Sleep(time.Second)
	}
}
