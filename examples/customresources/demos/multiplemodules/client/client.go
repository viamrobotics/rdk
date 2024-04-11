// Package main tests out all 2 custom models in the multiplemodules.
package main

import (
	"context"

	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
)

func main() {
	logger := logging.NewLogger("client")
	robot, err := client.New(
		context.Background(),
		"localhost:8080",
		logger,
	)
	if err != nil {
		logger.Fatal(err)
	}
	defer func() {
		if err := robot.Close(context.Background()); err != nil {
			logger.Fatal(err)
		}
	}()

	logger.Info("---- Testing gizmo1 (gizmoapi) -----")
	comp1, err := gizmoapi.FromRobot(robot, "gizmo1")
	if err != nil {
		logger.Fatal(err)
	}
	ret1, err := comp1.DoOne(context.Background(), "1.0")
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret1)

	ret2, err := comp1.DoOneClientStream(context.Background(), []string{"1.0", "2.0", "3.0"})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret2)

	ret3, err := comp1.DoOneServerStream(context.Background(), "1.0")
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret3)

	ret3, err = comp1.DoOneBiDiStream(context.Background(), []string{"1.0", "2.0", "3.0"})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret3)

	ret4, err := comp1.DoTwo(context.Background(), true)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret4)

	ret4, err = comp1.DoTwo(context.Background(), false)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret4)

	logger.Info("---- Testing adder (summationapi) -----")
	add, err := summationapi.FromRobot(robot, "adder")
	if err != nil {
		logger.Fatal(err)
	}

	nums := []float64{10, 0.5, 12}
	retAdd, err := add.Sum(context.Background(), nums)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(nums, " sum to ", retAdd)
}
