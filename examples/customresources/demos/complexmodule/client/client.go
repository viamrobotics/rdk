// Package main tests out all four custom models in the complexmodule.
package main

import (
	"context"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/navigation"
)

func main() {
	logger := logging.NewDevelopmentLogger("client").AsZap()
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

	logger.Info("---- Testing gizmo1 (gizmoapi) -----")
	comp1, err := gizmoapi.FromRobot(robot, "gizmo1")
	if err != nil {
		logger.Fatal(err)
	}
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

	logger.Info("---- Testing subtractor (summationapi) -----")
	sub, err := summationapi.FromRobot(robot, "subtractor")
	if err != nil {
		logger.Fatal(err)
	}
	retSub, err := sub.Sum(context.Background(), nums)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(nums, " subtract to ", retSub)

	logger.Info("---- Testing denali (navigation) -----")
	nav, err := navigation.FromRobot(robot, "denali")
	if err != nil {
		logger.Fatal(err)
	}
	geoPose, err := nav.Location(context.Background(), nil)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Infof("denali service reports its location as %0.8f, %0.8f", geoPose.Location().Lat(), geoPose.Location().Lng())

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
	mybase, err := base.FromRobot(robot, "base1")
	if err != nil {
		logger.Fatal(err)
	}

	logger.Info("generic echo")
	testCmd := map[string]interface{}{"foo": "bar"}
	ret, err := mybase.DoCommand(context.Background(), testCmd)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Infof("sent: %v received: %v", testCmd, ret)

	logger.Info("move forward")
	err = mybase.SetPower(context.Background(), r3.Vector{Y: 1}, r3.Vector{}, nil)
	if err != nil {
		logger.Fatal(err)
	}
	time.Sleep(time.Second)

	logger.Info("move backward")
	err = mybase.SetPower(context.Background(), r3.Vector{Y: -1}, r3.Vector{}, nil)
	if err != nil {
		logger.Fatal(err)
	}
	time.Sleep(time.Second)

	logger.Info("spin left")
	err = mybase.SetPower(context.Background(), r3.Vector{}, r3.Vector{Z: 1}, nil)
	if err != nil {
		logger.Fatal(err)
	}
	time.Sleep(time.Second)

	logger.Info("spin right")
	err = mybase.SetPower(context.Background(), r3.Vector{}, r3.Vector{Z: -1}, nil)
	if err != nil {
		logger.Fatal(err)
	}
	time.Sleep(time.Second * 1)

	logger.Info("stop")
	err = mybase.Stop(context.Background(), nil)
	if err != nil {
		logger.Fatal(err)
	}
}
