// Package main tests out a Gizmo client.
package main

import (
	"context"

	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
)

func main() {
	logger := logging.NewDebugLogger("client").AsZap()
	robot, err := client.New(
		context.Background(),
		"localhost:8080",
		logger,
	)
	if err != nil {
		logger.Fatal(err)
	}
	//nolint:errcheck
	defer robot.Close(context.Background())

	res, err := robot.ResourceByName(gizmoapi.Named("gizmo1"))
	if err != nil {
		logger.Fatal(err)
	}
	comp1 := res.(gizmoapi.Gizmo)
	ret1, err := comp1.DoOne(context.Background(), "hello")
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret1)

	ret2, err := comp1.DoOneClientStream(context.Background(), []string{"hello", "arg1", "foo"})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret2)

	ret2, err = comp1.DoOneClientStream(context.Background(), []string{"arg1", "arg1", "arg1"})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret2)

	ret3, err := comp1.DoOneServerStream(context.Background(), "hello")
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret3)

	ret3, err = comp1.DoOneBiDiStream(context.Background(), []string{"hello", "arg1", "foo"})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret3)

	ret3, err = comp1.DoOneBiDiStream(context.Background(), []string{"arg1", "arg1", "arg1"})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(ret3)
}
