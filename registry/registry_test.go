package registry

import (
	"context"
	"testing"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestRegistry(t *testing.T) {
	af := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
		return nil, nil
	}

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
	ff := func(name string) (referenceframe.Frame, error) {
		return nil, nil
	}

	// test panics
	test.That(t, func() { RegisterCamera("x", Camera{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterArm("x", Arm{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterBase("x", Base{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterGripper("x", Gripper{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterLidar("x", Lidar{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterSensor(sensor.Type("x"), "y", Sensor{}) }, test.ShouldPanic)

	// test register
	RegisterCamera("x", Camera{cf, ff})
	RegisterBase("x", Base{Constructor: bf, Frame: ff})
	RegisterArm("x", Arm{Constructor: af, Frame: ff})
	RegisterGripper("x", Gripper{gf, ff})
	RegisterLidar("x", Lidar{Constructor: lf})
	RegisterSensor(sensor.Type("x"), "y", Sensor{Constructor: sf, Frame: ff})

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

	test.That(t, ArmLookup("x"), test.ShouldNotBeNil)
	test.That(t, ArmLookup("z"), test.ShouldBeNil)
	test.That(t, ArmLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, ArmLookup("x").Frame, test.ShouldNotBeNil)
	comp = &config.Component{Type: config.ComponentTypeArm, Model: "x"}
	frameFunc, ok = FrameLookup(comp)
	test.That(t, frameFunc, test.ShouldEqual, ff)
	test.That(t, ok, test.ShouldEqual, true)
	// look up a component that doesn't exist
	comp = &config.Component{Type: config.ComponentTypeArm, Model: "z"}
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
}
