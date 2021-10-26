package registry

import (
	"context"
	"testing"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/input"
	"go.viam.com/core/lidar"
	"go.viam.com/core/motor"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/servo"

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

	servof := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (servo.Servo, error) {
		return nil, nil
	}

	motorf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (motor.Motor, error) {
		return nil, nil
	}

	inputf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (input.Controller, error) {
		return nil, nil
	}

	ff := func(name string) (referenceframe.Frame, error) {
		return nil, nil
	}

	// test panics
	test.That(t, func() { RegisterCamera("x", Camera{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterBase("x", Base{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterGripper("x", Gripper{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterLidar("x", Lidar{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterSensor(sensor.Type("x"), "y", Sensor{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterBoard("x", Board{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterServo("x", Servo{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterMotor("x", Motor{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterInputController("x", InputController{}) }, test.ShouldPanic)

	// test register
	RegisterCamera("x", Camera{cf, ff})
	RegisterBase("x", Base{Constructor: bf, Frame: ff})
	RegisterGripper("x", Gripper{gf, ff})
	RegisterLidar("x", Lidar{Constructor: lf})
	RegisterSensor(sensor.Type("x"), "y", Sensor{Constructor: sf, Frame: ff})
	RegisterBoard("x", Board{Constructor: bbf, Frame: ff})
	RegisterServo("x", Servo{Constructor: servof, Frame: ff})
	RegisterMotor("x", Motor{Constructor: motorf, Frame: ff})
	RegisterInputController("x", InputController{Constructor: inputf})

	// test look up

	test.That(t, CameraLookup("x"), test.ShouldNotBeNil)
	test.That(t, CameraLookup("z"), test.ShouldBeNil)
	test.That(t, CameraLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, CameraLookup("x").Frame, test.ShouldNotBeNil)
	comp := &config.Component{Type: config.ComponentTypeCamera, Model: "x"}
	frameFunc, ok := FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldEqual, ff)
	test.That(t, ok, test.ShouldEqual, true)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeCamera, Model: "z"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)

	test.That(t, BaseLookup("x"), test.ShouldNotBeNil)
	test.That(t, BaseLookup("z"), test.ShouldBeNil)
	test.That(t, BaseLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, BaseLookup("x").Frame, test.ShouldNotBeNil)
	comp = &config.Component{Type: config.ComponentTypeBase, Model: "x"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldEqual, ff)
	test.That(t, ok, test.ShouldEqual, true)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeBase, Model: "z"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)

	test.That(t, GripperLookup("x"), test.ShouldNotBeNil)
	test.That(t, GripperLookup("z"), test.ShouldBeNil)
	test.That(t, GripperLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, GripperLookup("x").Frame, test.ShouldNotBeNil)
	comp = &config.Component{Type: config.ComponentTypeGripper, Model: "x"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldEqual, ff)
	test.That(t, ok, test.ShouldEqual, true)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeGripper, Model: "z"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)

	test.That(t, LidarLookup("x"), test.ShouldNotBeNil)
	test.That(t, LidarLookup("z"), test.ShouldBeNil)
	test.That(t, LidarLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, LidarLookup("x").Frame, test.ShouldBeNil)
	comp = &config.Component{Type: config.ComponentTypeLidar, Model: "x"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeLidar, Model: "z"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)

	test.That(t, SensorLookup(sensor.Type("x"), "y"), test.ShouldNotBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "z"), test.ShouldBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "y").Constructor, test.ShouldNotBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "y").Frame, test.ShouldNotBeNil)
	comp = &config.Component{Type: config.ComponentTypeSensor, Model: "y", SubType: "x"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldEqual, ff)
	test.That(t, ok, test.ShouldEqual, true)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeSensor, Model: "z", SubType: "x"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)

	test.That(t, BoardLookup("x"), test.ShouldNotBeNil)
	test.That(t, BoardLookup("z"), test.ShouldBeNil)
	test.That(t, BoardLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, BoardLookup("x").Frame, test.ShouldNotBeNil)
	comp = &config.Component{Type: config.ComponentTypeBoard, Model: "x"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldEqual, ff)
	test.That(t, ok, test.ShouldEqual, true)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeBoard, Model: "z"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)

	test.That(t, ServoLookup("x"), test.ShouldNotBeNil)
	test.That(t, ServoLookup("z"), test.ShouldBeNil)
	test.That(t, ServoLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, ServoLookup("x").Frame, test.ShouldNotBeNil)
	comp = &config.Component{Type: config.ComponentTypeServo, Model: "x"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldEqual, ff)
	test.That(t, ok, test.ShouldEqual, true)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeServo, Model: "z"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)

	test.That(t, MotorLookup("x"), test.ShouldNotBeNil)
	test.That(t, MotorLookup("z"), test.ShouldBeNil)
	test.That(t, MotorLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, MotorLookup("x").Frame, test.ShouldNotBeNil)
	comp = &config.Component{Type: config.ComponentTypeMotor, Model: "x"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldEqual, ff)
	test.That(t, ok, test.ShouldEqual, true)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeMotor, Model: "z"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)

	test.That(t, InputControllerLookup("x"), test.ShouldNotBeNil)
	test.That(t, InputControllerLookup("z"), test.ShouldBeNil)
	test.That(t, InputControllerLookup("x").Constructor, test.ShouldNotBeNil)
	comp = &config.Component{Type: config.ComponentTypeInputController, Model: "x"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeInputController, Model: "z"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)
}

func TestComponentRegistry(t *testing.T) {
	af := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
		return nil, nil
	}
	ff := func(name string) (referenceframe.Frame, error) {
		return nil, nil
	}
	armResourceName := "x"
	test.That(t, func() { RegisterComponent(arm.Subtype, armResourceName, Component{}) }, test.ShouldPanic)
	RegisterComponent(arm.Subtype, armResourceName, Component{Constructor: af, Frame: ff})

	creator := ComponentLookup(arm.Subtype, armResourceName)
	test.That(t, creator, test.ShouldNotBeNil)
	test.That(t, ComponentLookup(arm.Subtype, "z"), test.ShouldBeNil)
	test.That(t, creator.Constructor, test.ShouldEqual, af)
	test.That(t, creator.Frame, test.ShouldEqual, ff)

	comp := &config.Component{Type: config.ComponentTypeArm, Model: armResourceName}
	frameFunc, ok := FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldEqual, ff)
	test.That(t, ok, test.ShouldEqual, true)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeArm, Model: "z"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldBeNil)
	test.That(t, ok, test.ShouldEqual, false)
}

func TestComponentSubtypeRegistry(t *testing.T) {
	rf := func(r interface{}) (resource.Reconfigurable, error) {
		return nil, nil
	}
	newSubtype := resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, arm.SubtypeName)
	test.That(t, func() { RegisterComponentSubtype(newSubtype, ComponentSubtype{}) }, test.ShouldPanic)

	RegisterComponentSubtype(newSubtype, ComponentSubtype{rf})
	creator := ComponentSubtypeLookup(newSubtype)
	test.That(t, creator, test.ShouldNotBeNil)

	subtype2 := resource.NewSubtype(resource.Namespace("acme2"), resource.ResourceTypeComponent, arm.SubtypeName)
	test.That(t, ComponentSubtypeLookup(subtype2), test.ShouldBeNil)
	test.That(t, creator.Reconfigurable, test.ShouldEqual, rf)
}
