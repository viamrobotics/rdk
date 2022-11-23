// Package main tests out all four custom models in the complexmodule.
package main

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
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
	defer robot.Close(context.Background())

	logger.Info("---- Testing gizmo1 (gizmoapi) -----")
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

	logger.Info("---- Testing adder (summationapi) -----")
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

	logger.Info("---- Testing subtractor (summationapi) -----")
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


	logger.Info("---- Testing denali (navigation) -----")

	res, err = robot.ResourceByName(navigation.Named("denali"))
	if err != nil {
		logger.Fatal(err)
	}
	nav := res.(navigation.Service)
	loc, err := nav.Location(context.Background(), nil)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Infof("denali service reports its location as %0.8f, %0.8f", loc.Lat(), loc.Lng())

	err = nav.AddWaypoint(context.Background(), geo.NewPoint(55.1, 22.2), nil)
	if err != nil {
		logger.Fatal(err)
	}
	err = nav.AddWaypoint(context.Background(), geo.NewPoint(10.77, 17.88), nil)
	if err != nil {
		logger.Fatal(err)
	}
	err = nav.AddWaypoint(context.Background(), geo.NewPoint(42.0, 42.0), nil)
	if err != nil {
		logger.Fatal(err)
	}
	waypoints, err := nav.Waypoints(context.Background(), nil)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Info("denali waypoints stored")
	for _, w := range waypoints {
		logger.Infof("%.8f %.8f", w.Lat, w.Long)
	}

	logger.Info("---- Testing base1 (base) -----")
	res, err = robot.ResourceByName(base.Named("base1"))
	if err != nil {
		logger.Fatal(err)
	}
	mybase := res.(base.Base)

	logger.Info("move forward")
	err = mybase.SetPower(context.Background(), r3.Vector{X: 1}, r3.Vector{}, nil)
	if err != nil {
		logger.Fatal(err)
	}
	time.Sleep(time.Second)

	logger.Info("move backward")
	err = mybase.SetPower(context.Background(), r3.Vector{X: -1}, r3.Vector{}, nil)
	if err != nil {
		logger.Fatal(err)
	}
	time.Sleep(time.Second)

	logger.Info("spin left")
	err = mybase.SetPower(context.Background(), r3.Vector{}, r3.Vector{Z: -1}, nil)
	if err != nil {
		logger.Fatal(err)
	}
	time.Sleep(time.Second)

	logger.Info("spin right")
	err = mybase.SetPower(context.Background(), r3.Vector{}, r3.Vector{Z: 1}, nil)
	if err != nil {
		logger.Fatal(err)
	}
	time.Sleep(time.Second *1)

	logger.Info("stop")
	err = mybase.Stop(context.Background(), nil)
	if err != nil {
		logger.Fatal(err)
	}

}
