//go:build !windows

package module_test

import (
	"context"
	"math"
	"net"
	"testing"

	armpb "go.viam.com/api/component/arm/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

func TestGetFrameSystem(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	testAPI := resource.APINamespaceRDK.WithComponentType(arm.SubtypeName)

	testName := resource.NewName(testAPI, "arm1")

	expectedInputs := referenceframe.FrameSystemInputs{
		testName.ShortName(): []referenceframe.Input{{0}, {math.Pi}, {-math.Pi}, {0}, {math.Pi}, {-math.Pi}},
	}
	injectArm := &inject.Arm{
		JointPositionsFunc: func(ctx context.Context, extra map[string]any) ([]referenceframe.Input, error) {
			return expectedInputs[testName.ShortName()], nil
		},
		KinematicsFunc: func(ctx context.Context) (referenceframe.Model, error) {
			return referenceframe.ParseModelJSONFile(rutils.ResolveFile("components/arm/example_kinematics/ur5e.json"), "")
		},
	}

	resourceNames := []resource.Name{testName}
	resources := map[resource.Name]arm.Arm{testName: injectArm}
	injectRobot := &inject.Robot{
		ResourceNamesFunc:  func() []resource.Name { return resourceNames },
		ResourceByNameFunc: func(n resource.Name) (resource.Resource, error) { return resources[n], nil },
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}

	armSvc, err := resource.NewAPIResourceCollection(arm.API, resources)
	test.That(t, err, test.ShouldBeNil)
	gServer.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc))
	robotpb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := client.New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	inputs, err := client.CurrentInputs(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(inputs), test.ShouldEqual, 1)
	test.That(t, inputs, test.ShouldResemble, expectedInputs)

	fsc := module.NewFrameSystemClient(client)
	fsCurrentInputs, err := fsc.CurrentInputs(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fsCurrentInputs, test.ShouldResemble, inputs)
}
