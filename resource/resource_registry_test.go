package resource_test

import (
	"context"
	"errors"
	"testing"

	"github.com/edaniels/golog"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
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
	test.That(t, func() {
		resource.Register(acme.Subtype, modelName, resource.Registration[arm.Arm, resource.NoNativeConfig]{})
	}, test.ShouldPanic)
	resource.Register(acme.Subtype, modelName, resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: rf})

	resInfo, ok := resource.LookupRegistration(acme.Subtype, modelName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resInfo, test.ShouldNotBeNil)
	_, ok = resource.LookupRegistration(acme.Subtype, resource.Model{Name: "z"})
	test.That(t, ok, test.ShouldBeFalse)
	res, err := resInfo.Constructor(context.Background(), nil, resource.Config{Name: "foo"}, logger)
	test.That(t, err, test.ShouldBeNil)
	resArm, err := resource.AsType[arm.Arm](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resArm.Name().Name, test.ShouldEqual, "foo")

	resource.Deregister(acme.Subtype, modelName)
	_, ok = resource.LookupRegistration(acme.Subtype, modelName)
	test.That(t, ok, test.ShouldBeFalse)

	modelName2 := resource.DefaultServiceModel
	test.That(t, func() {
		resource.Register(testService.Subtype, modelName2, resource.Registration[arm.Arm, resource.NoNativeConfig]{})
	}, test.ShouldPanic)
	resource.Register(testService.Subtype, modelName2, resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: rf})

	resInfo, ok = resource.LookupRegistration(testService.Subtype, modelName2)
	test.That(t, resInfo, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = resource.LookupRegistration(testService.Subtype, resource.NewDefaultModel("z"))
	test.That(t, ok, test.ShouldBeFalse)
	res, err = resInfo.Constructor(context.Background(), nil, resource.Config{Name: "bar"}, logger)
	test.That(t, err, test.ShouldBeNil)
	resArm, err = resource.AsType[arm.Arm](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resArm.Name().Name, test.ShouldEqual, "bar")

	resource.Deregister(testService.Subtype, modelName2)
	_, ok = resource.LookupRegistration(testService.Subtype, modelName2)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestResourceSubtypeRegistry(t *testing.T) {
	statf := func(context.Context, arm.Arm) (interface{}, error) {
		return nil, errors.New("one")
	}
	var capColl resource.SubtypeCollection[arm.Arm]

	sf := func(subtypeColl resource.SubtypeCollection[arm.Arm]) interface{} {
		capColl = subtypeColl
		return 5
	}
	rcf := func(_ context.Context, _ rpc.ClientConn, name resource.Name, _ golog.Logger) (arm.Arm, error) {
		return capColl.Resource(name.ShortName())
	}

	test.That(t, func() {
		resource.RegisterSubtype(acme.Subtype, resource.SubtypeRegistration[arm.Arm]{
			Status:                      statf,
			RPCServiceServerConstructor: sf,
			RPCServiceDesc:              &pb.RobotService_ServiceDesc,
		})
	}, test.ShouldPanic)
	test.That(t, func() {
		resource.RegisterSubtypeWithAssociation(acme.Subtype, resource.SubtypeRegistration[arm.Arm]{
			Status:                      statf,
			RPCServiceServerConstructor: sf,
			RPCServiceDesc:              &pb.RobotService_ServiceDesc,
		}, resource.AssociatedConfigRegistration[any]{})
	}, test.ShouldPanic)
	resource.RegisterSubtype(acme.Subtype, resource.SubtypeRegistration[arm.Arm]{
		Status:                      statf,
		RPCServiceServerConstructor: sf,
		RPCServiceHandler:           pb.RegisterRobotServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.RobotService_ServiceDesc,
	})
	subtypeInfo, ok, err := resource.LookupSubtypeRegistration[arm.Arm](acme.Subtype)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, subtypeInfo, test.ShouldNotBeNil)
	_, err = subtypeInfo.Status(nil, &fake.Arm{Named: arm.Named("foo").AsNamed()})
	test.That(t, err, test.ShouldBeError, errors.New("one"))
	coll, err := resource.NewSubtypeCollection(arm.Subtype, map[resource.Name]arm.Arm{
		arm.Named("foo"): &fake.Arm{Named: arm.Named("foo").AsNamed()},
	})
	test.That(t, err, test.ShouldBeNil)
	svcServer := subtypeInfo.RPCServiceServerConstructor(coll)
	test.That(t, svcServer, test.ShouldNotBeNil)
	test.That(t, subtypeInfo.RPCClient, test.ShouldBeNil)

	subtype2 := resource.NewSubtype(resource.Namespace("acme2"), resource.ResourceTypeComponent, button)
	_, ok, err = resource.LookupSubtypeRegistration[arm.Arm](subtype2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	resource.RegisterSubtype(subtype2, resource.SubtypeRegistration[arm.Arm]{
		RPCServiceServerConstructor: sf,
		RPCClient:                   rcf,
		RPCServiceDesc:              &pb.RobotService_ServiceDesc,
		RPCServiceHandler:           pb.RegisterRobotServiceHandlerFromEndpoint,
	})
	subtypeInfo, ok, err = resource.LookupSubtypeRegistration[arm.Arm](subtype2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, subtypeInfo.Status, test.ShouldBeNil)
	svcServer = subtypeInfo.RPCServiceServerConstructor(coll)
	test.That(t, svcServer, test.ShouldNotBeNil)
	res, err := subtypeInfo.RPCClient(nil, nil, arm.Named("foo"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Name().Name, test.ShouldEqual, "foo")
	test.That(t, subtypeInfo.RPCServiceDesc, test.ShouldEqual, &pb.RobotService_ServiceDesc)

	reflectSvcDesc, err := grpcreflect.LoadServiceDescriptor(subtypeInfo.RPCServiceDesc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, subtypeInfo.ReflectRPCServiceDesc, test.ShouldResemble, reflectSvcDesc)

	subtype3 := resource.NewSubtype(resource.Namespace("acme3"), resource.ResourceTypeComponent, button)
	_, ok, err = resource.LookupSubtypeRegistration[arm.Arm](subtype3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	resource.RegisterSubtype(subtype3, resource.SubtypeRegistration[arm.Arm]{RPCClient: rcf})
	subtypeInfo, ok, err = resource.LookupSubtypeRegistration[arm.Arm](subtype3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, subtypeInfo, test.ShouldNotBeNil)
	res, err = subtypeInfo.RPCClient(nil, nil, arm.Named("foo"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Name().Name, test.ShouldEqual, "foo")

	subtype4 := resource.NewSubtype(resource.Namespace("acme4"), resource.ResourceTypeComponent, button)
	_, ok, err = resource.LookupSubtypeRegistration[arm.Arm](subtype4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, func() {
		resource.RegisterSubtype(subtype4, resource.SubtypeRegistration[arm.Arm]{
			RPCServiceServerConstructor: sf,
			RPCClient:                   rcf,
			RPCServiceHandler:           pb.RegisterRobotServiceHandlerFromEndpoint,
		})
	}, test.ShouldPanic)
	test.That(t, func() {
		resource.RegisterSubtypeWithAssociation(subtype4, resource.SubtypeRegistration[arm.Arm]{
			RPCServiceServerConstructor: sf,
			RPCClient:                   rcf,
			RPCServiceHandler:           pb.RegisterRobotServiceHandlerFromEndpoint,
		}, resource.AssociatedConfigRegistration[any]{
			WithName: func(resName resource.Name, resAssociation any) error {
				return nil
			},
		})
	}, test.ShouldPanic)

	resource.DeregisterSubtype(subtype3)
	_, ok, err = resource.LookupSubtypeRegistration[arm.Arm](subtype3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestResourceSubtypeRegistryWithAssociation(t *testing.T) {
	statf := func(context.Context, arm.Arm) (interface{}, error) {
		return nil, errors.New("one")
	}
	sf := func(subtypeColl resource.SubtypeCollection[arm.Arm]) interface{} {
		return nil
	}

	type someType struct {
		Field1  string `json:"field1"`
		capName resource.Name
	}

	someName := resource.NewName(resource.Namespace(uuid.NewString()), resource.ResourceTypeComponent, button, "button1")
	resource.RegisterSubtypeWithAssociation(someName.Subtype, resource.SubtypeRegistration[arm.Arm]{
		Status:                      statf,
		RPCServiceServerConstructor: sf,
		RPCServiceHandler:           pb.RegisterRobotServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.RobotService_ServiceDesc,
	}, resource.AssociatedConfigRegistration[*someType]{
		WithName: func(resName resource.Name, resAssociation *someType) error {
			resAssociation.capName = resName
			return nil
		},
	})
	reg, ok := resource.LookupAssociatedConfigRegistration(someName.Subtype)
	test.That(t, ok, test.ShouldBeTrue)
	assoc, err := reg.AttributeMapConverter(utils.AttributeMap{"field1": "hey"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, assoc.(*someType).Field1, test.ShouldEqual, "hey")
	test.That(t, assoc.(*someType).capName, test.ShouldResemble, resource.Name{})
	test.That(t, reg.WithName(arm.Named("foo"), assoc), test.ShouldBeNil)
	test.That(t, assoc.(*someType).capName, test.ShouldResemble, arm.Named("foo"))
}

func TestDiscoveryFunctions(t *testing.T) {
	df := func(ctx context.Context, logger golog.Logger) (interface{}, error) {
		return []resource.Discovery{}, nil
	}
	validSubtypeQuery := resource.NewDiscoveryQuery(acme.Subtype, resource.Model{Name: "some model"})
	_, ok := resource.LookupRegistration(validSubtypeQuery.API, validSubtypeQuery.Model)
	test.That(t, ok, test.ShouldBeFalse)

	rf := func(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (arm.Arm, error) {
		return &fake.Arm{Named: conf.ResourceName().AsNamed()}, nil
	}

	resource.Register(validSubtypeQuery.API, validSubtypeQuery.Model, resource.Registration[arm.Arm, resource.NoNativeConfig]{
		Constructor: rf,
		Discover:    df,
	})

	reg, ok := resource.LookupRegistration(validSubtypeQuery.API, validSubtypeQuery.Model)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, reg.Discover, test.ShouldEqual, df)
}

func TestTransformAttributeMap(t *testing.T) {
	type myType struct {
		A          string            `json:"a"`
		B          string            `json:"b"`
		Attributes map[string]string `json:"attributes"`
	}

	attrs := utils.AttributeMap{
		"a": "1",
		"b": "2",
		"c": "3",
		"d": "4",
		"e": 5,
	}
	transformed, err := resource.TransformAttributeMap[*myType](attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformed, test.ShouldResemble, &myType{
		A: "1",
		B: "2",
		Attributes: map[string]string{
			"c": "3",
			"d": "4",
		},
	})

	transformed, err = resource.TransformAttributeMap[*myType](attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformed, test.ShouldResemble, &myType{
		A: "1",
		B: "2",
		Attributes: map[string]string{
			"c": "3",
			"d": "4",
		},
	})

	type myExtendedType struct {
		A          string             `json:"a"`
		B          string             `json:"b"`
		Attributes utils.AttributeMap `json:"attributes"`
	}

	transformedExt, err := resource.TransformAttributeMap[*myExtendedType](attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformedExt, test.ShouldResemble, &myExtendedType{
		A: "1",
		B: "2",
		Attributes: utils.AttributeMap{
			"c": "3",
			"d": "4",
			"e": 5,
		},
	})

	transformedExt, err = resource.TransformAttributeMap[*myExtendedType](attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformedExt, test.ShouldResemble, &myExtendedType{
		A: "1",
		B: "2",
		Attributes: utils.AttributeMap{
			"c": "3",
			"d": "4",
			"e": 5,
		},
	})
}
