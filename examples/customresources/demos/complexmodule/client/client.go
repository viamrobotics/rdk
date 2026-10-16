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
	comp1, err := gizmoapi.FromProvider(robot, "gizmo1")
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
	add, err := summationapi.FromProvider(robot, "adder")
	if err != nil {
		logger.Fatal(err)
	}

	nums := []float64{10, 0.5, 12}
	retAdd, err := add.Sum(context.Background(), nums)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(nums, "sum to", retAdd)

	logger.Info("---- Testing subtractor (summationapi) -----")
	sub, err := summationapi.FromProvider(robot, "subtractor")
	if err != nil {
		logger.Fatal(err)
	}
	retSub, err := sub.Sum(context.Background(), nums)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info(nums, "subtract to", retSub)

	logger.Info("---- Testing denali (navigation) -----")
	nav, err := navigation.FromProvider(robot, "denali")
	if err != nil {
		logger.Fatal(err)
	}
	pos, err := nav.GetPosition(context.Background(), nil)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Infof("denali service reports its location as %0.8f, %0.8f",
		pos.GeoPose.Location.Lat(), pos.GeoPose.Location.Lng())

	heading := 0.0
	_, err = nav.AddLocation(context.Background(), navigation.AddLocationReq{
		Name: "a",
		Position: &navigation.Position{GeoPose: &navigation.GeoPose{
			Location: geo.NewPoint(55.1, 22.2),
			Heading:  &heading,
		}},
	})
	if err != nil {
		logger.Fatal(err)
	}
	locs, err := nav.ListLocations(context.Background(), nil)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Infof("denali stored %d locations", len(locs))

	logger.Info("---- Testing base1 (base) -----")
	mybase, err := base.FromProvider(robot, "base1")
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
