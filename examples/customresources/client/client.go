// Package main tests out a Gizmo client.
package main

import (
	"context"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/examples/customresources/gizmoapi"
	"go.viam.com/rdk/examples/customresources/summationapi"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/navigation"
)

func main() {
	logger := golog.NewDebugLogger("client")
	robot, err := client.New(
		context.Background(),
		"localhost:8080",
		logger,
	)
	if err != nil {
		logger.Fatal(err)
	}

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

	res, err = robot.ResourceByName(summationapi.Named("adder"))
	if err != nil {
		logger.Fatal(err)
	}
	add := res.(summationapi.Summation)
	nums := []float64{10, 0.5, 12}
	retAdd, err := add.Sum(context.Background(), nums)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(nums, " sum to ", retAdd)

	res, err = robot.ResourceByName(summationapi.Named("subtractor"))
	if err != nil {
		logger.Fatal(err)
	}
	sub := res.(summationapi.Summation)
	retSub, err := sub.Sum(context.Background(), nums)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(nums, " subtract to ", retSub)


	res, err = robot.ResourceByName(navigation.Named("denali"))
	if err != nil {
		logger.Fatal(err)
	}
	nav := res.(navigation.Service)
	loc, err := nav.Location(context.Background())
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("denali service reports its location as ", loc)

	err = nav.AddWaypoint(context.Background(), geo.NewPoint(55, 22))
	if err != nil {
		logger.Fatal(err)
	}
	err = nav.AddWaypoint(context.Background(), geo.NewPoint(10, 17))
	if err != nil {
		logger.Fatal(err)
	}
	err = nav.AddWaypoint(context.Background(), geo.NewPoint(42, 42))
	if err != nil {
		logger.Fatal(err)
	}
	waypoints, err := nav.Waypoints(context.Background())
	if err != nil {
		logger.Fatal(err)
	}

	logger.Info("denali waypoints stored ", waypoints)

}
