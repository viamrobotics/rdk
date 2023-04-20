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

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
)

var (
	button      = resource.SubtypeName("button")
	acme        = resource.NewName(resource.Namespace("acme"), resource.ResourceTypeComponent, button, "button1")
	nav         = resource.SubtypeName("navigation")
	testService = resource.NewName(resource.Namespace("rdk"), resource.ResourceTypeComponent, nav, "nav1")
)

func TestComponentRegistry(t *testing.T) {
	logger := golog.NewTestLogger(t)
	rf := func(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (resource.Resource, error) {
		return nil, errors.New("one")
	}
	modelName := resource.Model{Name: "x"}
	test.That(t, func() { registry.RegisterResource(acme.Subtype, modelName, registry.Resource[resource.Resource]{}) }, test.ShouldPanic)
	registry.RegisterResource(acme.Subtype, modelName, registry.Resource[resource.Resource]{Constructor: rf})

	resInfo, ok := registry.ResourceLookup(acme.Subtype, modelName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resInfo, test.ShouldNotBeNil)
	_, ok = registry.ResourceLookup(acme.Subtype, resource.Model{Name: "z"})
	test.That(t, ok, test.ShouldBeFalse)
	_, err := resInfo.Constructor(context.Background(), nil, resource.Config{}, logger)
	test.That(t, err, test.ShouldBeError, errors.New("one"))

	registry.DeregisterResource(acme.Subtype, modelName)
	_, ok = registry.ResourceLookup(acme.Subtype, modelName)
	test.That(t, ok, test.ShouldBeFalse)

	modelName2 := resource.DefaultServiceModel
	test.That(t, func() {
		registry.RegisterResource(testService.Subtype, modelName2, registry.Resource[resource.Resource]{})
	}, test.ShouldPanic)
	registry.RegisterResource(testService.Subtype, modelName2, registry.Resource[resource.Resource]{Constructor: rf})

	resInfo, ok = registry.ResourceLookup(testService.Subtype, modelName2)
	test.That(t, resInfo, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = registry.ResourceLookup(testService.Subtype, resource.NewDefaultModel("z"))
	test.That(t, ok, test.ShouldBeFalse)
	_, err = resInfo.Constructor(context.Background(), nil, resource.Config{}, logger)
	test.That(t, err, test.ShouldBeError, errors.New("one"))

	registry.DeregisterResource(testService.Subtype, modelName2)
	_, ok = registry.ResourceLookup(testService.Subtype, modelName2)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestResourceSubtypeRegistry(t *testing.T) {
	statf := func(context.Context, resource.Resource) (interface{}, error) {
		return nil, errors.New("one")
	}
	sf := func(ctx context.Context, rpcServer rpc.Server, subtypeColl resource.SubtypeCollection[resource.Resource]) error {
		return errors.New("two")
	}
	rcf := func(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (resource.Resource, error) {
		return nil, errors.New("three")
	}

	registry.RegisterResourceSubtype(acme.Subtype, registry.ResourceSubtype[resource.Resource]{
		Status: statf, RegisterRPCService: sf, RPCServiceDesc: &pb.RobotService_ServiceDesc,
	})
	subtypeInfo, ok := registry.ResourceSubtypeLookup(acme.Subtype)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, subtypeInfo, test.ShouldNotBeNil)
	_, err := subtypeInfo.Status(nil, testutils.NewUnimplementedResource(generic.Named("foo")))
	test.That(t, err, test.ShouldBeError, errors.New("one"))
	coll, err := resource.NewSubtypeCollection[resource.Resource](generic.Subtype, nil)
	test.That(t, err, test.ShouldBeNil)
	err = subtypeInfo.RegisterRPCService(nil, nil, coll)
	test.That(t, err, test.ShouldBeError, errors.New("two"))
	test.That(t, subtypeInfo.RPCClient, test.ShouldBeNil)

	subtype2 := resource.NewSubtype(resource.Namespace("acme2"), resource.ResourceTypeComponent, button)
	_, ok = registry.ResourceSubtypeLookup(subtype2)
	test.That(t, ok, test.ShouldBeFalse)

	registry.RegisterResourceSubtype(subtype2, registry.ResourceSubtype[resource.Resource]{
		RegisterRPCService: sf, RPCClient: rcf, RPCServiceDesc: &pb.RobotService_ServiceDesc,
	})
	subtypeInfo, ok = registry.ResourceSubtypeLookup(subtype2)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, subtypeInfo.Status, test.ShouldBeNil)
	err = subtypeInfo.RegisterRPCService(nil, nil, coll)
	test.That(t, err, test.ShouldBeError, errors.New("two"))
	_, err = subtypeInfo.RPCClient(nil, nil, generic.Named("foo"), nil)
	test.That(t, err, test.ShouldBeError, errors.New("three"))
	test.That(t, subtypeInfo.RPCServiceDesc, test.ShouldEqual, &pb.RobotService_ServiceDesc)

	reflectSvcDesc, err := grpcreflect.LoadServiceDescriptor(subtypeInfo.RPCServiceDesc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, subtypeInfo.ReflectRPCServiceDesc, test.ShouldResemble, reflectSvcDesc)

	subtype3 := resource.NewSubtype(resource.Namespace("acme3"), resource.ResourceTypeComponent, button)
	_, ok = registry.ResourceSubtypeLookup(subtype3)
	test.That(t, ok, test.ShouldBeFalse)

	registry.RegisterResourceSubtype(subtype3, registry.ResourceSubtype[resource.Resource]{RPCClient: rcf})
	subtypeInfo, ok = registry.ResourceSubtypeLookup(subtype3)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, subtypeInfo, test.ShouldNotBeNil)
	_, err = subtypeInfo.RPCClient(nil, nil, generic.Named("foo"), nil)
	test.That(t, err, test.ShouldBeError, errors.New("three"))

	subtype4 := resource.NewSubtype(resource.Namespace("acme4"), resource.ResourceTypeComponent, button)
	_, ok = registry.ResourceSubtypeLookup(subtype4)
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, func() {
		registry.RegisterResourceSubtype(subtype4, registry.ResourceSubtype[resource.Resource]{
			RegisterRPCService: sf, RPCClient: rcf,
		})
	}, test.ShouldPanic)

	registry.DeregisterResourceSubtype(subtype3)
	_, ok = registry.ResourceSubtypeLookup(subtype3)
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
