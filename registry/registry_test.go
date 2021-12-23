package registry

import (
	"context"
	"testing"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/base"
	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/subtype"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestRegistry(t *testing.T) {
	sf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		return nil, nil
	}

	bf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (base.Base, error) {
		return nil, nil
	}

	// test panics
	test.That(t, func() { RegisterBase("x", Base{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterSensor(sensor.Type("x"), "y", Sensor{}) }, test.ShouldPanic)

	// test register
	RegisterBase("x", Base{Constructor: bf})
	RegisterSensor(sensor.Type("x"), "y", Sensor{Constructor: sf})

	// test look up
	test.That(t, BaseLookup("x"), test.ShouldNotBeNil)
	test.That(t, BaseLookup("z"), test.ShouldBeNil)
	test.That(t, BaseLookup("x").Constructor, test.ShouldNotBeNil)

	test.That(t, SensorLookup(sensor.Type("x"), "y"), test.ShouldNotBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "z"), test.ShouldBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "y").Constructor, test.ShouldNotBeNil)
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
	sf := func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
		return nil
	}
	rcf := func(conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
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
