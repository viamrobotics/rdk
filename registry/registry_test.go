package registry

import (
	"context"
	"testing"

	"go.viam.com/utils/rpc/dialer"
	rpcserver "go.viam.com/utils/rpc/server"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/input"
	"go.viam.com/core/lidar"
	"go.viam.com/core/motor"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"

	"go.viam.com/core/subtype"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestRegistry(t *testing.T) {
	cf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (camera.Camera, error) {
		return nil, nil
	}

	gf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
		return nil, nil
	}

	lf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		return nil, nil
	}

	sf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		return nil, nil
	}

	bf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (base.Base, error) {
		return nil, nil
	}

	bbf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (board.Board, error) {
		return nil, nil
	}

	motorf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (motor.Motor, error) {
		return nil, nil
	}

	inputf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (input.Controller, error) {
		return nil, nil
	}

	// test panics
	test.That(t, func() { RegisterCamera("x", Camera{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterBase("x", Base{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterGripper("x", Gripper{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterLidar("x", Lidar{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterSensor(sensor.Type("x"), "y", Sensor{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterBoard("x", Board{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterMotor("x", Motor{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterInputController("x", InputController{}) }, test.ShouldPanic)

	// test register
	RegisterCamera("x", Camera{Constructor: cf})
	RegisterBase("x", Base{Constructor: bf})
	RegisterGripper("x", Gripper{Constructor: gf})
	RegisterLidar("x", Lidar{Constructor: lf})
	RegisterSensor(sensor.Type("x"), "y", Sensor{Constructor: sf})
	RegisterBoard("x", Board{Constructor: bbf})
	RegisterMotor("x", Motor{Constructor: motorf})
	RegisterInputController("x", InputController{Constructor: inputf})

	// test look up

	test.That(t, CameraLookup("x"), test.ShouldNotBeNil)
	test.That(t, CameraLookup("z"), test.ShouldBeNil)
	test.That(t, CameraLookup("x").Constructor, test.ShouldNotBeNil)

	test.That(t, BaseLookup("x"), test.ShouldNotBeNil)
	test.That(t, BaseLookup("z"), test.ShouldBeNil)
	test.That(t, BaseLookup("x").Constructor, test.ShouldNotBeNil)

	test.That(t, GripperLookup("x"), test.ShouldNotBeNil)
	test.That(t, GripperLookup("z"), test.ShouldBeNil)
	test.That(t, GripperLookup("x").Constructor, test.ShouldNotBeNil)

	test.That(t, LidarLookup("x"), test.ShouldNotBeNil)
	test.That(t, LidarLookup("z"), test.ShouldBeNil)
	test.That(t, LidarLookup("x").Constructor, test.ShouldNotBeNil)

	test.That(t, SensorLookup(sensor.Type("x"), "y"), test.ShouldNotBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "z"), test.ShouldBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "y").Constructor, test.ShouldNotBeNil)

	test.That(t, BoardLookup("x"), test.ShouldNotBeNil)
	test.That(t, BoardLookup("z"), test.ShouldBeNil)
	test.That(t, BoardLookup("x").Constructor, test.ShouldNotBeNil)

	test.That(t, MotorLookup("x"), test.ShouldNotBeNil)
	test.That(t, MotorLookup("z"), test.ShouldBeNil)
	test.That(t, MotorLookup("x").Constructor, test.ShouldNotBeNil)

	test.That(t, InputControllerLookup("x"), test.ShouldNotBeNil)
	test.That(t, InputControllerLookup("z"), test.ShouldBeNil)
	test.That(t, InputControllerLookup("x").Constructor, test.ShouldNotBeNil)
}

func TestComponentRegistry(t *testing.T) {
	af := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		return nil, nil
	}
	armResourceName := "x"
	test.That(t, func() { RegisterComponent(arm.Subtype, armResourceName, Component{}) }, test.ShouldPanic)
	RegisterComponent(arm.Subtype, armResourceName, Component{Constructor: af})

	creator := ComponentLookup(arm.Subtype, armResourceName)
	test.That(t, creator, test.ShouldNotBeNil)
	test.That(t, ComponentLookup(arm.Subtype, "z"), test.ShouldBeNil)
	test.That(t, creator.Constructor, test.ShouldEqual, af)

}

func TestResourceSubtypeRegistry(t *testing.T) {
	rf := func(r interface{}) (resource.Reconfigurable, error) {
		return nil, nil
	}
	sf := func(ctx context.Context, rpcServer rpcserver.Server, subtypeSvc subtype.Service) error {
		return nil
	}
	rcf := func(conn dialer.ClientConn, name string, logger golog.Logger) interface{} {
		return nil
	}
	newSubtype := resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, arm.SubtypeName)
	test.That(t, func() { RegisterResourceSubtype(newSubtype, ResourceSubtype{}) }, test.ShouldPanic)

	RegisterResourceSubtype(newSubtype, ResourceSubtype{Reconfigurable: rf, RegisterRPCService: sf})
	creator := ResourceSubtypeLookup(newSubtype)
	test.That(t, creator, test.ShouldNotBeNil)
	test.That(t, creator.Reconfigurable, test.ShouldEqual, rf)
	test.That(t, creator.RegisterRPCService, test.ShouldEqual, sf)
	test.That(t, creator.RPCClient, test.ShouldBeNil)

	subtype2 := resource.NewSubtype(resource.Namespace("acme2"), resource.ResourceTypeComponent, arm.SubtypeName)
	test.That(t, ResourceSubtypeLookup(subtype2), test.ShouldBeNil)

	RegisterResourceSubtype(subtype2, ResourceSubtype{RegisterRPCService: sf, RPCClient: rcf})
	creator = ResourceSubtypeLookup(subtype2)
	test.That(t, creator, test.ShouldNotBeNil)
	test.That(t, creator.RegisterRPCService, test.ShouldEqual, sf)
	test.That(t, creator.RPCClient, test.ShouldEqual, rcf)

	subtype3 := resource.NewSubtype(resource.Namespace("acme3"), resource.ResourceTypeComponent, arm.SubtypeName)
	test.That(t, ResourceSubtypeLookup(subtype3), test.ShouldBeNil)

	RegisterResourceSubtype(subtype3, ResourceSubtype{RPCClient: rcf})
	creator = ResourceSubtypeLookup(subtype3)
	test.That(t, creator, test.ShouldNotBeNil)
	test.That(t, creator.RPCClient, test.ShouldEqual, rcf)
}
