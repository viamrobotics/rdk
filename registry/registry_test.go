package registry_test

import (
	"context"
	"errors"
	"testing"

	"github.com/edaniels/golog"
	"github.com/jhump/protoreflect/grpcreflect"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var (
	button      = resource.SubtypeName("button")
	acme        = resource.NewName(resource.Namespace("acme"), resource.ResourceTypeComponent, button, "button1")
	nav         = resource.SubtypeName("navigation")
	testService = resource.NewName(resource.Namespace("rdk"), resource.ResourceTypeComponent, nav, "nav1")
)

func TestComponentRegistry(t *testing.T) {
	logger := golog.NewTestLogger(t)
	rf := func(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (arm.Arm, error) {
		return &fake.Arm{Named: conf.ResourceName().AsNamed()}, nil
	}
	modelName := resource.Model{Name: "x"}
	test.That(t, func() { registry.RegisterResource(acme.Subtype, modelName, registry.Resource[arm.Arm]{}) }, test.ShouldPanic)
	registry.RegisterResource(acme.Subtype, modelName, registry.Resource[arm.Arm]{Constructor: rf})

	resInfo, ok := registry.ResourceLookup(acme.Subtype, modelName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resInfo, test.ShouldNotBeNil)
	_, ok = registry.ResourceLookup(acme.Subtype, resource.Model{Name: "z"})
	test.That(t, ok, test.ShouldBeFalse)
	res, err := resInfo.Constructor(context.Background(), nil, resource.Config{Name: "foo"}, logger)
	test.That(t, err, test.ShouldBeNil)
	resArm, err := resource.AsType[arm.Arm](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resArm.Name().Name, test.ShouldEqual, "foo")

	registry.DeregisterResource(acme.Subtype, modelName)
	_, ok = registry.ResourceLookup(acme.Subtype, modelName)
	test.That(t, ok, test.ShouldBeFalse)

	modelName2 := resource.DefaultServiceModel
	test.That(t, func() {
		registry.RegisterResource(testService.Subtype, modelName2, registry.Resource[arm.Arm]{})
	}, test.ShouldPanic)
	registry.RegisterResource(testService.Subtype, modelName2, registry.Resource[arm.Arm]{Constructor: rf})

	resInfo, ok = registry.ResourceLookup(testService.Subtype, modelName2)
	test.That(t, resInfo, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = registry.ResourceLookup(testService.Subtype, resource.NewDefaultModel("z"))
	test.That(t, ok, test.ShouldBeFalse)
	res, err = resInfo.Constructor(context.Background(), nil, resource.Config{Name: "bar"}, logger)
	test.That(t, err, test.ShouldBeNil)
	resArm, err = resource.AsType[arm.Arm](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resArm.Name().Name, test.ShouldEqual, "bar")

	registry.DeregisterResource(testService.Subtype, modelName2)
	_, ok = registry.ResourceLookup(testService.Subtype, modelName2)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestResourceSubtypeRegistry(t *testing.T) {
	statf := func(context.Context, arm.Arm) (interface{}, error) {
		return nil, errors.New("one")
	}
	var capColl resource.SubtypeCollection[arm.Arm]
	//nolint:unparam
	sf := func(_ context.Context, _ rpc.Server, subtypeColl resource.SubtypeCollection[arm.Arm]) error {
		capColl = subtypeColl
		return nil
	}
	rcf := func(_ context.Context, _ rpc.ClientConn, name resource.Name, _ golog.Logger) (arm.Arm, error) {
		return capColl.Resource(name.ShortName())
	}

	registry.RegisterResourceSubtype(acme.Subtype, registry.ResourceSubtype[arm.Arm]{
		Status: statf, RegisterRPCService: sf, RPCServiceDesc: &pb.RobotService_ServiceDesc,
	})
	subtypeInfo, ok, err := registry.ResourceSubtypeLookup[arm.Arm](acme.Subtype)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, subtypeInfo, test.ShouldNotBeNil)
	_, err = subtypeInfo.Status(nil, &fake.Arm{Named: arm.Named("foo").AsNamed()})
	test.That(t, err, test.ShouldBeError, errors.New("one"))
	coll, err := resource.NewSubtypeCollection(arm.Subtype, map[resource.Name]arm.Arm{
		arm.Named("foo"): &fake.Arm{Named: arm.Named("foo").AsNamed()},
	})
	test.That(t, err, test.ShouldBeNil)
	err = subtypeInfo.RegisterRPCService(nil, nil, coll)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, subtypeInfo.RPCClient, test.ShouldBeNil)

	subtype2 := resource.NewSubtype(resource.Namespace("acme2"), resource.ResourceTypeComponent, button)
	_, ok, err = registry.ResourceSubtypeLookup[arm.Arm](subtype2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	registry.RegisterResourceSubtype(subtype2, registry.ResourceSubtype[arm.Arm]{
		RegisterRPCService: sf, RPCClient: rcf, RPCServiceDesc: &pb.RobotService_ServiceDesc,
	})
	subtypeInfo, ok, err = registry.ResourceSubtypeLookup[arm.Arm](subtype2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, subtypeInfo.Status, test.ShouldBeNil)
	err = subtypeInfo.RegisterRPCService(nil, nil, coll)
	test.That(t, err, test.ShouldBeNil)
	res, err := subtypeInfo.RPCClient(nil, nil, arm.Named("foo"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Name().Name, test.ShouldEqual, "foo")
	test.That(t, subtypeInfo.RPCServiceDesc, test.ShouldEqual, &pb.RobotService_ServiceDesc)

	reflectSvcDesc, err := grpcreflect.LoadServiceDescriptor(subtypeInfo.RPCServiceDesc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, subtypeInfo.ReflectRPCServiceDesc, test.ShouldResemble, reflectSvcDesc)

	subtype3 := resource.NewSubtype(resource.Namespace("acme3"), resource.ResourceTypeComponent, button)
	_, ok, err = registry.ResourceSubtypeLookup[arm.Arm](subtype3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	registry.RegisterResourceSubtype(subtype3, registry.ResourceSubtype[arm.Arm]{RPCClient: rcf})
	subtypeInfo, ok, err = registry.ResourceSubtypeLookup[arm.Arm](subtype3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, subtypeInfo, test.ShouldNotBeNil)
	res, err = subtypeInfo.RPCClient(nil, nil, arm.Named("foo"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Name().Name, test.ShouldEqual, "foo")

	subtype4 := resource.NewSubtype(resource.Namespace("acme4"), resource.ResourceTypeComponent, button)
	_, ok, err = registry.ResourceSubtypeLookup[arm.Arm](subtype4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, func() {
		registry.RegisterResourceSubtype(subtype4, registry.ResourceSubtype[arm.Arm]{
			RegisterRPCService: sf, RPCClient: rcf,
		})
	}, test.ShouldPanic)

	registry.DeregisterResourceSubtype(subtype3)
	_, ok, err = registry.ResourceSubtypeLookup[arm.Arm](subtype3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestDiscoveryFunctionRegistry(t *testing.T) {
	df := func(ctx context.Context, logger golog.Logger) (interface{}, error) {
		return []discovery.Discovery{}, nil
	}
	invalidSubtypeQuery := discovery.NewQuery(resource.Subtype{ResourceSubtype: "some subtype"}, resource.Model{Name: "some model"})
	test.That(t, func() { registry.RegisterDiscoveryFunction(invalidSubtypeQuery, df) }, test.ShouldPanic)

	validSubtypeQuery := discovery.NewQuery(acme.Subtype, resource.Model{Name: "some model"})
	_, ok := registry.DiscoveryFunctionLookup(validSubtypeQuery)
	test.That(t, ok, test.ShouldBeFalse)

	test.That(t, func() { registry.RegisterDiscoveryFunction(validSubtypeQuery, df) }, test.ShouldNotPanic)
	acmeDF, ok := registry.DiscoveryFunctionLookup(validSubtypeQuery)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, acmeDF, test.ShouldEqual, df)
}
