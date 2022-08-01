package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	robotimpl "go.viam.com/rdk/robot/impl"
)

func main() {
	logger := golog.NewDevelopmentLogger("client")
	flag.Parse()

	ctx := context.Background()
	cfg, err := config.Read(ctx, flag.Arg(0), logger)
	if err != nil {
		logger.Fatal(err)
	}

	robot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		logger.Fatal(err)
	}
	defer robot.Close(ctx)

	fmt.Println("new robot created")
	res, err := arm.FromRobot(robot, "arm1")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Println("got robot arm")
	for {
		fmt.Printf("robot.ResourceNames(): %v\n", robot.ResourceNames())
		fmt.Println(res.GetEndPosition(context.Background(), map[string]interface{}{}))
		time.Sleep(time.Second)
	}
}
